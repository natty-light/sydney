package irgen

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sydney/ast"
	"sydney/types"
)

type irLocal struct {
	alloca string // e.g. "%x.addr" or "@g0"
	typ    IrType
}

type funcSig struct {
	retType    IrType   // IR return type string
	paramTypes []string // IR param type strings
	name       string   // IR function name (e.g. "@add", "@Circle.area")
}

type Emitter struct {
	buf    bytes.Buffer
	tmpIdx int
	lblIdx int
	locals map[string]irLocal
	inFunc bool
	depth  int

	// Collected metadata
	structTypes    map[string]types.StructType
	interfaceTypes map[string]types.InterfaceType
	vtables        map[string]map[string][]string // vtables[struct][iface] → @vtable global
	stringConsts   map[string]int                 // string value → index (@.str.0, ...)
	stringIdx      int
	topLevelFuncs  map[string]bool

	globals    map[string]irLocal   // global variable name → { allocaName, irType }
	funcSigs   map[string]funcSig   // function name → { retType, paramTypes }
	scopeStack []map[string]irLocal // stack of local scopes for nested functions
}

func New() *Emitter {
	return &Emitter{
		structTypes:    make(map[string]types.StructType),
		interfaceTypes: make(map[string]types.InterfaceType),
		stringConsts:   make(map[string]int),
		stringIdx:      0,
		vtables:        make(map[string]map[string][]string),
		locals:         make(map[string]irLocal),
		topLevelFuncs:  make(map[string]bool),
		globals:        make(map[string]irLocal),
		funcSigs:       make(map[string]funcSig),
		scopeStack:     make([]map[string]irLocal, 0),

		buf: bytes.Buffer{},
	}
}

func (e *Emitter) Emit(n ast.Node) error {
	program := e.collect(n)
	e.preamble()
	e.functions(program)

	err := e.mainWrapper(n)
	if err != nil {
		return err
	}

	return nil
}

func (e *Emitter) Write(name string) {
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	_, err = e.buf.WriteTo(f)
	if err != nil {
		panic(err)
	}
	err = f.Close()
	if err != nil {
		panic(err)
	}
}

func (e *Emitter) collect(n ast.Node) *ast.Program {
	switch node := n.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			e.collect(stmt)
		}
		return node
	case *ast.StructDefinitionStmt:
		e.structTypes[node.Name.Value] = node.Type
	case *ast.InterfaceDefinitionStmt:
		t := node.Type
		if t.MethodIndices == nil {
			t.MethodIndices = make(map[string]int)
			for i, mn := range t.Methods {
				t.MethodIndices[mn] = i
			}
		}

		e.interfaceTypes[node.Name.Value] = node.Type
	case *ast.InterfaceImplementationStmt:
		sname := node.StructName.Value
		if e.vtables[sname] == nil {
			e.vtables[sname] = make(map[string][]string)
		}
		for _, iname := range node.InterfaceNames {
			e.vtables[sname][iname.Value] = nil
		}
	case *ast.FunctionDeclarationStmt:
		e.topLevelFuncs[node.Name.Value] = true
	}

	e.collectStrings(n)

	for sname, ifaces := range e.vtables {
		for iname := range ifaces {
			iface := e.interfaceTypes[iname]
			methods := make([]string, len(iface.Methods))
			for i, method := range iface.Methods {
				methods[i] = method
				e.interfaceTypes[iname].MethodIndices[method] = i
			}
			e.vtables[sname][iname] = methods
		}
	}
	return nil
}

