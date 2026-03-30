package compiler

import (
	"fmt"
	"slices"
	"sort"

	"sydney/ast"
	"sydney/code"
	"sydney/loader"
	"sydney/object"
	"sydney/token"
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
	fileName      string

	loopContexts []*LoopContext
	loopIndex    int

	shouldEmitDebug bool
}

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
	SourceMap    *code.SourceMap
	DebugSymbols *code.DebugSymbols
}

type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

type CompilationScope struct {
	instructions        code.Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
	sourceMap           *code.SourceMap
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
		sourceMap:           code.New(),
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

func (c *Compiler) ShouldEmitDebug(flag bool) {
	c.shouldEmitDebug = flag
}

func (c *Compiler) SetFileName(fn string) {
	c.fileName = fn
}

func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts { // set interface types, struct types, and hoist functions
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

			if def, ok := stmt.(*ast.StructDefinitionStmt); ok {
				name := def.Name.Value
				if c.currentModule != "" {
					name = c.mangleModule(c.currentModule, name)
				}
				c.setStruct(name, def.Type)
			}

			if fn, ok := stmt.(*ast.FunctionDeclarationStmt); ok && !fn.IsExtern {
				name := fn.Name.Value
				if fn.MangledName != "" {
					name = fn.MangledName
				}
				if c.currentModule != "" {
					name = c.mangleModule(c.currentModule, name)
				}
				sym := c.symbolTable.DefineImmutable(name)
				c.symbolTable.AnnotateType(name, fn.Type)
				if fn.MangledName != "" {
					c.symbolTable.DefineAlias(fn.Name.Value, sym)
					c.symbolTable.DefineAlias(fn.MangledName, sym)
					if c.currentModule != "" {
						c.symbolTable.DefineAlias(c.mangleModule(c.currentModule, fn.Name.Value), sym)
					}
				}

			}
		}

		c.buildItabsFromTypes()

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
				c.symbolTable.AnnotateType(name, fn.Type)
				if fn.MangledName != "" {
					c.symbolTable.DefineAlias(fn.Name.Value, sym)
				}
			}
		}

		c.pushBlockScope()
		for _, s := range node.Stmts {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}
		c.popBlockScope()
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
				err := c.emitZeroValue(node, node.Type)
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
			c.symbolTable.AnnotateType(name, node.Type)
			cde := code.OpSetImmutableLocal

			if symbol.Scope == GlobalScope {
				cde = code.OpSetImmutableGlobal
			}

			c.emitAt(node, cde, symbol.Index)
		} else {
			symbol := c.symbolTable.DefineMutable(name)
			c.symbolTable.AnnotateType(name, node.Type)
			cde := code.OpSetMutableLocal
			if symbol.Scope == GlobalScope {
				cde = code.OpSetMutableGlobal
			}
			c.emitAt(node, cde, symbol.Index)

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
			c.emitAt(node, code.OpSetMutableGlobal, symbol.Index)
		} else {
			c.emitAt(node, code.OpSetMutableLocal, symbol.Index)
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

		c.emitAt(node, code.OpIndexSet)
	case *ast.ReturnStmt:
		err := c.Compile(node.ReturnValue)
		if err != nil {
			return err
		}
		c.emitAt(node, code.OpReturnValue)
	case *ast.ForStmt:
		c.pushBlockScope()
		if node.Init != nil {
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
		jumpNotTruthyPos := c.emitAt(node, code.OpJumpNotTruthy, 9999)

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
		c.popBlockScope()

		c.emit(code.OpNull)
		c.emit(code.OpPop) // this clears the condition value from the stack
	case *ast.ForInStmt:
		err := c.compileForInStmt(node)
		if err != nil {
			return err
		}
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
				c.emitAt(node, code.OpGt)
			} else {
				c.emitAt(node, code.OpGte)
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
			c.emitAt(node, code.OpAdd)
		case "-":
			c.emitAt(node, code.OpSub)
		case "*":
			c.emitAt(node, code.OpMul)
		case "/":
			c.emitAt(node, code.OpDiv)
		case "==":
			c.emitAt(node, code.OpEqual)
		case "!=":
			c.emitAt(node, code.OpNotEqual)
		case ">":
			c.emitAt(node, code.OpGt)
		case ">=":
			c.emitAt(node, code.OpGte)
		case "&&":
			c.emitAt(node, code.OpAnd)
		case "||":
			c.emitAt(node, code.OpOr)
		case "%":
			c.emitAt(node, code.OpModulo)
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
			c.emitAt(node, code.OpBang)
		case "-":
			c.emitAt(node, code.OpMinus)
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

		if node.GetResolvedType() == types.Unit {
			c.emit(code.OpNull)
		} else if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}

		// emit an OpJump with operand to be replaced later
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

			if node.GetResolvedType() == types.Unit {
				c.emit(code.OpNull)
			} else if c.lastInstructionIs(code.OpPop) {
				c.removeLastPop()
			}
		}

		afterAlternativePos := len(c.currentInstructions())
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

		c.emitAt(node, code.OpIndex)
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

		c.emitAt(node, code.OpCall, len(node.Arguments))
	case *ast.IntegerLiteral:
		integer := &object.Integer{Value: node.Value}
		c.emitAt(node, code.OpConstant, c.addConstant(integer))
	case *ast.ByteLiteral:
		byt := &object.Byte{Value: node.Value}
		c.emitAt(node, code.OpConstant, c.addConstant(byt))
	case *ast.FloatLiteral:
		float := &object.Float{Value: node.Value}
		c.emitAt(node, code.OpConstant, c.addConstant(float))
	case *ast.BooleanLiteral:
		if node.Value {
			c.emitAt(node, code.OpTrue)
		} else {
			c.emitAt(node, code.OpFalse)
		}
	case *ast.NullLiteral:
		c.emitAt(node, code.OpNull)
	case *ast.StringLiteral:
		str := &object.String{Value: node.Value}
		c.emitAt(node, code.OpConstant, c.addConstant(str))
	case *ast.ArrayLiteral:
		for _, el := range node.Elements {
			err := c.Compile(el)
			if err != nil {
				return err
			}
		}
		c.emitAt(node, code.OpArray, len(node.Elements))
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

		c.emitAt(node, code.OpHash, len(node.Pairs)*2)
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
			c.symbolTable.DefineMutable(p.Value)
			c.symbolTable.AnnotateType(p.Value, node.Type)
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
		instructions, fnSourceMap := c.leaveScope()

		// iterate over free symbols and load them onto stack
		for _, s := range freeSymbols {
			c.loadSymbol(s)
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.Params),
			SourceMap:     fnSourceMap,
		}

		if c.shouldEmitDebug {
			symbols := make([]*code.DebugSymbol, len(c.symbolTable.store))
			for n, sym := range c.symbolTable.store {
				if sym.Scope == BuiltinScope {
					continue
				}
				dbg := &code.DebugSymbol{Name: n, Scope: string(sym.Scope)}
				if sym.Type != nil {
					dbg.Type = (*sym.Type).Signature()
				}
				symbols[sym.Index] = dbg
			}
			compiledFn.DebugSymbols = &code.DebugSymbols{
				Locals: symbols,
			}
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
			c.symbolTable.AnnotateType(p.Value, p.GetResolvedType())
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
		instructions, fnSourceMap := c.leaveScope()

		// iterate over free symbols and load them onto stack
		for _, s := range freeSymbols {
			c.loadSymbol(s)
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.Parameters),
			SourceMap:     fnSourceMap,
		}

		if c.shouldEmitDebug {
			symbols := make([]*code.DebugSymbol, len(c.symbolTable.store))
			for n, sym := range c.symbolTable.store {
				if sym.Scope == BuiltinScope {
					continue
				}
				dbg := &code.DebugSymbol{Name: n, Scope: string(sym.Scope)}
				if sym.Type != nil {
					dbg.Type = (*sym.Type).Signature()
				}
				symbols[sym.Index] = dbg
			}
			compiledFn.DebugSymbols = &code.DebugSymbols{
				Locals: symbols,
			}
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
		t := node.ContainerType.(types.StructType)
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		fieldIdent := node.Value.(*ast.Identifier)
		// resolved type is appended in typechecker
		idx := slices.Index(t.Fields, fieldIdent.Value)

		c.emit(code.OpGetField, idx)
	case *ast.SelectorAssignmentStmt:
		t := node.Left.ContainerType.(types.StructType)
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
			// Fall back to bare name for extern builtins
			symbol, _, ok = c.symbolTable.Resolve(node.Member.Value)
		}
		if !ok {
			return fmt.Errorf("undefined %s", mangled)
		}
		c.loadSymbol(symbol)
	case *ast.MatchExpr:
		if node.SomeArm != nil {
			err := c.compileOptionMatch(node)
			if err != nil {
				return err
			}
		} else {
			err := c.compileResultMatch(node)
			if err != nil {
				return err
			}
		}
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
	case *ast.SliceExpr:
		if node.Start != nil {
			err := c.Compile(node.Start)
			if err != nil {
				return err
			}
		} else {
			err := c.emitZeroValue(node, types.Int)
			if err != nil {
				return err
			}
		}

		if node.End != nil {
			err := c.Compile(node.End)
			if err != nil {
				return err
			}
		} else {
			fake := &ast.IntegerLiteral{
				Token: token.Token{
					Type:    token.Integer,
					Literal: "-1",
					Line:    -1,
					Column:  -1,
				},
				Value: -1,
			}
			err := c.Compile(fake)
			if err != nil {
				return err
			}
		}
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}
		c.emit(code.OpSlice)
	case *ast.SpawnStmt:
		callExpr := node.CallExpr.(*ast.CallExpr)
		err := c.Compile(callExpr.Function)
		if err != nil {
			return err
		}
		for _, arg := range callExpr.Arguments {
			err = c.Compile(arg)
			if err != nil {
				return err
			}
		}
		c.emit(code.OpSpawn, len(callExpr.Arguments))
	case *ast.ChannelConstructorExpr:
		if node.Capacity != nil {
			err := c.Compile(node.Capacity)
			if err != nil {
				return err
			}
		} else {
			c.emit(code.OpConstant, c.addConstant(&object.Integer{Value: 0}))
		}
		c.emit(code.OpMakeChannel)
	case *ast.SendStmt:
		err := c.Compile(node.Chan)
		if err != nil {
			return err
		}
		err = c.Compile(node.Value)
		if err != nil {
			return err
		}
		c.emit(code.OpSend)
	case *ast.ReceiveExpr:
		err := c.Compile(node.Chan)
		if err != nil {
			return err
		}
		c.emit(code.OpReceive)
	case *ast.MatchTypeExpr:
		err := c.compileTypeMatch(node)
		if err != nil {
			return err
		}
	}

	if expr, ok := node.(ast.Expr); ok {
		if castTo := expr.GetCastTo(); castTo != nil {
			concreteName := getConcreteType(expr)

			ifaceName := castTo.Name
			if castTo.Module != "" {
				ifaceName = c.mangleModule(castTo.Module, ifaceName)
			}
			itabKey := getItabKey(concreteName, ifaceName)
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
		merged := &ast.Program{}
		for _, program := range pkg.Programs {
			merged.Stmts = append(merged.Stmts, program.Stmts...)
		}
		c.SetFileName(pkg.Name)
		err := c.Compile(merged)
		if err != nil {
			return err
		}
		c.currentModule = ""
	}

	return nil
}

