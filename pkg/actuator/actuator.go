package actuator

type Actuator interface {
	Apply(balloonTargetBytes uint64) error
}
