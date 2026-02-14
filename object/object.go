package object

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"strconv"
	"sydney/ast"
	"sydney/code"

	"strings"
)

type ObjectType string
type BuiltInFunction func(args ...Object) Object

const (
	IntegerObj          ObjectType = "Integer"
	BooleanObj          ObjectType = "Boolean"
	NullObj             ObjectType = "Null"
	ReturnValueObj      ObjectType = "ReturnValue"
	ErrorObj            ObjectType = "Error"
	VariableObj         ObjectType = "Variable"
	FunctionObj         ObjectType = "Function"
	StringObj           ObjectType = "String"
	BuiltInObj          ObjectType = "BuiltIn"
	ArrayObj            ObjectType = "Array"
	HashObj             ObjectType = "Hash"
	FloatObj            ObjectType = "Float"
	QuoteObj            ObjectType = "Quote"
	MacroObj            ObjectType = "Macro"
	CompiledFunctionObj ObjectType = "CompiledFunction"
	ClosureObj          ObjectType = "Closure"
)

type (
	Object interface {
		Type() ObjectType
		Inspect() string
	}

	Hashable interface {
		HashKey() HashKey
	}
)

type (
	Integer struct {
		Value int64
	}

	Boolean struct {
		Value bool
	}

	Null struct{}

	ReturnValue struct {
		Value Object
	}

	Error struct {
		Message string
	}

	Variable struct {
		Value    Object
		Constant bool
	}

	Function struct {
		Parameters []*ast.Identifier
		Body       *ast.BlockStmt
		Scope      *Scope
	}

	String struct {
		Value string
	}

	BuiltIn struct {
		Fn BuiltInFunction
	}

	Array struct {
		Elements []Object
	}

	HashKey struct {
		Type        ObjectType
		HashValue   uint64
		ObjectValue interface{}
	}

	HashPair struct {
		Key   Object
		Value Object
	}

	Hash struct {
		Pairs map[HashKey]HashPair
	}

	Float struct {
		Value float64
	}

	Quote struct {
		Node ast.Node
	}

	Macro struct {
		Parameters []*ast.Identifier
		Body       *ast.BlockStmt
		Scope      *Scope
	}

	CompiledFunction struct {
		Instructions  code.Instructions
		NumLocals     int
		NumParameters int
	}

	Closure struct {
		Fn   *CompiledFunction
		Free []Object
	}
)

func (i *Integer) Type() ObjectType {
	return IntegerObj
}

func (b *Boolean) Type() ObjectType {
	return BooleanObj
}

func (n *Null) Type() ObjectType {
	return NullObj
}

func (r *ReturnValue) Type() ObjectType {
	return ReturnValueObj
}

func (e *Error) Type() ObjectType {
	return ErrorObj
}

func (v *Variable) Type() ObjectType {
	return VariableObj
}

func (f *Function) Type() ObjectType {
	return FunctionObj
}

func (b *BuiltIn) Type() ObjectType {
	return BuiltInObj
}

func (s *String) Type() ObjectType {
	return StringObj
}

func (a *Array) Type() ObjectType {
	return ArrayObj
}

func (h *Hash) Type() ObjectType {
	return HashObj
}

func (f *Float) Type() ObjectType {
	return FloatObj
}

func (q *Quote) Type() ObjectType {
	return QuoteObj
}

func (m *Macro) Type() ObjectType {
	return MacroObj
}

func (c *CompiledFunction) Type() ObjectType {
	return CompiledFunctionObj
}

func (c *Closure) Type() ObjectType {
	return ClosureObj
}

func (i *Integer) Inspect() string {
	return fmt.Sprintf("%d", i.Value)
}

func (b *Boolean) Inspect() string {
	return fmt.Sprintf("%t", b.Value)
}

func (r *ReturnValue) Inspect() string {
	return r.Value.Inspect()
}

func (n *Null) Inspect() string {
	return "null"
}

func (e *Error) Inspect() string {
	return fmt.Sprintf("Honk! Error: %s", e.Message)
}

func (v *Variable) Inspect() string {
	return v.Value.Inspect()
}

func (f *Function) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range f.Parameters {
		params = append(params, p.String())
	}

	out.WriteString("func(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(f.Body.String())
	out.WriteString("\n}")

	return out.String()
}

func (s *String) Inspect() string {
	return s.Value
}

func (b *BuiltIn) Inspect() string {
	return "builtin function"
}

func (a *Array) Inspect() string {
	var out bytes.Buffer

	elements := []string{}
	for _, e := range a.Elements {
		elements = append(elements, e.Inspect())
	}

	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")

	return out.String()
}

func (h *Hash) Inspect() string {
	var out bytes.Buffer

	pairs := []string{}
	for _, pair := range h.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s", pair.Key.Inspect(), pair.Value.Inspect()))
	}
	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")

	return out.String()
}

func (f *Float) Inspect() string {
	return strconv.FormatFloat(f.Value, 'f', -1, 64)
}

func (q *Quote) Inspect() string {
	return "QUOTE(" + q.Node.String() + ")"
}

func (m *Macro) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range m.Parameters {
		params = append(params, p.String())
	}

	out.WriteString("macro(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(m.Body.String())
	out.WriteString("\n}")

	return out.String()
}

func (c *CompiledFunction) Inspect() string {
	return fmt.Sprintf("CompiledFunction[%p]", c)
}

func (c *Closure) Inspect() string {
	return fmt.Sprintf("Closure[%p]", c)
}

// HashKey functions
func (b *Boolean) HashKey() HashKey {
	var val uint64

	if b.Value {
		val = 1
	} else {
		val = 0
	}

	return HashKey{Type: b.Type(), HashValue: val, ObjectValue: b.Value}
}

func (i *Integer) HashKey() HashKey {
	return HashKey{Type: i.Type(), HashValue: uint64(i.Value), ObjectValue: i.Value}
}

func (s *String) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(s.Value))

	return HashKey{Type: s.Type(), HashValue: h.Sum64(), ObjectValue: s.Value}
}
