package compiler

import (
	"fmt"
	"slices"
	"sort"
	"sydney/ast"
	"sydney/code"
	"sydney/loader"
	"sydney/object"
	"sydney/types"
)

// ItabKey struct name:interface name
type ItabKey string

type Compiler struct {
	constants []object.Object

	symbolTable *SymbolTable

	scopes     []*CompilationScope
	scopeIndex int

	structTypes    map[string]types.StructType
	interfaceTypes map[string]types.InterfaceType
	itabMapping    map[ItabKey]int

	currentModule string

	loopContexts []*LoopContext
	loopIndex    int
}

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
}

type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

type CompilationScope struct {
	instructions        code.Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
}

type LoopContext struct {
	conditionPos      int
	hasPost           bool
	breakPositions    []int
	continuePositions []int
}

func New() *Compiler {
	mainScope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	symbolTable := NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	return &Compiler{
		constants:   []object.Object{},
		symbolTable: symbolTable,
		scopes:      []*CompilationScope{&mainScope},
		scopeIndex:  0,

		structTypes:    make(map[string]types.StructType),
		interfaceTypes: make(map[string]types.InterfaceType),

		itabMapping: make(map[ItabKey]int),

		loopContexts: make([]*LoopContext, 0),
		loopIndex:    0,
	}
}

func NewWithState(symbolTable *SymbolTable, constants []object.Object) *Compiler {
	compiler := New()
	compiler.symbolTable = symbolTable
	compiler.constants = constants
	return compiler
}

