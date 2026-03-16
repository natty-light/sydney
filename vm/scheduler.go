package vm

type Scheduler struct {
	fibers   []*Fiber
	runQueue []*Fiber
	current  *Fiber
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		fibers:   make([]*Fiber, 0),
		runQueue: make([]*Fiber, 0),
		current:  NewFiber(0),
	}
}
