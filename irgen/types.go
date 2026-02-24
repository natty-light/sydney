package irgen

import (
	"bytes"
	"sydney/types"
)

type BasicIrType string

const (
	IrInt   BasicIrType = "i64"
	IrFloat BasicIrType = "double"
	IrBool  BasicIrType = "i1"
	IrPtr   BasicIrType = "ptr"
	IrNull  BasicIrType = "ptr" // nullptr
	IrUnit  BasicIrType = "void"
)

type IrStruct struct {
	Name  string
	Types []IrType
}

type IrType interface {
	String() string
}

func (s *IrStruct) String() string {
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

	return out.String()
}

func (b BasicIrType) String() string {
	return string(b)
}

func SydneyTypeToIrType(t types.Type) IrType {
	switch t {
	case types.Int:
		return IrInt
	case types.Float:
		return IrFloat
	case types.Bool:
		return IrBool
	case types.String:
		return IrPtr
	case types.Null:
		return IrNull
	case types.Unit:
		return IrUnit
	}

	switch t.(type) {
	case types.StructType:
		return IrPtr // ptr — structs passed by pointer
	case types.FunctionType:
		return IrPtr // ptr — function pointer
	case types.InterfaceType:
		return IrPtr // ptr — interface fat pointer
	}
	return IrUnit
}