func (e *Emitter) collectStrings(n ast.Node) {
	switch node := n.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			e.collectStrings(stmt)
		}
	case *ast.ExpressionStmt:
		e.collectStrings(node.Expr)
	case *ast.BlockStmt:
		for _, stmt := range node.Stmts {
			e.collectStrings(stmt)
		}
	case *ast.VarDeclarationStmt:
		e.collectStrings(node.Value)
	case *ast.VarAssignmentStmt:
		e.collectStrings(node.Value)
	case *ast.IndexAssignmentStmt:
		e.collectStrings(node.Value)
	case *ast.ReturnStmt:
		e.collectStrings(node.ReturnValue)
	case *ast.ForStmt:
		e.collectStrings(node.Condition)
		e.collectStrings(node.Body)
	case *ast.FunctionDeclarationStmt:
		e.collectStrings(node.Body)
	case *ast.SelectorAssignmentStmt:
		e.collectStrings(node.Value)

	case *ast.InfixExpr:
		e.collectStrings(node.Left)
		e.collectStrings(node.Right)
	case *ast.PrefixExpr:
		e.collectStrings(node.Right)
	case *ast.IfExpr:
		e.collectStrings(node.Condition)
		e.collectStrings(node.Consequence)
		if node.Alternative != nil {
			e.collectStrings(node.Alternative)
		}
	case *ast.IndexExpr:
		e.collectStrings(node.Index)

	case *ast.CallExpr:
		for _, arg := range node.Arguments {
			e.collectStrings(arg)
		}

	case *ast.StringLiteral:
		e.addStr(node.Value)
	case *ast.ArrayLiteral:
		for _, elem := range node.Elements {
			e.collectStrings(elem)

		}
	case *ast.HashLiteral:
		for key, value := range node.Pairs {
			e.collectStrings(key)
			e.collectStrings(value)
		}
	case *ast.FunctionLiteral:
		e.collectStrings(node.Body)
	case *ast.StructLiteral:
		for _, val := range node.Values {
			e.collectStrings(val)
		}
	}
}

func (e *Emitter) preamble() {
	e.emit("declare void @sydney_print_int(i64)")
	e.emit("declare void @sydney_print_float(double)")
	e.emit("declare void @sydney_print_string(ptr)")
	e.emit("declare ptr @sydney_strcat(ptr, ptr)")
	e.emit("declare void @sydney_print_newline()")
	e.emit("declare void @sydney_gc_init()")
	e.emit("declare void @sydney_print_bool(i8)")
	e.emit("declare ptr @sydney_gc_alloc(i64)")
	e.emit("declare void @sydney_gc_collect()")
	e.emit("declare void @sydney_gc_add_global_root(ptr)")
	e.emit("declare void @sydney_gc_shutdown()")
	e.emit("declare i64 @sydney_strlen(ptr)")
	e.emit("")

	for _, v := range e.structTypes {
		e.emitStructType(v)
	}

	for s, i := range e.stringConsts {
		e.emitStringConst(s, i)
	}

	for sname, imap := range e.vtables {
		for iname, methods := range imap {
			e.emitInterfaceType(sname, iname, methods)
		}
	}
}

func (e *Emitter) functions(node *ast.Program) {
	for _, stmt := range node.Stmts {
		if fdecl, ok := stmt.(*ast.FunctionDeclarationStmt); ok {
			e.emitFunction(fdecl)
		}
	}
}

func (e *Emitter) mainWrapper(node ast.Node) error {
	e.emit("define i32 @main() {")
	e.emit("entry: ")
	e.depth++
	e.emit("call void @sydney_gc_init()")

	e.main(node)
	e.emit("ret i32 0")
	e.depth--
	e.emit("}")

	return nil
}

func (e *Emitter) main(node ast.Node) (string, IrType) {
	var val string
	var valType IrType = IrUnit
	switch node := node.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			val, valType, _ = e.emitStmt(stmt)
		}
	}

	return val, valType
}

func (e *Emitter) tmp() string {
	name := fmt.Sprintf("%%t%d", e.tmpIdx)
	e.tmpIdx++
	return name
}

func (e *Emitter) addStr(s string) {
	if _, ok := e.stringConsts[s]; !ok {
		e.stringConsts[s] = e.stringIdx
		e.stringIdx++
	}
}

func (e *Emitter) emit(line string) {
	withIndent := strings.Repeat("  ", e.depth) + line + "\n"
	e.buf.WriteString(withIndent)
}

func (e *Emitter) emitStmt(stmt ast.Node) (string, IrType, bool) {
	hasReturn := false
	var val string
	var valType IrType = IrUnit
	switch s := stmt.(type) {
	case *ast.ExpressionStmt:
		val, valType = e.emitExpr(s.Expr)
	case *ast.VarDeclarationStmt:
		val, valType = e.emitVarDecl(s)
	case *ast.VarAssignmentStmt:
		val, valType = e.emitVariableAssignment(s)
	case *ast.ReturnStmt:
		val, valType = e.emitReturnStmt(s)
		hasReturn = true
	case *ast.ForStmt:
		val, valType = e.emitForStmt(s)
	case *ast.SelectorAssignmentStmt:
		val, valType = e.emitSelectorAssignment(s)
	}
	return val, valType, hasReturn
}

