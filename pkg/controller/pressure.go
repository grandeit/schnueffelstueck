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

	return &PressureController{
		maxStepPct:       config.PressureGuestMaxStepPct,
		minStepPct:       config.PressureGuestMinStepPct,
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

	if guest.Total == 0 || host.Total == 0 {
		return nil, fmt.Errorf("invalid sample: guest.Total=%d host.Total=%d", guest.Total, host.Total)
	}

	hostFreePct := float64(host.Available) / float64(host.Total)
	guestFreePct := float64(guest.Available) / float64(guest.Total)

	pressure := exponential(min(1, (1-hostFreePct)/(1-c.hostReservedPct)), c.hostSteepness)
	generosity := exponential(min(1, guestFreePct/(1-c.guestReservedPct)), c.guestSteepness)

	reclaimPct := pressure * generosity * (1 - c.guestReservedPct)

	reclaimBytes := reclaimPct * float64(guest.Total)

	desiredBalloon := guest.Total - uint64(reclaimBytes)

	current := guest.Balloon
	delta := int64(desiredBalloon) - int64(current)
	maxStep := int64(c.maxStepPct * float64(guest.Total))

	if delta > maxStep {
		delta = maxStep
	} else if delta < -maxStep {
		delta = -maxStep
	}

	slog.Debug("host stats",
		"total_mib", host.Total/(1024*1024),
		"available_mib", host.Available/(1024*1024),
		"free_pct", fmt.Sprintf("%.1f%%", hostFreePct*100),
	)
	slog.Debug("guest stats",
		"total_mib", guest.Total/(1024*1024),
		"available_mib", guest.Available/(1024*1024),
		"free_pct", fmt.Sprintf("%.1f%%", guestFreePct*100),
	)

	slog.Debug("pressure controller stats",
		"pressure", fmt.Sprintf("%.2f", pressure),
		"generosity", fmt.Sprintf("%.2f", generosity),
		"reclaim_pct", fmt.Sprintf("%.1f%%", reclaimPct*100),
		"current_mib", current/mib,
		"desired_mib", desiredBalloon/mib,
		"delta_mib", delta/int64(mib),
	)

	minStep := int64(c.minStepPct * float64(guest.Total))
	if delta > -minStep && delta < minStep {
		slog.Debug("pressure controller: within dead band, skipping",
			"delta_mib", delta/int64(mib),
			"min_step_mib", minStep/int64(mib),
		)
		return nil, nil
	}

	newBalloon := uint64(int64(current) + delta)

	slog.Info("pressure controller decision",
		"pressure", fmt.Sprintf("%.2f", pressure),
		"generosity", fmt.Sprintf("%.2f", generosity),
		"reclaim_pct", fmt.Sprintf("%.1f%%", reclaimPct*100),
		"current_mib", current/mib,
		"desired_mib", desiredBalloon/mib,
		"new_mib", newBalloon/mib,
		"delta_mib", delta/int64(mib),
	)

	return &Decision{
		BalloonTargetBytes: newBalloon,
		Reason:             fmt.Sprintf("pressure=%.2f generosity=%.2f reclaim=%.1f%%", pressure, generosity, reclaimPct*100),
	}, nil
}