func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts { // set interface types and populate method indices map
			if pub, ok := stmt.(*ast.PubStatement); ok {
				stmt = pub.Stmt
			}
			if def, ok := stmt.(*ast.InterfaceDefinitionStmt); ok {
				name := def.Name.Value
				if c.currentModule != "" {
					name = c.mangleModule(c.currentModule, name)
				}
				c.setInterface(name, def.Type)
			}

			if fn, ok := stmt.(*ast.FunctionDeclarationStmt); ok { // hoist functions for interfaces
				name := fn.Name.Value
				if fn.MangledName != "" {
					name = fn.MangledName
				}
				if c.currentModule != "" {
					name = c.mangleModule(c.currentModule, name)
				}
				sym := c.symbolTable.DefineImmutable(name)
				if fn.MangledName != "" {
					// register as struct method
					c.symbolTable.DefineAlias(fn.Name.Value, sym)
					if c.currentModule != "" {
						// register as exported function as well
						c.symbolTable.DefineAlias(c.mangleModule(c.currentModule, fn.Name.Value), sym)
					}
				}

			}
		}

		for _, stmt := range node.Stmts { // create itabs
			if impl, ok := stmt.(*ast.InterfaceImplementationStmt); ok {
				c.compileInterfaceImplementation(impl)
			}
		}

		for _, s := range node.Stmts { // compile program
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}
	case *ast.PubStatement:
		err := c.Compile(node.Stmt)
		if err != nil {
			return err
		}
	case *ast.ExpressionStmt:
		err := c.Compile(node.Expr)
		if err != nil {
			return err
		}
		c.emit(code.OpPop)
	case *ast.BlockStmt:
		for _, s := range node.Stmts {
			if fn, ok := s.(*ast.FunctionDeclarationStmt); ok {
				name := fn.Name.Value
				if fn.MangledName != "" {
					name = fn.MangledName
				}
				if c.currentModule != "" {
					name = c.mangleModule(c.currentModule, name)
				}
				sym := c.symbolTable.DefineImmutable(name)
				if fn.MangledName != "" {
					c.symbolTable.DefineAlias(fn.Name.Value, sym)
				}
			}
		}

		for _, s := range node.Stmts {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}
	case *ast.VarDeclarationStmt:
		name := node.Name.Value
		if c.currentModule != "" && c.scopeIndex == 0 {
			name = c.mangleModule(c.currentModule, name)
		}
		sym, fromOuter, ok := c.symbolTable.Resolve(name)

		// if the variable exists in this scope, cannot redeclare
		if ok && !fromOuter && sym.Scope != FunctionScope {
			return fmt.Errorf("variable %s already declared", name)
		}

		if node.Value == nil {
			if node.Type != nil {
				err := c.emitZeroValue(node.Type)
				if err != nil {
					return err
				}
			} else {
				c.emit(code.OpNull)
			}
		} else {
			err := c.Compile(node.Value)
			if err != nil {
				return err
			}
		}

		if node.Constant {
			symbol := c.symbolTable.DefineImmutable(name)
			cde := code.OpSetImmutableLocal

			if symbol.Scope == GlobalScope {
				cde = code.OpSetImmutableGlobal
			}

			c.emit(cde, symbol.Index)
		} else {
			symbol := c.symbolTable.DefineMutable(name)
			cde := code.OpSetMutableLocal
			if symbol.Scope == GlobalScope {
				cde = code.OpSetMutableGlobal
			}
			c.emit(cde, symbol.Index)

		}
	case *ast.VarAssignmentStmt:
		err := c.Compile(node.Value)
		if err != nil {
			return err
		}

		symbol, _, ok := c.symbolTable.Resolve(node.Identifier.Value)
		if !ok {
			return fmt.Errorf("undefined variable %s", node.Identifier.Value)
		}

		if symbol.IsConstant {
			return fmt.Errorf("cannot assign to constant %s", node.Identifier.Value)
		}

		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetMutableGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetMutableLocal, symbol.Index)
		}
	case *ast.IndexAssignmentStmt:
		err := c.Compile(node.Left.Left) // compile collection ident
		if err != nil {
			return err
		}
		err = c.Compile(node.Left.Index) // compile index
		if err != nil {
			return err
		}

		err = c.Compile(node.Value)
		if err != nil {
			return err
		}

		c.emit(code.OpIndexSet)
	case *ast.ReturnStmt:
		err := c.Compile(node.ReturnValue)
		if err != nil {
			return err
		}
		c.emit(code.OpReturnValue)
	case *ast.ForStmt:
		var initVarName string
		if node.Init != nil {
			if varDecl, ok := node.Init.(*ast.VarDeclarationStmt); ok {
				initVarName = varDecl.Name.Value
			}
			err := c.Compile(node.Init)
			if err != nil {
				return err
			}
		}

		conditionPos := len(c.currentInstructions())
		c.enterLoop(conditionPos, node.Post != nil)

		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}
		// emit with operand to be replaced later
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(node.Body)
		if err != nil {
			return err
		}

		if node.Post != nil {
			postPos := len(c.currentInstructions())
			loop := c.getLoop()
			if loop != nil {
				for _, pos := range loop.continuePositions {
					c.changeOperand(pos, postPos)
				}
			}

			err = c.Compile(node.Post)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpJump, conditionPos)

		escapePos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, escapePos)
		loop := c.getLoop()
		if loop != nil {
			for _, pos := range c.getLoop().breakPositions {
				c.changeOperand(pos, escapePos)
			}
		}
		c.leaveLoop()

		if initVarName != "" {
			delete(c.symbolTable.store, initVarName)
		}

		c.emit(code.OpNull)
		c.emit(code.OpPop) // this clears the condition value from the stack

	case *ast.InfixExpr:
		if node.Operator == "<" || node.Operator == "<=" {
			err := c.Compile(node.Right)
			if err != nil {
				return err
			}

			err = c.Compile(node.Left)
			if err != nil {
				return err
			}

			if node.Operator == "<" {
				c.emit(code.OpGt)
			} else {
				c.emit(code.OpGte)
			}
			return nil
		}

		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {

		case "+":
			c.emit(code.OpAdd)
		case "-":
			c.emit(code.OpSub)
		case "*":
			c.emit(code.OpMul)
		case "/":
			c.emit(code.OpDiv)
		case "==":
			c.emit(code.OpEqual)
		case "!=":
			c.emit(code.OpNotEqual)
		case ">":
			c.emit(code.OpGt)
		case ">=":
			c.emit(code.OpGte)
		case "&&":
			c.emit(code.OpAnd)
		case "||":
			c.emit(code.OpOr)
		case "%":
			c.emit(code.OpModulo)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}
	case *ast.PrefixExpr:
		err := c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "!":
			c.emit(code.OpBang)
		case "-":
			c.emit(code.OpMinus)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}
	case *ast.IfExpr:
		// we don't need to update t here because we're not bubbling the value back up like in expressions
		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}
		// emit with operand to be replaced later
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(node.Consequence)
		if err != nil {
			return err
		}

		// remove last pop after compiling consequence so we don't inadvertently pop too many times
		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}

		//emit an OpJump with operand to be replaced later
		jumpPos := c.emit(code.OpJump, 9999)

		afterConsequencePos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, afterConsequencePos)

		// only if there is no alternative do we jump to immediately after the consequence
		if node.Alternative == nil {
			c.emit(code.OpNull)
		} else {

			err = c.Compile(node.Alternative)
			if err != nil {
				return err
			}

			if c.lastInstructionIs(code.OpPop) {
				c.removeLastPop()
			}
		}

		afterAlternativePos := len(c.currentInstructions())
		if node.GetResolvedType() == types.Unit {
			c.emit(code.OpNull)
		}
		c.changeOperand(jumpPos, afterAlternativePos)
	case *ast.IndexExpr:
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Index)
		if err != nil {
			return err
		}

		c.emit(code.OpIndex)
	case *ast.Identifier:
		name := node.Value
		symbol, _, ok := c.symbolTable.Resolve(name)
		if !ok && c.currentModule != "" {
			mangled := c.mangleModule(c.currentModule, name)
			symbol, _, ok = c.symbolTable.Resolve(mangled)
		}
		if !ok {
			return fmt.Errorf("undefined variable %s", node.Value)
		}
		c.loadSymbol(symbol)

	case *ast.CallExpr:
		if s, ok := node.Function.(*ast.SelectorExpr); ok {
			if _, ok := c.isInterfaceType(s.Left); ok {
				return c.compileInterfaceMethodCall(node)
			}
		}

		if node.MangledName != "" {
			symbol, _, ok := c.symbolTable.Resolve(node.MangledName)
			if !ok {
				return fmt.Errorf("undefined variable %s", node.MangledName)
			}
			c.loadSymbol(symbol)
		} else {
			err := c.Compile(node.Function)
			if err != nil {
				return err
			}
		}

		// push arguments on to stack
		for _, arg := range node.Arguments {
			err := c.Compile(arg)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpCall, len(node.Arguments))
	case *ast.IntegerLiteral:
		integer := &object.Integer{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(integer))
	case *ast.ByteLiteral:
		byt := &object.Byte{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(byt))
	case *ast.FloatLiteral:
		float := &object.Float{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(float))
	case *ast.BooleanLiteral:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}
	case *ast.NullLiteral:
		c.emit(code.OpNull)
	case *ast.StringLiteral:
		str := &object.String{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(str))
	case *ast.ArrayLiteral:
		for _, el := range node.Elements {
			err := c.Compile(el)
			if err != nil {
				return err
			}
		}
		c.emit(code.OpArray, len(node.Elements))
	case *ast.HashLiteral:
		keys := make([]ast.Expr, 0)
		for k := range node.Pairs {
			keys = append(keys, k)
		}

		// This sort is for the sake of the tests
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})

		for _, k := range keys {
			err := c.Compile(k)
			if err != nil {
				return err
			}

			err = c.Compile(node.Pairs[k])
			if err != nil {
				return err
			}
		}

		c.emit(code.OpHash, len(node.Pairs)*2)
	case *ast.FunctionDeclarationStmt:
		if node.IsExtern {
			return nil
		}

		name := node.Name.Value
		if node.MangledName != "" {
			name = node.MangledName
		}
		if c.currentModule != "" {
			name = c.mangleModule(c.currentModule, name)
		}

		symbol, _, ok := c.symbolTable.Resolve(name)
		if !ok {
			return fmt.Errorf("undefined function %s", name)
		}
		c.enterScope()

		c.symbolTable.DefineFunctionName(node.Name.Value)
		for _, p := range node.Params {
			c.symbolTable.DefineImmutable(p.Value)
		}

		err := c.Compile(node.Body)
		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.replaceLastPopWithReturn()
		}

		if !c.lastInstructionIs(code.OpReturn) {
			c.emit(code.OpReturn)
		}

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		// iterate over free symbols and load them onto stack
		for _, s := range freeSymbols {
			c.loadSymbol(s)
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.Params),
		}

		fnIdx := c.addConstant(compiledFn)

		c.emit(code.OpClosure, fnIdx, len(freeSymbols))
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetImmutableGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetImmutableLocal, symbol.Index)
		}

	case *ast.FunctionLiteral:
		c.enterScope()

		if node.Name != "" {
			c.symbolTable.DefineFunctionName(node.Name)
		}

		for _, p := range node.Parameters {
			c.symbolTable.DefineImmutable(p.Value)
		}

		err := c.Compile(node.Body)
		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.replaceLastPopWithReturn()
		}

		if !c.lastInstructionIs(code.OpReturnValue) {
			c.emit(code.OpReturn)
		}

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		// leave scope so we can load free symbols into enclosing scope
		instructions := c.leaveScope()

		// iterate over free symbols and load them onto stack
		for _, s := range freeSymbols {
			c.loadSymbol(s)
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.Parameters),
		}

		fnIdx := c.addConstant(compiledFn)

		c.emit(code.OpClosure, fnIdx, len(freeSymbols))
	case *ast.StructLiteral:
		t := node.ResolvedType.(types.StructType)
		for _, field := range t.Fields {
			idx := slices.Index(node.Fields, field)
			err := c.Compile(node.Values[idx])
			if err != nil {
				return err
			}
		}

		typeObj := &object.TypeObject{T: t}
		idx := c.addConstant(typeObj)

		c.emit(code.OpStruct, idx, len(t.Fields))
	case *ast.SelectorExpr:
		t := node.ResolvedType.(types.StructType)
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		fieldIdent := node.Value.(*ast.Identifier)
		// resolved type is appended in typechecker
		idx := slices.Index(t.Fields, fieldIdent.Value)

		c.emit(code.OpGetField, idx)
	case *ast.SelectorAssignmentStmt:
		t := node.Left.ResolvedType.(types.StructType)
		err := c.Compile(node.Left.Left) // compile collection ident
		if err != nil {
			return err
		}
		err = c.Compile(node.Value) // compile index
		if err != nil {
			return err
		}

		fieldIdent := node.Left.Value.(*ast.Identifier)
		idx := slices.Index(t.Fields, fieldIdent.Value)

		c.emit(code.OpSetField, idx)
	case *ast.ScopeAccessExpr:
		mangled := c.mangleModule(node.Module.Value, node.Member.Value)
		symbol, _, ok := c.symbolTable.Resolve(mangled)
		if !ok {
			return fmt.Errorf("undefined %s", mangled)
		}
		c.loadSymbol(symbol)
	case *ast.MatchExpr:
		err := c.Compile(node.Subject)
		if err != nil {
			return err
		}
		c.emit(code.OpResultTag)
		notTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)
		// ok arm
		err = c.Compile(node.Subject)
		if err != nil {
			return err
		}
		c.emit(code.OpResultValue)

		sym := c.symbolTable.DefineImmutable(node.OkArm.Pattern.Binding.Value)
		if sym.Scope == GlobalScope {
			c.emit(code.OpSetImmutableGlobal, sym.Index)
		} else {
			c.emit(code.OpSetImmutableLocal, sym.Index)
		}

		err = c.Compile(node.OkArm.Body)
		if err != nil {
			return err
		}
		// this makes sure the value at the end of the block is what is pushed into the expr result
		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}

		jumpPos := c.emit(code.OpJump, 9999)
		afterOkPos := len(c.currentInstructions())
		c.changeOperand(notTruthyPos, afterOkPos)

		err = c.Compile(node.Subject)
		if err != nil {
			return err
		}
		c.emit(code.OpResultValue)
		sym = c.symbolTable.DefineImmutable(node.ErrArm.Pattern.Binding.Value)
		if sym.Scope == GlobalScope {
			c.emit(code.OpSetImmutableGlobal, sym.Index)
		} else {
			c.emit(code.OpSetImmutableLocal, sym.Index)
		}

		err = c.Compile(node.ErrArm.Body)
		if err != nil {
			return err
		}
		// this makes sure the value at the end of the block is what is pushed into the expr result
		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}

		afterErrPos := len(c.currentInstructions())
		c.changeOperand(jumpPos, afterErrPos)
	case *ast.BreakStmt:
		loop := c.getLoop()
		pos := c.emit(code.OpJump, 9999)
		loop.breakPositions = append(loop.breakPositions, pos)
	case *ast.ContinueStmt:
		loop := c.getLoop()
		if loop.hasPost {
			pos := c.emit(code.OpJump, 9999)
			loop.continuePositions = append(loop.continuePositions, pos)
		} else {
			c.emit(code.OpJump, loop.conditionPos)
		}
	}

	if expr, ok := node.(ast.Expr); ok {
		if castTo := expr.GetCastTo(); castTo != nil {
			concreteName := getConcreteType(expr)

			itabKey := getItabKey(concreteName, castTo.Name)
			if itabIdx, ok := c.itabMapping[itabKey]; ok {
				c.emit(code.OpBox, itabIdx)
			} else {
				return fmt.Errorf("struct %s does not implement %s", concreteName, castTo.Name)
			}
		}
	}
	return nil
}