func (c *Compiler) Bytecode() *Bytecode {
	count := 0
	for _, sym := range c.symbolTable.store {
		if sym.Scope != BuiltinScope {
			count++
		}
	}
	var dbgs *code.DebugSymbols
	if c.shouldEmitDebug {
		symbols := make([]*code.DebugSymbol, count)
		for n, sym := range c.symbolTable.store {
			if sym.Scope == BuiltinScope {
				continue
			}
			dbg := &code.DebugSymbol{Name: n, Scope: string(sym.Scope)}
			if sym.Type != nil {
				dbg.Type = (*sym.Type).Signature()
			}
			symbols[sym.Index] = dbg
		}
		dbgs = &code.DebugSymbols{Locals: symbols}
	}

	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
		SourceMap:    c.scopes[c.scopeIndex].sourceMap,
		DebugSymbols: dbgs,
	}
}

func (c *Compiler) emitAt(node ast.Node, op code.Opcode, operands ...int) int {
	ins := c.emit(op, operands...)

	line, col := node.Pos()

	fileName := c.fileName
	if c.currentModule != "" {
		fileName = c.currentModule
	}

	c.scopes[c.scopeIndex].sourceMap.Mappings[ins] = &code.SourceMapping{
		InstructionOffset: ins,
		Line:              line,
		Col:               col,
		File:              fileName,
	}

	return ins
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
		sourceMap:           code.New(),
	}

	c.scopes = append(c.scopes, scope)
	c.scopeIndex++

	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

