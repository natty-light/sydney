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
	case types.Any:
		return IrPtr // ptr to { i8, i64 } tagged union
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
	case types.OptionType:
		return IrPtr
	case *types.OptionType:
		return IrPtr
	}
	return IrUnit
}

const IrAnyTaggedUnion BasicIrType = "{ i8, i64 }"

// Tag values for the any tagged union
const (
	AnyTagInt    = 0
	AnyTagFloat  = 1
	AnyTagString = 2
	AnyTagBool   = 3
	AnyTagByte   = 4
)

func AnyTagForType(t IrType) int {
	switch t {
	case IrInt:
		return AnyTagInt
	case IrFloat:
		return AnyTagFloat
	case IrPtr:
		return AnyTagString
	case IrBool:
		return AnyTagBool
	case IrInt8:
		return AnyTagByte
	}
	return -1
}

func IrTypeForAnyTag(tag int) IrType {
	switch tag {
	case AnyTagInt:
		return IrInt
	case AnyTagFloat:
		return IrFloat
	case AnyTagString:
		return IrPtr
	case AnyTagBool:
		return IrBool
	case AnyTagByte:
		return IrInt8
	}
	return IrPtr
}

func GetResultTaggedUnion(t IrType) IrType {
	return BasicIrType(fmt.Sprintf("{ i1 , %s, ptr }", t))
}

func GetOptionTaggedUnion(t IrType) IrType {
	return BasicIrType(fmt.Sprintf("{ i1 , %s }", t))
}
