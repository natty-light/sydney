package scheduler

import (
	"sydney/object"
	"sydney/vm"
)

type Fiber struct {
	id         int
	stack      []*object.Object
	sp         int
	frames     []*vm.Frame
	frameIdx   int
	state      FiberState
	blockCause *Channel
}

type FiberState int

const (
	Ready FiberState = iota
	Running
	Blocked
	Done
)

func NewFiber(id int) *Fiber {
	return &Fiber{
		id:         id,
		stack:      make([]*object.Object, vm.StackSize),
		sp:         0,
		frames:     make([]*vm.Frame, vm.MaxFrames),
		frameIdx:   0,
		state:      Ready,
		blockCause: nil,
	}
}
