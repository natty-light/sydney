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

func (s *Scheduler) Add(f *Fiber) {
	s.fibers = append(s.fibers, f)
	s.runQueue = append(s.runQueue, f)
}

func (s *Scheduler) next() *Fiber {
	if len(s.runQueue) == 0 {
		return nil
	}
	next := s.runQueue[0]
	s.runQueue = s.runQueue[1:]
	return next
}

func (s *Scheduler) spawn() {

}

func (s *Scheduler) yield() {

}

func (s *Scheduler) block() {

}

func (s *Scheduler) unblock(f *Fiber) {}
