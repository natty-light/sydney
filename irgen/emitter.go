package irgen

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"sydney/ast"
	"sydney/loader"
	"sydney/types"
)

type irLocal struct {
	alloca string // e.g. "%x.addr" or "@g0"
	typ    IrType
}

type irGlobal struct {
	name string
	typ  IrType
}

type funcSig struct {
	retType    IrType   // IR return type string
	paramTypes []IrType // IR param type strings
	name       string   // IR function name (e.g. "@add", "@Circle.area")
}

type freeVar struct {
	name string
	typ  IrType
}

type funcState struct {
	locals    map[string]irLocal
	tmpIdx    int
	lblIdx    int
	depth     int
	allocaBuf *bytes.Buffer
	bodyBuf   *bytes.Buffer
}

type LoopLabels struct {
	condLabel   string
	postLabel   string // empty if no post
	escapeLabel string
}

type Emitter struct {
	buf     *bytes.Buffer
	funcBuf *bytes.Buffer
	tmpIdx  int
	anonIdx int
	lblIdx  int
	locals  map[string]irLocal
	globals map[string]irGlobal
	inFunc  bool
	depth   int

	// Collected metadata
	structTypes    map[string]types.StructType
	interfaceTypes map[string]types.InterfaceType
	vtables        map[string]map[string][]string // vtables[struct][iface] → @vtable global
	stringConsts   map[string]int                 // string value → index (@.str.0, ...)
	stringIdx      int

	funcSigs   map[string]funcSig   // function name → { retType, paramTypes }
	scopeStack []map[string]irLocal // stack of local scopes for nested functions

	allocaBuf *bytes.Buffer
	bodyBuf   *bytes.Buffer

	currentModule  string
	emittedModules []string

	loopStack       []*LoopLabels
	loopIdx         int
	blockTerminated bool
}

func New() *Emitter {
	return &Emitter{
		structTypes:    make(map[string]types.StructType),
		interfaceTypes: make(map[string]types.InterfaceType),
		stringConsts:   make(map[string]int),
		stringIdx:      0,
		vtables:        make(map[string]map[string][]string),
		locals:         make(map[string]irLocal),
		globals:        make(map[string]irGlobal),
		funcSigs:       make(map[string]funcSig),
		scopeStack:     make([]map[string]irLocal, 0),

		buf:     &bytes.Buffer{},
		funcBuf: &bytes.Buffer{},

		emittedModules:  make([]string, 0),
		loopStack:       make([]*LoopLabels, 0),
		blockTerminated: false,
		loopIdx:         0,
	}
}

func (e *Emitter) Emit(n ast.Node, packages []*loader.Package) error {
	for _, pkg := range packages {
		e.currentModule = pkg.Name
		for _, program := range pkg.Programs {
			e.collect(program)
		}
		e.currentModule = ""
	}
	program := e.collect(n)
	e.preamble()

	for _, pkg := range packages {
		e.currentModule = pkg.Name
		for _, program := range pkg.Programs {
			e.functions(program)
		}
		e.emitPackageInit(pkg)
		e.currentModule = ""
	}

	e.functions(program)

	err := e.mainWrapper(n)
	if err != nil {
		return err
	}
	_, err = e.funcBuf.WriteTo(e.buf)
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
	case *ast.PubStatement:
		return e.collect(node.Stmt)
	case *ast.StructDefinitionStmt:
		name := node.Name.Value
		if e.currentModule != "" {
			name = e.moduleMangle(e.currentModule, name)
		}
		e.structTypes[name] = node.Type
	case *ast.InterfaceDefinitionStmt:
		t := node.Type
		if t.MethodIndices == nil {
			t.MethodIndices = make(map[string]int)
			for i, mn := range t.Methods {
				t.MethodIndices[mn] = i
			}
		}
		name := node.Name.Value
		if e.currentModule != "" {
			name = e.moduleMangle(e.currentModule, name)
		}

		e.interfaceTypes[name] = node.Type
	case *ast.InterfaceImplementationStmt:
		sname := node.StructName.Value
		if e.vtables[sname] == nil {
			e.vtables[sname] = make(map[string][]string)
		}
		for _, iname := range node.InterfaceNames {
			e.vtables[sname][iname.Value] = nil
		}
	case *ast.VarDeclarationStmt:
		name := node.Name.Value
		if e.currentModule != "" {
			name = e.moduleMangle(e.currentModule, name)
		}
		globalName := e.global(name)
		e.globals[globalName] = irGlobal{
			name: globalName,
			typ:  SydneyTypeToIrType(node.Type),
		}
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
		if !node.IsExtern {
			e.collectStrings(node.Body)
		}
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
		keys := make([]ast.Expr, 0, len(node.Pairs))
		for key := range node.Pairs {
			keys = append(keys, key)
		}
		slices.SortFunc(keys, func(a, b ast.Expr) int {
			return strings.Compare(a.String(), b.String())
		})
		for _, key := range keys {
			e.collectStrings(key)
			e.collectStrings(node.Pairs[key])
		}
	case *ast.FunctionLiteral:
		e.collectStrings(node.Body)
	case *ast.StructLiteral:
		for _, val := range node.Values {
			e.collectStrings(val)
		}
	case *ast.MatchExpr:
		e.collectStrings(node.Subject)
		e.collectStrings(node.OkArm.Body)
		e.collectStrings(node.ErrArm.Body)
	}

}

func (e *Emitter) CollectPackage() {

}