func (c *Compiler) leaveScope() (code.Instructions, *code.SourceMap) {
	instructions := c.currentInstructions()
	sourceMap := c.scopes[c.scopeIndex].sourceMap

	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--

	c.symbolTable = c.symbolTable.Outer

	return instructions, sourceMap
}

func (c *Compiler) compileResultMatch(node *ast.MatchExpr) error {
	err := c.Compile(node.Subject)
	if err != nil {
		return err
	}
	c.emit(code.OpResultTag)
	notTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

	// ok arm
	c.enterBlockScope()
	err = c.Compile(node.Subject)
	if err != nil {
		return err
	}
	c.emit(code.OpResultValue)

	sym := c.symbolTable.DefineImmutable(node.OkArm.Pattern.Binding.Value)
	c.symbolTable.AnnotateType(node.OkArm.Pattern.Binding.Value, node.GetResolvedType())
	if sym.Scope == GlobalScope {
		c.emit(code.OpSetImmutableGlobal, sym.Index)
	} else {
		c.emit(code.OpSetImmutableLocal, sym.Index)
	}

	err = c.Compile(node.OkArm.Body)
	if err != nil {
		return err
	}
	if c.lastInstructionIs(code.OpPop) {
		c.removeLastPop()
	}
	if node.GetResolvedType() == types.Unit {
		c.emit(code.OpNull)
	}
	c.leaveBlockScope()

	jumpPos := c.emit(code.OpJump, 9999)
	afterOkPos := len(c.currentInstructions())
	c.changeOperand(notTruthyPos, afterOkPos)

	// err arm
	c.enterBlockScope()
	err = c.Compile(node.Subject)
	if err != nil {
		return err
	}
	c.emit(code.OpResultValue)
	sym = c.symbolTable.DefineImmutable(node.ErrArm.Pattern.Binding.Value)
	c.symbolTable.AnnotateType(node.ErrArm.Pattern.Binding.Value, node.GetResolvedType())
	if sym.Scope == GlobalScope {
		c.emit(code.OpSetImmutableGlobal, sym.Index)
	} else {
		c.emit(code.OpSetImmutableLocal, sym.Index)
	}

	err = c.Compile(node.ErrArm.Body)
	if err != nil {
		return err
	}
	if c.lastInstructionIs(code.OpPop) {
		c.removeLastPop()
	}
	if node.GetResolvedType() == types.Unit {
		c.emit(code.OpNull)
	}
	c.leaveBlockScope()

	afterErrPos := len(c.currentInstructions())
	c.changeOperand(jumpPos, afterErrPos)
	return nil
}

