package controller

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/grandeit/schnueffelstueck/pkg/collector"
)

type WatermarkController struct {
	highPct          float64
	lowPct           float64
	guestReservedPct float64
	maxStepPct       float64
	minStepPct       float64
}

func NewWatermarkController(config Config) *WatermarkController {
	high := config.WatermarkHighPct
	low := config.WatermarkLowPct

	if high < 0 || high > 1 {
		slog.Warn("watermark high pct must be in [0, 1], clamping", "value", high, "controller", "watermark")
		high = min(max(0, high), 1)
	}
	if low < 0 || low > 1 {
		slog.Warn("watermark low pct must be in [0, 1], clamping", "value", low, "controller", "watermark")
		low = min(max(0, low), 1)
	}
	if high < low {
		slog.Warn("watermark high < low, swapping", "high", high, "low", low, "controller", "watermark")
		high, low = low, high
	}
	if config.GuestOvercommit < 1 {
		slog.Warn("guest overcommit less than 1 is not allowed, setting to 1", "value", config.GuestOvercommit, "controller", "watermark")
		config.GuestOvercommit = 1
	}

	return &WatermarkController{
		highPct:          high,
		lowPct:           low,
		guestReservedPct: 1 / config.GuestOvercommit,
		maxStepPct:       config.GuestMaxStepPct,
		minStepPct:       config.GuestMinStepPct,
	}
}

func (c *WatermarkController) Decide(sample collector.Sample) (*Decision, error) {
	guest := sample.Guest
	host := sample.Host

	if guest.Total == 0 || guest.Total == math.MaxUint64 || guest.Available == math.MaxUint64 || host.Total == 0 {
		return nil, fmt.Errorf("invalid sample: guest.Total=%d guest.Available=%d host.Total=%d", guest.Total, guest.Available, host.Total)
	}

	hostFreePct := float64(host.Available) / float64(host.Total)

	slog.Debug("host memory statistics",
		"total_mib", host.Total/mib,
		"available_mib", host.Available/mib,
		"free_pct", fmt.Sprintf("%.1f%%", hostFreePct*100),
		"controller", "watermark",
	)
	slog.Debug("guest memory statistics",
		"total_mib", guest.Total/mib,
		"available_mib", guest.Available/mib,
		"free_pct", fmt.Sprintf("%.1f%%", float64(guest.Available)/float64(guest.Total)*100),
		"balloon_mib", guest.Balloon/mib,
		"controller", "watermark",
	)

	targetFreePct := max(c.lowPct, min(c.highPct, hostFreePct))
	if targetFreePct == hostFreePct {
		slog.Debug("host free pct is in-between watermarks, skipping",
			"host_free_pct", fmt.Sprintf("%.1f%%", hostFreePct*100),
			"low_pct", fmt.Sprintf("%.1f%%", c.lowPct*100),
			"high_pct", fmt.Sprintf("%.1f%%", c.highPct*100),
			"controller", "watermark",
		)
		return nil, nil
	}

	vmFraction := float64(guest.Total) / float64(host.Total)
	reclaim := min(max(0, (targetFreePct-hostFreePct)/vmFraction), 1-c.guestReservedPct) * float64(guest.Total)

	current := guest.Balloon
	desired := guest.Total - uint64(reclaim)

	maxStep := int64(c.maxStepPct * float64(guest.Total))
	delta := max(-maxStep, min(int64(desired)-int64(current), maxStep))

	slog.Debug("watermark controller statistics",
		"host_free_pct", fmt.Sprintf("%.1f%%", hostFreePct*100),
		"target_free_pct", fmt.Sprintf("%.1f%%", targetFreePct*100),
		"reclaim_pct", fmt.Sprintf("%.1f%%", reclaim/float64(guest.Total)*100),
		"current_mib", current/mib,
		"desired_mib", desired/mib,
		"delta_mib", delta/int64(mib),
		"controller", "watermark",
	)

	minStep := int64(c.minStepPct * float64(guest.Total))
	if delta > -minStep && delta < minStep {
		slog.Debug("delta is smaller than min step size, skipping",
			"delta_mib", delta/int64(mib),
			"min_step_mib", minStep/int64(mib),
			"controller", "watermark",
		)
		return nil, nil
	}

	balloon := uint64(int64(current) + delta)

	slog.Info("watermark controller decision",
		"host_free_pct", fmt.Sprintf("%.1f%%", hostFreePct*100),
		"target_free_pct", fmt.Sprintf("%.1f%%", targetFreePct*100),
		"reclaim_pct", fmt.Sprintf("%.1f%%", reclaim/float64(guest.Total)*100),
		"current_mib", current/mib,
		"desired_mib", desired/mib,
		"new_mib", balloon/mib,
		"delta_mib", delta/int64(mib),
		"controller", "watermark",
	)

	return &Decision{
		BalloonTargetBytes: balloon,
		Reason:             fmt.Sprintf("target_free_pct=%.1f%% reclaim_pct=%.1f%%", targetFreePct*100, reclaim/float64(guest.Total)*100),
	}, nil
}