func (c *Compiler) CompilePackages(packages []*loader.Package) error {
	for _, pkg := range packages {
		c.currentModule = pkg.Name
		for _, program := range pkg.Programs {
			err := c.Compile(program)
			if err != nil {
				return err
			}
		}
		c.currentModule = ""
	}

	return nil
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
	}
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)

	return pos
}

func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.currentInstructions())
	updatedInstructions := append(c.currentInstructions(), ins...)

	c.scopes[c.scopeIndex].instructions = updatedInstructions

	return posNewInstruction
}

func (c *Compiler) setLastInstruction(op code.Opcode, pos int) {
	previous := c.scopes[c.scopeIndex].lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}

	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {

	if len(c.currentInstructions()) == 0 {
		return false
	}

	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

func (c *Compiler) removeLastPop() {
	last := c.scopes[c.scopeIndex].lastInstruction
	previous := c.scopes[c.scopeIndex].previousInstruction

	old := c.currentInstructions()
	n := old[:last.Position]

	c.scopes[c.scopeIndex].instructions = n
	c.scopes[c.scopeIndex].lastInstruction = previous
}

func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {

	ins := c.currentInstructions()

	for i := 0; i < len(newInstruction); i++ {
		ins[pos+i] = newInstruction[i]
	}
}

func (c *Compiler) changeOperand(opPos int, operand int) {
	op := code.Opcode(c.currentInstructions()[opPos])
	newInstruction := code.Make(op, operand)
	c.replaceInstruction(opPos, newInstruction)
}

func (c *Compiler) currentInstructions() code.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

func (c *Compiler) enterScope() {
	scope := &CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	c.scopes = append(c.scopes, scope)
	c.scopeIndex++

	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

func (c *Compiler) leaveScope() code.Instructions {
	instructions := c.currentInstructions()

	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--

	c.symbolTable = c.symbolTable.Outer

	return instructions
}

func (c *Compiler) enterLoop(conditionPos int, hasPost bool) {
	loop := &LoopContext{
		conditionPos:      conditionPos,
		hasPost:           hasPost,
		breakPositions:    make([]int, 0),
		continuePositions: make([]int, 0),
	}
	c.loopContexts = append(c.loopContexts, loop)
	c.loopIndex++
}

func (c *Compiler) leaveLoop() {
	c.loopIndex--
	c.loopContexts = c.loopContexts[:c.loopIndex]
}

func (c *Compiler) getLoop() *LoopContext {
	if len(c.loopContexts) == 0 {
		return nil
	}
	return c.loopContexts[c.loopIndex-1]
}

func (c *Compiler) replaceLastPopWithReturn() {
	lastPos := c.scopes[c.scopeIndex].lastInstruction.Position
	c.replaceInstruction(lastPos, code.Make(code.OpReturnValue))

	c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpReturnValue
}

func (c *Compiler) loadSymbol(s Symbol) {
	switch s.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobal, s.Index)
	case LocalScope:
		c.emit(code.OpGetLocal, s.Index)
	case BuiltinScope:
		c.emit(code.OpGetBuiltIn, s.Index)
	case FreeScope:
		c.emit(code.OpGetFree, s.Index)
	case FunctionScope:
		c.emit(code.OpCurrentClosure)
	}
}

