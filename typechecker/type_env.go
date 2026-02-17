package typechecker

import "sydney/types"

type TypeEnv struct {
	store map[string]types.Type
	outer *TypeEnv
}

func NewTypeEnv(parent *TypeEnv) *TypeEnv {
	return &TypeEnv{
		store: make(map[string]types.Type),
		outer: parent,
	}
}

func (e *TypeEnv) Set(name string, t types.Type) {
	e.store[name] = t
}

func (e *TypeEnv) Get(name string) (types.Type, bool) {
	t, ok := e.store[name]
	if !ok && e.outer != nil {
		return e.outer.Get(name)
	}
	return t, ok
}
