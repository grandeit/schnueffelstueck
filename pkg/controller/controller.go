package controller

import "github.com/grandeit/schnueffelstueck/pkg/collector"

type Decision struct {
	BalloonTargetBytes uint64
	Reason             string
}

type Controller interface {
	Decide(sample collector.Sample) (*Decision, error)
}