func (e *Emitter) emitBlock(block *ast.BlockStmt) (string, IrType, bool) {
	e.depth++
	var lastVal string
	var lastType IrType = IrUnit
	hasReturn := false
	for _, stmt := range block.Stmts {
		lastVal, lastType, hasReturn = e.emitStmt(stmt)
	}
	e.depth--
	return lastVal, lastType, hasReturn
}

func (e *Emitter) emitExpr(expr ast.Expr) (string, IrType) {
	val, valType := e.emitExprInner(expr)
	if castTo := expr.GetCastTo(); castTo != nil {
		concreteName := e.getConcreteType(expr)
		iface := castTo.Name
		ifaceAlloca := e.tmp()
		line := fmt.Sprintf("%s = alloca { ptr, ptr }", ifaceAlloca)
		e.emit(line)

		// store value ptr at index 0
		valSlot := e.tmp()
		line = fmt.Sprintf("%s = getelementptr { ptr, ptr }, ptr %s, i32 0, i32 0", valSlot, ifaceAlloca)
		e.emit(line)
		line = fmt.Sprintf("store ptr %s, ptr %s", val, valSlot)
		e.emit(line)

		// Store vtable pointer at index 1
		vtableSlot := e.tmp()
		line = fmt.Sprintf("%s = getelementptr { ptr, ptr }, ptr %s, i32 0, i32 1", vtableSlot, ifaceAlloca)
		e.emit(line)
		line = fmt.Sprintf("store ptr @vtable.%s.%s, ptr %s", concreteName, iface, vtableSlot)
		e.emit(line)

		return ifaceAlloca, IrPtr
	}

	return val, valType
}

func (e *Emitter) emitExprInner(expr ast.Expr) (string, IrType) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", expr.Value), IrInt
	case *ast.FloatLiteral:
		return fmt.Sprintf("%f", expr.Value), IrFloat
	case *ast.StringLiteral:
		idx := e.stringConsts[expr.Value]
		name := fmt.Sprintf("@.str.%d", idx)
		return name, IrPtr
	case *ast.BooleanLiteral:
		if expr.Value {
			return "1", IrBool
		}
		return "0", IrBool
	case *ast.StructLiteral:
		return e.emitStructLiteral(expr)
	case *ast.InfixExpr:
		return e.emitInfixExpr(expr)
	case *ast.PrefixExpr:
		return e.emitPrefixExpr(expr)
	case *ast.CallExpr:
		return e.emitCallExpr(expr)
	case *ast.Identifier:
		local := e.locals[expr.Value]
		result := e.tmp()
		line := fmt.Sprintf("%s = load %s, %s %s", result, local.typ, IrPtr, local.alloca)
		e.emit(line)
		return result, local.typ
	case *ast.IfExpr:
		return e.emitIfExpr(expr)
	case *ast.SelectorExpr:
		return e.emitSelectorExpr(expr)
	}
	return "", IrUnit
}

