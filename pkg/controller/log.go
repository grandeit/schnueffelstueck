package controller

import (
	"log/slog"

	"github.com/grandeit/schnueffelstueck/pkg/collector"
)

type LogController struct{}

func NewLogController() *LogController {
	return &LogController{}
}

func (c *LogController) Decide(sample collector.Sample) (*Decision, error) {
	slog.Debug("raw sample data", "sample", sample, "controller", "log")

	guest := sample.Guest
	host := sample.Host

	guestFreePct := 0
	if guest.Total > 0 {
		guestFreePct = int(float64(guest.Available) / float64(guest.Total) * 100)
	}

	hostFreePct := 0
	if host.Total > 0 {
		hostFreePct = int(float64(host.Available) / float64(host.Total) * 100)
	}

	slog.Info("memory summary",
		"guest_balloon_current_mib", guest.Balloon/mib,
		"guest_total_mib", guest.Total/mib,
		"guest_available_mib", guest.Available/mib,
		"guest_free_pct", guestFreePct,
		"guest_swap_in_mib", guest.SwapIn/mib,
		"guest_swap_out_mib", guest.SwapOut/mib,
		"guest_minor_faults", guest.MinorFaults,
		"guest_major_faults", guest.MajorFaults,
		"host_total_mib", host.Total/mib,
		"host_available_mib", host.Available/mib,
		"host_free_pct", hostFreePct,
		"host_swap_free_mib", host.SwapFree/mib,
		"host_swap_total_mib", host.SwapTotal/mib,
		"controller", "log",
	)

	return nil, nil
}