func (c *Compiler) emitZeroValue(t types.Type) error {
	switch t {
	case types.Bool:
		c.emit(code.OpFalse)
	case types.Int:
		i := &object.Integer{Value: 0}
		c.emit(code.OpConstant, c.addConstant(i))
	case types.Float:
		f := &object.Float{Value: 0}
		c.emit(code.OpConstant, c.addConstant(f))
	case types.String:
		s := &object.String{Value: ""}
		c.emit(code.OpConstant, c.addConstant(s))
	case types.Byte:
		b := &object.Byte{Value: 0}
		c.emit(code.OpConstant, c.addConstant(b))
	default:
		c.emit(code.OpNull)
	}

	return nil
}

func (c *Compiler) lookUpStruct(name string) (types.StructType, bool) {
	t, ok := c.structTypes[name]

	return t, ok
}

func (c *Compiler) setStruct(name string, t types.StructType) {
	c.structTypes[name] = t
}

func (c *Compiler) compileInterfaceImplementation(impl *ast.InterfaceImplementationStmt) {
	sn := impl.StructName.Value
	for _, ident := range impl.InterfaceNames {
		in := ident.Value
		it, ok := c.fetchInterfaceType(in)
		if !ok {
			panic(fmt.Errorf("interface type %s not found", in))
		}

		itab := &object.Itab{
			InterfaceName:  in,
			ConcreteName:   sn,
			MethodsIndices: make([]int, len(it.Methods)),
		}

		for mn, idx := range it.MethodIndices {
			mangled := mangle(sn, mn)
			sym, _, ok := c.symbolTable.Resolve(mangled)
			if !ok {
				panic(fmt.Errorf("symbol %s not found", mangled))
			}
			itab.MethodsIndices[idx] = sym.Index
		}
		itabIdx := c.addConstant(itab)
		itabKey := getItabKey(sn, in)
		c.itabMapping[itabKey] = itabIdx
	}
}

