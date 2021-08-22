package ecs

import (
	"time"
	"sync"
)

type System struct {
	Name string
	Func func(dt time.Duration)
}

func (s *System) Run(dt time.Duration) {
	s.Func(dt)
}

type Signal struct {
	mu sync.Mutex
	value bool
}

func (s *Signal) Set(val bool) {
	s.mu.Lock()
	s.value = val
	s.mu.Unlock()
}

func (s *Signal) Get() bool {
	s.mu.Lock()
	ret := s.value
	s.mu.Unlock()
	return ret
}

// Note: Would be nice to sleep or something to prevent spinning while we wait for work to do
// Could also separate the render loop from the physics loop (requires some thread safety in ECS)
func RunGame(inputSystems, physicsSystems, renderSystems []System, quit *Signal) {
	const fixedTimeStep = 16 * time.Millisecond

	frameStart := time.Now()
	dt := fixedTimeStep
	var accumulator time.Duration

	for !quit.Get() {
		// Input Systems
		for _,sys := range inputSystems {
			sys.Run(dt)
		}

		// Physics Systems
		if accumulator >= fixedTimeStep {
			for _,sys := range physicsSystems {
				sys.Run(fixedTimeStep)
			}
			accumulator -= fixedTimeStep
		}

		// Render Systems
		for _,sys := range renderSystems {
			sys.Run(dt)
		}

		// Capture Frame time
		dt = time.Since(frameStart)
		frameStart = time.Now()

		accumulator += dt
	}
}
