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
	locals map[string]string
	inFunc bool

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
		locals:         make(map[string]string),
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

	err = e.main(n)
	if err != nil {
		return err
	}

	return err
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
			for i, methodName := range iface.Methods {
				method := fmt.Sprintf("@%s.%s", sname, methodName)
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

		err = e.collectStrings(node.Alternative)
		if err != nil {
			return err
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

func (e *Emitter) main(node ast.Node) error {
	e.emit("define i32 @main() {")
	e.emit("call void @sydney_gc_init()")

	e.emit("ret i32 0")
	e.emit("}")

	return nil
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
	e.buf.WriteString(line + "\n")
}

func (e *Emitter) emitExpr(expr ast.Expr) (string, IrType) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", expr.Value), IrInt
	case *ast.FloatLiteral:
		return fmt.Sprintf("%f", expr.Value), IrFloat
	case *ast.InfixExpr:
		return e.emitInfixExpr(expr)
	case *ast.CallExpr:
		return e.emitCallExpr(expr)
	}
	return "", IrUnit
}

func (e *Emitter) emitInfixExpr(expr *ast.InfixExpr) (string, IrType) {
	left, lType := e.emitExpr(expr.Left)
	right, _ := e.emitExpr(expr.Right) // discarding because typechecker has enforced this
	result := e.tmp()
	var op string
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
	}
	opStr := e.infixOpStr(op, lType, left, right)
	line := fmt.Sprintf("%s = %s", result, opStr)
	e.emit(line)

	return result, lType
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

func (e *Emitter) emitStructType(t types.StructType) {
	var out bytes.Buffer
	out.WriteString("%struct.")
	out.WriteString(t.Name)
	out.WriteString(" = type { ")
	for i, tt := range t.Types {
		if i > 0 {
			out.WriteString(",")
		}
		ttt := SydneyTypeToIrType(tt)
		out.WriteString(ttt.String())
	}
	out.WriteString(" }")

	e.emit(out.String())
}

func (e *Emitter) emitStringConst(s string, idx int) {
	s, size := llvmEscapeString(s)
	line := fmt.Sprintf("@.str.%d = private unnamed_addr constant [%d x i8] c\"%s\\00\"", idx, size, s)
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
	methodStrs := make([]string, numMethods)
	for i, m := range methods {
		methodStrs[i] = fmt.Sprintf("%s @%s.%s", IrPtr, sname, m)
	}
	methodsStr := fmt.Sprintf("[%s]", strings.Join(methodStrs, ", "))
	line := fmt.Sprintf("@vtable.%s.%s = constant [%d x ptr] %s", sname, iname, numMethods, methodsStr)
	e.emit(line)
}
