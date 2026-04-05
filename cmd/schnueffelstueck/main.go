package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/grandeit/schnueffelstueck/pkg/actuator"
	"github.com/grandeit/schnueffelstueck/pkg/collector"
	"github.com/grandeit/schnueffelstueck/pkg/controller"
	"github.com/grandeit/schnueffelstueck/pkg/hook"
	"github.com/grandeit/schnueffelstueck/pkg/qmp"
)

const qemuSocketTimeout = 60 * time.Second

func main() {
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	flag.Parse()

	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		slog.Error("invalid log level", "level", *logLevel, "error", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
	slog.Info("schnueffelstueck starting")

	hookServer, err := hook.NewServer()
	if err != nil {
		slog.Error("failed to create gRPC server", "error", err)
		os.Exit(1)
	}

	go func() {
		if err := run(hookServer); err != nil {
			slog.Error("main loop exited with error", "error", err)
		}
	}()

	if err := hookServer.Run(); err != nil {
		slog.Error("gRPC server exited with error", "error", err)
		os.Exit(1)
	}

	slog.Info("schnueffelstueck shutting down")
}

func run(hookServer *hook.Server) error {
	qemuSocket := hookServer.QEMUMonitorSocketPath
	slog.Info("waiting for QEMU socket", "socket", qemuSocket)

	if !waitForSocket(qemuSocket, hookServer.Done()) {
		return fmt.Errorf("QEMU socket was not available in time: %s", qemuSocket)
	}

	slog.Info("QEMU socket found, connecting", "socket", qemuSocket)

	qmpClient := qmp.NewClient(qemuSocket, "/machine/peripheral/balloon0")
	defer qmpClient.Close()

	if err := qmpClient.Connect(); err != nil {
		return fmt.Errorf("connecting to QEMU socket: %w", err)
	}

	slog.Info("connection to QEMU socket established")

	config := controller.NewConfigFromSettings(hookServer.SettingsFromAnnotations())

	col := collector.NewCollector(qmpClient)
	ctl := controller.NewController(config)

	var act actuator.Actuator
	if config.DryRun {
		act = actuator.NewLogActuator()
	} else {
		act = actuator.NewQEMUActuator(qmpClient)
	}

	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-hookServer.Done():
			slog.Info("main control loop shutting down, received shutdown request")
			return nil
		case <-ticker.C:
			sample, err := col.Collect()
			if err != nil {
				slog.Error("collection failed", "error", err)
				continue
			}

			decision, err := ctl.Decide(sample)
			if err != nil {
				slog.Error("controller decision failed", "error", err)
				continue
			}

			if decision != nil {
				if err := act.Apply(decision.BalloonTargetBytes); err != nil {
					slog.Error("actuator failed", "error", err)
				}
			}
		}
	}
}

func waitForSocket(path string, done <-chan struct{}) bool {
	deadline := time.After(qemuSocketTimeout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return false
		case <-deadline:
			slog.Error("timed out waiting for QEMU socket", "path", path)
			return false
		case <-ticker.C:
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
	}
}
