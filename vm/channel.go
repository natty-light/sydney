package vm

import (
	"sydney/object"
	"sydney/types"
)

// Channel holds the runtime state for a channel.
// The object.Channel that sits on the Sydney stack is just a handle (an int ID)
// that maps to one of these via the scheduler's channels map.
type Channel struct {
	elemType types.Type
	closed   bool

	// Ring buffer for buffered channels. The buffer is a fixed-size array
	// allocated at channel creation. head and tail are indices that wrap
	// around using modulo arithmetic:
	//
	//   buffer:  [ _ | A | B | C | _ | _ ]
	//              ^               ^
	//              tail            head
	//
	// - head: index of the next value to read (oldest item)
	// - tail: index of the next empty slot to write into
	// - count: number of items currently in the buffer
	//
	// To send: write to buffer[tail], advance tail = (tail + 1) % capacity
	// To receive: read from buffer[head], advance head = (head + 1) % capacity
	//
	// When head == tail and count > 0, the buffer is full.
	// When count == 0, the buffer is empty.
	//
	// For unbuffered channels (capacity == 0), the buffer is never used —
	// sender and receiver must rendezvous directly via the wait queues.
	buffer     []object.Object
	capacity   int
	head, tail int
	count      int

	// sendQueue holds fibers that tried to send but couldn't (buffer full
	// or unbuffered with no receiver). Each entry pairs the blocked fiber
	// with the value it wants to send, since the value has already been
	// popped off the sender's stack by OpSend and would be lost otherwise.
	sendQueue []*SenderWait

	// recvQueue holds fibers that tried to receive but couldn't (buffer
	// empty and no sender waiting). When a value becomes available, the
	// scheduler pushes it directly onto the receiver's stack before waking
	// it up — this is necessary because the receiver's OpReceive has already
	// popped the channel and yielded, so there's no opcode left to push
	// the value. The fiber just resumes at the next instruction with the
	// value already on top of its stack.
	recvQueue []*Fiber
}

// SenderWait pairs a blocked sender fiber with the value it's trying to send.
// We need to hold the value here because OpSend already popped it off the
// sender's stack before the fiber was suspended.
type SenderWait struct {
	fiber *Fiber
	value object.Object
}
