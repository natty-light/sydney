package types

import (
	"bytes"
	"fmt"
)

type Type interface {
	Signature() string
}

type BasicType string

type CollectionType struct {
	IsEmpty bool
}

type ArrayType struct {
	ElemType Type
	CollectionType
}

type FunctionType struct {
	Params     []Type
	Return     Type
	Variadic   bool
	TypeParams []TypeParam
}

type MapType struct {
	KeyType   Type
	ValueType Type
	CollectionType
}

type StructType struct {
	Name                string
	Module              string
	Fields              []string
	Types               []Type
	Interfaces          []Type
	SatisfiedInterfaces []string
	TypeParams          []TypeParam
}

type InterfaceType struct {
	Name          string
	Module        string
	Methods       []string
	MethodIndices map[string]int
	Types         []Type
}

type ResultType struct {
	T Type
}

type ScopeType struct {
	Module string
	Name   string
}

type TypeParam struct {
	Constraint Type
	Name       string
}

type TypeParamRef struct {
	Name string
}

const (
	Int    BasicType = "int"
	Float  BasicType = "float"
	String BasicType = "string"
	Bool   BasicType = "bool"
	Null   BasicType = "null"
	Unit   BasicType = "unit"
	Any    BasicType = "any"
	Byte   BasicType = "byte"
	Infer  BasicType = "infer"
)

func (b BasicType) Signature() string {
	return fmt.Sprintf("%s", b)
}

func (a ArrayType) Signature() string {
	return fmt.Sprintf("array<%s>", a.ElemType.Signature())
}

func (f FunctionType) Signature() string {
	var out bytes.Buffer
	out.WriteString("func<(")
	for i, param := range f.Params {
		out.WriteString(param.Signature())
		if i < len(f.Params)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(") -> ")
	out.WriteString(f.Return.Signature())
	out.WriteString(">")

	return out.String()
}

func (m MapType) Signature() string {
	var out bytes.Buffer
	out.WriteString("map<")
	out.WriteString(m.KeyType.Signature())
	out.WriteString(", ")
	out.WriteString(m.ValueType.Signature())
	out.WriteString(">")
	return out.String()
}

func (s StructType) Signature() string {
	return s.Name
}

func (s InterfaceType) Signature() string {
	var out bytes.Buffer
	out.WriteString(s.Name)
	out.WriteString(" { ")
	for i, method := range s.Methods {
		t := s.Types[i].(FunctionType)
		out.WriteString(method)
		out.WriteString("(")
		for i, tt := range t.Params {
			out.WriteString(tt.Signature())
			if i < len(t.Params)-1 {
				out.WriteString(", ")
			}
		}
		out.WriteString(")")
		out.WriteString("->")
		out.WriteString(t.Return.Signature())
		if i < len(s.Methods)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(" }")

	return out.String()
}

func (r ResultType) Signature() string {
	var out bytes.Buffer
	out.WriteString("result<")
	out.WriteString(r.T.Signature())
	out.WriteString(">")
	return out.String()
}

func (s ScopeType) Signature() string {
	return s.Module + ":" + s.Name
}

func (t TypeParam) Signature() string {
	var out bytes.Buffer
	out.WriteString(t.Name)
	if t.Constraint != nil {
		out.WriteString(":")
		out.WriteString(t.Constraint.Signature())
	}
	return out.String()
}

func (t TypeParamRef) Signature() string {
	return t.Name
}

func SubstituteTypeParams(t Type, subs map[string]Type) Type {
	switch tt := t.(type) {
	case TypeParamRef:
		if sub, ok := subs[tt.Name]; ok {
			return sub
		}
	case MapType:
		return MapType{
			KeyType:   SubstituteTypeParams(tt.KeyType, subs),
			ValueType: SubstituteTypeParams(tt.ValueType, subs),
		}
	case ArrayType:
		return ArrayType{
			ElemType: SubstituteTypeParams(tt.ElemType, subs),
		}
	case ResultType:
		return ResultType{
			T: SubstituteTypeParams(tt.T, subs),
		}
	case FunctionType:
		params := make([]Type, len(tt.Params))
		for i, param := range tt.Params {
			params[i] = SubstituteTypeParams(param, subs)
		}
		tt.Params = params
		tt.Return = SubstituteTypeParams(tt.Return, subs)
		return tt
	case StructType:
		types := make([]Type, len(tt.Types))
		for i, t := range tt.Types {
			types[i] = SubstituteTypeParams(t, subs)
		}
		tt.Types = types
		return tt
	}
	return t
}
