package controller

import (
	"log/slog"
	"strconv"
	"time"
)

const mib uint64 = 1024 * 1024

type Config struct {
	Kind            string
	Interval        time.Duration
	DryRun          bool
	GuestOvercommit float64
	GuestMinStepPct float64
	GuestMaxStepPct float64
	HostReservedPct float64

	PressureHostSteepness  float64
	PressureGuestSteepness float64

	WatermarkHighPct float64
	WatermarkLowPct  float64
}

func NewConfigFromSettings(settings map[string]string) Config {
	c := Config{
		Kind:            settingString(settings, "controller", "log"),
		Interval:        settingDuration(settings, "interval", time.Second),
		DryRun:          settingBool(settings, "dry-run", false),
		GuestOvercommit: settingFloat(settings, "guest-overcommit", 2.0),
		GuestMinStepPct: settingFloat(settings, "guest-min-step-pct", 0.01),
		GuestMaxStepPct: settingFloat(settings, "guest-max-step-pct", 0.1),
		HostReservedPct: settingFloat(settings, "host-reserved-pct", 0.1),

		PressureHostSteepness:  settingFloat(settings, "pressure-host-steepness", 2),
		PressureGuestSteepness: settingFloat(settings, "pressure-guest-steepness", 2),

		WatermarkHighPct: settingFloat(settings, "watermark-high-pct", 0.2),
		WatermarkLowPct:  settingFloat(settings, "watermark-low-pct", 0.1),
	}

	slog.Info("loaded controller config from settings", "config", c)

	return c
}

func settingString(settings map[string]string, key string, defaultVal string) string {
	s, ok := settings[key]
	if !ok {
		return defaultVal
	}
	return s
}

func settingBool(settings map[string]string, key string, defaultVal bool) bool {
	s, ok := settings[key]
	if !ok {
		return defaultVal
	}
	switch s {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		slog.Warn("invalid bool setting, using default", "key", key, "value", s, "default", defaultVal)
		return defaultVal
	}
}

func settingDuration(settings map[string]string, key string, defaultVal time.Duration) time.Duration {
	s, ok := settings[key]
	if !ok {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		slog.Warn("invalid duration setting, using default", "key", key, "value", s, "default", defaultVal)
		return defaultVal
	}
	return d
}

func settingInt(settings map[string]string, key string, defaultVal int) int {
	s, ok := settings[key]
	if !ok {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		slog.Warn("invalid int setting, using default", "key", key, "value", s, "default", defaultVal)
		return defaultVal
	}
	return v
}

func settingFloat(settings map[string]string, key string, defaultVal float64) float64 {
	s, ok := settings[key]
	if !ok {
		return defaultVal
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		slog.Warn("invalid float setting, using default", "key", key, "value", s, "default", defaultVal)
		return defaultVal
	}
	return v
}
