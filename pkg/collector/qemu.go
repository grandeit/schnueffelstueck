package collector

import (
	"fmt"
	"time"

	"github.com/grandeit/schnueffelstueck/pkg/qmp"
)

type QEMUCollector struct {
	client *qmp.Client
}

func NewQEMUCollector(client *qmp.Client) *QEMUCollector {
	return &QEMUCollector{client: client}
}

func (c *QEMUCollector) Collect() (GuestMemory, error) {
	balloonInfo, err := c.client.GetBalloonTarget()
	if err != nil {
		return GuestMemory{}, fmt.Errorf("getting qemu balloon target: %w", err)
	}

	guestStats, err := c.client.GetBalloonGuestStats()
	if err != nil {
		return GuestMemory{}, fmt.Errorf("getting qemu guest stats: %w", err)
	}

	s := guestStats.Stats
	return GuestMemory{
		Balloon:        balloonInfo.Actual,
		Total:          s.TotalMemory,
		Free:           s.FreeMemory,
		Available:      s.AvailableMemory,
		SwapIn:         s.SwapIn,
		SwapOut:        s.SwapOut,
		MajorFaults:    s.MajorFaults,
		MinorFaults:    s.MinorFaults,
		DiskCaches:     s.DiskCaches,
		HtlbPgalloc:    s.HtlbPgalloc,
		HtlbPgfail:     s.HtlbPgfail,
		OOMKills:       s.OOMKills,
		AllocStalls:    s.AllocStalls,
		AsyncScans:     s.AsyncScans,
		DirectScans:    s.DirectScans,
		AsyncReclaims:  s.AsyncReclaims,
		DirectReclaims: s.DirectReclaims,
		LastUpdate:     time.Unix(guestStats.LastUpdate, 0),
	}, nil
}
