package code

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type Instructions []byte

type Opcode byte

const (
	OpConstant Opcode = iota
	OpPop
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpTrue
	OpFalse
	OpEqual
	OpNotEqual
	OpGt
	OpGte
	OpAnd
	OpOr
	OpMinus
	OpBang
	OpJumpNotTruthy
	OpJump
	OpNull
	OpSetMutableGlobal
	OpSetImmutableGlobal
	OpGetGlobal
	OpArray
	OpHash
	OpIndex
	OpCall
	OpReturnValue
	OpReturn
	OpSetImmutableLocal
	OpSetMutableLocal
	OpGetLocal
	OpGetBuiltIn
	OpClosure
	OpGetFree
	OpCurrentClosure
	OpIndexSet
	OpStruct
	OpGetField
	OpSetField
)

type (
	Definition struct {
		Name          string
		OperandWidths []int
	}
)

var definitions = map[Opcode]*Definition{
	OpConstant:           {"OpConstant", []int{2}},
	OpPop:                {"OpPop", []int{}},
	OpAdd:                {"OpAdd", []int{}},
	OpSub:                {"OpSub", []int{}},
	OpMul:                {"OpMul", []int{}},
	OpDiv:                {"OpDiv", []int{}},
	OpTrue:               {"OpTrue", []int{}},
	OpFalse:              {"OpFalse", []int{}},
	OpEqual:              {"OpEqual", []int{}},
	OpNotEqual:           {"OpNotEqual", []int{}},
	OpGt:                 {"OpGt", []int{}},
	OpGte:                {"OpGte", []int{}},
	OpAnd:                {"OpAnd", []int{}},
	OpOr:                 {"OpOr", []int{}},
	OpMinus:              {"OpMinus", []int{}},
	OpBang:               {"OpBang", []int{}},
	OpJumpNotTruthy:      {"OpJumpNotTruthy", []int{2}},
	OpJump:               {"OpJump", []int{2}},
	OpNull:               {"OpNull", []int{}},
	OpSetMutableGlobal:   {"OpSetMutableGlobal", []int{2}},
	OpSetImmutableGlobal: {"OpSetImmutableGlobal", []int{2}},
	OpGetGlobal:          {"OpGetGlobal", []int{2}},
	OpArray:              {"OpArray", []int{2}},
	OpHash:               {"OpHash", []int{2}},
	OpIndex:              {"OpIndex", []int{}},
	OpCall:               {"OpCall", []int{1}},
	OpReturnValue:        {"OpReturnValue", []int{}},
	OpReturn:             {"OpReturn", []int{}},
	OpSetImmutableLocal:  {"OpSetImmutableLocal", []int{1}},
	OpSetMutableLocal:    {"OpSetMutableLocal", []int{1}},
	OpGetLocal:           {"OpGetLocal", []int{1}},
	OpGetBuiltIn:         {"OpGetBuiltIn", []int{1}},
	OpClosure:            {"OpClosure", []int{2, 1}},
	OpGetFree:            {"OpGetFree", []int{1}},
	OpCurrentClosure:     {"OpCurrentClosure", []int{}},
	OpIndexSet:           {"OpIndexSet", []int{}},   // values on the stack
	OpStruct:             {"OpStruct", []int{2, 1}}, // num fields
	OpGetField:           {"OpGetField", []int{1}},  // idx
	OpSetField:           {"OpSetField", []int{1}},
}

func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}

	return def, nil
}

func Make(op Opcode, operands ...int) Instructions {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	instructionLen := 1
	for _, w := range def.OperandWidths {
		instructionLen += w
	}

	instruction := make([]byte, instructionLen)
	instruction[0] = byte(op)

	offset := 1
	for i, o := range operands {
		width := def.OperandWidths[i]
		switch width {
		case 1:
			instruction[offset] = byte(o)
		case 2:
			binary.BigEndian.PutUint16(instruction[offset:], uint16(o))
		}
		offset += width
	}

	return instruction
}

func ReadOperands(def *Definition, ins Instructions) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, width := range def.OperandWidths {
		switch width {
		case 1:
			operands[i] = int(ReadUint8(ins[offset:]))
		case 2:
			operands[i] = int(ReadUint16(ins[offset:]))
		}

		offset += width
	}

	return operands, offset
}

// ReadUint16 reads 2 bytes from ins and returns them as a uint16
func ReadUint16(ins Instructions) uint16 {
	return binary.BigEndian.Uint16(ins)
}

func (ins Instructions) String() string {
	var out bytes.Buffer

	i := 0
	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "Error: %s\n", err)
			continue
		}

		operands, read := ReadOperands(def, ins[i+1:])

		fmt.Fprintf(&out, "%04d %s\n", i, ins.fmtInstruction(def, operands))
		i += 1 + read
	}

	return out.String()
}

func ReadUint8(ins Instructions) uint8 {
	return uint8(ins[0])
}

func (ins Instructions) fmtInstruction(def *Definition, operands []int) string {
	operandCount := len(def.OperandWidths)

	if len(operands) != operandCount {
		return fmt.Sprintf("Error: operand len %d does not match defined %d\n", len(operands), operandCount)
	}
	switch operandCount {
	case 0:
		return def.Name
	case 1:
		return fmt.Sprintf("%s %d", def.Name, operands[0])
	case 2:
		return fmt.Sprintf("%s %d %d", def.Name, operands[0], operands[1])
	}

	return fmt.Sprintf("Error: unhandled operandCount for %s\n", def.Name)
}