func (e *Emitter) preamble() {
	e.emit("declare void @sydney_print_int(i64)")
	e.emit("declare void @sydney_print_float(double)")
	e.emit("declare void @sydney_print_string(ptr)")
	e.emit("declare void @sydney_print_byte(i8)")
	e.emit("declare ptr @sydney_strcat(ptr, ptr)")
	e.emit("declare void @sydney_print_newline()")
	e.emit("declare void @sydney_gc_init()")
	e.emit("declare void @sydney_print_bool(i8)")
	e.emit("declare ptr @sydney_gc_alloc(i64)")
	e.emit("declare void @sydney_gc_collect()")
	e.emit("declare void @sydney_gc_add_global_root(ptr)")
	e.emit("declare void @sydney_gc_shutdown()")
	e.emit("declare i64 @sydney_strlen(ptr)")
	e.emit("declare ptr @sydney_map_create_int()")
	e.emit("declare ptr @sydney_map_create_string()")
	e.emit("declare void @sydney_map_set_str(ptr, ptr, i64)")
	e.emit("declare i64 @sydney_map_get_str(ptr, ptr)")
	e.emit("declare void @sydney_map_set_int(ptr, i64, i64)")
	e.emit("declare i64 @sydney_map_get_int(ptr, i64)")
	e.emit("declare i64 @sydney_file_open(ptr)")
	e.emit("declare ptr @sydney_file_read(i64)")
	e.emit("declare i64 @sydney_file_write(i64, ptr)")
	e.emit("declare i64 @sydney_file_close(i64)")
	e.emit("declare ptr @sydney_get_last_error()")
	e.emit("declare ptr @sydney_byte_to_string(i8)")
	e.emit("declare void @llvm.memcpy.p0.p0.i64(ptr, ptr, i64, i1)")
	e.emit("declare i1 @sydney_str_equals(ptr, ptr)")
	e.emit("declare ptr @sydney_map_keys_str(ptr)")
	e.emit("declare ptr @sydney_map_values_str(ptr)")
	e.emit("declare ptr @sydney_map_keys_int(ptr)")
	e.emit("declare ptr @sydney_map_values_int(ptr)")
	e.emit("declare ptr @sydney_atof(ptr)")
	e.emit("declare double @conv__atof(ptr)")
	e.emit("")

	structs := make([]string, 0, len(e.structTypes))
	for name, _ := range e.structTypes {
		structs = append(structs, name)
	}
	slices.Sort(structs)
	for _, name := range structs {
		s := e.structTypes[name]
		e.emitStructType(s)
	}

	strs := make([]string, len(e.stringConsts))
	for s, i := range e.stringConsts {
		strs[i] = s
	}
	for i, s := range strs {
		e.emitStringConst(s, i)
	}

	vtableStructs := make([]string, len(e.interfaceTypes))
	for sname, _ := range e.vtables {
		vtableStructs = append(vtableStructs, sname)
	}
	slices.Sort(vtableStructs)

	for _, sname := range vtableStructs {
		imap := e.vtables[sname]
		ifaceNames := make([]string, len(imap))
		for iname := range imap {
			ifaceNames = append(ifaceNames, iname)
		}
		slices.Sort(ifaceNames)
		for _, iname := range ifaceNames {
			e.emitInterfaceType(sname, iname, imap[iname])
		}
	}

	globals := make([]string, 0, len(e.globals))
	for _, global := range e.globals {
		globals = append(globals, global.name)
	}
	slices.Sort(globals)
	for _, name := range globals {
		global := e.globals[name]
		zeroVal := e.getZeroValueFromIrType(global.typ)
		line := fmt.Sprintf("%s = global %s %s", global.name, global.typ, zeroVal)
		e.emit(line)
	}
}

func (e *Emitter) functions(node *ast.Program) {
	for _, stmt := range node.Stmts {
		if pub, ok := stmt.(*ast.PubStatement); ok {
			if fdecl, ok := pub.Stmt.(*ast.FunctionDeclarationStmt); ok {
				e.emitFunction(fdecl)
			}
		} else if fdecl, ok := stmt.(*ast.FunctionDeclarationStmt); ok {
			e.emitFunction(fdecl)
		}
	}
}

