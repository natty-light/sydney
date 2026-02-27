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

func TestStructs(t *testing.T) {
	source := `define struct Point { x int, y int }
const Point p = Point { x: 0, y: 0 };
print(p.x);
print(p.y);`
	expected := `%struct.Point = type { i64, i64 }
define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_gc_alloc(i64 16)
  %t1 = getelementptr %struct.Point, ptr %t0, i32 0, i32 0
  store i64 0, ptr %t1
  %t2 = getelementptr %struct.Point, ptr %t0, i32 0, i32 1
  store i64 0, ptr %t2
  %p.addr = alloca ptr
  store ptr %t0, ptr %p.addr
  %t3 = load ptr, ptr %p.addr
  %t4 = getelementptr %struct.Point, ptr %t3, i32 0, i32 0
  %t5 = load i64, ptr %t4
  call void @sydney_print_int(i64 %t5)
  call void @sydney_print_newline()
  %t6 = load ptr, ptr %p.addr
  %t7 = getelementptr %struct.Point, ptr %t6, i32 0, i32 1
  %t8 = load i64, ptr %t7
  call void @sydney_print_int(i64 %t8)
  call void @sydney_print_newline()
  ret i32 0
}
`
	runEmitterTest(t, source, expected)
}

func TestSelectorAssignment(t *testing.T) {
	source := `define struct Point { x int, y int }
const Point p = Point { x: 0, y: 0 };
print(p.x);
print(p.y);
p.x = 1;
p.y = 2;
print(p.x);
print(p.y);`

	expected := `%struct.Point = type { i64, i64 }
define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_gc_alloc(i64 16)
  %t1 = getelementptr %struct.Point, ptr %t0, i32 0, i32 0
  store i64 0, ptr %t1
  %t2 = getelementptr %struct.Point, ptr %t0, i32 0, i32 1
  store i64 0, ptr %t2
  %p.addr = alloca ptr
  store ptr %t0, ptr %p.addr
  %t3 = load ptr, ptr %p.addr
  %t4 = getelementptr %struct.Point, ptr %t3, i32 0, i32 0
  %t5 = load i64, ptr %t4
  call void @sydney_print_int(i64 %t5)
  call void @sydney_print_newline()
  %t6 = load ptr, ptr %p.addr
  %t7 = getelementptr %struct.Point, ptr %t6, i32 0, i32 1
  %t8 = load i64, ptr %t7
  call void @sydney_print_int(i64 %t8)
  call void @sydney_print_newline()
  %t9 = load ptr, ptr %p.addr
  %t10 = getelementptr %struct.Point, ptr %t9, i32 0, i32 0
  store i64 1, ptr %t10
  %t11 = load ptr, ptr %p.addr
  %t12 = getelementptr %struct.Point, ptr %t11, i32 0, i32 1
  store i64 2, ptr %t12
  %t13 = load ptr, ptr %p.addr
  %t14 = getelementptr %struct.Point, ptr %t13, i32 0, i32 0
  %t15 = load i64, ptr %t14
  call void @sydney_print_int(i64 %t15)
  call void @sydney_print_newline()
  %t16 = load ptr, ptr %p.addr
  %t17 = getelementptr %struct.Point, ptr %t16, i32 0, i32 1
  %t18 = load i64, ptr %t17
  call void @sydney_print_int(i64 %t18)
  call void @sydney_print_newline()
  ret i32 0
}
`
	runEmitterTest(t, source, expected)
}