func (e *Emitter) emitInfixExpr(expr *ast.InfixExpr) (string, IrType) {
	left, lType := e.emitExpr(expr.Left)
	right, _ := e.emitExpr(expr.Right) // discarding because typechecker has enforced this
	result := e.tmp()
	icmp := "icmp"
	fcmp := "fcmp"
	var op, cmpType string
	retType := lType
	switch expr.Operator {
	case "+":
		switch lType {
		case IrFloat:
			op = "fadd"
		case IrPtr:
			line := fmt.Sprintf("%s = call ptr @sydney_strcat(%s, %s)", result, left, right)
			e.emit(line)
			return result, IrPtr
		default:
			op = "add"
		}
	case "-":
		if lType == IrFloat {
			op = "fsub"
		} else {
			op = "sub"
		}
	case "*":
		if lType == IrFloat {
			op = "fmul"
		} else {
			op = "mul"
		}
	case "/":
		if lType == IrFloat {
			op = "fdiv"
		} else {
			op = "sdiv"
		}
	case "%":
		if lType == IrFloat {
			op = "frem"
		} else {
			op = "srem"
		}
	case "==":
		//%t0 = icmp eq i64 %left, %right    ; ==
		if lType == IrFloat {
			cmpType = fcmp
			op = "oeq"
		} else {
			cmpType = icmp
			op = "eq"
		}
		retType = IrBool
	case "!=":
		//%t0 = icmp ne i64 %left, %right    ; !=
		if lType == IrFloat {
			cmpType = fcmp
			op = "one"
		} else {
			cmpType = icmp
			op = "ne"
		}
		retType = IrBool
	case ">":
		//%t0 = icmp sgt i64 %left, %right   ; >
		if lType == IrFloat {
			cmpType = fcmp
			op = "ogt"
		} else {
			cmpType = icmp
			op = "sgt"
		}
		retType = IrBool
	case ">=":
		//%t0 = icmp sge i64 %left, %right   ; >=
		if lType == IrFloat {
			cmpType = fcmp
			op = "oge"
		} else {
			cmpType = icmp
			op = "sge"
		}
		retType = IrBool
	case "<":
		//%t0 = icmp slt i64 %left, %right   ; <
		if lType == IrFloat {
			cmpType = fcmp
			op = "olt"
		} else {
			cmpType = icmp
			op = "slt"
		}
		retType = IrBool
	case "<=":
		//%t0 = icmp sle i64 %left, %right   ; <=
		if lType == IrFloat {
			cmpType = fcmp
			op = "ole"
		} else {
			cmpType = icmp
			op = "sle"
		}
		retType = IrBool
	case "||":
		op = "or"
	case "&&":
		op = "and"
	}
	var opStr string
	if cmpType != "" {
		opStr = e.emitInfixCmpStr(cmpType, op, lType, left, right)
	} else {
		opStr = e.infixOpStr(op, lType, left, right)
	}

	line := fmt.Sprintf("%s = %s", result, opStr)
	e.emit(line)

	return result, retType
}

func (e *Emitter) emitPrefixExpr(expr *ast.PrefixExpr) (string, IrType) {
	val, valType := e.emitExpr(expr.Right)
	result := e.tmp()
	var opStr string
	if expr.Operator == "!" {
		//%t0 = xor i1 %right, 1
		opStr = e.infixOpStr("xor", IrBool, val, "1")
	} else if expr.Operator == "-" {
		if valType == IrFloat {
			opStr = fmt.Sprintf("fneg %s %s", IrFloat, val)
		} else {
			opStr = e.infixOpStr("sub", IrInt, "0", val)
		}
	}
	line := fmt.Sprintf("%s = %s", result, opStr)
	e.emit(line)

	return result, valType
}

func (e *Emitter) emitCallExpr(expr *ast.CallExpr) (string, IrType) {
	if ident, ok := expr.Function.(*ast.Identifier); ok {
		if ident.Value == "print" {
			for _, a := range expr.Arguments {
				arg, argType := e.emitExpr(a)
				switch argType {
				case IrInt:
					e.emit(fmt.Sprintf("call void @sydney_print_int(i64 %s)", arg))
				case IrFloat:
					e.emit(fmt.Sprintf("call void @sydney_print_float(double %s)", arg))
				case IrPtr:
					e.emit(fmt.Sprintf("call void @sydney_print_string(ptr %s)", arg))
				case IrBool:
					e.emit(fmt.Sprintf("call void @sydney_print_bool(i8 %s)", arg))
				}
			}
			e.emit("call void @sydney_print_newline()")
			return "", IrUnit
		}
		sig, exists := e.funcSigs[ident.Value]
		if exists {
			args := make([]string, len(expr.Arguments))
			for i, arg := range expr.Arguments {
				val, _ := e.emitExpr(arg)
				args[i] = fmt.Sprintf("%s %s", sig.paramTypes[i], val)
			}
			argsStr := strings.Join(args, ", ")
			if sig.retType == IrUnit {
				line := fmt.Sprintf("call void %s(%s)", sig.name, argsStr)
				e.emit(line)
				return "", IrUnit
			}

			result := e.tmp()
			line := fmt.Sprintf("%s = call %s %s(%s)", result, sig.retType, sig.name, argsStr)
			e.emit(line)
			return result, sig.retType
		}
	}

	// Check for interface method call via mangled name
	if expr.MangledName != "" {
		e.emitStructMethodCall(expr)
	}

	if sel, ok := expr.Function.(*ast.SelectorExpr); ok {
		if castTo := sel.Left.GetCastTo(); castTo != nil {
			return e.emitInterfaceMethodCall(expr, sel, castTo)
		}

		if ident, ok := sel.Left.(*ast.Identifier); ok {
			if iface, ok := ident.ResolvedType.(types.InterfaceType); ok {
				return e.emitInterfaceMethodCall(expr, sel, &iface)
			}
		}
	}

	return "", IrUnit
}