func (e *Emitter) mainWrapper(node ast.Node) error {
	e.emit("define i32 @main() {")

	e.allocaBuf = &bytes.Buffer{}
	e.bodyBuf = &bytes.Buffer{}
	e.depth = 1

	e.emit("call void @sydney_gc_init()")
	e.emitPackageInits()
	e.main(node)
	e.emit("call void @sydney_gc_shutdown()")
	e.emit("ret i32 0")

	e.assembleFunction(e.buf)
	e.depth = 0
	e.allocaBuf = nil
	e.bodyBuf = nil
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

func (e *Emitter) alloca(name string) string {
	allocaName := fmt.Sprintf("%%%s.%d.addr", name, e.tmpIdx)
	e.tmpIdx++
	return allocaName
}

func (e *Emitter) anon() string {
	name := fmt.Sprintf("@anon.%d", e.anonIdx)
	e.anonIdx++
	return name
}

func (e *Emitter) addStr(s string) {
	if _, ok := e.stringConsts[s]; !ok {
		e.stringConsts[s] = e.stringIdx
		e.stringIdx++
	}
}

func (e *Emitter) emit(line string) {
	if e.blockTerminated {
		return
	}
	withIndent := strings.Repeat("  ", e.depth) + line + "\n"
	if e.bodyBuf != nil {
		e.bodyBuf.WriteString(withIndent)
	} else {
		e.buf.WriteString(withIndent)
	}
}

func (e *Emitter) emitBypass(line string) {
	withIndent := strings.Repeat("  ", e.depth) + line + "\n"
	e.buf.WriteString(withIndent)
}

func (e *Emitter) label(prefix string) string {
	name := fmt.Sprintf("%s.%d", prefix, e.lblIdx)
	e.lblIdx++
	return name
}

func (e *Emitter) global(name string) string {
	return fmt.Sprintf("@%s", name)
}

func (e *Emitter) emitLabel(name string) {
	e.blockTerminated = false
	line := name + ":\n"
	if e.bodyBuf != nil {
		e.bodyBuf.WriteString(line)
	} else {
		e.buf.WriteString(line)
	}
}

func (e *Emitter) emitAlloca(name string, typ IrType) {
	line := fmt.Sprintf("  %s = alloca %s\n", name, typ)
	e.allocaBuf.WriteString(line)
}

func (e *Emitter) emitBranch(cond, l1, l2 string) {
	line := fmt.Sprintf("br i1 %s, label %%%s, label %%%s", cond, l1, l2)
	e.emit(line)
}

func (e *Emitter) emitJmp(label string) {
	line := fmt.Sprintf("br label %%%s", label)
	e.emit(line)
}

func (e *Emitter) beginFunction() funcState {
	e.inFunc = true
	state := funcState{
		locals:    e.locals,
		tmpIdx:    e.tmpIdx,
		lblIdx:    e.lblIdx,
		depth:     e.depth,
		allocaBuf: e.allocaBuf,
		bodyBuf:   e.bodyBuf,
	}

	e.locals = make(map[string]irLocal)
	e.tmpIdx = 0
	e.lblIdx = 0
	e.allocaBuf = &bytes.Buffer{}
	e.bodyBuf = &bytes.Buffer{}
	return state
}

func (e *Emitter) endFunction(state funcState) {
	e.locals = state.locals
	e.tmpIdx = state.tmpIdx
	e.lblIdx = state.lblIdx
	e.depth = state.depth
	e.allocaBuf = state.allocaBuf
	e.bodyBuf = state.bodyBuf
	e.inFunc = false
	e.blockTerminated = false
}

func (e *Emitter) assembleFunction(target *bytes.Buffer) {
	target.WriteString("entry:\n")
	e.allocaBuf.WriteTo(target)
	e.bodyBuf.WriteTo(target)
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
		e.blockTerminated = true
	case *ast.ForStmt:
		val, valType = e.emitForStmt(s)
	case *ast.SelectorAssignmentStmt:
		val, valType = e.emitSelectorAssignment(s)
	case *ast.IndexAssignmentStmt:
		t := s.Left.Left.GetResolvedType()
		if _, ok := t.(types.ArrayType); ok {
			val, valType = e.emitArrayIndexAssignment(s)
		} else if _, ok := t.(types.MapType); ok {
			val, valType = e.emitMapIndexAssignment(s)
		}
	case *ast.ContinueStmt:
		loop := e.getLoop()
		if loop != nil {
			if loop.postLabel != "" {
				e.emitJmp(loop.postLabel)
			} else {
				e.emitJmp(loop.condLabel)
			}
		}
		e.blockTerminated = true
		return "", IrUnit, false
	case *ast.BreakStmt:
		loop := e.getLoop()
		if loop != nil {
			e.emitJmp(loop.escapeLabel)
		}
		e.blockTerminated = true
		return "", IrUnit, false
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
		e.emitAlloca(ifaceAlloca, IrFatPtr)

		// store value ptr at index 0
		valSlot := e.tmp()
		line := fmt.Sprintf("%s = getelementptr { ptr, ptr }, ptr %s, i32 0, i32 0", valSlot, ifaceAlloca)
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
	case *ast.ByteLiteral:
		return fmt.Sprintf("%d", expr.Value), IrInt8
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
	case *ast.NullLiteral:
		return "null", IrNull
	case *ast.StructLiteral:
		return e.emitStructLiteral(expr)
	case *ast.ArrayLiteral:
		return e.emitArrayLiteral(expr)
	case *ast.FunctionLiteral:
		return e.emitClosure(expr)
	case *ast.HashLiteral:
		return e.emitHashLiteral(expr)
	case *ast.InfixExpr:
		return e.emitInfixExpr(expr)
	case *ast.PrefixExpr:
		return e.emitPrefixExpr(expr)
	case *ast.CallExpr:
		return e.emitCallExpr(expr)
	case *ast.Identifier:
		name := expr.Value
		irRep, ok := e.locals[name]
		var irName string
		var irType IrType
		if ok {
			irName = irRep.alloca
			irType = irRep.typ
		} else {
			global, _ := e.globals[e.global(name)]
			irName = global.name
			irType = global.typ
		}
		result := e.tmp()
		line := fmt.Sprintf("%s = load %s, %s %s", result, irType, IrPtr, irName)
		e.emit(line)
		return result, irType
	case *ast.IfExpr:
		return e.emitIfExpr(expr)
	case *ast.SelectorExpr:
		return e.emitSelectorExpr(expr)
	case *ast.IndexExpr:
		lt := expr.Left.GetResolvedType()

		if lt == types.String {
			return e.emitStringIndexExpr(expr)
		}

		if _, ok := lt.(types.ArrayType); ok {
			return e.emitArrayIndexExpr(expr)
		}

		if _, ok := lt.(types.MapType); ok {
			return e.emitMapIndexExpr(expr)
		}
	case *ast.ScopeAccessExpr:
		return e.emitScopeAccessExpr(expr)
	case *ast.MatchExpr:
		return e.emitMatchExpr(expr)
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
			line := fmt.Sprintf("%s = call ptr @sydney_strcat(ptr %s, ptr %s)", result, left, right)
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
		if expr.Left.GetResolvedType() == types.String {
			line := fmt.Sprintf("%s = call i1 @sydney_str_equals(ptr %s, ptr %s)", result, left, right)
			e.emit(line)
			return result, IrBool
		}

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
		if expr.Left.GetResolvedType() == types.String {
			eq := e.tmp()
			line := fmt.Sprintf("%s = call i1 @sydney_str_equals(ptr %s, ptr %s)", eq, left, right)
			e.emit(line)
			line = fmt.Sprintf("%s = xor i1 %s, 1", result, eq)
			e.emit(line)
			return result, IrBool
		}

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
		name := ident.Value

		if e.currentModule != "" {
			mangled := e.moduleMangle(e.currentModule, name)
			if fn, ok := runtimeBuiltins[mangled]; ok {
				return e.emitRuntimeCall(fn, expr)
			}
			if sig, ok := e.funcSigs[mangled]; ok {
				return e.emitFunctionCall(expr, sig)
			}
		}

		switch name {
		case "print":
			return e.emitPrintCall(expr)
		case "len":
			return e.emitLenCall(expr)
		case "ok":
			return e.emitResultConstructorCall(true, expr)
		case "err":
			return e.emitResultConstructorCall(false, expr)
		case "int":
			return e.emitIntConvCall(expr)
		case "byte":
			return e.emitByteConvCall(expr)
		case "char":
			return e.emitCharConvCall(expr)
		case "append":
			return e.emitAppendCall(expr)
		case "keys":
			if mt, ok := expr.Arguments[0].GetResolvedType().(types.MapType); ok {
				if mt.KeyType == types.String {
					return e.emitStrKeysCall(expr)
				}
				return e.emitIntKeysCall(expr)
			}
		case "values":
			if mt, ok := expr.Arguments[0].GetResolvedType().(types.MapType); ok {
				if mt.KeyType == types.String {
					return e.emitStrValuesCall(expr)
				}
				return e.emitIntValuesCall(expr)
			}
		}

		if fn, ok := runtimeBuiltins[name]; ok {
			return e.emitRuntimeCall(fn, expr)
		}

		sig, exists := e.funcSigs[name]
		if exists {
			return e.emitFunctionCall(expr, sig)
		}
		local, exists := e.locals[name]
		if exists {
			return e.emitClosureCall(expr, local.alloca, local.typ)
		}
		global, gExists := e.globals[e.global(name)]
		if gExists {
			return e.emitClosureCall(expr, global.name, global.typ)
		}
	}

	if scope, ok := expr.Function.(*ast.ScopeAccessExpr); ok {
		mangled := scope.Module.Value + "__" + scope.Member.Value

		if fn, ok := runtimeBuiltins[mangled]; ok {
			switch fn {
			case "sydney_file_open":
				return e.emitFileOpen(expr)
			case "sydney_file_read":
				return e.emitFileRead(expr)
			case "sydney_file_write":
				return e.emitFileWrite(expr)
			case "sydney_file_close":
				return e.emitFileClose(expr)
			case "sydney_atof":
				return e.emitStrToFloatCall(expr)
			}
		}

		sig, exists := e.funcSigs[mangled]
		if exists {
			return e.emitFunctionCall(expr, sig)
		}
	}

	// Check for interface method call via mangled name
	if expr.MangledName != "" {
		return e.emitStructMethodCall(expr)
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
	global, isGlobal := e.globals[e.global(stmt.Name.Value)]
	if e.inFunc {
		isGlobal = false
	}

	val, valType := e.emitExpr(stmt.Value)
	if valType == IrUnit {
		val, valType = e.getZeroValue(stmt.Type)
	}

	name := stmt.Name.Value
	if isGlobal {
		name = global.name
		line := fmt.Sprintf("store %s %s, ptr %s", valType, val, name)
		e.emit(line)
		line = fmt.Sprintf("call void @sydney_gc_add_global_root(ptr %s)", name)
		e.emit(line)
	} else if e.currentModule != "" && !e.inFunc {
		name = e.global(e.moduleMangle(e.currentModule, name))
		line := fmt.Sprintf("store %s %s, ptr %s", valType, val, name)
		e.emit(line)
		line = fmt.Sprintf("call void @sydney_gc_add_global_root(ptr %s)", name)
		e.emit(line)
	} else {
		allocaName := e.alloca(name)
		e.emitAlloca(allocaName, valType)

		line := fmt.Sprintf("store %s %s, ptr %s", valType, val, allocaName)
		e.locals[name] = irLocal{alloca: allocaName, typ: valType}
		e.emit(line)
	}

	return val, valType
}

func (e *Emitter) emitVariableAssignment(stmt *ast.VarAssignmentStmt) (string, IrType) {
	val, valType := e.emitExpr(stmt.Value)
	name := stmt.Identifier.Value
	irRep, ok := e.locals[name]
	var irName string
	if ok {
		irName = irRep.alloca
	} else {
		global, _ := e.globals[e.global(name)]
		irName = global.name
	}
	line := fmt.Sprintf("store %s %s, ptr %s", valType, val, irName)
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

func (e *Emitter) getZeroValueFromIrType(t IrType) string {
	switch t {
	case IrInt:
		return "0"
	case IrFloat:
		return "0.0"
	case IrBool:
		return "0"
	case IrInt8:
		return "0"
	}

	return "null"
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

	t := st.ResolvedType.(types.StructType)
	for i, fieldName := range t.Fields {
		if fieldName == val {
			fieldIdx = i
			break
		}
	}

	strPtr, _ := e.emitExpr(st.Left) // %t0 = load ptr, ptr %p.addr

	tmp := e.tmp()
	line := fmt.Sprintf("%s = getelementptr %%struct.%s, ptr %s, i32 0, i32 %d", tmp, t.Name, strPtr, fieldIdx)
	e.emit(line)

	val, valType := e.emitExpr(stmt.Value)

	line = fmt.Sprintf("store %s %s, ptr %s", valType, val, tmp)
	e.emit(line)

	return "", IrUnit
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

func (e *Emitter) emitForStmt(stmt *ast.ForStmt) (string, IrType) {
	condLabel := e.label("cond")
	loopLabel := e.label("loop")
	postLabel := ""
	if stmt.Post != nil {
		postLabel = e.label("post")
	}
	escapeLabel := e.label("escape")

	e.enterLoop(condLabel, postLabel, escapeLabel)

	e.pushScope()
	if stmt.Init != nil {
		e.emitStmt(stmt.Init)
	}

	// so we can branch back here
	e.emitJmp(condLabel)
	e.emitLabel(condLabel)
	cond, _ := e.emitExpr(stmt.Condition)
	e.emitBranch(cond, loopLabel, escapeLabel)

	// set loop body
	e.emitLabel(loopLabel)
	e.emitBlock(stmt.Body)

	if stmt.Post != nil {
		e.emitJmp(postLabel)
		e.emitLabel(postLabel)
		e.emitStmt(stmt.Post)
	}

	// jump back to above condition
	e.emitJmp(condLabel)

	// how to get out of loop
	e.emitLabel(escapeLabel)
	e.popScope()
	e.leaveLoop()

	return "", IrUnit
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
		e.emitAlloca(resultAddr, IrInt) // i think this is wrong, and the if expr needs a resolved type
	}

	falseLabel := mergeLabel
	if hasElse {
		falseLabel = elseLabel
	}
	// branch to our labels
	e.emitBranch(cond, thenLabel, falseLabel)

	// consequence block
	e.emitLabel(thenLabel)
	e.pushScope()
	thenVal, thenType, _ := e.emitBlock(expr.Consequence)
	e.popScope()
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
		e.pushScope()
		elseVal, _, _ := e.emitBlock(expr.Alternative)
		e.popScope()
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

func (e *Emitter) emitFunction(decl *ast.FunctionDeclarationStmt) (string, IrType) {
	// construct signature struct
	fType, _ := decl.Type.(types.FunctionType)
	paramIrTypes := make([]IrType, len(decl.Params))
	paramParts := make([]string, len(decl.Params))

	for i, p := range fType.Params {
		t := SydneyTypeToIrType(p)
		paramIrTypes[i] = t
		paramParts[i] = fmt.Sprintf("%s %%%s", t, decl.Params[i].Value)
	}

	name := decl.Name.Value
	if decl.MangledName != "" {
		name = decl.MangledName
	}
	if e.currentModule != "" {
		name = e.moduleMangle(e.currentModule, name)
	}
	if _, isRuntime := runtimeBuiltins[name]; isRuntime {
		return "", IrUnit
	}

	ret := SydneyTypeToIrType(fType.Return)

	e.funcSigs[name] = funcSig{name: "@" + name, paramTypes: paramIrTypes, retType: ret}
	if decl.MangledName != "" {
		e.funcSigs[decl.Name.Value] = e.funcSigs[name] // declare mangled
		if e.currentModule != "" {
			e.funcSigs[e.moduleMangle(e.currentModule, decl.Name.Value)] = e.funcSigs[name] // declare non mangled module export
		}

	}

	if decl.IsExtern {
		return "", IrUnit
	}

	state := e.beginFunction()

	argStr := strings.Join(paramParts, ", ")
	line := fmt.Sprintf("define %s @%s(%s) {", ret, name, argStr)
	e.emitBypass(line)

	e.depth = 1

	for i, paramName := range decl.Params {
		pName := paramName.Value
		allocaName := e.alloca(pName)
		e.emitAlloca(allocaName, paramIrTypes[i])

		line = fmt.Sprintf("store %s %%%s, ptr %s", paramIrTypes[i], pName, allocaName)
		e.emit(line)
		e.locals[pName] = irLocal{alloca: allocaName, typ: SydneyTypeToIrType(fType.Params[i])}
	}

	e.depth = 0
	e.pushScope()
	val, valType, hasReturn := e.emitBlock(decl.Body)
	e.depth = 1
	e.popScope()
	if !hasReturn {
		if ret == IrUnit {
			e.emit("ret void")
		} else {
			line = fmt.Sprintf("ret %s %s", valType, val)
			e.emit(line)
		}
	}

	e.assembleFunction(e.buf)
	e.emitBypass("}\n\n")
	e.endFunction(state)

	return val, valType
}

func (e *Emitter) emitClosure(expr *ast.FunctionLiteral) (string, IrType) {
	// no captures
	// define i64 @anon.0(ptr %env, i64 %x) {
	//entry:
	//  %x.addr = alloca i64
	//  store i64 %x, ptr %x.addr
	//  %t0 = load i64, ptr %x.addr
	//  %t1 = add i64 %t0, 1
	//  ret i64 %t1
	//}

	anon := e.anon()
	freeVars := e.findFreeVars(expr.Body, expr.Parameters)

	envTypes := make([]string, len(freeVars))
	for i, fv := range freeVars {
		envTypes[i] = fv.typ.String()
	}
	envTypeStr := strings.Join(envTypes, ", ")
	retType := SydneyTypeToIrType(expr.Type.Return)

	paramIrTypes := make([]IrType, len(expr.Parameters))
	paramParts := make([]string, len(expr.Parameters))

	for i, p := range expr.Type.Params {
		t := SydneyTypeToIrType(p)
		paramIrTypes[i] = t
		paramParts[i] = fmt.Sprintf("%s %%%s", t, expr.Parameters[i].Value)
	}

	argStr := strings.Join(paramParts, ", ")

	// save state
	closureBuf := &bytes.Buffer{}
	state := e.beginFunction()

	var line string
	if len(paramParts) > 0 {
		line = fmt.Sprintf("define %s %s(ptr %%env, %s) {", retType, anon, argStr)
	} else {
		line = fmt.Sprintf("define %s %s(ptr %%env) {", retType, anon)
	}

	closureBuf.WriteString(line + "\n")

	e.depth = 1

	// store free vars in function body
	for i, fv := range freeVars {
		gepPtr := e.tmp()
		line = fmt.Sprintf("%s = getelementptr { %s }, ptr %%env, i32 0, i32 %d", gepPtr, envTypeStr, i)
		e.emit(line)

		loaded := e.tmp()
		line = fmt.Sprintf("%s = load %s, ptr %s", loaded, fv.typ, gepPtr)
		e.emit(line)

		allocaName := e.alloca(fv.name)
		e.emitAlloca(allocaName, fv.typ)

		line = fmt.Sprintf("store %s %s, ptr %s", fv.typ, loaded, allocaName)
		e.emit(line)
		e.locals[fv.name] = irLocal{allocaName, fv.typ}
	}

	// allocate params
	for i, paramName := range expr.Parameters {
		pName := paramName.Value
		allocaName := e.alloca(pName)
		e.emitAlloca(allocaName, paramIrTypes[i])

		line = fmt.Sprintf("store %s %%%s, ptr %s", paramIrTypes[i], pName, allocaName)
		e.emit(line)
		e.locals[pName] = irLocal{alloca: allocaName, typ: SydneyTypeToIrType(expr.Type.Params[i])}
	}
	e.depth = 0

	// self-reference for recursive closures
	if expr.Name != "" && e.containsIdentifier(expr.Body, expr.Name) {
		selfAlloca := "%self"
		e.emitAlloca(selfAlloca, IrFatPtr)
		selfFnSlot := e.tmp()
		line = fmt.Sprintf("%s = getelementptr { ptr, ptr }, ptr %s, i32 0, i32 0", selfFnSlot, selfAlloca)
		e.emit(line)
		line = fmt.Sprintf("store ptr %s, ptr %s", anon, selfFnSlot)
		e.emit(line)
		selfEnvSlot := e.tmp()
		line = fmt.Sprintf("%s = getelementptr { ptr, ptr }, ptr %s, i32 0, i32 1", selfEnvSlot, selfAlloca)
		e.emit(line)
		line = fmt.Sprintf("store ptr %%env, ptr %s", selfEnvSlot)
		e.emit(line)
		e.locals[expr.Name] = irLocal{selfAlloca, IrFatPtr}
	}

	e.pushScope()
	val, valType, hasReturn := e.emitBlock(expr.Body)
	e.popScope()

	e.depth = 1
	if !hasReturn {
		if retType == IrUnit {
			e.emit("ret void")
		} else {
			line = fmt.Sprintf("ret %s %s", valType, val)
			e.emit(line)
		}
	}

	// assemble function into temporay closure buffer
	e.assembleFunction(closureBuf)
	closureBuf.WriteString("}\n\n")

	// restore state
	e.endFunction(state)

	e.funcBuf.Write(closureBuf.Bytes())

	// allocate closure and store
	closure := e.tmp()
	line = fmt.Sprintf("%s = call ptr @sydney_gc_alloc(i64 16)", closure)
	e.emit(line)
	fnSlot := e.tmp()
	line = fmt.Sprintf("%s = getelementptr { ptr, ptr }, ptr %s, i32 0, i32 0", fnSlot, closure)
	e.emit(line)
	line = fmt.Sprintf("store ptr %s, ptr %s", anon, fnSlot)
	e.emit(line)
	envSlot := e.tmp()
	line = fmt.Sprintf("%s = getelementptr { ptr, ptr }, ptr %s, i32 0, i32 1", envSlot, closure)
	e.emit(line)

	// create env ptr
	if len(freeVars) > 0 {
		size := len(freeVars) * 8
		envPtr := e.tmp()
		line = fmt.Sprintf("%s = call ptr @sydney_gc_alloc(i64 %d)", envPtr, size)
		e.emit(line)

		for i, fv := range freeVars {
			slot := e.tmp()
			line = fmt.Sprintf("%s = getelementptr { %s }, ptr %s, i32 0, i32 %d", slot, envTypeStr, envPtr, i)
			e.emit(line)

			fvVal := e.tmp()
			local := state.locals[fv.name]
			line = fmt.Sprintf("%s = load %s, ptr %s", fvVal, fv.typ, local.alloca)
			e.emit(line)

			line = fmt.Sprintf("store %s %s, ptr %s", fv.typ, fvVal, slot)
			e.emit(line)
		}

		line = fmt.Sprintf("store ptr %s, ptr %s", envPtr, envSlot)
		e.emit(line)
	} else {
		line = fmt.Sprintf("store ptr null, ptr %s", envSlot)
		e.emit(line) // null env for no captures
	}

	return closure, IrPtr
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

	val := expr.Value.(*ast.Identifier).Value // this needs to be changed since we might have Circle.Point.x

	t := expr.ResolvedType.(types.StructType)
	fieldIdx := -1
	for i, fieldName := range t.Fields {
		if fieldName == val {
			fieldIdx = i
			break
		}
	}
	retType := SydneyTypeToIrType(t.Types[fieldIdx])

	structPtr, _ := e.emitExpr(expr.Left)

	gepTmp := e.tmp()

	line := fmt.Sprintf("%s = getelementptr %%struct.%s, ptr %s, i32 0, i32 %d", gepTmp, t.Name, structPtr, fieldIdx)
	e.emit(line)

	result := e.tmp()
	line = fmt.Sprintf("%s = load %s, ptr %s", result, retType, gepTmp)
	e.emit(line)

	return result, retType
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

	st := lit.ResolvedType.(types.StructType)
	for i, fieldName := range st.Fields {
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

func (e *Emitter) emitClosureCall(expr *ast.CallExpr, name string, typ IrType) (string, IrType) {
	var closurePtr string
	if typ == IrFatPtr {
		closurePtr = name
	} else {
		closurePtr = e.tmp()
		line := fmt.Sprintf("%s = load ptr, ptr %s", closurePtr, name)
		e.emit(line)
	}

	fnAddr := e.tmp()
	line := fmt.Sprintf("%s = getelementptr { ptr, ptr }, ptr %s, i32 0, i32 0", fnAddr, closurePtr)
	e.emit(line)
	fnPtr := e.tmp()
	line = fmt.Sprintf("%s = load ptr, ptr %s", fnPtr, fnAddr)
	e.emit(line)

	envAddr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr { ptr, ptr }, ptr %s, i32 0, i32 1", envAddr, closurePtr)
	e.emit(line)
	envPtr := e.tmp()
	line = fmt.Sprintf("%s = load ptr, ptr %s", envPtr, envAddr)
	e.emit(line)

	args := []string{fmt.Sprintf("ptr %s", envPtr)}
	for _, arg := range expr.Arguments {
		val, valType := e.emitExpr(arg)
		args = append(args, fmt.Sprintf("%s %s", valType, val))
	}
	argStr := strings.Join(args, ", ")

	retType := SydneyTypeToIrType(expr.ResolvedType)

	if retType == IrUnit {
		line = fmt.Sprintf("call void %s(%s)", fnPtr, argStr)
		e.emit(line)
		return "", IrUnit
	}

	result := e.tmp()
	line = fmt.Sprintf("%s = call %s %s(%s)", result, retType, fnPtr, argStr)
	e.emit(line)

	return result, retType
}

func (e *Emitter) emitFunctionCall(expr *ast.CallExpr, sig funcSig) (string, IrType) {
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

func (e *Emitter) findFreeVars(stmt *ast.BlockStmt, params []*ast.Identifier) []freeVar {
	paramSet := make(map[string]bool)
	for _, p := range params {
		paramSet[p.Value] = true
	}

	seen := make(map[string]bool)

	var freeVars []freeVar

	var walk func(node ast.Node)
	walk = func(node ast.Node) {
		if node == nil {
			return
		}

		switch n := node.(type) {
		case *ast.Identifier:
			if !paramSet[n.Value] && !seen[n.Value] {
				if local, ok := e.locals[n.Value]; ok {
					seen[n.Value] = true
					freeVars = append(freeVars, freeVar{
						name: n.Value,
						typ:  local.typ,
					})
				}
			}
		case *ast.BlockStmt:
			for _, s := range n.Stmts {
				walk(s)
			}
		case *ast.ExpressionStmt:
			walk(n.Expr)
		case *ast.VarDeclarationStmt:
			paramSet[n.Name.Value] = true
			walk(n.Value)
		case *ast.VarAssignmentStmt:
			walk(n.Value)
		case *ast.IndexAssignmentStmt:
			walk(n.Left)
			walk(n.Value)
		case *ast.SelectorAssignmentStmt:
			walk(n.Left)
			walk(n.Value)
		case *ast.ReturnStmt:
			walk(n.ReturnValue)
		case *ast.ForStmt:
			walk(n.Condition)
			walk(n.Body)
		case *ast.InfixExpr:
			walk(n.Left)
			walk(n.Right)
		case *ast.PrefixExpr:
			walk(n.Right)
		case *ast.IfExpr:
			walk(n.Condition)
			walk(n.Consequence)
			if n.Alternative != nil {
				walk(n.Alternative)
			}
		case *ast.CallExpr:
			walk(n.Function)
			for _, arg := range n.Arguments {
				walk(arg)
			}
		case *ast.IndexExpr:
			walk(n.Left)
			walk(n.Index)
		case *ast.SelectorExpr:
			walk(n.Left)
			// value is a name
		case *ast.ArrayLiteral:
			for _, elem := range n.Elements {
				walk(elem)
			}
		case *ast.FunctionLiteral:
			walk(n.Body)

		}
	}

	for _, s := range stmt.Stmts {
		walk(s)
	}

	return freeVars
}

func (e *Emitter) emitArrayIndexAssignment(stmt *ast.IndexAssignmentStmt) (string, IrType) {
	elemPtr, elemType := e.emitArrayElementPtr(stmt.Left)
	val, _ := e.emitExpr(stmt.Value)
	line := fmt.Sprintf("store %s %s, ptr %s", elemType, val, elemPtr)
	e.emit(line)

	return "", IrUnit
}

func (e *Emitter) emitMapIndexAssignment(stmt *ast.IndexAssignmentStmt) (string, IrType) {
	mapPtr, _ := e.emitExpr(stmt.Left.Left)
	idx, _ := e.emitExpr(stmt.Left.Index)

	val, valType := e.emitExpr(stmt.Value)
	if valType == IrPtr {
		val = e.emitPtrToInt(val)
	}

	var line string
	if stmt.Left.Index.GetResolvedType() == types.String {
		line = fmt.Sprintf("call void @sydney_map_set_str(ptr %s, ptr %s, i64 %s)", mapPtr, idx, val)
	} else {
		line = fmt.Sprintf("call void @sydney_map_set_int(ptr %s, i64 %s, i64 %s)", mapPtr, idx, val)
	}
	e.emit(line)

	return "", IrUnit
}

func (e *Emitter) emitArrayElementPtr(expr *ast.IndexExpr) (string, IrType) {
	// 1. Load the header from the variable
	//  2. GEP index 1 to get the data pointer
	//  3. Load the data pointer
	//  4. GEP with the index into the data buffer
	idx, _ := e.emitExpr(expr.Index)

	headPtr, _ := e.emitExpr(expr.Left)

	dataPtr := e.tmp()
	line := fmt.Sprintf("%s = getelementptr { i64, ptr }, ptr %s, i32 0, i32 1", dataPtr, headPtr)
	e.emit(line)

	data := e.tmp()
	line = fmt.Sprintf("%s = load ptr, ptr %s", data, dataPtr)
	e.emit(line)

	elemType := SydneyTypeToIrType(expr.ResolvedType)

	elemPtr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i64 %s", elemPtr, elemType, data, idx)
	e.emit(line)

	return elemPtr, elemType
}

func (e *Emitter) emitArrayIndexExpr(expr *ast.IndexExpr) (string, IrType) {
	//  5. Load the element
	elemPtr, elemType := e.emitArrayElementPtr(expr)

	result := e.tmp()
	line := fmt.Sprintf("%s = load %s, ptr %s", result, elemType, elemPtr)
	e.emit(line)

	return result, elemType
}

func (e *Emitter) emitMapIndexExpr(expr *ast.IndexExpr) (string, IrType) {
	mapPtr, _ := e.emitExpr(expr.Left)

	var line string
	key, keyType := e.emitExpr(expr.Index)
	val := e.tmp()
	valType := IrInt
	if expr.Index.GetResolvedType() == types.String {
		line = fmt.Sprintf("%s = call i64 @sydney_map_get_str(ptr %s, ptr %s)", val, mapPtr, key)
	} else {
		line = fmt.Sprintf("%s = call i64 @sydney_map_get_int(ptr %s, %s %s)", val, mapPtr, keyType, key)
	}
	e.emit(line)

	switch t := expr.GetResolvedType().(type) {
	case types.BasicType:
		if t == types.String {
			val = e.emitIntToPtr(val)
			valType = IrPtr
		}
	case types.MapType:
		val = e.emitIntToPtr(val)
		valType = IrPtr
	case types.ArrayType:
		val = e.emitIntToPtr(val)
		valType = IrPtr
	case types.StructType:
		val = e.emitIntToPtr(val)
		valType = IrPtr
	}

	return val, valType
}

func (e *Emitter) emitStringIndexExpr(expr *ast.IndexExpr) (string, IrType) {
	str, _ := e.emitExpr(expr.Left)
	idx, _ := e.emitExpr(expr.Index)

	elemPtr := e.tmp()
	line := fmt.Sprintf("%s = getelementptr i8, ptr %s, i64 %s", elemPtr, str, idx)
	e.emit(line)

	b := e.tmp()
	line = fmt.Sprintf("%s = load i8, ptr %s", b, elemPtr)
	e.emit(line)
	return b, IrInt8
}

func (e *Emitter) emitArrayLiteral(arr *ast.ArrayLiteral) (string, IrType) {
	//  { i64, ptr }   ; { length, data_ptr }
	// Where data_ptr points to a gc_alloc'd buffer of N * element_size bytes.
	//
	//  For [1, 2, 3] the IR would look like:
	//
	//  ; Allocate data buffer (3 elements * 8 bytes)
	//  %t0 = call ptr @sydney_gc_alloc(i64 24)
	l := len(arr.Elements) * 8 // 8 bytes since we use 64bit sizes
	buf := e.tmp()
	line := fmt.Sprintf("%s = call ptr @sydney_gc_alloc(i64 %d)", buf, l)

	//  Store each element
	//  %t1 = getelementptr i64, ptr %t0, i32 0
	//  store i64 1, ptr %t1
	//  %t2 = getelementptr i64, ptr %t0, i32 1
	//  store i64 2, ptr %t2
	e.emit(line)
	for i, element := range arr.Elements {
		val, valType := e.emitExpr(element)
		elemPtr := e.tmp()
		line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 %d", elemPtr, valType, buf, i)
		e.emit(line)
		line = fmt.Sprintf("store %s %s, ptr %s", valType, val, elemPtr)
		e.emit(line)
	}

	//  Allocate the header struct { len, data }
	//  %t4 = call ptr @sydney_gc_alloc(i64 16)
	//  %t5 = getelementptr { i64, ptr }, ptr %t4, i32 0, i32 0
	//  store i64 3, ptr %t5
	//  %t6 = getelementptr { i64, ptr }, ptr %t4, i32 0, i32 1
	//  store ptr %t0, ptr %t6
	headerPtr := e.tmp()
	line = fmt.Sprintf("%s = call ptr @sydney_gc_alloc(i64 16)", headerPtr) // 2 bytes since header is fixed size
	e.emit(line)
	lenPtr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr { i64, ptr }, ptr %s, i32 0, i32 0", lenPtr, headerPtr)
	e.emit(line)
	line = fmt.Sprintf("store i64 %d, ptr %s", len(arr.Elements), lenPtr) // store length
	e.emit(line)
	dPtr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr { i64, ptr }, ptr %s, i32 0, i32 1", dPtr, headerPtr) // store data
	e.emit(line)
	line = fmt.Sprintf("store ptr %s, ptr %s", buf, dPtr)
	e.emit(line)

	//  ; result is %t4 (ptr to the header)
	return headerPtr, IrPtr
}

func (e *Emitter) emitHashLiteral(lit *ast.HashLiteral) (string, IrType) {
	result := e.tmp()
	t := lit.ResolvedType.(types.MapType)
	var line string
	switch t.KeyType {
	case types.Int:
		line = fmt.Sprintf("%s = call ptr @sydney_map_create_int()", result)
	case types.Bool:
		line = fmt.Sprintf("%s = call ptr @sydney_map_create_int()", result)
	case types.String:
		line = fmt.Sprintf("%s = call ptr @sydney_map_create_string()", result)
	}
	e.emit(line)

	sortedKeys := make([]ast.Expr, 0, len(lit.Pairs))
	for k := range lit.Pairs {
		sortedKeys = append(sortedKeys, k)
	}
	slices.SortFunc(sortedKeys, func(a, b ast.Expr) int {
		return strings.Compare(a.String(), b.String())
	})
	for _, k := range sortedKeys {
		v := lit.Pairs[k]
		key, _ := e.emitExpr(k)
		val, valType := e.emitExpr(v)
		if valType == IrPtr {
			val = e.emitPtrToInt(val)
		}

		if t.KeyType == types.String {
			line = fmt.Sprintf("call void @sydney_map_set_str(ptr %s, ptr %s, i64 %s)", result, key, val)
		} else {
			line = fmt.Sprintf("call void @sydney_map_set_int(ptr %s, i64 %s, i64 %s)", result, key, val)
		}

		e.emit(line)
	}

	return result, IrPtr
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

func (e *Emitter) emitPtrToInt(val string) string {
	asInt := e.tmp()
	line := fmt.Sprintf("%s = ptrtoint ptr %s to i64", asInt, val)
	e.emit(line)
	return asInt
}

func (e *Emitter) emitIntToPtr(val string) string {
	asPtr := e.tmp()
	line := fmt.Sprintf("%s = inttoptr i64 %s to ptr", asPtr, val)
	e.emit(line)
	return asPtr
}

func (e *Emitter) containsIdentifier(node ast.Node, name string) bool {
	if node == nil {
		return false
	}
	if ident, ok := node.(*ast.Identifier); ok {
		return ident.Value == name
	}
	switch node := node.(type) {
	case *ast.BlockStmt:
		for _, stmt := range node.Stmts {
			if e.containsIdentifier(stmt, name) {
				return true
			}
		}
	case *ast.ExpressionStmt:
		return e.containsIdentifier(node.Expr, name)
	case *ast.ReturnStmt:
		return e.containsIdentifier(node.ReturnValue, name)
	case *ast.CallExpr:
		if e.containsIdentifier(node.Function, name) {
			return true
		}
		for _, arg := range node.Arguments {
			if e.containsIdentifier(arg, name) {
				return true
			}
		}
	case *ast.PrefixExpr:
		return e.containsIdentifier(node.Right, name)
	case *ast.InfixExpr:
		return e.containsIdentifier(node.Left, name) || e.containsIdentifier(node.Right, name)
	case *ast.IfExpr:
		return e.containsIdentifier(node.Condition, name) || e.containsIdentifier(node.Consequence, name) || (node.Alternative != nil && e.containsIdentifier(node.Alternative, name))
	case *ast.ForStmt:
		return e.containsIdentifier(node.Body, name) || e.containsIdentifier(node.Condition, name)
	case *ast.VarDeclarationStmt:
		return e.containsIdentifier(node.Value, name)
	case *ast.VarAssignmentStmt:
		return e.containsIdentifier(node.Value, name)
	case *ast.IndexExpr:
		return e.containsIdentifier(node.Left, name) || e.containsIdentifier(node.Index, name)
	case *ast.SelectorExpr:
		return e.containsIdentifier(node.Left, name)
	case *ast.FunctionLiteral:
		return e.containsIdentifier(node.Body, name)
	}

	return false
}

func (e *Emitter) emitScopeAccessExpr(expr *ast.ScopeAccessExpr) (string, IrType) {
	mangled := expr.Module.Value + "__" + expr.Member.Value

	if g, ok := e.globals[e.global(mangled)]; ok {
		val := e.tmp()
		line := fmt.Sprintf("%s = load %s, ptr %s", val, g.typ, g.name)
		e.emit(line)
		return val, g.typ
	}

	if sig, ok := e.funcSigs[mangled]; ok {
		return sig.name, IrPtr
	}

	return "", IrUnit
}

func (e *Emitter) moduleMangle(module, name string) string {
	return module + "__" + name
}

func (e *Emitter) emitPackageInit(pkg *loader.Package) {
	line := fmt.Sprintf("define void @%s__init() {", e.currentModule)
	e.emit(line)
	e.depth = 1
	for _, program := range pkg.Programs {
		e.emitPackageInitInner(program)
	}
	line = fmt.Sprintf("ret void")
	e.emit(line)
	e.depth = 0
	line = fmt.Sprintf("}\n")
	e.emit(line)
}

func (e *Emitter) emitPackageInitInner(node ast.Node) {
	if node == nil {
		return
	}
	switch node := node.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			e.emitPackageInitInner(stmt)
		}
	case *ast.PubStatement:
		e.emitPackageInitInner(node.Stmt)
	case *ast.VarDeclarationStmt:
		e.emitVarDecl(node)
	}
}

func (e *Emitter) emitPackageInits() {
	for _, m := range e.emittedModules {
		line := fmt.Sprintf("call void @%s__init()", m)
		e.emit(line)
	}
}

func (e *Emitter) emitResultConstructorCall(isOk bool, expr *ast.CallExpr) (string, IrType) {
	arg, argTyp := e.emitExpr(expr.Arguments[0])

	var typ IrType
	if !isOk {
		rt := expr.ResolvedType.(*types.ResultType)
		argTyp = SydneyTypeToIrType(rt.T)
	}

	typ = GetResultTaggedUnion(argTyp)

	result := e.tmp()
	e.emitAlloca(result, typ)
	okPtr := e.tmp()
	line := fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 0", okPtr, typ, result)
	e.emit(line)

	if isOk {
		line = fmt.Sprintf("store i1 %d, ptr %s", 1, okPtr)
		e.emit(line)

		valPtr := e.tmp()
		line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 1", valPtr, typ, result)
		e.emit(line)

		line = fmt.Sprintf("store %s %s, ptr %s", argTyp, arg, valPtr)
		e.emit(line)

		errPtr := e.tmp()
		line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 2", errPtr, typ, result)
		e.emit(line)

		line = fmt.Sprintf("store ptr null, ptr %s", errPtr)
		e.emit(line)
	} else {
		line = fmt.Sprintf("store i1 %d, ptr %s", 0, okPtr)
		e.emit(line)

		valPtr := e.tmp()
		line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 1", valPtr, typ, result)
		e.emit(line)

		line = fmt.Sprintf("store %s %s, ptr %s", argTyp, e.getZeroValueFromIrType(argTyp), valPtr)
		e.emit(line)

		errPtr := e.tmp()
		line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 2", errPtr, typ, result)
		e.emit(line)

		line = fmt.Sprintf("store ptr %s, ptr %s", arg, errPtr)
		e.emit(line)
	}

	return result, IrPtr
}

func (e *Emitter) emitMatchExpr(expr *ast.MatchExpr) (string, IrType) {
	subj, _ := e.emitExpr(expr.Subject)
	rt := SydneyTypeToIrType(expr.ResolvedType)
	innerType := SydneyTypeToIrType(expr.SubjectType)
	ut := GetResultTaggedUnion(innerType)

	// load tag
	tagPtr := e.tmp()
	line := fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 0", tagPtr, ut, subj)
	e.emit(line)
	tag := e.tmp()
	line = fmt.Sprintf("%s = load i1, ptr %s", tag, tagPtr)
	e.emit(line)

	// set up label
	okLab := e.label("match.ok")
	errLab := e.label("match.err")
	endLab := e.label("match.end")

	// emit result alloca, replace with phi node later
	result := ""
	if rt != IrUnit {
		result = e.tmp()
		e.emitAlloca(result, rt)
	}

	// emit branches
	e.emitBranch(tag, okLab, errLab)

	// ok branch
	e.emitLabel(okLab)
	valPtr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 1", valPtr, ut, subj)
	e.emit(line)
	val := e.tmp()
	line = fmt.Sprintf("%s = load %s, ptr %s", val, innerType, valPtr)
	e.emit(line)
	// bind
	bindAlloca := e.tmp() + ".addr"
	e.emitAlloca(bindAlloca, innerType)
	line = fmt.Sprintf("store %s %s, ptr %s", innerType, val, bindAlloca)
	e.emit(line)
	e.pushScope()
	e.locals[expr.OkArm.Pattern.Binding.Value] = irLocal{alloca: bindAlloca, typ: innerType}
	// emit block
	okResult, _, _ := e.emitBlock(expr.OkArm.Body)
	e.popScope()
	if rt != IrUnit {
		line = fmt.Sprintf("store %s %s, ptr %s", rt, okResult, result)
		e.emit(line)
	}
	e.emitJmp(endLab)

	// err branch
	e.emitLabel(errLab)
	errPtr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 2", errPtr, ut, subj)
	e.emit(line)
	err := e.tmp()
	line = fmt.Sprintf("%s = load %s, ptr %s", err, IrPtr, errPtr)
	e.emit(line)
	// bind
	bindAlloca = e.tmp() + ".addr"
	e.emitAlloca(bindAlloca, IrPtr)
	line = fmt.Sprintf("store ptr %s, ptr %s", err, bindAlloca)
	e.emit(line)
	e.pushScope()
	e.locals[expr.ErrArm.Pattern.Binding.Value] = irLocal{alloca: bindAlloca, typ: IrPtr}
	// emit block
	errResult, _, _ := e.emitBlock(expr.ErrArm.Body)
	e.popScope()
	if rt != IrUnit {
		line = fmt.Sprintf("store %s %s, ptr %s", rt, errResult, result)
		e.emit(line)
	}
	e.emitJmp(endLab)

	e.emitLabel(endLab)
	finalVal := ""
	if rt != IrUnit {
		finalVal = e.tmp()
		line = fmt.Sprintf("%s = load %s, ptr %s", finalVal, rt, result)
		e.emit(line)
	}

	return finalVal, rt
}

func (e *Emitter) emitRuntimeCall(fn string, expr *ast.CallExpr) (string, IrType) {
	switch fn {
	case "sydney_file_open":
		return e.emitFileOpen(expr)
	case "sydney_file_read":
		return e.emitFileRead(expr)
	case "sydney_file_write":
		return e.emitFileWrite(expr)
	case "sydney_file_close":
		return e.emitFileClose(expr)
	case "sydney_atof":
		return e.emitStrToFloatCall(expr)
	}

	return "", IrUnit
}

func (e *Emitter) pushScope() {
	e.scopeStack = append(e.scopeStack, e.locals)
	newLocals := make(map[string]irLocal)
	for k, v := range e.locals {
		newLocals[k] = v
	}
	e.locals = newLocals
}

func (e *Emitter) popScope() {
	e.locals = e.scopeStack[len(e.scopeStack)-1]
	e.scopeStack = e.scopeStack[:len(e.scopeStack)-1]
}

func (e *Emitter) enterLoop(condLabel, postLabel, escapeLabel string) {
	loop := &LoopLabels{
		condLabel:   condLabel,
		postLabel:   postLabel,
		escapeLabel: escapeLabel,
	}
	e.loopStack = append(e.loopStack, loop)
	e.loopIdx++
}

func (e *Emitter) leaveLoop() {
	e.loopStack = e.loopStack[:len(e.loopStack)-1]
	e.loopIdx--
}

func (e *Emitter) getLoop() *LoopLabels {
	if len(e.loopStack) == 0 {
		return nil
	}
	return e.loopStack[e.loopIdx-1]
}
