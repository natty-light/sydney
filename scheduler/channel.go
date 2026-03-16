package scheduler

import (
	"sydney/object"
	"sydney/types"
)

type Channel struct {
	elemType   types.Type
	buffer     []object.Object
	capacity   int
	head, tail int
	count      int
	sendQueue  []*Fiber
	recvQueue  []*Fiber
	closed     bool
}
