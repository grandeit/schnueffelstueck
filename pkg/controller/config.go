package controller

import (
	"log/slog"
	"time"
)

type Config struct {
	Kind     string
	Interval time.Duration
	DryRun   bool
}

func NewConfigFromSettings(settings map[string]string) Config {
	c := Config{
		Kind:     settingString(settings, "controller", "log"),
		Interval: settingDuration(settings, "controller-interval", time.Second),
		DryRun:   settingBool(settings, "controller-dry-run", false),
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
