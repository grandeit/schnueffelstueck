package collector

import (
	"fmt"
	"time"

	"github.com/grandeit/schnueffelstueck/pkg/qmp"
)

type Collector struct {
	qemu *QEMUCollector
	host *HostCollector
}

func NewCollector(client *qmp.Client) *Collector {
	return &Collector{
		qemu: NewQEMUCollector(client),
		host: NewHostCollector(),
	}
}

func (c *Collector) Collect() (Sample, error) {
	guest, err := c.qemu.Collect()
	if err != nil {
		return Sample{}, fmt.Errorf("collecting guest memory: %w", err)
	}

	host, err := c.host.Collect()
	if err != nil {
		return Sample{}, fmt.Errorf("collecting host memory: %w", err)
	}

	return Sample{
		Timestamp: time.Now(),
		Guest:     guest,
		Host:      host,
	}, nil
}
