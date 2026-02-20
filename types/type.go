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
	Params []Type
	Return Type
}

type MapType struct {
	KeyType   Type
	ValueType Type
	CollectionType
}

const (
	Int    BasicType = "int"
	Float  BasicType = "float"
	String BasicType = "string"
	Bool   BasicType = "bool"
	Null   BasicType = "null"
	Unit   BasicType = "unit"
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