func (e *Emitter) emitStructMethodCall(expr *ast.CallExpr) (string, IrType) {
	sig, exists := e.funcSigs[expr.MangledName]
	if exists {
		// this is a struct receiver, not interface polymorphism
		args := make([]string, len(expr.Arguments))
		for i, arg := range expr.Arguments {
			val, _ := e.emitExpr(arg)
			args[i] = fmt.Sprintf("%s %s", sig.paramTypes[i], val)
		}
		argsStr := strings.Join(args, ", ")
		if sig.retType == IrUnit {
			e.emit(fmt.Sprintf("call void %s(%s)", sig.name, argsStr))
			return "", IrUnit
		}
		result := e.tmp()
		e.emit(fmt.Sprintf("%s = call %s %s(%s)", result, sig.retType, sig.name, argsStr))
		return result, sig.retType
	}

	return "", IrUnit
}

func (e *Emitter) emitInterfaceMethodCall(expr *ast.CallExpr, sel *ast.SelectorExpr, iface *types.InterfaceType) (string, IrType) {
	// CastTo boxing handles emitting of { ptr, ptr } alloca
	ifacePtr, _ := e.emitExpr(sel.Left)

	// load fat pointer
	ifaceVal := e.tmp()
	line := fmt.Sprintf("%s = load { ptr, ptr }, ptr %s", ifaceVal, ifacePtr)
	e.emit(line)

	// Extract value pointer (index 0)
	//%val.ptr = extractvalue { ptr, ptr } %iface, 0
	valPtr := e.tmp()
	line = fmt.Sprintf("%s = extractvalue { ptr, ptr } %s, 0", valPtr, ifaceVal)
	e.emit(line)

	//; Extract vtable pointer (index 1)
	//%vtable.ptr = extractvalue { ptr, ptr } %iface, 1
	vtablePtr := e.tmp()
	line = fmt.Sprintf("%s = extractvalue { ptr, ptr } %s, 1", vtablePtr, ifaceVal)
	e.emit(line)

	methodName := sel.Value.(*ast.Identifier).Value
	methodIdx := iface.MethodIndices[methodName]
	numMethods := len(iface.Methods)

	// GEP into vtable to get the method function pointer
	// "area" is method index 0 in Shape
	//%fn.ptr.addr = getelementptr [1 x ptr], ptr %vtable.ptr, i32 0, i32 0
	fnPtrAddr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr [%d x ptr], ptr %s, i32 0, i32 %d", fnPtrAddr, numMethods, vtablePtr, methodIdx)
	e.emit(line)

	// load function pointer
	//%fn.ptr = load ptr, ptr %fn.ptr.addr
	fnPtr := e.tmp()
	line = fmt.Sprintf("%s = load ptr, ptr %s", fnPtr, fnPtrAddr)
	e.emit(line)

	args := make([]string, len(expr.Arguments)+1)
	args[0] = fmt.Sprintf("ptr %s", valPtr)
	for i, arg := range expr.Arguments {
		val, valType := e.emitExpr(arg)
		args[i+1] = fmt.Sprintf("%s %s", valType, val)
	}
	argStr := strings.Join(args, ", ")

	// Call the method — first arg is the receiver (value pointer)
	method := iface.Types[methodIdx]
	retType := SydneyTypeToIrType(method.(types.FunctionType).Return)
	if retType == IrUnit {
		line = fmt.Sprintf("call void %s(%s)", fnPtr, argStr)
		e.emit(line)
		return "", IrUnit
	}

	//%result = call double %fn.ptr(ptr %val.ptr)
	result := e.tmp()
	line = fmt.Sprintf("%s = call %s %s(%s)", result, retType, fnPtr, argStr)
	e.emit(line)

	return result, retType
}

func (e *Emitter) infixOpStr(op string, t IrType, left, right string) string {
	return fmt.Sprintf("%s %s %s, %s", op, t, left, right)
}

func (e *Emitter) emitInfixCmpStr(cmpType, op string, t IrType, left, right string) string {
	return fmt.Sprintf("%s %s %s %s, %s", cmpType, op, t, left, right)
}

