package controller

import (
	"log/slog"

	"github.com/grandeit/schnueffelstueck/pkg/collector"
)

type Decision struct {
	BalloonTargetBytes uint64
	Reason             string
}

type Controller interface {
	Decide(sample collector.Sample) (*Decision, error)
}

func NewController(config Config) Controller {
	switch config.Kind {
	case "log":
		slog.Info("selected log controller")
		return NewLogController()
	default:
		slog.Warn("selected unknown controller, falling back to default (log)", "controller", config.Kind)
		return NewLogController()
	}
}
