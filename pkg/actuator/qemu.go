package actuator

import (
	"fmt"
	"log/slog"

	"github.com/grandeit/schnueffelstueck/pkg/qmp"
)

type QEMUActuator struct {
	client                   *qmp.Client
	lastAppliedBalloonTarget uint64
}

func NewQEMUActuator(client *qmp.Client) *QEMUActuator {
	return &QEMUActuator{client: client}
}

func (a *QEMUActuator) Apply(balloonTargetBytes uint64) error {
	if balloonTargetBytes == a.lastAppliedBalloonTarget {
		return nil
	}

	if err := a.client.SetBalloonTarget(balloonTargetBytes); err != nil {
		return fmt.Errorf("setting balloon target to %d bytes: %w", balloonTargetBytes, err)
	}

	a.lastAppliedBalloonTarget = balloonTargetBytes

	slog.Info("balloon target set", "balloon_target_bytes", balloonTargetBytes, "balloon_target_mib", balloonTargetBytes/(1024*1024), "actuator", "qemu")
	return nil
}
