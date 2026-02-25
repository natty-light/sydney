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

func TestIfExpr(t *testing.T) {
	source := `mut int x = 0;
mut int y = 0;
if (x == y) {
    print("true");
} else {
    print("false");
}

mut z = if (x == y) { 1; } else { 0; };
print(z);`

	expected := `@.str.0 = private unnamed_addr constant [5 x i8] c"true\00"
@.str.1 = private unnamed_addr constant [6 x i8] c"false\00"
define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %x.addr = alloca i64
  store i64 0, ptr %x.addr
  %y.addr = alloca i64
  store i64 0, ptr %y.addr
  %t0 = load i64, ptr %x.addr
  %t1 = load i64, ptr %y.addr
  %t2 = icmp eq i64 %t0, %t1
  %t3 = alloca i64
  br i1 %t2, label %then.0, label %else.1
then.0:
    call void @sydney_print_string(ptr @.str.0)
    call void @sydney_print_newline()
  br label %merge.2
else.1:
    call void @sydney_print_string(ptr @.str.1)
    call void @sydney_print_newline()
  br label %merge.2
merge.2:
  %t4 = load i64, ptr %x.addr
  %t5 = load i64, ptr %y.addr
  %t6 = icmp eq i64 %t4, %t5
  %t7 = alloca i64
  br i1 %t6, label %then.3, label %else.4
then.3:
  store i64 1, ptr %t7
  br label %merge.5
else.4:
  store i64 0, ptr %t7
  br label %merge.5
merge.5:
  %t8 = load i64, ptr %t7
  %z.addr = alloca i64
  store i64 %t8, ptr %z.addr
  %t9 = load i64, ptr %z.addr
  call void @sydney_print_int(i64 %t9)
  call void @sydney_print_newline()
  ret i32 0
}
`
	runEmitterTest(t, source, expected)
}

func TestForLoop(t *testing.T) {
	source := `mut int i = 0;
for (i < 5) {
    print(i);
    i = i + 1;
}`
	expected := `define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %i.addr = alloca i64
  store i64 0, ptr %i.addr
  br label %cond.0
cond.0:
  %t0 = load i64, ptr %i.addr
  %t1 = icmp slt i64 %t0, 5
  br i1 %t1, label %loop.1, label %escape.2
loop.1:
    %t2 = load i64, ptr %i.addr
    call void @sydney_print_int(i64 %t2)
    call void @sydney_print_newline()
    %t3 = load i64, ptr %i.addr
    %t4 = add i64 %t3, 1
    store i64 %t4, ptr %i.addr
  br label %cond.0
escape.2:
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestNestedBlocks(t *testing.T) {
	source := `mut int i = 0;
for (i < 10) {
   if (i % 2 == 0) {
        print(i);
   }
   i = i + 1;
}`

	expected := `define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %i.addr = alloca i64
  store i64 0, ptr %i.addr
  br label %cond.0
cond.0:
  %t0 = load i64, ptr %i.addr
  %t1 = icmp slt i64 %t0, 10
  br i1 %t1, label %loop.1, label %escape.2
loop.1:
    %t2 = load i64, ptr %i.addr
    %t3 = srem i64 %t2, 2
    %t4 = icmp eq i64 %t3, 0
    br i1 %t4, label %then.3, label %merge.4
then.3:
      %t5 = load i64, ptr %i.addr
      call void @sydney_print_int(i64 %t5)
      call void @sydney_print_newline()
    br label %merge.4
merge.4:
    %t6 = load i64, ptr %i.addr
    %t7 = add i64 %t6, 1
    store i64 %t7, ptr %i.addr
  br label %cond.0
escape.2:
  ret i32 0
}
`
	runEmitterTest(t, source, expected)
}

func TestFunctions(t *testing.T) {
	source := `func addFive(int i) -> int {
    const int result = i + 5;
    result;
}

func addSix(int i) -> int {
    const int result = i + 6;
    return result;
}

const int x = 0;
const xPlusSix = addSix(x);
const xPlusFive = addFive(x);

print("x = ", x);
print("x + 5 = ", xPlusFive);
print("x + 6 = ", xPlusSix);`

	expected := `@.str.0 = private unnamed_addr constant [5 x i8] c"x = \00"
@.str.1 = private unnamed_addr constant [9 x i8] c"x + 5 = \00"
@.str.2 = private unnamed_addr constant [9 x i8] c"x + 6 = \00"
define i64 @addFive(i64 %i) {
entry:
  %i.addr = alloca i64
  store i64 %i, ptr %i.addr
  %t0 = load i64, ptr %i.addr
  %t1 = add i64 %t0, 5
  %result.addr = alloca i64
  store i64 %t1, ptr %result.addr
  %t2 = load i64, ptr %result.addr
  ret i64 %t2
}

define i64 @addSix(i64 %i) {
entry:
  %i.addr = alloca i64
  store i64 %i, ptr %i.addr
  %t0 = load i64, ptr %i.addr
  %t1 = add i64 %t0, 6
  %result.addr = alloca i64
  store i64 %t1, ptr %result.addr
  %t2 = load i64, ptr %result.addr
  ret i64 %t2
}

define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %x.addr = alloca i64
  store i64 0, ptr %x.addr
  %t0 = load i64, ptr %x.addr
  %t1 = call i64 @addSix(i64 %t0)
  %xPlusSix.addr = alloca i64
  store i64 %t1, ptr %xPlusSix.addr
  %t2 = load i64, ptr %x.addr
  %t3 = call i64 @addFive(i64 %t2)
  %xPlusFive.addr = alloca i64
  store i64 %t3, ptr %xPlusFive.addr
  call void @sydney_print_string(ptr @.str.0)
  %t4 = load i64, ptr %x.addr
  call void @sydney_print_int(i64 %t4)
  call void @sydney_print_newline()
  call void @sydney_print_string(ptr @.str.1)
  %t5 = load i64, ptr %xPlusFive.addr
  call void @sydney_print_int(i64 %t5)
  call void @sydney_print_newline()
  call void @sydney_print_string(ptr @.str.2)
  %t6 = load i64, ptr %xPlusSix.addr
  call void @sydney_print_int(i64 %t6)
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