func (e *Emitter) emitStructType(t types.StructType) {
	var out bytes.Buffer
	out.WriteString("%struct.")
	out.WriteString(t.Name)
	out.WriteString(" = type { ")
	for i, tt := range t.Types {
		if i > 0 {
			out.WriteString(", ")
		}
		ttt := SydneyTypeToIrType(tt)
		out.WriteString(ttt.String())
	}
	out.WriteString(" }")

	e.emit(out.String())
}

func (e *Emitter) emitStringConst(s string, idx int) {
	s, size := llvmEscapeString(s)
	line := fmt.Sprintf("@.str.%d = private unnamed_addr constant [%d x i8] c\"%s\"", idx, size, s)
	e.emit(line)
}

func llvmEscapeString(s string) (string, int) {
	var buf bytes.Buffer
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 32 && b <= 126 && b != '\\' && b != '"' {
			buf.WriteByte(b)
		} else {
			_, err := fmt.Fprintf(&buf, "\\%02X", b)
			if err != nil {
				return "", 0
			}
		}
	}
	buf.WriteString("\\00")
	return buf.String(), len(s) + 1 // byte count including null
}

func (e *Emitter) emitInterfaceType(sname, iname string, methods []string) {
	numMethods := len(methods)
	// @vtable.Circle.Shape = constant [1 x ptr] [ptr @Circle.area]
	entries := make([]string, numMethods)
	for i, m := range methods {
		entries[i] = fmt.Sprintf("ptr @%s.%s", sname, m)
	}
	methodsStr := fmt.Sprintf("[%s]", strings.Join(entries, ", "))
	line := fmt.Sprintf("@vtable.%s.%s = constant [%d x ptr] %s", sname, iname, numMethods, methodsStr)
	e.emit(line)
}

func (e *Emitter) emitVarDecl(stmt *ast.VarDeclarationStmt) (string, IrType) {
	val, valType := e.emitExpr(stmt.Value)
	if valType == IrUnit {
		val, valType = e.getZeroValue(stmt.Type)
	}

	name := stmt.Name.Value
	allocaName := "%" + name + ".addr"
	line := fmt.Sprintf("%s = alloca %s", allocaName, valType)
	e.emit(line)
	line = fmt.Sprintf("store %s %s, ptr %s", valType, val, allocaName)
	e.locals[name] = irLocal{alloca: allocaName, typ: valType}
	e.emit(line)

	return val, valType
}

func (e *Emitter) emitVariableAssignment(stmt *ast.VarAssignmentStmt) (string, IrType) {
	val, valType := e.emitExpr(stmt.Value)
	name := stmt.Identifier.Value
	allocaName := e.locals[name]
	line := fmt.Sprintf("store %s %s, ptr %s", valType, val, allocaName.alloca)
	e.emit(line)

	return val, valType
}

func (e *Emitter) getZeroValue(t types.Type) (string, IrType) {
	switch t {
	case types.Int:
		return "0", IrInt
	case types.Float:
		return "0.0", IrFloat
	case types.Bool:
		return "0", IrBool
	}

	return "", IrPtr
}

func (e *Emitter) label(prefix string) string {
	name := fmt.Sprintf("%s.%d", prefix, e.lblIdx)
	e.lblIdx++
	return name
}

func (e *Emitter) emitLabel(name string) {
	e.buf.WriteString(name + ":\n")
}

func (e *Emitter) emitReturnStmt(stmt *ast.ReturnStmt) (string, IrType) {
	if stmt.ReturnValue == nil {
		e.emit("ret void")
		return "", IrUnit
	}
	val, typ := e.emitExpr(stmt.ReturnValue)
	line := fmt.Sprintf("ret %s %s", typ, val)
	e.emit(line)
	return val, typ
}

