package actuator

import (
	"log/slog"
)

type LogActuator struct{}

func NewLogActuator() *LogActuator {
	return &LogActuator{}
}

func (n *LogActuator) Apply(balloonTargetBytes uint64) error {
	slog.Info("balloon target set", "balloon_target_bytes", balloonTargetBytes, "balloon_target_mib", balloonTargetBytes/(1024*1024), "actuator", "log")
	return nil
}
