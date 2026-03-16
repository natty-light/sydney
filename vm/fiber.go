package vm

import (
	"sydney/object"
)

type Fiber struct {
	id         int
	stack      []object.Object
	sp         int
	frames     []*Frame
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
		stack:      make([]object.Object, StackSize),
		sp:         0,
		frames:     make([]*Frame, MaxFrames),
		frameIdx:   0,
		state:      Ready,
		blockCause: nil,
	}
}

func (f *Fiber) PushFrame(frame *Frame, cl *object.Closure) {
	f.frames[0] = frame
	f.frameIdx = 1
	f.sp = 1 + cl.Fn.NumLocals
}
