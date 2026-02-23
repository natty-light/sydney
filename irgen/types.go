package irgen

import (
	"bytes"
)

type BasicIrType string

const (
	IrInt    BasicIrType = "i64"
	IrFloat  BasicIrType = "double"
	IrBool   BasicIrType = "i1"
	IrString BasicIrType = "ptr"
	IrNull   BasicIrType = "ptr" // nullptr
	IrUnit   BasicIrType = "void"
)

type IrStruct struct {
	Name  string
	Types []IrType
}

type IrType interface {
	String() string
}

func (s *IrStruct) String() {
	var out bytes.Buffer
	out.WriteString("%struct.")
	out.WriteString(s.Name)
	out.WriteString(" = type { ")
	for i, t := range s.Types {
		if i > 0 {
			out.WriteString(", ")
			out.WriteString(t.String())
		}
	}
	out.WriteString(" }")
}

func (b BasicIrType) String() string {
	return string(b)
}
