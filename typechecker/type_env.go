package typechecker

import (
	"fmt"
	"runtime/debug"
	"strings"
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
	if ft, ok := t.(types.FunctionType); ok {
		for _, p := range ft.Params {
			if containsTypeParamRef(p) {
				panic(fmt.Sprintf("invariant violation: env.Set(%q) has TypeParamRef param %s\n%s",
					name, p.Signature(), debug.Stack()))
			}
		}
		if containsTypeParamRef(ft.Return) {
			panic(fmt.Sprintf("invariant violation: env.Set(%q) has TypeParamRef in return type %s\n%s",
				name, ft.Return.Signature(), debug.Stack()))
		}
	}
	e.store[name] = t
}

func isTypeParamRef(t types.Type) bool {
	_, ok := t.(*types.TypeParamRef)
	return ok
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

func (e *TypeEnv) MethodsOf(structName string) []string {
	prefix := structName + "."
	var methods []string
	for name := range e.store {
		if strings.HasPrefix(name, prefix) {
			methods = append(methods, name[len(prefix):])
		}
	}
	if e.outer != nil {
		methods = append(methods, e.outer.MethodsOf(structName)...)
	}
	return methods
}

func (e *TypeEnv) IsConst(name string) bool {
	isConst, ok := e.constants[name]
	if !ok && e.outer != nil {
		isConst = e.outer.IsConst(name)
	}

	return isConst
}