func (e *Emitter) emitIfExpr(expr *ast.IfExpr) (string, IrType) {
	cond, _ := e.emitExpr(expr.Condition) // emit condition, typechecker enforces this is bool

	hasElse := expr.Alternative != nil // controls if else block is emitted

	// labels for blocks, else before merge for targets
	thenLabel := e.label("then")
	elseLabel := ""
	if hasElse {
		elseLabel = e.label("else")
	}
	mergeLabel := e.label("merge")

	// alloca for result
	var resultAddr string
	if hasElse {
		resultAddr = e.tmp()
		line := fmt.Sprintf("%s = alloca i64", resultAddr)
		e.emit(line)
	}

	falseLabel := mergeLabel
	if hasElse {
		falseLabel = elseLabel
	}
	// branch to our labels
	e.emitBranch(cond, thenLabel, falseLabel)

	// consequence block
	e.emitLabel(thenLabel)
	thenVal, thenType, _ := e.emitBlock(expr.Consequence)
	// controls whether result is stored
	isExpr := hasElse && thenType != IrUnit
	// if there is an else, then there's an expression -- typechecker needs to enforce this more cleanly
	if isExpr {
		line := fmt.Sprintf("store %s %s, ptr %s", thenType, thenVal, resultAddr) // need to store result if this is an expression
		e.emit(line)
	}
	e.emitJmp(mergeLabel) // mergeLabel might be elseLabel if no alternative

	// alternative block
	if hasElse {
		e.emitLabel(elseLabel)
		elseVal, _, _ := e.emitBlock(expr.Alternative)
		if isExpr {
			line := fmt.Sprintf("store %s %s, ptr %s", thenType, elseVal, resultAddr)
			e.emit(line)
		}
		e.emitJmp(mergeLabel)
	}

	// merge block
	e.emitLabel(mergeLabel)

	if isExpr {
		result := e.tmp()
		line := fmt.Sprintf("%s = load %s, ptr %s", result, thenType, resultAddr)
		e.emit(line)
		return result, thenType
	}

	return "", IrUnit
}

func (e *Emitter) emitForStmt(stmt *ast.ForStmt) (string, IrType) {
	condLabel := e.label("cond")
	loopLabel := e.label("loop")
	escapeLabel := e.label("escape")

	// so we can branch back here
	e.emitJmp(condLabel)
	e.emitLabel(condLabel)
	cond, _ := e.emitExpr(stmt.Condition)
	e.emitBranch(cond, loopLabel, escapeLabel)

	// set loop body
	e.emitLabel(loopLabel)
	e.emitBlock(stmt.Body)

	// jump back to above condition
	e.emitJmp(condLabel)

	// how to get out of loop
	e.emitLabel(escapeLabel)

	return "", IrUnit
}

func (e *Emitter) emitBranch(cond, l1, l2 string) {
	line := fmt.Sprintf("br i1 %s, label %%%s, label %%%s", cond, l1, l2)
	e.emit(line)
}

func (e *Emitter) emitJmp(label string) {
	line := fmt.Sprintf("br label %%%s", label)
	e.emit(line)
}

func (e *Emitter) emitFunction(decl *ast.FunctionDeclarationStmt) (string, IrType) {
	// construct signature struct
	fType, _ := decl.Type.(types.FunctionType)
	paramIrTypes := make([]string, len(decl.Params))
	paramParts := make([]string, len(decl.Params))

	for i, p := range fType.Params {
		t := SydneyTypeToIrType(p)
		paramIrTypes[i] = t.String()
		paramParts[i] = fmt.Sprintf("%s %%%s", t, decl.Params[i].Value)
	}

	name := decl.Name.Value
	if decl.MangledName != "" {
		name = decl.MangledName
	}

	ret := SydneyTypeToIrType(fType.Return)

	e.funcSigs[name] = funcSig{name: "@" + name, paramTypes: paramIrTypes, retType: ret}

	// save state
	oldLocals := e.locals
	oldTmpIdx := e.tmpIdx
	oldLblIdx := e.lblIdx
	e.locals = make(map[string]irLocal)
	e.tmpIdx = 0
	e.lblIdx = 0

	argStr := strings.Join(paramParts, ", ")
	line := fmt.Sprintf("define %s @%s(%s) {", ret, name, argStr)
	e.emit(line)
	e.emitLabel("entry")

	e.depth++
	for i, paramName := range decl.Params {
		pName := paramName.Value
		allocaName := "%" + pName + ".addr"
		line = fmt.Sprintf("%s = alloca %s", allocaName, paramIrTypes[i])
		e.emit(line)
		line = fmt.Sprintf("store %s %%%s, ptr %s", paramIrTypes[i], pName, allocaName)
		e.emit(line)
		e.locals[pName] = irLocal{alloca: allocaName, typ: SydneyTypeToIrType(fType.Params[i])}
	}

	e.depth--

	val, valType, hasReturn := e.emitBlock(decl.Body)
	e.depth++
	if !hasReturn {
		if ret == IrUnit {
			e.emit("ret void")
		} else {
			line = fmt.Sprintf("ret %s %s", valType, val)
			e.emit(line)
		}
	}

	e.depth--
	e.emit("}")
	e.emit("")
	// restore state
	e.lblIdx = oldLblIdx
	e.tmpIdx = oldTmpIdx
	e.locals = oldLocals

	return val, valType
}

