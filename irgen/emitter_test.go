package irgen

import (
	"sydney/lexer"
	"sydney/parser"
	"sydney/typechecker"
	"testing"
)

var declarations string = `declare void @sydney_print_int(i64)
declare void @sydney_print_float(double)
declare void @sydney_print_string(ptr)
declare ptr @sydney_strcat(ptr, ptr)
declare void @sydney_print_newline()
declare void @sydney_gc_init()
declare void @sydney_print_bool(i8)
declare ptr @sydney_gc_alloc(i64)
declare void @sydney_gc_collect()
declare void @sydney_gc_add_global_root(ptr)
declare void @sydney_gc_shutdown()
declare i64 @sydney_strlen(ptr)`

func TestIntInfixExpr(t *testing.T) {
	source := "print(1 + 2);"
	expected := `define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %t0 = add i64 1, 2
  call void @sydney_print_int(i64 %t0)
  call void @sydney_print_newline()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestVarDeclarations(t *testing.T) {
	source := `mut float pi = 3.0 + 0.14; print(pi);`
	expected := `define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %t0 = fadd double 3.000000, 0.140000
  %pi.addr = alloca double
  store double %t0, ptr %pi.addr
  %t1 = load double, ptr %pi.addr
  call void @sydney_print_float(double %t1)
  call void @sydney_print_newline()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestVarAssignment(t *testing.T) {
	source := `mut float pi = 3.0 + 0.14;
pi = 2.0;
print(pi);`
	expected := `define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %t0 = fadd double 3.000000, 0.140000
  %pi.addr = alloca double
  store double %t0, ptr %pi.addr
  store double 2.000000, ptr %pi.addr
  %t1 = load double, ptr %pi.addr
  call void @sydney_print_float(double %t1)
  call void @sydney_print_newline()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func buildExpected(expected string) string {
	return declarations + "\n\n" + expected
}

func runEmitterTest(t *testing.T, source string, expected string) {
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors not empty: %v", p.Errors())
	}
	c := typechecker.New(nil)
	c.Check(program)
	if len(c.Errors()) != 0 {
		t.Fatalf("typechecker errors not empty: %v", c.Errors())
	}
	e := New()
	err := e.Emit(program)
	if err != nil {
		t.Fatal(err)
	}
	actual := e.buf.String()

	expected = buildExpected(expected)
	if actual != expected {
		t.Fatalf("expected:\n%s\ngot:\n%s", expected, actual)
	}
}
