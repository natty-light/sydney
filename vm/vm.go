package vm

import (
	"fmt"
	"sydney/code"
	"sydney/compiler"
	"sydney/object"
)

const StackSize = 2048
const GlobalsSize = 65536
const MaxFrames = 1024

var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}
var Null = &object.Null{}

type VM struct {
	constants []object.Object

	stack []object.Object
	sp    int // always points to next value. top of stack is stack[sp - 1]

	globals []object.Object

	frames      []*Frame
	framesIndex int
}

func New(bytecode *compiler.Bytecode) *VM {

	mainFn := &object.CompiledFunction{Instructions: bytecode.Instructions}
	mainClosure := &object.Closure{Fn: mainFn}
	mainFrame := NewFrame(mainClosure, 0)

	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	return &VM{
		constants: bytecode.Constants,

		stack: make([]object.Object, StackSize),
		sp:    0,

		globals: make([]object.Object, GlobalsSize),

		frames:      frames,
		framesIndex: 1,
	}
}

func NewWithGlobalStore(bytecode *compiler.Bytecode, globals []object.Object) *VM {
	vm := New(bytecode)
	vm.globals = globals
	return vm
}

func (vm *VM) StackTop() object.Object {
	if vm.sp == 0 {
		return nil
	}
	return vm.stack[vm.sp-1]
}