func (e *Emitter) emitStructLiteral(lit *ast.StructLiteral) (string, IrType) {
	//; Point { x: 1, y: 2 }
	//%t0 = call ptr @sydney_gc_alloc(i64 16)
	//%t1 = getelementptr %struct.Point, ptr %t0, i32 0, i32 0
	//store i64 1, ptr %t1
	//%t2 = getelementptr %struct.Point, ptr %t0, i32 0, i32 1
	//store i64 2, ptr %t2
	//; result is %t0 (ptr)
	result := e.tmp()
	size := len(lit.Fields) * 8 // all values at 8 bytes as int64, float64, etc

	// how to compute size?
	line := fmt.Sprintf("%s = call %s @sydney_gc_alloc(%s %d)", result, IrPtr, IrInt, size)
	e.emit(line)
	for i, fieldName := range lit.ResolvedType.Fields {
		litIdx := -1
		for j, field := range lit.Fields {
			if field == fieldName {
				litIdx = j
			}
		}
		fTmp := e.tmp()

		line = fmt.Sprintf("%s = getelementptr %%struct.%s, %s %s, %s %s, %s %d", fTmp, lit.Name, IrPtr, result, IrInt32, "0", IrInt32, i)
		e.emit(line)
		val, valType := e.emitExpr(lit.Values[litIdx])
		line = fmt.Sprintf("store %s %s, %s %s", valType, val, IrPtr, fTmp)
		e.emit(line)
	}

	return result, IrPtr
}

func (e *Emitter) emitSelectorExpr(expr *ast.SelectorExpr) (string, IrType) {
	// ; load the struct pointer
	//  %t0 = load ptr, ptr %p.addr
	//
	//  ; GEP to field x (index 0)
	//  %t1 = getelementptr %struct.Point, ptr %t0, i32 0, i32 0
	//
	//  ; load the field value
	//  %t2 = load i64, ptr %t1
	//  ; %t2 is the result

	val := expr.Value.(*ast.Identifier).Value
	fieldIdx := -1
	for i, fieldName := range expr.ResolvedType.Fields {
		if fieldName == val {
			fieldIdx = i
			break
		}
	}
	retType := SydneyTypeToIrType(expr.ResolvedType.Types[fieldIdx])

	structPtr, _ := e.emitExpr(expr.Left)

	gepTmp := e.tmp()

	line := fmt.Sprintf("%s = getelementptr %%struct.%s, ptr %s, i32 0, i32 %d", gepTmp, expr.ResolvedType.Name, structPtr, fieldIdx)
	e.emit(line)

	result := e.tmp()
	line = fmt.Sprintf("%s = load %s, ptr %s", result, retType, gepTmp)
	e.emit(line)

	return result, retType
}

func (e *Emitter) emitSelectorAssignment(stmt *ast.SelectorAssignmentStmt) (string, IrType) {
	// ; load the struct pointer
	//  %t0 = load ptr, ptr %p.addr
	//
	//  ; GEP to field x (index 0)
	//  %t1 = getelementptr %struct.Point, ptr %t0, i32 0, i32 0
	//
	//  ; load the field value
	//  store i64 2, ptr %t1

	st := stmt.Left

	val := st.Value.(*ast.Identifier).Value
	fieldIdx := -1
	for i, fieldName := range st.ResolvedType.Fields {
		if fieldName == val {
			fieldIdx = i
			break
		}
	}

	strPtr, _ := e.emitExpr(st.Left) // %t0 = load ptr, ptr %p.addr

	tmp := e.tmp()
	line := fmt.Sprintf("%s = getelementptr %%struct.%s, ptr %s, i32 0, i32 %d", tmp, stmt.Left.ResolvedType.Name, strPtr, fieldIdx)
	e.emit(line)

	val, valType := e.emitExpr(stmt.Value)

	line = fmt.Sprintf("store %s %s, ptr %s", valType, val, tmp)
	e.emit(line)

	return "", IrUnit
}

func (e *Emitter) getConcreteType(expr ast.Expr) string {
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
