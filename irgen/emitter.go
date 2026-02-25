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
	retType    string   // IR return type string
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
	err := e.collect(n)
	if err != nil {
		return err
	}

	e.preamble()

	err = e.mainWrapper(n)
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

func (e *Emitter) collect(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			err := e.collect(stmt)
			if err != nil {
				return err
			}
		}
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

	err := e.collectStrings(n)
	if err != nil {
		return err
	}

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

func (e *Emitter) collectStrings(n ast.Node) error {
	switch node := n.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			err := e.collectStrings(stmt)
			if err != nil {
				return err
			}
		}
	case *ast.ExpressionStmt:
		err := e.collectStrings(node.Expr)
		if err != nil {
			return err
		}
	case *ast.BlockStmt:
		for _, stmt := range node.Stmts {
			err := e.collectStrings(stmt)
			if err != nil {
				return err
			}
		}
	case *ast.VarDeclarationStmt:
		err := e.collectStrings(node.Value)
		if err != nil {
			return err
		}
	case *ast.VarAssignmentStmt:
		err := e.collectStrings(node.Value)
		if err != nil {
			return err
		}
	case *ast.IndexAssignmentStmt:
		err := e.collectStrings(node.Value)
		if err != nil {
			return err
		}
	case *ast.ReturnStmt:
		err := e.collectStrings(node.ReturnValue)
		if err != nil {
			return err
		}
	case *ast.ForStmt:
		err := e.collectStrings(node.Condition)
		if err != nil {
			return err
		}

		err = e.collectStrings(node.Body)
		if err != nil {
			return err
		}
	case *ast.FunctionDeclarationStmt:
		err := e.collectStrings(node.Body)
		if err != nil {
			return err
		}
	case *ast.SelectorAssignmentStmt:
		err := e.collectStrings(node.Value)
		if err != nil {
			return err
		}

	case *ast.InfixExpr:
		err := e.collectStrings(node.Left)
		if err != nil {
			return err
		}
		err = e.collectStrings(node.Right)
		if err != nil {
			return err
		}
	case *ast.PrefixExpr:
		err := e.collectStrings(node.Right)
		if err != nil {
			return err
		}
	case *ast.IfExpr:
		err := e.collectStrings(node.Condition)
		if err != nil {
			return err
		}

		err = e.collectStrings(node.Consequence)
		if err != nil {
			return err
		}
		if node.Alternative != nil {
			err = e.collectStrings(node.Alternative)
			if err != nil {
				return err
			}
		}
	case *ast.IndexExpr:
		err := e.collectStrings(node.Index)
		if err != nil {
			return err
		}
	case *ast.CallExpr:
		for _, arg := range node.Arguments {
			err := e.collectStrings(arg)
			if err != nil {
				return err
			}
		}

	case *ast.StringLiteral:
		e.addStr(node.Value)
	case *ast.ArrayLiteral:
		for _, elem := range node.Elements {
			err := e.collectStrings(elem)
			if err != nil {
				return err
			}
		}
	case *ast.HashLiteral:
		for key, value := range node.Pairs {
			err := e.collectStrings(key)
			if err != nil {
				return err
			}
			err = e.collectStrings(value)
			if err != nil {
				return err
			}
		}
	case *ast.FunctionLiteral:
		err := e.collectStrings(node.Body)
		if err != nil {
			return err
		}
	case *ast.StructLiteral:
		for _, val := range node.Values {
			err := e.collectStrings(val)
			if err != nil {
				return err
			}
		}
	}

	return nil
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

func (e *Emitter) functions(node ast.Node) error {
	return nil
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
			val, valType = e.emitStmt(stmt)
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

func (e *Emitter) emitStmt(stmt ast.Node) (string, IrType) {
	switch s := stmt.(type) {
	case *ast.ExpressionStmt:
		return e.emitExpr(s.Expr)
	case *ast.VarDeclarationStmt:
		return e.emitVarDecl(s)
	case *ast.VarAssignmentStmt:
		return e.emitVariableAssignment(s)
	case *ast.ReturnStmt:
		return e.emitReturnStmt(s)
	case *ast.ForStmt:
		return e.emitForStmt(s)
	}
	return "", IrUnit
}

func (e *Emitter) emitBlock(block *ast.BlockStmt) (string, IrType) {
	e.depth++
	var lastVal string
	var lastType IrType = IrUnit
	for _, stmt := range block.Stmts {
		lastVal, lastType = e.emitStmt(stmt)
	}
	e.depth--
	return lastVal, lastType
}

func (e *Emitter) emitExpr(expr ast.Expr) (string, IrType) {
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
	case *ast.InfixExpr:
		return e.emitInfixExpr(expr)
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
			op = "ogt"
		}
		retType = IrBool
	case "<":
		//%t0 = icmp slt i64 %left, %right   ; <
		if lType == IrFloat {
			cmpType = fcmp
			op = "ole"
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
	if ident, ok := expr.Function.(*ast.Identifier); ok && ident.Value == "print" {
		arg, argType := e.emitExpr(expr.Arguments[0])
		switch argType {
		case IrInt:
			e.emit(fmt.Sprintf("call void @sydney_print_int(i64 %s)", arg))
		case IrFloat:
			e.emit(fmt.Sprintf("call void @sydney_print_float(double %s)", arg))
		case IrPtr:
			e.emit(fmt.Sprintf("call void @sydney_print_string(ptr %s)", arg))
		}
		e.emit("call void @sydney_print_newline()")
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
		val, valType = e.emitZeroValue(stmt.Type)
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

func (e *Emitter) emitZeroValue(t types.Type) (string, IrType) {
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
	thenVal, thenType := e.emitBlock(expr.Consequence)
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
		elseVal, _ := e.emitBlock(expr.Alternative)
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