func (c *Compiler) setInterface(name string, t types.InterfaceType) {
	if t.MethodIndices == nil {
		t.MethodIndices = make(map[string]int)
		for i, mn := range t.Methods {
			t.MethodIndices[mn] = i
		}
	}

	c.interfaceTypes[name] = t
}

func (c *Compiler) fetchInterfaceType(name string) (types.InterfaceType, bool) {
	it, ok := c.interfaceTypes[name]
	return it, ok
}

func mangle(sn string, mn string) string {
	return fmt.Sprintf("%s.%s", sn, mn)
}

func getItabKey(sn string, in string) ItabKey {
	asStr := fmt.Sprintf("%s:%s", sn, in)
	return ItabKey(asStr)
}

func getConcreteType(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.StructLiteral:
		return node.ResolvedType.Signature()
	case *ast.Identifier:
		return node.ResolvedType.Signature()
	case *ast.CallExpr:
		return node.ResolvedType.Signature()
	}

	return ""
}

func (c *Compiler) isInterfaceType(expr ast.Expr) (*types.InterfaceType, bool) {
	var t types.Type
	switch node := expr.(type) {
	case *ast.Identifier:
		t = node.ResolvedType
	case *ast.CallExpr:
		t = node.ResolvedType
	case *ast.SelectorExpr:
		t = node.ResolvedType
	case *ast.IndexExpr:
		t = node.ResolvedType
	}

	if t == nil {
		return nil, false
	}
	it, ok := t.(types.InterfaceType)
	return &it, ok
}

