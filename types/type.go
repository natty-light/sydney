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
	Params   []Type
	Return   Type
	Variadic bool
}

type MapType struct {
	KeyType   Type
	ValueType Type
	CollectionType
}

type StructType struct {
	Name   string
	Fields []string
	Types  []Type
}

type InterfaceType struct {
	Name    string
	Methods []string
	Types   []Type
}

const (
	Int    BasicType = "int"
	Float  BasicType = "float"
	String BasicType = "string"
	Bool   BasicType = "bool"
	Null   BasicType = "null"
	Unit   BasicType = "unit"
	Any    BasicType = "any"
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
	var out bytes.Buffer
	out.WriteString(s.Name)
	out.WriteString(" { ")
	for i, field := range s.Fields {
		out.WriteString(field)
		out.WriteString(" ")
		out.WriteString(s.Types[i].Signature())
		if i < len(s.Fields)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(" }")

	return out.String()
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
