package controller

import (
	"log/slog"
	"strconv"
	"time"
)

type Config struct {
	Kind     string
	Interval time.Duration
	DryRun   bool

	GuestOvercommit float64
	HostReservedPct float64

	PressureHostSteepness   float64
	PressureGuestSteepness  float64
	PressureGuestMaxStepPct float64
	PressureGuestMinStepPct float64
}

func NewConfigFromSettings(settings map[string]string) Config {
	c := Config{
		Kind:     settingString(settings, "controller", "log"),
		Interval: settingDuration(settings, "controller-interval", time.Second),
		DryRun:   settingBool(settings, "controller-dry-run", false),

		GuestOvercommit: settingFloat(settings, "guest-overcommit", 2.0),
		HostReservedPct: settingFloat(settings, "host-reserved-pct", 0.05),

		PressureHostSteepness:   settingFloat(settings, "pressure-host-steepness", 4),
		PressureGuestSteepness:  settingFloat(settings, "pressure-guest-steepness", 4),
		PressureGuestMaxStepPct: settingFloat(settings, "pressure-guest-max-step-pct", 0.1),
		PressureGuestMinStepPct: settingFloat(settings, "pressure-guest-min-step-pct", 0.01),
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