func (vm *VM) Run() error {

	var ip int
	var ins code.Instructions
	var op code.Opcode

	for vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		vm.currentFrame().ip++

		ip = vm.currentFrame().ip
		ins = vm.currentFrame().Instructions()

		op = code.Opcode(ins[ip])

		switch op {
		case code.OpConstant:
			// get constant index from instruction
			constIdx := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2 // update instruction pointer

			// push constant onto stack
			err := vm.push(vm.constants[constIdx])
			if err != nil {
				return err
			}
		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv:
			err := vm.executeBinaryOperation(op)
			if err != nil {
				return err
			}
		case code.OpPop:
			vm.pop()
		case code.OpTrue:
			err := vm.push(True)
			if err != nil {
				return err
			}
		case code.OpFalse:
			err := vm.push(False)
			if err != nil {
				return err
			}
		case code.OpEqual, code.OpNotEqual, code.OpGt, code.OpGte, code.OpAnd, code.OpOr:
			err := vm.executeComparison(op)
			if err != nil {
				return err
			}
		case code.OpBang:
			err := vm.executeBangOperator()
			if err != nil {
				return err
			}
		case code.OpMinus:
			err := vm.executeMinusOperator()
			if err != nil {
				return err
			}
		case code.OpJump:
			// read jump position from operand of instruction at ip+1
			pos := int(code.ReadUint16(ins[ip+1:]))
			// ip will be incremented as part of loop, so we set it to right before the jump position
			vm.currentFrame().ip = pos - 1
		case code.OpJumpNotTruthy:
			// read jump position from operand of instruction at ip+1
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2 // move past operand

			condition := vm.pop()
			// if the condition is not truthy, we jump back to the position right before the target
			if !isTruthy(condition) {
				// ip will be incremented as part of loop, so we set it to right before the jump position

				vm.currentFrame().ip = pos - 1
			}
		case code.OpNull:
			err := vm.push(Null)
			if err != nil {
				return err
			}
		case code.OpSetImmutableGlobal, code.OpSetMutableGlobal:
			// get index of variable from operand
			globalIdx := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2 // move past operand
			// pull value off stack and put it into the globals
			vm.globals[globalIdx] = vm.pop()
		case code.OpGetGlobal:
			// get index from operand
			globalIdx := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2 // move past operand

			// put value of variable on to stack
			err := vm.push(vm.globals[globalIdx])
			if err != nil {
				return err
			}
		case code.OpArray:
			// get number of elements from operand
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2 // move past operand

			// create array and push it onto stack
			array := vm.buildArray(vm.sp-numElements, vm.sp)
			vm.sp -= numElements
			err := vm.push(array)
			if err != nil {
				return err
			}
		case code.OpHash:
			// get number of elements from operand
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2 // move past operand

			// create hash and push it onto stack
			hash, err := vm.buildHash(vm.sp-numElements, vm.sp)
			if err != nil {
				return err
			}

			vm.sp -= numElements
			err = vm.push(hash)
			if err != nil {
				return err
			}
		case code.OpIndex:
			// get index from top of stack
			index := vm.pop()
			// get object from top of stack
			left := vm.pop()

			err := vm.executeIndexExpression(left, index)
			if err != nil {
				return err
			}
		case code.OpCall:
			// get number of arguments from operand
			numArgs := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			err := vm.executeCall(int(numArgs))
			if err != nil {
				return err
			}
		case code.OpReturnValue:
			returnValue := vm.pop()

			// pop frame
			frame := vm.popFrame()
			// restore stack pointer
			vm.sp = frame.basePointer - 1

			// push return value onto stack
			err := vm.push(returnValue)
			if err != nil {
				return err
			}
		case code.OpReturn:
			// pop frame
			frame := vm.popFrame()
			// restore stack pointer, also has effect of popping last value off stack
			vm.sp = frame.basePointer - 1

			err := vm.push(Null)
			if err != nil {
				return err
			}
		case code.OpSetImmutableLocal, code.OpSetMutableLocal:
			// get local index from operand
			localIdx := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1 // move past operand

			frame := vm.currentFrame()

			vm.stack[frame.basePointer+int(localIdx)] = vm.pop()
		case code.OpGetLocal:
			// get local index from operand
			localIdx := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1 // move past operand

			frame := vm.currentFrame()

			// index into the reserved space for local variables in the stack and push the value onto the stack
			err := vm.push(vm.stack[frame.basePointer+int(localIdx)])
			if err != nil {
				return err
			}
		case code.OpGetBuiltIn:
			// get built-in index from operand
			builtinIdx := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1 // move past operand

			// get built-in function from index and push it onto the stack
			def := object.Builtins[builtinIdx]

			err := vm.push(def.BuiltIn)
			if err != nil {
				return err
			}
		case code.OpClosure:
			// get index of compiled fn
			constIdx := code.ReadUint16(ins[ip+1:])
			// get number of free variables
			numFree := code.ReadUint8(ins[ip+3:])
			// advance past operands
			vm.currentFrame().ip += 3

			err := vm.pushClosure(int(constIdx), int(numFree))
			if err != nil {
				return err
			}
		case code.OpGetFree:
			freeIdx := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1 // move past operand

			currentClosure := vm.currentFrame().cl

			err := vm.push(currentClosure.Free[freeIdx]) // push free var onto stack
			if err != nil {
				return err
			}
		case code.OpCurrentClosure:
			currentClosure := vm.currentFrame().cl
			err := vm.push(currentClosure)
			if err != nil {
				return err
			}
		case code.OpIndexSet:
			value := vm.pop()
			index := vm.pop()
			left := vm.pop()

			err := vm.executeIndexAssignment(left, index, value)
			if err != nil {
				return err
			}
		case code.OpStruct:
			objIdx := code.ReadUint16(ins[ip+1:])
			typeObj := vm.constants[objIdx].(*object.TypeObject)
			numFields := code.ReadUint8(ins[ip+3:])
			vm.currentFrame().ip += 3
			objs := make([]object.Object, numFields)
			for i := 0; i < int(numFields); i++ {
				obj := vm.pop()
				objs[i] = obj
			}

			obj := &object.Struct{T: typeObj, Fields: objs}
			err := vm.push(obj)
			if err != nil {
				return err
			}
		case code.OpGetField:
			fieldIdx := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			left := vm.pop()
			s, ok := left.(*object.Struct)
			if !ok {
				return fmt.Errorf("expected struct, got %T", left)
			}

			err := vm.push(s.Fields[fieldIdx])
			if err != nil {
				return err
			}
		case code.OpSetField:
			fieldIdx := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			value := vm.pop()
			left := vm.pop()

			s, ok := left.(*object.Struct)
			if !ok {
				return fmt.Errorf("expected struct, got %T", left)
			}

			s.Fields[fieldIdx] = value
		}
	}
	return nil
}

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

func (vm *VM) push(o object.Object) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}
	vm.stack[vm.sp] = o
	vm.sp++

	return nil
}

func (vm *VM) pop() object.Object {
	o := vm.head()
	vm.sp--
	return o
}

func (vm *VM) head() object.Object {
	return vm.stack[vm.sp-1]
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}

func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
}

func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	leftType := left.Type()
	rightType := right.Type()

	switch {
	case leftType == object.IntegerObj && rightType == object.IntegerObj:
		return vm.executeBinaryIntegerOperation(op, left, right)
	case leftType == object.FloatObj && rightType == object.FloatObj:
		return vm.executeBinaryFloatOperation(op, left, right)
	case leftType == object.StringObj && rightType == object.StringObj:
		return vm.executeBinaryStringOperation(op, left, right)
	}
	return fmt.Errorf("unsupported types for binary operation: %s %s", leftType, rightType)
}

