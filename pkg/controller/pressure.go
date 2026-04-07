package controller

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/grandeit/schnueffelstueck/pkg/collector"
)

type PressureController struct {
	maxStepPct       float64
	minStepPct       float64
	hostReservedPct  float64
	hostSteepness    float64
	guestReservedPct float64
	guestSteepness   float64
}

func NewPressureController(config Config) *PressureController {

	if config.GuestOvercommit < 1 {
		slog.Warn("guest overcommit less than 1 is not allowed, setting to 1", "value", config.GuestOvercommit)
		config.GuestOvercommit = 1
	}
	if config.HostReservedPct < 0 || config.HostReservedPct >= 1 {
		slog.Warn("host reserved pct must be in [0, 1), setting to default 0.1", "value", config.HostReservedPct)
		config.HostReservedPct = 0.1
	}

	return &PressureController{
		maxStepPct:       config.GuestMaxStepPct,
		minStepPct:       config.GuestMinStepPct,
		hostReservedPct:  config.HostReservedPct,
		hostSteepness:    config.PressureHostSteepness,
		guestReservedPct: 1 / config.GuestOvercommit,
		guestSteepness:   config.PressureGuestSteepness,
	}
}

func exponential(x, s float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	return (math.Exp(s*x) - 1) / (math.Exp(s) - 1)
}

func (c *PressureController) Decide(sample collector.Sample) (*Decision, error) {
	guest := sample.Guest
	host := sample.Host

	if guest.Total == 0 || guest.Total == math.MaxUint64 || guest.Available == math.MaxUint64 || host.Total == 0 {
		return nil, fmt.Errorf("invalid sample: guest.Total=%d guest.Available=%d host.Total=%d", guest.Total, guest.Available, host.Total)
	}

	hostFreePct := float64(host.Available) / float64(host.Total)
	guestFreePct := float64(guest.Available) / float64(guest.Total)

	slog.Debug("host memory statistics",
		"total_mib", host.Total/mib,
		"available_mib", host.Available/mib,
		"free_pct", fmt.Sprintf("%.1f%%", hostFreePct*100),
		"controller", "pressure",
	)
	slog.Debug("guest memory statistics",
		"total_mib", guest.Total/mib,
		"available_mib", guest.Available/mib,
		"free_pct", fmt.Sprintf("%.1f%%", guestFreePct*100),
		"controller", "pressure",
	)

	pressure := exponential(min(1, (1-hostFreePct)/(1-c.hostReservedPct)), c.hostSteepness)
	generosity := exponential(min(1, guestFreePct/(1-c.guestReservedPct)), c.guestSteepness)

	reclaim := pressure * (generosity + pressure*(1-generosity)) * (1 - c.guestReservedPct) * float64(guest.Total)

	current := guest.Balloon
	desired := guest.Total - uint64(reclaim)

	maxStep := int64(c.maxStepPct * float64(guest.Total))
	delta := max(-maxStep, min(int64(desired)-int64(current), maxStep))

	slog.Debug("pressure controller statistics",
		"pressure", fmt.Sprintf("%.2f", pressure),
		"generosity", fmt.Sprintf("%.2f", generosity),
		"reclaim_mib", uint64(reclaim)/mib,
		"reclaim_pct", fmt.Sprintf("%.1f%%", reclaim/float64(guest.Total)*100),
		"current_mib", current/mib,
		"desired_mib", desired/mib,
		"delta_mib", delta/int64(mib),
		"controller", "pressure",
	)

	minStep := int64(c.minStepPct * float64(guest.Total))
	if delta > -minStep && delta < minStep {
		slog.Debug("delta is smaller than min step size, skipping",
			"delta_mib", delta/int64(mib),
			"min_step_mib", minStep/int64(mib),
			"controller", "pressure",
		)
		return nil, nil
	}

	balloon := uint64(int64(current) + delta)

	slog.Info("pressure controller decision",
		"pressure", fmt.Sprintf("%.2f", pressure),
		"generosity", fmt.Sprintf("%.2f", generosity),
		"reclaim_mib", uint64(reclaim)/mib,
		"reclaim_pct", fmt.Sprintf("%.1f%%", reclaim/float64(guest.Total)*100),
		"current_mib", current/mib,
		"desired_mib", desired/mib,
		"new_mib", balloon/mib,
		"delta_mib", delta/int64(mib),
		"controller", "pressure",
	)

	return &Decision{
		BalloonTargetBytes: balloon,
		Reason:             fmt.Sprintf("pressure=%.2f generosity=%.2f reclaim_pct=%.1f%%", pressure, generosity, reclaim/float64(guest.Total)*100),
	}, nil
}