func TestInterfaceDispatch(t *testing.T) {
	source := `define struct Circle { radius float }
define struct Rect { w float, h float}
define interface Area { area() -> float }
define implementation Circle -> Area
define implementation Rect -> Area

func area(Circle c) -> float {
    const pi = 3.14;
    return c.radius * c.radius * pi;
}

func area(Rect r) -> float {
    return r.w * r.h;
}

const Circle c = Circle { radius: 2.0 };
const Rect r = Rect { w: 2.0, h: 2.0 };

func getArea(Area a) -> float {
    return a.area();
}

const rectA = getArea(r);
print("rect area: ", rectA);

const circA = getArea(c);
print("circle area: ", circA);`

	expected := `%struct.Circle = type { double }
%struct.Rect = type { double, double }
@.str.0 = private unnamed_addr constant [12 x i8] c"rect area: \00"
@.str.1 = private unnamed_addr constant [14 x i8] c"circle area: \00"
@vtable.Circle.Area = constant [1 x ptr] [ptr @Circle.area]
@vtable.Rect.Area = constant [1 x ptr] [ptr @Rect.area]
define double @Circle.area(ptr %c) {
entry:
  %c.addr = alloca ptr
  store ptr %c, ptr %c.addr
  %pi.addr = alloca double
  store double 3.140000, ptr %pi.addr
  %t0 = load ptr, ptr %c.addr
  %t1 = getelementptr %struct.Circle, ptr %t0, i32 0, i32 0
  %t2 = load double, ptr %t1
  %t3 = load ptr, ptr %c.addr
  %t4 = getelementptr %struct.Circle, ptr %t3, i32 0, i32 0
  %t5 = load double, ptr %t4
  %t6 = fmul double %t2, %t5
  %t7 = load double, ptr %pi.addr
  %t8 = fmul double %t6, %t7
  ret double %t8
}

define double @Rect.area(ptr %r) {
entry:
  %r.addr = alloca ptr
  store ptr %r, ptr %r.addr
  %t0 = load ptr, ptr %r.addr
  %t1 = getelementptr %struct.Rect, ptr %t0, i32 0, i32 0
  %t2 = load double, ptr %t1
  %t3 = load ptr, ptr %r.addr
  %t4 = getelementptr %struct.Rect, ptr %t3, i32 0, i32 1
  %t5 = load double, ptr %t4
  %t6 = fmul double %t2, %t5
  ret double %t6
}

define double @getArea(ptr %a) {
entry:
  %a.addr = alloca ptr
  store ptr %a, ptr %a.addr
  %t0 = load ptr, ptr %a.addr
  %t1 = load { ptr, ptr }, ptr %t0
  %t2 = extractvalue { ptr, ptr } %t1, 0
  %t3 = extractvalue { ptr, ptr } %t1, 1
  %t4 = getelementptr [1 x ptr], ptr %t3, i32 0, i32 0
  %t5 = load ptr, ptr %t4
  %t6 = call double %t5(ptr %t2)
  ret double %t6
}

define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_gc_alloc(i64 8)
  %t1 = getelementptr %struct.Circle, ptr %t0, i32 0, i32 0
  store double 2.000000, ptr %t1
  %c.addr = alloca ptr
  store ptr %t0, ptr %c.addr
  %t2 = call ptr @sydney_gc_alloc(i64 16)
  %t3 = getelementptr %struct.Rect, ptr %t2, i32 0, i32 0
  store double 2.000000, ptr %t3
  %t4 = getelementptr %struct.Rect, ptr %t2, i32 0, i32 1
  store double 2.000000, ptr %t4
  %r.addr = alloca ptr
  store ptr %t2, ptr %r.addr
  %t5 = load ptr, ptr %r.addr
  %t6 = alloca { ptr, ptr }
  %t7 = getelementptr { ptr, ptr }, ptr %t6, i32 0, i32 0
  store ptr %t5, ptr %t7
  %t8 = getelementptr { ptr, ptr }, ptr %t6, i32 0, i32 1
  store ptr @vtable.Rect.Area, ptr %t8
  %t9 = call double @getArea(ptr %t6)
  %rectA.addr = alloca double
  store double %t9, ptr %rectA.addr
  call void @sydney_print_string(ptr @.str.0)
  %t10 = load double, ptr %rectA.addr
  call void @sydney_print_float(double %t10)
  call void @sydney_print_newline()
  %t11 = load ptr, ptr %c.addr
  %t12 = alloca { ptr, ptr }
  %t13 = getelementptr { ptr, ptr }, ptr %t12, i32 0, i32 0
  store ptr %t11, ptr %t13
  %t14 = getelementptr { ptr, ptr }, ptr %t12, i32 0, i32 1
  store ptr @vtable.Circle.Area, ptr %t14
  %t15 = call double @getArea(ptr %t12)
  %circA.addr = alloca double
  store double %t15, ptr %circA.addr
  call void @sydney_print_string(ptr @.str.1)
  %t16 = load double, ptr %circA.addr
  call void @sydney_print_float(double %t16)
  call void @sydney_print_newline()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestArrays(t *testing.T) {
	source := `const array<int> a = [0, 1, 2, 3];
print(a[1]);`

	expected := `define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_gc_alloc(i64 32)
  %t1 = getelementptr i64, ptr %t0, i32 0
  store i64 0, ptr %t1
  %t2 = getelementptr i64, ptr %t0, i32 1
  store i64 1, ptr %t2
  %t3 = getelementptr i64, ptr %t0, i32 2
  store i64 2, ptr %t3
  %t4 = getelementptr i64, ptr %t0, i32 3
  store i64 3, ptr %t4
  %t5 = call ptr @sydney_gc_alloc(i64 16)
  %t6 = getelementptr { i64, ptr }, ptr %t5, i32 0, i32 0
  store i64 4, ptr %t6
  %t7 = getelementptr { i64, ptr }, ptr %t5, i32 0, i32 1
  store ptr %t0, ptr %t7
  %a.addr = alloca ptr
  store ptr %t5, ptr %a.addr
  %t8 = load ptr, ptr %a.addr
  %t9 = getelementptr { i64, ptr }, ptr %t8, i32 0, i32 1
  %t10 = load ptr, ptr %t9
  %t11 = getelementptr i64, ptr %t10, i64 1
  %t12 = load i64, ptr %t11
  call void @sydney_print_int(i64 %t12)
  call void @sydney_print_newline()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestAnonymousFunction(t *testing.T) {
	source := `const anon = func() -> int { return 1; };
print(anon());

const adder = func(int a, int b) -> int { return a + b };
const sum = adder(1, 2);
print(sum);`

	expected := `define i32 @main() {
entry: 
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_gc_alloc(i64 16)
  %t1 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 0
  store ptr @anon.0, ptr %t1
  %t2 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 1
  store ptr null, ptr %t2
  %anon.addr = alloca ptr
  store ptr %t0, ptr %anon.addr
  %t3 = load ptr, ptr %anon.addr
  %t4 = getelementptr { ptr, ptr }, ptr %t3, i32 0, i32 0
  %t5 = load ptr, ptr %t4
  %t6 = getelementptr { ptr, ptr }, ptr %t3, i32 0, i32 1
  %t7 = load ptr, ptr %t6
  %t8 = call i64 %t5(ptr %t7)
  call void @sydney_print_int(i64 %t8)
  call void @sydney_print_newline()
  %t9 = call ptr @sydney_gc_alloc(i64 16)
  %t10 = getelementptr { ptr, ptr }, ptr %t9, i32 0, i32 0
  store ptr @anon.1, ptr %t10
  %t11 = getelementptr { ptr, ptr }, ptr %t9, i32 0, i32 1
  store ptr null, ptr %t11
  %adder.addr = alloca ptr
  store ptr %t9, ptr %adder.addr
  %t12 = load ptr, ptr %adder.addr
  %t13 = getelementptr { ptr, ptr }, ptr %t12, i32 0, i32 0
  %t14 = load ptr, ptr %t13
  %t15 = getelementptr { ptr, ptr }, ptr %t12, i32 0, i32 1
  %t16 = load ptr, ptr %t15
  %t17 = call i64 %t14(ptr %t16, i64 1, i64 2)
  %sum.addr = alloca i64
  store i64 %t17, ptr %sum.addr
  %t18 = load i64, ptr %sum.addr
  call void @sydney_print_int(i64 %t18)
  call void @sydney_print_newline()
  ret i32 0
}
  define i64 @anon.0(ptr %env) {
entry:
    ret i64 1
  }
  
  define i64 @anon.1(ptr %env, i64 %a, i64 %b) {
entry:
    %a.addr = alloca i64
    store i64 %a, ptr %a.addr
    %b.addr = alloca i64
    store i64 %b, ptr %b.addr
    %t0 = load i64, ptr %a.addr
    %t1 = load i64, ptr %b.addr
    %t2 = add i64 %t0, %t1
    ret i64 %t2
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
