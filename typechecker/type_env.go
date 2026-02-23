package typechecker

import (
	"sydney/types"
)

type TypeEnv struct {
	store     map[string]types.Type
	outer     *TypeEnv
	constants map[string]bool
}

func NewTypeEnv(parent *TypeEnv) *TypeEnv {
	return &TypeEnv{
		store:     make(map[string]types.Type),
		outer:     parent,
		constants: make(map[string]bool),
	}
}

func (e *TypeEnv) Set(name string, t types.Type) {
	e.store[name] = t
}

func (e *TypeEnv) Get(name string) (types.Type, bool, bool) {
	fromOuter := false
	t, ok := e.store[name]
	if !ok && e.outer != nil {
		t, _, ok = e.outer.Get(name)
		fromOuter = true
	}
	return t, fromOuter, ok
}

func (e *TypeEnv) SetConst(name string) {
	e.constants[name] = true
}

func (e *TypeEnv) IsConst(name string) bool {
	isConst, ok := e.constants[name]
	if !ok && e.outer != nil {
		isConst = e.outer.IsConst(name)
	}

	return isConst
}