func (c *Compiler) compileOptionMatch(node *ast.MatchExpr) error {
	err := c.Compile(node.Subject)
	if err != nil {
		return err
	}
	c.emit(code.OpResultTag)
	notTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

	// some arm
	c.enterBlockScope()
	err = c.Compile(node.Subject)
	if err != nil {
		return err
	}
	c.emit(code.OpResultValue)

	sym := c.symbolTable.DefineImmutable(node.SomeArm.Pattern.Binding.Value)
	c.symbolTable.AnnotateType(node.SomeArm.Pattern.Binding.Value, node.GetResolvedType())
	if sym.Scope == GlobalScope {
		c.emit(code.OpSetImmutableGlobal, sym.Index)
	} else {
		c.emit(code.OpSetImmutableLocal, sym.Index)
	}

	err = c.Compile(node.SomeArm.Body)
	if err != nil {
		return err
	}
	if c.lastInstructionIs(code.OpPop) {
		c.removeLastPop()
	}
	if node.GetResolvedType() == types.Unit {
		c.emit(code.OpNull)
	}
	c.leaveBlockScope()

	jumpPos := c.emit(code.OpJump, 9999)
	afterSomePos := len(c.currentInstructions())
	c.changeOperand(notTruthyPos, afterSomePos)

	// none arm — no binding
	err = c.Compile(node.NoneArm.Body)
	if err != nil {
		return err
	}
	if c.lastInstructionIs(code.OpPop) {
		c.removeLastPop()
	}
	if node.GetResolvedType() == types.Unit {
		c.emit(code.OpNull)
	}

	afterNonePos := len(c.currentInstructions())
	c.changeOperand(jumpPos, afterNonePos)
	return nil
}

