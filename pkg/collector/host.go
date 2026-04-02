package collector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultProcMemInfoPath = "/proc/meminfo"

type HostCollector struct {
	procMemInfoPath string
}

func NewHostCollector() *HostCollector {
	return &HostCollector{procMemInfoPath: defaultProcMemInfoPath}
}

func (c *HostCollector) Collect() (HostMemory, error) {
	f, err := os.Open(c.procMemInfoPath)
	if err != nil {
		return HostMemory{}, fmt.Errorf("opening %s: %w", c.procMemInfoPath, err)
	}
	defer f.Close()

	var stats HostMemory
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		key := fields[0][:len(fields[0])-1]

		val, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return HostMemory{}, fmt.Errorf("parsing %s value %q: %w", key, fields[1], err)
		}

		switch key {
		case "MemTotal":
			stats.Total = val * 1024
		case "MemFree":
			stats.Free = val * 1024
		case "MemAvailable":
			stats.Available = val * 1024
		case "SwapTotal":
			stats.SwapTotal = val * 1024
		case "SwapFree":
			stats.SwapFree = val * 1024
		default:
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return HostMemory{}, fmt.Errorf("reading %s: %w", c.procMemInfoPath, err)
	}

	return stats, nil
}
