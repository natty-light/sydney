package object

import (
	"fmt"
)

func NewScope() *Scope {
	s := make(map[string]Variable)

	return &Scope{store: s, outer: nil}
}

func NewEnclosedScope(outer *Scope) *Scope {
	s := NewScope()
	s.outer = outer

	return s
}

type Scope struct {
	store map[string]Variable
	outer *Scope
}

// var, fromOuter, ok
func (s *Scope) Get(name string) (Variable, bool, bool) {
	obj, ok := s.store[name]
	fromOuter := false
	if !ok && s.outer != nil {
		obj, _, ok = s.outer.Get(name)
		fromOuter = true
	}
	return obj, fromOuter, ok
}

func (s *Scope) Set(name string, val Object, isConstant bool) Object {
	s.store[name] = Variable{Value: val, Constant: isConstant}
	return val
}

func (s *Scope) DeclareVar(name string, val Object, isConst bool) Object {
	if isConst && val.Type() == NullObj {
		return newError("const variable %s must be initialize", name)
	}

	_, fromOuter, ok := s.Get(name)

	// If the variable already exists in this scope we cannot redeclare it
	if ok && !fromOuter {
		return newError("cannot redeclare block scoped variable %s", name)
	} else {
		// if the variable doesn't exist or its from the parent scope
		s.store[name] = Variable{Value: val, Constant: isConst}
		return val
	}
}

func (s *Scope) AssignVar(name string, val Object) Object {
	scope, ok := s.Resolve(name)

	if !ok {
		return newError("cannot resolve variable %s", name)
	}
	// if we get here, we know the variable exists so we can ignore the boolean return values
	existing, _, _ := scope.Get(name)

	if existing.Constant {
		return newError("cannot assign value to constant %s", name)
	}

	return scope.Set(name, val, false)
}

func (s *Scope) Resolve(name string) (*Scope, bool) {
	// all we need to know is if the variable exists in this scope
	_, fromOuter, ok := s.Get(name)
	if ok && !fromOuter {
		return s, true
	}
	if s.outer == nil {
		return nil, false
	}
	// since Parent is a pointer to allow for nil, Scope will always be a pointer
	return s.outer.Resolve(name)
}

func newError(format string, a ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}