func (c *Compiler) compileTypeMatch(expr *ast.MatchTypeExpr) error {
	prevJmp := -1
	jmpEndPos := make([]int, 0)
	for _, arm := range expr.Arms {
		armStart := len(c.currentInstructions())
		err := c.Compile(expr.Subject)
		if err != nil {
			return err
		}
		var typeName string
		if st, ok := arm.Type.(types.StructType); ok {
			typeName = st.Name
		} else {
			typeName = arm.Type.Signature()
		}
		nameIdx := c.addConstant(&object.String{Value: typeName})
		c.emit(code.OpMatchType, nameIdx)
		insPtr := c.emit(code.OpJumpNotTruthy, 9999)
		if prevJmp != -1 {
			c.changeOperand(prevJmp, armStart)
		}
		prevJmp = insPtr

		err = c.Compile(expr.Subject)
		if err != nil {
			return err
		}
		c.emit(code.OpUnboxInterface)
		c.enterBlockScope()
		sym := c.symbolTable.DefineImmutable(arm.Binding.Value)
		c.symbolTable.AnnotateType(arm.Binding.Value, arm.Type)
		if sym.Scope == GlobalScope {
			c.emit(code.OpSetImmutableGlobal, sym.Index)
		} else {
			c.emit(code.OpSetImmutableLocal, sym.Index)
		}
		err = c.Compile(arm.Body)
		if err != nil {
			return err
		}
		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}
		if expr.GetResolvedType() == types.Unit {
			c.emit(code.OpNull)
		}
		jmpPos := c.emit(code.OpJump, 9999)
		jmpEndPos = append(jmpEndPos, jmpPos)
		c.leaveBlockScope()
	}

	if prevJmp != -1 {
		c.changeOperand(prevJmp, len(c.currentInstructions()))
	}

	if expr.Default != nil {
		err := c.Compile(expr.Default)
		if err != nil {
			return err
		}
		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}
		if expr.GetResolvedType() == types.Unit {
			c.emit(code.OpNull)
		}
	}

	jmpPos := len(c.currentInstructions())
	for _, jmp := range jmpEndPos {
		c.changeOperand(jmp, jmpPos)
	}

	return nil
}

