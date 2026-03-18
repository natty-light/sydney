package irgen

import (
	"bytes"
	"fmt"
	"sydney/types"
)

type BasicIrType string

const (
	IrInt    BasicIrType = "i64"
	IrFloat  BasicIrType = "double"
	IrBool   BasicIrType = "i1"
	IrPtr    BasicIrType = "ptr"
	IrNull   BasicIrType = "ptr" // nullptr
	IrUnit   BasicIrType = "void"
	IrInt32  BasicIrType = "i32"
	IrFatPtr BasicIrType = "{ ptr, ptr }"
	IrInt8   BasicIrType = "i8"
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
		}
		out.WriteString(t.String())
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
	case types.Byte:
		return IrInt8
	}

	switch t.(type) {
	case types.StructType:
		return IrPtr // ptr — structs passed by pointer
	case types.FunctionType:
		return IrPtr // ptr — function pointer
	case types.InterfaceType:
		return IrPtr // ptr — interface fat pointer
	case types.ArrayType:
		return IrPtr
	case types.MapType:
		return IrPtr
	case types.ResultType:
		return IrPtr // tagged union ptr
	case *types.ResultType:
		return IrPtr // this is indicative of an issue where type structs are not pointers consistently
	case types.ChannelType:
		return IrInt
	}
	return IrUnit
}

func GetResultTaggedUnion(t IrType) IrType {
	return BasicIrType(fmt.Sprintf("{ i1 , %s, ptr }", t))
}