func (vm *VM) executeBinaryIntegerOperation(op code.Opcode, left, right object.Object) error {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	var result int64
	switch op {
	case code.OpAdd:
		result = leftVal + rightVal
	case code.OpSub:
		result = leftVal - rightVal
	case code.OpMul:
		result = leftVal * rightVal
	case code.OpDiv:
		result = leftVal / rightVal
	default:
		return fmt.Errorf("unknown integer operator: %d", op)
	}

	return vm.push(&object.Integer{Value: result})
}

func (vm *VM) executeBinaryFloatOperation(op code.Opcode, left, right object.Object) error {
	leftVal := left.(*object.Float).Value
	rightVal := right.(*object.Float).Value

	var result float64
	switch op {
	case code.OpAdd:
		result = leftVal + rightVal
	case code.OpSub:
		result = leftVal - rightVal
	case code.OpMul:
		result = leftVal * rightVal
	case code.OpDiv:
		result = leftVal / rightVal
	default:
		return fmt.Errorf("unknown float operator: %d", op)
	}

	return vm.push(&object.Float{Value: result})
}

func (vm *VM) executeComparison(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	leftType := left.Type()
	rightType := right.Type()

	if leftType == object.IntegerObj && rightType == object.IntegerObj {
		return vm.executeIntegerComparison(op, left, right)
	}

	if leftType == object.FloatObj && rightType == object.FloatObj {
		return vm.executeFloatComparison(op, left, right)

	}

	if leftType == object.StringObj && rightType == object.StringObj {
		return vm.executeStringComparison(op, left, right)
	}
	// objects should be boolean at this point

	if left.Type() == right.Type() && left.Type() != object.BooleanObj {
		return fmt.Errorf("unknown operation for type %s", left.Type())
	}

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(left == right))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(left != right))
	case code.OpAnd:
		return vm.push(nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right)))
	case code.OpOr:
		return vm.push(nativeBoolToBooleanObject(isTruthy(left) || isTruthy(right)))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

func (vm *VM) executeIntegerComparison(op code.Opcode, left, right object.Object) error {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	var result bool
	switch op {
	case code.OpEqual:
		result = leftVal == rightVal
	case code.OpNotEqual:
		result = leftVal != rightVal
	case code.OpGt:
		result = leftVal > rightVal
	case code.OpGte:
		result = leftVal >= rightVal
	default:
		return fmt.Errorf("unknown integer operator: %d", op)
	}

	return vm.push(nativeBoolToBooleanObject(result))
}

func (vm *VM) executeFloatComparison(op code.Opcode, left, right object.Object) error {
	leftVal := left.(*object.Float).Value
	rightVal := right.(*object.Float).Value

	var result bool
	switch op {
	case code.OpEqual:
		result = leftVal == rightVal
	case code.OpNotEqual:
		result = leftVal != rightVal
	case code.OpGt:
		result = leftVal > rightVal
	case code.OpGte:
		result = leftVal >= rightVal
	default:
		return fmt.Errorf("unknown float operator: %d", op)
	}

	return vm.push(nativeBoolToBooleanObject(result))
}

func (vm *VM) executeBangOperator() error {
	operand := vm.pop()
	switch operand {
	case True:
		return vm.push(False)
	case False:
		return vm.push(True)
	default:
		if operand.Type() == object.IntegerObj && operand.(*object.Integer).Value == 0 {
			return vm.push(True)
		} else if operand.Type() == object.FloatObj && operand.(*object.Float).Value == 0 {
			return vm.push(True)
		}
		return vm.push(False)
	}
}

func (vm *VM) executeMinusOperator() error {
	operand := vm.pop()
	switch operand := operand.(type) {
	case *object.Integer:
		return vm.push(&object.Integer{Value: -operand.Value})
	case *object.Float:
		return vm.push(&object.Float{Value: -operand.Value})
	default:
		return fmt.Errorf("unsupported type for negation: %s", operand.Type())
	}
}

func (vm *VM) executeBinaryStringOperation(op code.Opcode, left, right object.Object) error {
	if op != code.OpAdd {
		return fmt.Errorf("unknown string operator: %d", op)
	}

	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value

	return vm.push(&object.String{Value: leftVal + rightVal})
}

func (vm *VM) executeStringComparison(op code.Opcode, left, right object.Object) error {
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value

	var result bool
	switch op {
	case code.OpEqual:
		result = leftVal == rightVal
	case code.OpNotEqual:
		result = leftVal != rightVal
	default:
		return fmt.Errorf("unknown string operator: %d", op)
	}

	return vm.push(nativeBoolToBooleanObject(result))
}

func (vm *VM) buildArray(startIndex, endIndex int) object.Object {
	elements := make([]object.Object, endIndex-startIndex)
	for i := startIndex; i < endIndex; i++ {
		elements[i-startIndex] = vm.stack[i]
	}
	return &object.Array{Elements: elements}
}