func (c *Compiler) enterBlockScope() {
	c.symbolTable = NewBlockScopedSymbolTable(c.symbolTable)
}

func (c *Compiler) leaveBlockScope() {
	c.symbolTable = c.symbolTable.Outer
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

func (c *Compiler) emitZeroValue(node ast.Node, t types.Type) error {
	switch t {
	case types.Bool:
		c.emitAt(node, code.OpFalse)
	case types.Int:
		i := &object.Integer{Value: 0}
		c.emitAt(node, code.OpConstant, c.addConstant(i))
	case types.Float:
		f := &object.Float{Value: 0}
		c.emitAt(node, code.OpConstant, c.addConstant(f))
	case types.String:
		s := &object.String{Value: ""}
		c.emitAt(node, code.OpConstant, c.addConstant(s))
	case types.Byte:
		b := &object.Byte{Value: 0}
		c.emitAt(node, code.OpConstant, c.addConstant(b))
	default:
		c.emitAt(node, code.OpNull)
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

func (c *Compiler) buildItabsFromTypes() {
	// sort for tests
	structNames := make([]string, 0, len(c.structTypes))
	for sn := range c.structTypes {
		structNames = append(structNames, sn)
	}
	sort.Strings(structNames)

	for _, sn := range structNames {
		st := c.structTypes[sn]
		for _, ifaceRaw := range st.Interfaces {
			iface, ok := ifaceRaw.(types.InterfaceType)
			if !ok {
				continue
			}

			in := iface.Name
			it, ok := c.fetchInterfaceType(in)
			if !ok && c.currentModule != "" {
				it, ok = c.fetchInterfaceType(c.mangleModule(c.currentModule, in))
			}
			if !ok {
				continue
			}

			itab := &object.Itab{
				InterfaceName:  in,
				ConcreteName:   sn,
				MethodsIndices: make([]int, len(it.Methods)),
			}

			for mn, idx := range it.MethodIndices {
				mangled := mangle(sn, mn)
				sym, _, ok := c.symbolTable.Resolve(mangled)
				if !ok && c.currentModule != "" {
					sym, _, ok = c.symbolTable.Resolve(c.mangleModule(c.currentModule, mangled))
				}
				if !ok {
					continue
				}
				itab.MethodsIndices[idx] = sym.Index
			}
			itabIdx := c.addConstant(itab)
			itabKey := getItabKey(sn, in)
			c.itabMapping[itabKey] = itabIdx
			if c.currentModule != "" {
				mangledKey := getItabKey(sn, c.mangleModule(c.currentModule, in))
				c.itabMapping[mangledKey] = itabIdx
			}
		}
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
		fetchName := it.Name
		if it.Module != "" {
			fetchName = c.mangleModule(it.Module, fetchName)
		}
		authIt, exists := c.fetchInterfaceType(fetchName)
		if exists {
			it = &authIt
		}
		// push args onto stack
		for _, arg := range node.Arguments {
			err := c.Compile(arg)
			if err != nil {
				return err
			}
		}

		// compile interface object (must be on top of stack)
		err := c.Compile(s.Left)
		if err != nil {
			return err
		}

		methodIdx, ok := it.MethodIndices[s.Value.String()]
		if !ok {
			return nil
		}

		c.emit(code.OpCallInterface, methodIdx, len(node.Arguments))
	} else if it := s.Left.GetCastTo(); it != nil {
		// push args onto stack
		for _, arg := range node.Arguments {
			err := c.Compile(arg)
			if err != nil {
				return err
			}
		}

		// compile interface object (must be on top of stack)
		err := c.Compile(s.Left)
		if err != nil {
			return err
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

func (c *Compiler) compileForInStmt(node *ast.ForInStmt) error {
	_, mok := node.Iterable.GetResolvedType().(types.MapType)
	_, aok := node.Iterable.GetResolvedType().(types.ArrayType)
	if mok {
		return c.compileForInStmtMap(node)
	}

	if aok {
		return c.compileForInStmtArr(node)
	}

	return nil
}

func (c *Compiler) getLoopHiddenVar(str string) string {
	return fmt.Sprintf("__%s__%d__", str, c.loopIndex)
}

func (c *Compiler) compileForInStmtArr(node *ast.ForInStmt) error {
	iter := c.getLoopHiddenVar("forin_iter")
	leng := c.getLoopHiddenVar("forin_len")
	idx := c.getLoopHiddenVar("forin_idx")

	c.pushBlockScope()
	err := c.Compile(node.Iterable)
	if err != nil {
		return err
	}
	// store iterable in hidden var
	iterSym := c.symbolTable.DefineMutable(iter)
	c.symbolTable.AnnotateType(iter, node.Iterable.GetResolvedType())
	c.emitSet(iterSym)

	// insert len compilation for array case
	c.emit(code.OpGetBuiltIn, 0)
	c.emitGet(iterSym)
	c.emit(code.OpCall, 1)
	lenSym := c.symbolTable.DefineMutable(leng)
	c.symbolTable.AnnotateType(leng, types.Int)
	c.emitSet(lenSym)

	// Init index = 0
	c.emit(code.OpConstant, c.addConstant(&object.Integer{Value: 0}))
	idxSym := c.symbolTable.DefineMutable(idx)
	c.symbolTable.AnnotateType(idx, types.Int)
	c.emitSet(idxSym)

	conditionPos := len(c.currentInstructions())
	c.enterLoop(conditionPos, true)
	c.emitGet(lenSym)
	c.emitGet(idxSym)
	c.emit(code.OpGt)
	jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

	// Bind loop variables
	if node.Key != nil {
		c.emitGet(idxSym)
		keySym := c.symbolTable.DefineMutable(node.Key.Value)
		c.symbolTable.AnnotateType(node.Key.Value, types.Int)
		c.emitSet(keySym)
	}
	c.emitGet(iterSym)
	c.emitGet(idxSym)
	c.emit(code.OpIndex)
	valSym := c.symbolTable.DefineMutable(node.Value.Value)
	c.symbolTable.AnnotateType(node.Value.Value, node.Value.GetResolvedType())
	c.emitSet(valSym)

	// Compile body
	err = c.Compile(node.Body)
	if err != nil {
		return err
	}

	postPos := len(c.currentInstructions())
	loop := c.getLoop()
	if loop != nil {
		for _, pos := range loop.continuePositions {
			c.changeOperand(pos, postPos)
		}
	}
	c.emitGet(idxSym)
	c.emit(code.OpConstant, c.addConstant(&object.Integer{Value: 1}))
	c.emit(code.OpAdd)
	c.emitSet(idxSym)

	// 8. Jump back to condition
	c.emit(code.OpJump, conditionPos)

	// 9. Escape
	escapePos := len(c.currentInstructions())
	c.changeOperand(jumpNotTruthyPos, escapePos)
	loop = c.getLoop()
	if loop != nil {
		for _, pos := range loop.breakPositions {
			c.changeOperand(pos, escapePos)
		}
	}
	c.leaveLoop()
	c.popBlockScope()

	c.emit(code.OpNull)
	c.emit(code.OpPop)

	return nil
}

func (c *Compiler) compileForInStmtMap(node *ast.ForInStmt) error {
	c.pushBlockScope()
	iter := c.getLoopHiddenVar("forin_iter")
	leng := c.getLoopHiddenVar("forin_len")
	idx := c.getLoopHiddenVar("forin_idx")
	keys := c.getLoopHiddenVar("forin_keys")

	err := c.Compile(node.Iterable)
	if err != nil {
		return err
	}
	iterSym := c.symbolTable.DefineMutable(iter)
	c.symbolTable.AnnotateType(iter, node.Iterable.GetResolvedType())
	c.emitSet(iterSym)

	c.emit(code.OpGetBuiltIn, 3) // keys builtin
	c.emitGet(iterSym)
	c.emit(code.OpCall, 1)
	keysSym := c.symbolTable.DefineMutable(keys)
	c.symbolTable.AnnotateType(keys, types.ArrayType{ElemType: node.Key.GetResolvedType()})
	c.emitSet(keysSym)

	// compute len(keys)
	c.emit(code.OpGetBuiltIn, 0) // len builtin
	c.emitGet(keysSym)
	c.emit(code.OpCall, 1)
	lenSym := c.symbolTable.DefineMutable(leng)
	c.symbolTable.AnnotateType(leng, types.Int)
	c.emitSet(lenSym)

	// idx = 0
	c.emit(code.OpConstant, c.addConstant(&object.Integer{Value: 0}))
	idxSym := c.symbolTable.DefineMutable(idx)
	c.symbolTable.AnnotateType(idx, types.Int)
	c.emitSet(idxSym)

	// condition: len > idx
	conditionPos := len(c.currentInstructions())
	c.enterLoop(conditionPos, true)
	c.emitGet(lenSym)
	c.emitGet(idxSym)
	c.emit(code.OpGt)
	jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

	// k = keys[idx]
	c.emitGet(keysSym)
	c.emitGet(idxSym)
	c.emit(code.OpIndex)
	keySym := c.symbolTable.DefineMutable(node.Key.Value)
	c.symbolTable.AnnotateType(node.Key.Value, node.Key.GetResolvedType())
	c.emitSet(keySym)

	// v = map[k] (unwrap option — key is guaranteed to exist)
	c.emitGet(iterSym)
	c.emitGet(keySym)
	c.emit(code.OpIndex)
	c.emit(code.OpResultValue)
	valSym := c.symbolTable.DefineMutable(node.Value.Value)
	c.symbolTable.AnnotateType(node.Value.Value, node.Value.GetResolvedType())
	c.emitSet(valSym)

	// Compile body
	err = c.Compile(node.Body)
	if err != nil {
		return err
	}

	postPos := len(c.currentInstructions())
	loop := c.getLoop()
	if loop != nil {
		for _, pos := range loop.continuePositions {
			c.changeOperand(pos, postPos)
		}
	}
	c.emitGet(idxSym)
	c.emit(code.OpConstant, c.addConstant(&object.Integer{Value: 1}))
	c.emit(code.OpAdd)
	c.emitSet(idxSym)

	// Jump back to condition
	c.emit(code.OpJump, conditionPos)

	// Escape
	escapePos := len(c.currentInstructions())
	c.changeOperand(jumpNotTruthyPos, escapePos)
	loop = c.getLoop()
	if loop != nil {
		for _, pos := range loop.breakPositions {
			c.changeOperand(pos, escapePos)
		}
	}
	c.leaveLoop()
	c.popBlockScope()

	c.emit(code.OpNull)
	c.emit(code.OpPop)

	return nil
}

func (c *Compiler) emitSet(sym Symbol) {
	if sym.Scope == GlobalScope {
		c.emit(code.OpSetMutableGlobal, sym.Index)
	} else {
		c.emit(code.OpSetMutableLocal, sym.Index)
	}
}

func (c *Compiler) emitGet(sym Symbol) {
	if sym.Scope == GlobalScope {
		c.emit(code.OpGetGlobal, sym.Index)
	} else {
		c.emit(code.OpGetLocal, sym.Index)
	}
}

func (c *Compiler) pushBlockScope() {
	c.symbolTable = NewBlockScopedSymbolTable(c.symbolTable)
}

func (c *Compiler) popBlockScope() {
	c.symbolTable = c.symbolTable.Outer
}
