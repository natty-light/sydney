package irgen

import (
	"bytes"
	"fmt"
	"os"
	"sydney/ast"
	"sydney/types"
)

type Emitter struct {
	buf    bytes.Buffer
	tmpIdx int
	lblIdx int
	locals map[string]string
	inFunc bool

	// Collected metadata
	structTypes    map[string]types.StructType
	interfaceTypes map[string]types.InterfaceType
	vtables        map[string]map[string]string // vtables[struct][iface] → @vtable global
	stringConsts   map[string]int               // string value → index (@.str.0, ...)
	stringIdx      int
}

func New() *Emitter {
	return &Emitter{
		structTypes:    make(map[string]types.StructType),
		interfaceTypes: make(map[string]types.InterfaceType),
		stringConsts:   make(map[string]int),
		stringIdx:      0,
		vtables:        make(map[string]map[string]string),

		buf:    bytes.Buffer{},
		locals: make(map[string]string),
	}
}

func (e *Emitter) Emit(n ast.Node) error {
	err := e.preamble()
	if err != nil {
		return err
	}
	err = e.main(n.(*ast.Program))
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

func (e *Emitter) Collect(node ast.Node) error {
	return nil
}

func (e *Emitter) preamble() error {
	e.emit("declare void @sydney_print_int(i64)")
	e.emit("declare void @sydney_print_float(double)")
	e.emit("declare void @sydney_print_string(ptr)")
	e.emit("declare void @sydney_strcat(ptr, ptr)")
	e.emit("declare void @sydney_print_newline()")
	e.emit("declare void @sydney_gc_init()")
	e.emit("")

	return nil
}

func (e *Emitter) functions(node ast.Node) error {
	return nil
}

func (e *Emitter) main(node *ast.Program) error {
	e.emit("define i32 @main() {")
	e.emit("call void @sydney_gc_init()")

	var err error
	for _, stmt := range node.Stmts {
		switch s := stmt.(type) {
		case *ast.ExpressionStmt:
			e.emitExpr(s.Expr)
		}
	}

	e.emit("ret i32 0")
	e.emit("}")

	return err
}

func (e *Emitter) tmp() string {
	name := fmt.Sprintf("%%t%d", e.tmpIdx)
	e.tmpIdx++
	return name
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
		case IrString:
			line := "%s = " + fmt.Sprintf("call ptr @sydney_strcat(%s, %s)", left, right)
			e.emit(line)
			return result, IrString
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
			op = "div"
		}
	}
	opStr := e.infixOpStr(op, lType, left, right)
	line := "%t0 = " + opStr
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
		case IrString:
			e.emit(fmt.Sprintf("call void @sydney_print_string(ptr %s)", arg))
		}
		e.emit("call void @sydney_print_newline()")
	}

	return "", IrUnit
}

func (e *Emitter) infixOpStr(op string, t IrType, left, right string) string {
	return fmt.Sprintf("%s %s %s, %s", op, t, left, right)
}