func (vm *VM) buildHash(startIndex, endIndex int) (*object.Hash, error) {
	hashedPairs := make(map[object.HashKey]object.HashPair)
	for i := startIndex; i < endIndex; i += 2 {
		key := vm.stack[i]
		value := vm.stack[i+1]

		hashKey, ok := key.(object.Hashable)
		if !ok {
			return nil, fmt.Errorf("unusable as hash key: %s", key.Type())
		}

		hashedPairs[hashKey.HashKey()] = object.HashPair{Key: key, Value: value}
	}

	return &object.Hash{Pairs: hashedPairs}, nil
}

func (vm *VM) executeIndexExpression(left, index object.Object) error {
	switch {
	case left.Type() == object.ArrayObj && index.Type() == object.IntegerObj:
		return vm.executeArrayIndex(left, index)
	case left.Type() == object.HashObj:
		return vm.executeHashIndex(left, index)
	default:
		return fmt.Errorf("index operator not supported: %s", left.Type())
	}
}

func (vm *VM) executeArrayIndex(array, index object.Object) error {
	arrayObj := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObj.Elements) - 1)

	if idx < 0 || idx > max {
		return vm.push(Null)
	}

	return vm.push(arrayObj.Elements[idx])
}

func (vm *VM) executeHashIndex(hash, index object.Object) error {
	hashObject := hash.(*object.Hash)

	key, ok := index.(object.Hashable)
	if !ok {
		return fmt.Errorf("unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return vm.push(Null)
	}

	return vm.push(pair.Value)
}

func (vm *VM) executeCall(numArgs int) error {
	// the function is at the bottom of the stack, below the args
	callee := vm.stack[vm.sp-1-numArgs]

	switch callee := callee.(type) {
	case *object.Closure:
		return vm.callClosure(callee, numArgs)
	case *object.BuiltIn:
		return vm.callBuiltIn(callee, numArgs)
	default:
		return fmt.Errorf("calling non-function %s", callee.Inspect())
	}
}

func (vm *VM) callClosure(cl *object.Closure, numArgs int) error {
	if numArgs != cl.Fn.NumParameters {
		return fmt.Errorf("wrong number of arguments. want=%d, got=%d", cl.Fn.NumParameters, numArgs)
	}

	// frame's base pointer is the first argument,
	// since stack pointer is always pointing to the next value
	// we need to subtract the number of arguments to get the base pointer
	frame := NewFrame(cl, vm.sp-numArgs)
	vm.pushFrame(frame)

	// set the stack pointer to the base pointer of the new frame
	vm.sp = frame.basePointer + cl.Fn.NumLocals

	return nil
}

func (vm *VM) callBuiltIn(builtin *object.BuiltIn, numArgs int) error {
	args := vm.stack[vm.sp-numArgs : vm.sp] // pull slice of args off stack

	result := builtin.Fn(args...)
	vm.sp = vm.sp - numArgs - 1 // pop args and function

	if result != nil {
		vm.push(result)
	} else {
		vm.push(Null)
	}

	return nil
}

func (vm *VM) pushClosure(constIdx, numFree int) error {
	constant := vm.constants[constIdx]
	fn, ok := constant.(*object.CompiledFunction)
	if !ok {
		return fmt.Errorf("not a function: %+v", constant)
	}

	free := make([]object.Object, numFree)
	for i := 0; i < numFree; i++ {
		free[i] = vm.stack[vm.sp-numFree+i]
	}
	vm.sp = vm.sp - numFree // clean up stack
	closure := &object.Closure{Fn: fn, Free: free}

	return vm.push(closure)
}

func (vm *VM) executeIndexAssignment(collection, index, value object.Object) error {
	switch col := collection.(type) {
	case *object.Array:
		idx, ok := index.(*object.Integer)
		if !ok {
			return fmt.Errorf("index must be an integer: %s", index.Type())
		}

		i := int(idx.Value)
		if i < 0 || i > len(col.Elements) {
			return fmt.Errorf("index out of bounds: %d", i)
		}

		col.Elements[i] = value
		return nil
	case *object.Hash:
		key, ok := index.(object.Hashable)
		if !ok {
			return fmt.Errorf("unusable as hash key: %s", index.Type())
		}
		col.Pairs[key.HashKey()] = object.HashPair{Key: index, Value: value}

		return nil
	default:
		return fmt.Errorf("unusable as index assignment: %s", collection.Type())
	}
}

// utility functions
func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	}
	return False
}

func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	default:
		return true
	}
}