func (c *Compiler) compileInterfaceMethodCall(node *ast.CallExpr) error {
	s := node.Function.(*ast.SelectorExpr) // prereq to being in this fn
	if it, ok := c.isInterfaceType(s.Left); ok {
		authIt, exists := c.fetchInterfaceType(it.Name)
		if exists {
			it = &authIt
		}
		// compile interface object
		err := c.Compile(s.Left)
		if err != nil {
			return err
		}

		// push args onto stack
		for _, arg := range node.Arguments {
			err := c.Compile(arg)
			if err != nil {
				return err
			}
		}

		methodIdx, ok := it.MethodIndices[s.Value.String()]
		if !ok {
			return nil
		}

		c.emit(code.OpCallInterface, methodIdx, len(node.Arguments))
	} else if it := s.Left.GetCastTo(); it != nil {
		// compile interface object
		err := c.Compile(s.Left)
		if err != nil {
			return err
		}

		// push args onto stack
		for _, arg := range node.Arguments {
			err := c.Compile(arg)
			if err != nil {
				return err
			}
		}

		// get method from indices for dynamic dispatch
		methodName := s.Value.(*ast.Identifier).Value
		methodIdx := it.MethodIndices[methodName]

		c.emit(code.OpCallInterface, methodIdx, len(node.Arguments))
	}

	return nil
}

func (c *Compiler) mangleModule(module, name string) string {
	return module + "__" + name
}
