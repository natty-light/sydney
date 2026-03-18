package vm

import "sydney/object"

type Scheduler struct {
	fibers         []*Fiber
	runQueue       []*Fiber
	current        *Fiber
	mainFiber      *Fiber
	channels       map[int]*Channel
	nextChanId     int
	pendingWakeups chan wakeup
	ioBlockedCount int
}

type wakeup struct {
	fiber  *Fiber
	result object.Object
}

func NewScheduler() *Scheduler {
	main := NewFiber(0)
	return &Scheduler{
		fibers:         make([]*Fiber, 0),
		runQueue:       make([]*Fiber, 0),
		current:        main,
		mainFiber:      main,
		channels:       make(map[int]*Channel),
		pendingWakeups: make(chan wakeup, 64),
		ioBlockedCount: 0,
	}
}

func (s *Scheduler) Add(f *Fiber) {
	s.fibers = append(s.fibers, f)
	s.runQueue = append(s.runQueue, f)
}

// next pops the next ready fiber from the run queue.
// Returns nil if the queue is empty.
func (s *Scheduler) next() *Fiber {
	if len(s.runQueue) == 0 {
		return nil
	}
	next := s.runQueue[0]
	s.runQueue = s.runQueue[1:]
	return next
}

// enqueue marks a fiber as ready and adds it back to the run queue.
func (s *Scheduler) enqueue(f *Fiber) {
	f.state = Ready
	s.runQueue = append(s.runQueue, f)
}

// pushToFiberStack puts a value onto a suspended fiber's stack.
//
// When a fiber blocks on OpReceive, the opcode has already popped the
// channel off the stack and yielded (returned from runFiber). When the
// scheduler later has a value for that fiber, there's no running code
// that can call vm.push() — the fiber is asleep. So we write directly
// to the fiber's stack at its current sp and bump sp.
//
// When runFiber() resumes this fiber, it picks up at the instruction
// AFTER OpReceive, and the value is already on top of the stack exactly
// where the next instruction expects it.
func pushToFiberStack(f *Fiber, value object.Object) {
	f.stack[f.sp] = value
	f.sp++
}

func (s *Scheduler) registerChannel(id, capacity int) {
	s.channels[id] = &Channel{
		buffer:    make([]object.Object, capacity),
		capacity:  capacity,
		sendQueue: make([]*SenderWait, 0),
		recvQueue: make([]*Fiber, 0),
	}
}

func (s *Scheduler) nextChannelId() int {
	s.nextChanId++
	return s.nextChanId
}

// send attempts to send a value on a channel. The current fiber always
// yields after a send to give other fibers a chance to run.
//
// Three cases:
//  1. A receiver is already waiting → hand the value directly to it.
//     Both fibers go back in the run queue.
//  2. Buffered channel with space → store in ring buffer. Sender goes
//     back in the run queue.
//  3. Neither → sender blocks until a receiver shows up.
func (s *Scheduler) send(id int, val object.Object) {
	ch := s.channels[id]

	// Case 1: a fiber is blocked on receive for this channel.
	// Transfer the value directly onto its stack and wake it up.
	if len(ch.recvQueue) > 0 {
		receiver := ch.recvQueue[0]
		ch.recvQueue = ch.recvQueue[1:]

		pushToFiberStack(receiver, val)
		s.enqueue(receiver)
		s.enqueue(s.current)
		return
	}

	// Case 2: buffered channel with room in the ring buffer.
	if ch.capacity > 0 && ch.count < ch.capacity {
		ch.buffer[ch.tail] = val
		ch.tail = (ch.tail + 1) % ch.capacity
		ch.count++

		s.enqueue(s.current)
		return
	}

	// Case 3: can't send right now — block the sender.
	// We hold onto the value because OpSend already popped it off the
	// sender's stack. If we didn't save it here, it would be lost.
	s.current.state = Blocked
	ch.sendQueue = append(ch.sendQueue, &SenderWait{
		fiber: s.current,
		value: val,
	})
}

// receive attempts to receive a value from a channel. The current fiber
// always yields after a receive.
//
// Three cases:
//  1. Buffer has data → take from ring buffer. If a sender was blocked
//     waiting for space, unblock it and move its value into the buffer.
//  2. A sender is already waiting (unbuffered or empty buffer) → take
//     its value directly.
//  3. Neither → receiver blocks until a sender shows up.
func (s *Scheduler) receive(chanID int) {
	ch := s.channels[chanID]

	// Case 1: buffered channel with data available.
	// Read from the head of the ring buffer.
	if ch.capacity > 0 && ch.count > 0 {
		value := ch.buffer[ch.head]
		ch.head = (ch.head + 1) % ch.capacity
		ch.count--

		pushToFiberStack(s.current, value)

		// If a sender was blocked waiting for buffer space, unblock it
		// and write its value into the newly freed slot.
		if len(ch.sendQueue) > 0 {
			sender := ch.sendQueue[0]
			ch.sendQueue = ch.sendQueue[1:]

			ch.buffer[ch.tail] = sender.value
			ch.tail = (ch.tail + 1) % ch.capacity
			ch.count++

			s.enqueue(sender.fiber)
		}

		s.enqueue(s.current)
		return
	}

	// Case 2: a sender is waiting with a value (unbuffered rendezvous,
	// or the buffer was empty but a sender queued before us).
	// Take the value directly from the sender.
	if len(ch.sendQueue) > 0 {
		sender := ch.sendQueue[0]
		ch.sendQueue = ch.sendQueue[1:]

		pushToFiberStack(s.current, sender.value)
		s.enqueue(sender.fiber)
		s.enqueue(s.current)
		return
	}

	// Case 3: nothing available — block the receiver.
	// When a sender eventually arrives, it will call pushToFiberStack
	// on this fiber to deposit the value before waking it up.
	s.current.state = Blocked
	ch.recvQueue = append(ch.recvQueue, s.current)
}

func (s *Scheduler) hasBlockedFibers() bool {
	for _, f := range s.fibers {
		if f.state == Blocked {
			return true
		}
	}
	return false
}

func (s *Scheduler) drainWakeups() {
	for {
		select {
		case w := <-s.pendingWakeups:
			pushToFiberStack(w.fiber, w.result)
			s.ioBlockedCount--
			s.enqueue(w.fiber)
		default:
			return
		}
	}
}
