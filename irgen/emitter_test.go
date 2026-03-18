package irgen

import (
	"os"
	"os/exec"
	"path/filepath"
	"sydney/ast"
	"sydney/lexer"
	"sydney/parser"
	"sydney/typechecker"
	"testing"
)

var declarations = `declare void @sydney_print_int(i64)
declare void @sydney_print_float(double)
declare void @sydney_print_string(ptr)
declare void @sydney_print_byte(i8)
declare ptr @sydney_strcat(ptr, ptr)
declare void @sydney_print_newline()
declare void @sydney_gc_init()
declare void @sydney_print_bool(i8)
declare ptr @sydney_gc_alloc(i64)
declare void @sydney_gc_collect()
declare void @sydney_gc_add_global_root(ptr)
declare void @sydney_gc_shutdown()
declare i64 @sydney_strlen(ptr)
declare ptr @sydney_map_create_int()
declare ptr @sydney_map_create_string()
declare void @sydney_map_set_str(ptr, ptr, i64)
declare i64 @sydney_map_get_str(ptr, ptr)
declare void @sydney_map_set_int(ptr, i64, i64)
declare i64 @sydney_map_get_int(ptr, i64)
declare i64 @sydney_file_open(ptr)
declare ptr @sydney_file_read(i64)
declare i64 @sydney_file_write(i64, ptr)
declare i64 @sydney_file_close(i64)
declare ptr @sydney_get_last_error()
declare ptr @sydney_byte_to_string(i8)
declare void @llvm.memcpy.p0.p0.i64(ptr, ptr, i64, i1)
declare i1 @sydney_str_equals(ptr, ptr)
declare ptr @sydney_map_keys_str(ptr)
declare ptr @sydney_map_values_str(ptr)
declare ptr @sydney_map_keys_int(ptr)
declare ptr @sydney_map_values_int(ptr)
declare ptr @sydney_atof(ptr)
declare void @sydney_panic(ptr)
declare i64 @sydney_channel_create(i64)
declare void @sydney_channel_send(i64, i64)
declare i64 @sydney_channel_recv(i64)
declare void @sydney_spawn(ptr, ptr)
declare void @sydney_join_all()`

func TestIntInfixExpr(t *testing.T) {
	source := "print(1 + 2);"
	expected := `define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = add i64 1, 2
  call void @sydney_print_int(i64 %t0)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestVarDeclarations(t *testing.T) {
	source := `mut float pi = 3.0 + 0.14; print(pi);`
	expected := `@pi = global double 0.0
define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = fadd double 3.000000, 0.140000
  store double %t0, ptr @pi
  call void @sydney_gc_add_global_root(ptr @pi)
  %t1 = load double, ptr @pi
  call void @sydney_print_float(double %t1)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestVarAssignment(t *testing.T) {
	source := `mut float pi = 3.0 + 0.14;
pi = 2.0;
print(pi);`
	expected := `@pi = global double 0.0
define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = fadd double 3.000000, 0.140000
  store double %t0, ptr @pi
  call void @sydney_gc_add_global_root(ptr @pi)
  store double 2.000000, ptr @pi
  %t1 = load double, ptr @pi
  call void @sydney_print_float(double %t1)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
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
@x = global i64 0
@y = global i64 0
@z = global i64 0
define i32 @main() {
entry:
  call void @sydney_gc_init()
  store i64 0, ptr @x
  call void @sydney_gc_add_global_root(ptr @x)
  store i64 0, ptr @y
  call void @sydney_gc_add_global_root(ptr @y)
  %t0 = load i64, ptr @x
  %t1 = load i64, ptr @y
  %t2 = icmp eq i64 %t0, %t1
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
  %t3 = load i64, ptr @x
  %t4 = load i64, ptr @y
  %t5 = icmp eq i64 %t3, %t4
  br i1 %t5, label %then.3, label %else.4
then.3:
  br label %merge.5
else.4:
  br label %merge.5
merge.5:
  %t6 = phi i64 [ 1, %then.3 ], [ 0, %else.4 ]
  store i64 %t6, ptr @z
  call void @sydney_gc_add_global_root(ptr @z)
  %t7 = load i64, ptr @z
  call void @sydney_print_int(i64 %t7)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
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
	expected := `@i = global i64 0
define i32 @main() {
entry:
  call void @sydney_gc_init()
  store i64 0, ptr @i
  call void @sydney_gc_add_global_root(ptr @i)
  br label %cond.0
cond.0:
  %t0 = load i64, ptr @i
  %t1 = icmp slt i64 %t0, 5
  br i1 %t1, label %loop.1, label %escape.2
loop.1:
    %t2 = load i64, ptr @i
    call void @sydney_print_int(i64 %t2)
    call void @sydney_print_newline()
    %t3 = load i64, ptr @i
    %t4 = add i64 %t3, 1
    store i64 %t4, ptr @i
  br label %cond.0
escape.2:
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
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

	expected := `@i = global i64 0
define i32 @main() {
entry:
  call void @sydney_gc_init()
  store i64 0, ptr @i
  call void @sydney_gc_add_global_root(ptr @i)
  br label %cond.0
cond.0:
  %t0 = load i64, ptr @i
  %t1 = icmp slt i64 %t0, 10
  br i1 %t1, label %loop.1, label %escape.2
loop.1:
    %t2 = load i64, ptr @i
    %t3 = srem i64 %t2, 2
    %t4 = icmp eq i64 %t3, 0
    br i1 %t4, label %then.3, label %merge.4
then.3:
      %t5 = load i64, ptr @i
      call void @sydney_print_int(i64 %t5)
      call void @sydney_print_newline()
    br label %merge.4
merge.4:
    %t6 = load i64, ptr @i
    %t7 = add i64 %t6, 1
    store i64 %t7, ptr @i
  br label %cond.0
escape.2:
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
`
	runEmitterTest(t, source, expected)
}

func TestFunctions(t *testing.T) {
	source := `func addFive(int i) -> int {
    const int r = i + 5;
    r;
}

func addSix(int i) -> int {
    const int r = i + 6;
    return r;
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
@x = global i64 0
@xPlusFive = global i64 0
@xPlusSix = global i64 0
define i64 @addFive(i64 %i) {
entry:
  %i.0.addr = alloca i64
  %r.3.addr = alloca i64
  store i64 %i, ptr %i.0.addr
  %t1 = load i64, ptr %i.0.addr
  %t2 = add i64 %t1, 5
  store i64 %t2, ptr %r.3.addr
  %t4 = load i64, ptr %r.3.addr
  ret i64 %t4
  }


define i64 @addSix(i64 %i) {
entry:
  %i.0.addr = alloca i64
  %r.3.addr = alloca i64
  store i64 %i, ptr %i.0.addr
  %t1 = load i64, ptr %i.0.addr
  %t2 = add i64 %t1, 6
  store i64 %t2, ptr %r.3.addr
  %t4 = load i64, ptr %r.3.addr
  ret i64 %t4
  }


define i32 @main() {
entry:
  call void @sydney_gc_init()
  store i64 0, ptr @x
  call void @sydney_gc_add_global_root(ptr @x)
  %t0 = load i64, ptr @x
  %t1 = call i64 @addSix(i64 %t0)
  store i64 %t1, ptr @xPlusSix
  call void @sydney_gc_add_global_root(ptr @xPlusSix)
  %t2 = load i64, ptr @x
  %t3 = call i64 @addFive(i64 %t2)
  store i64 %t3, ptr @xPlusFive
  call void @sydney_gc_add_global_root(ptr @xPlusFive)
  call void @sydney_print_string(ptr @.str.0)
  %t4 = load i64, ptr @x
  call void @sydney_print_int(i64 %t4)
  call void @sydney_print_newline()
  call void @sydney_print_string(ptr @.str.1)
  %t5 = load i64, ptr @xPlusFive
  call void @sydney_print_int(i64 %t5)
  call void @sydney_print_newline()
  call void @sydney_print_string(ptr @.str.2)
  %t6 = load i64, ptr @xPlusSix
  call void @sydney_print_int(i64 %t6)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
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
@p = global ptr null
define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_gc_alloc(i64 16)
  %t1 = getelementptr %struct.Point, ptr %t0, i32 0, i32 0
  store i64 0, ptr %t1
  %t2 = getelementptr %struct.Point, ptr %t0, i32 0, i32 1
  store i64 0, ptr %t2
  store ptr %t0, ptr @p
  call void @sydney_gc_add_global_root(ptr @p)
  %t3 = load ptr, ptr @p
  %t4 = getelementptr %struct.Point, ptr %t3, i32 0, i32 0
  %t5 = load i64, ptr %t4
  call void @sydney_print_int(i64 %t5)
  call void @sydney_print_newline()
  %t6 = load ptr, ptr @p
  %t7 = getelementptr %struct.Point, ptr %t6, i32 0, i32 1
  %t8 = load i64, ptr %t7
  call void @sydney_print_int(i64 %t8)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
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
@p = global ptr null
define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_gc_alloc(i64 16)
  %t1 = getelementptr %struct.Point, ptr %t0, i32 0, i32 0
  store i64 0, ptr %t1
  %t2 = getelementptr %struct.Point, ptr %t0, i32 0, i32 1
  store i64 0, ptr %t2
  store ptr %t0, ptr @p
  call void @sydney_gc_add_global_root(ptr @p)
  %t3 = load ptr, ptr @p
  %t4 = getelementptr %struct.Point, ptr %t3, i32 0, i32 0
  %t5 = load i64, ptr %t4
  call void @sydney_print_int(i64 %t5)
  call void @sydney_print_newline()
  %t6 = load ptr, ptr @p
  %t7 = getelementptr %struct.Point, ptr %t6, i32 0, i32 1
  %t8 = load i64, ptr %t7
  call void @sydney_print_int(i64 %t8)
  call void @sydney_print_newline()
  %t9 = load ptr, ptr @p
  %t10 = getelementptr %struct.Point, ptr %t9, i32 0, i32 0
  store i64 1, ptr %t10
  %t11 = load ptr, ptr @p
  %t12 = getelementptr %struct.Point, ptr %t11, i32 0, i32 1
  store i64 2, ptr %t12
  %t13 = load ptr, ptr @p
  %t14 = getelementptr %struct.Point, ptr %t13, i32 0, i32 0
  %t15 = load i64, ptr %t14
  call void @sydney_print_int(i64 %t15)
  call void @sydney_print_newline()
  %t16 = load ptr, ptr @p
  %t17 = getelementptr %struct.Point, ptr %t16, i32 0, i32 1
  %t18 = load i64, ptr %t17
  call void @sydney_print_int(i64 %t18)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
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
@vtable.Circle. = constant [0 x ptr] []
@vtable.Circle.Area = constant [1 x ptr] [ptr @Circle.area]
@vtable.Rect. = constant [0 x ptr] []
@vtable.Rect.Area = constant [1 x ptr] [ptr @Rect.area]
@c = global ptr null
@circA = global double 0.0
@r = global ptr null
@rectA = global double 0.0
define double @Circle.area(ptr %c) {
entry:
  %c.0.addr = alloca ptr
  %pi.1.addr = alloca double
  store ptr %c, ptr %c.0.addr
  store double 3.140000, ptr %pi.1.addr
  %t2 = load ptr, ptr %c.0.addr
  %t3 = getelementptr %struct.Circle, ptr %t2, i32 0, i32 0
  %t4 = load double, ptr %t3
  %t5 = load ptr, ptr %c.0.addr
  %t6 = getelementptr %struct.Circle, ptr %t5, i32 0, i32 0
  %t7 = load double, ptr %t6
  %t8 = fmul double %t4, %t7
  %t9 = load double, ptr %pi.1.addr
  %t10 = fmul double %t8, %t9
  ret double %t10
  }


define double @Rect.area(ptr %r) {
entry:
  %r.0.addr = alloca ptr
  store ptr %r, ptr %r.0.addr
  %t1 = load ptr, ptr %r.0.addr
  %t2 = getelementptr %struct.Rect, ptr %t1, i32 0, i32 0
  %t3 = load double, ptr %t2
  %t4 = load ptr, ptr %r.0.addr
  %t5 = getelementptr %struct.Rect, ptr %t4, i32 0, i32 1
  %t6 = load double, ptr %t5
  %t7 = fmul double %t3, %t6
  ret double %t7
  }


define double @getArea(ptr %a) {
entry:
  %a.0.addr = alloca ptr
  store ptr %a, ptr %a.0.addr
  %t1 = load ptr, ptr %a.0.addr
  %t2 = load { ptr, ptr }, ptr %t1
  %t3 = extractvalue { ptr, ptr } %t2, 0
  %t4 = extractvalue { ptr, ptr } %t2, 1
  %t5 = getelementptr [1 x ptr], ptr %t4, i32 0, i32 0
  %t6 = load ptr, ptr %t5
  %t7 = call double %t6(ptr %t3)
  ret double %t7
  }


define i32 @main() {
entry:
  %t6 = alloca { ptr, ptr }
  %t12 = alloca { ptr, ptr }
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_gc_alloc(i64 8)
  %t1 = getelementptr %struct.Circle, ptr %t0, i32 0, i32 0
  store double 2.000000, ptr %t1
  store ptr %t0, ptr @c
  call void @sydney_gc_add_global_root(ptr @c)
  %t2 = call ptr @sydney_gc_alloc(i64 16)
  %t3 = getelementptr %struct.Rect, ptr %t2, i32 0, i32 0
  store double 2.000000, ptr %t3
  %t4 = getelementptr %struct.Rect, ptr %t2, i32 0, i32 1
  store double 2.000000, ptr %t4
  store ptr %t2, ptr @r
  call void @sydney_gc_add_global_root(ptr @r)
  %t5 = load ptr, ptr @r
  %t7 = getelementptr { ptr, ptr }, ptr %t6, i32 0, i32 0
  store ptr %t5, ptr %t7
  %t8 = getelementptr { ptr, ptr }, ptr %t6, i32 0, i32 1
  store ptr @vtable.Rect.Area, ptr %t8
  %t9 = call double @getArea(ptr %t6)
  store double %t9, ptr @rectA
  call void @sydney_gc_add_global_root(ptr @rectA)
  call void @sydney_print_string(ptr @.str.0)
  %t10 = load double, ptr @rectA
  call void @sydney_print_float(double %t10)
  call void @sydney_print_newline()
  %t11 = load ptr, ptr @c
  %t13 = getelementptr { ptr, ptr }, ptr %t12, i32 0, i32 0
  store ptr %t11, ptr %t13
  %t14 = getelementptr { ptr, ptr }, ptr %t12, i32 0, i32 1
  store ptr @vtable.Circle.Area, ptr %t14
  %t15 = call double @getArea(ptr %t12)
  store double %t15, ptr @circA
  call void @sydney_gc_add_global_root(ptr @circA)
  call void @sydney_print_string(ptr @.str.1)
  %t16 = load double, ptr @circA
  call void @sydney_print_float(double %t16)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestArrays(t *testing.T) {
	source := `const array<int> a = [0, 1, 2, 3];
print(a[1]);`

	expected := `@a = global ptr null
define i32 @main() {
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
  store ptr %t5, ptr @a
  call void @sydney_gc_add_global_root(ptr @a)
  %t8 = load ptr, ptr @a
  %t9 = getelementptr { i64, ptr }, ptr %t8, i32 0, i32 1
  %t10 = load ptr, ptr %t9
  %t11 = getelementptr i64, ptr %t10, i64 1
  %t12 = load i64, ptr %t11
  call void @sydney_print_int(i64 %t12)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
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

	expected := `@adder = global ptr null
@anon = global ptr null
@sum = global i64 0
define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_gc_alloc(i64 16)
  %t1 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 0
  store ptr @anon.0, ptr %t1
  %t2 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 1
  store ptr null, ptr %t2
  store ptr %t0, ptr @anon
  call void @sydney_gc_add_global_root(ptr @anon)
  %t3 = load ptr, ptr @anon
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
  store ptr %t9, ptr @adder
  call void @sydney_gc_add_global_root(ptr @adder)
  %t12 = load ptr, ptr @adder
  %t13 = getelementptr { ptr, ptr }, ptr %t12, i32 0, i32 0
  %t14 = load ptr, ptr %t13
  %t15 = getelementptr { ptr, ptr }, ptr %t12, i32 0, i32 1
  %t16 = load ptr, ptr %t15
  %t17 = call i64 %t14(ptr %t16, i64 1, i64 2)
  store i64 %t17, ptr @sum
  call void @sydney_gc_add_global_root(ptr @sum)
  %t18 = load i64, ptr @sum
  call void @sydney_print_int(i64 %t18)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
define i64 @anon.0(ptr %env) {
entry:
  ret i64 1
}

define i64 @anon.1(ptr %env, i64 %a, i64 %b) {
entry:
  %a.0.addr = alloca i64
  %b.1.addr = alloca i64
  store i64 %a, ptr %a.0.addr
  store i64 %b, ptr %b.1.addr
  %t2 = load i64, ptr %a.0.addr
  %t3 = load i64, ptr %b.1.addr
  %t4 = add i64 %t2, %t3
  ret i64 %t4
}

`

	runEmitterTest(t, source, expected)
}

func TestCaptures(t *testing.T) {
	source := `const int x = 10;
const f = func(int y) -> int { return x + y };
print(f(5));`

	expected := `@f = global ptr null
@x = global i64 0
define i32 @main() {
entry:
  call void @sydney_gc_init()
  store i64 10, ptr @x
  call void @sydney_gc_add_global_root(ptr @x)
  %t0 = call ptr @sydney_gc_alloc(i64 16)
  %t1 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 0
  store ptr @anon.0, ptr %t1
  %t2 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 1
  store ptr null, ptr %t2
  store ptr %t0, ptr @f
  call void @sydney_gc_add_global_root(ptr @f)
  %t3 = load ptr, ptr @f
  %t4 = getelementptr { ptr, ptr }, ptr %t3, i32 0, i32 0
  %t5 = load ptr, ptr %t4
  %t6 = getelementptr { ptr, ptr }, ptr %t3, i32 0, i32 1
  %t7 = load ptr, ptr %t6
  %t8 = call i64 %t5(ptr %t7, i64 5)
  call void @sydney_print_int(i64 %t8)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
define i64 @anon.0(ptr %env, i64 %y) {
entry:
  %y.0.addr = alloca i64
  store i64 %y, ptr %y.0.addr
  %t1 = load i64, ptr @x
  %t2 = load i64, ptr %y.0.addr
  %t3 = add i64 %t1, %t2
  ret i64 %t3
}

`

	runEmitterTest(t, source, expected)
}

func TestNestedClosures(t *testing.T) {
	source := `const x = 10;

const getAdder = func(int y) -> fn<(int) -> int> { return func(int z) -> int { return x + y + z }; };
const fiveAdder = getAdder(5);
const sixAdder = getAdder(6);
print(fiveAdder(5));
print(sixAdder(5));`

	expected := `@fiveAdder = global ptr null
@getAdder = global ptr null
@sixAdder = global ptr null
@x = global i64 0
define i32 @main() {
entry:
  call void @sydney_gc_init()
  store i64 10, ptr @x
  call void @sydney_gc_add_global_root(ptr @x)
  %t0 = call ptr @sydney_gc_alloc(i64 16)
  %t1 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 0
  store ptr @anon.0, ptr %t1
  %t2 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 1
  store ptr null, ptr %t2
  store ptr %t0, ptr @getAdder
  call void @sydney_gc_add_global_root(ptr @getAdder)
  %t3 = load ptr, ptr @getAdder
  %t4 = getelementptr { ptr, ptr }, ptr %t3, i32 0, i32 0
  %t5 = load ptr, ptr %t4
  %t6 = getelementptr { ptr, ptr }, ptr %t3, i32 0, i32 1
  %t7 = load ptr, ptr %t6
  %t8 = call ptr %t5(ptr %t7, i64 5)
  store ptr %t8, ptr @fiveAdder
  call void @sydney_gc_add_global_root(ptr @fiveAdder)
  %t9 = load ptr, ptr @getAdder
  %t10 = getelementptr { ptr, ptr }, ptr %t9, i32 0, i32 0
  %t11 = load ptr, ptr %t10
  %t12 = getelementptr { ptr, ptr }, ptr %t9, i32 0, i32 1
  %t13 = load ptr, ptr %t12
  %t14 = call ptr %t11(ptr %t13, i64 6)
  store ptr %t14, ptr @sixAdder
  call void @sydney_gc_add_global_root(ptr @sixAdder)
  %t15 = load ptr, ptr @fiveAdder
  %t16 = getelementptr { ptr, ptr }, ptr %t15, i32 0, i32 0
  %t17 = load ptr, ptr %t16
  %t18 = getelementptr { ptr, ptr }, ptr %t15, i32 0, i32 1
  %t19 = load ptr, ptr %t18
  %t20 = call i64 %t17(ptr %t19, i64 5)
  call void @sydney_print_int(i64 %t20)
  call void @sydney_print_newline()
  %t21 = load ptr, ptr @sixAdder
  %t22 = getelementptr { ptr, ptr }, ptr %t21, i32 0, i32 0
  %t23 = load ptr, ptr %t22
  %t24 = getelementptr { ptr, ptr }, ptr %t21, i32 0, i32 1
  %t25 = load ptr, ptr %t24
  %t26 = call i64 %t23(ptr %t25, i64 5)
  call void @sydney_print_int(i64 %t26)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
define i64 @anon.1(ptr %env, i64 %z) {
entry:
  %y.2.addr = alloca i64
  %z.3.addr = alloca i64
  %t0 = getelementptr { i64 }, ptr %env, i32 0, i32 0
  %t1 = load i64, ptr %t0
  store i64 %t1, ptr %y.2.addr
  store i64 %z, ptr %z.3.addr
  %t4 = load i64, ptr @x
  %t5 = load i64, ptr %y.2.addr
  %t6 = add i64 %t4, %t5
  %t7 = load i64, ptr %z.3.addr
  %t8 = add i64 %t6, %t7
  ret i64 %t8
}

define ptr @anon.0(ptr %env, i64 %y) {
entry:
  %y.0.addr = alloca i64
  store i64 %y, ptr %y.0.addr
  %t1 = call ptr @sydney_gc_alloc(i64 16)
  %t2 = getelementptr { ptr, ptr }, ptr %t1, i32 0, i32 0
  store ptr @anon.1, ptr %t2
  %t3 = getelementptr { ptr, ptr }, ptr %t1, i32 0, i32 1
  %t4 = call ptr @sydney_gc_alloc(i64 8)
  %t5 = getelementptr { i64 }, ptr %t4, i32 0, i32 0
  %t6 = load i64, ptr %y.0.addr
  store i64 %t6, ptr %t5
  store ptr %t4, ptr %t3
  ret ptr %t1
}

`

	runEmitterTest(t, source, expected)
}

func TestLocalVariables(t *testing.T) {
	source := `func addTwo(int x) -> int { return x + 2 };
print(addTwo(3));`

	expected := `define i64 @addTwo(i64 %x) {
entry:
  %x.0.addr = alloca i64
  store i64 %x, ptr %x.0.addr
  %t1 = load i64, ptr %x.0.addr
  %t2 = add i64 %t1, 2
  ret i64 %t2
  }


define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = call i64 @addTwo(i64 3)
  call void @sydney_print_int(i64 %t0)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestMapAllocationAndAssignment(t *testing.T) {
	source := `const map<int, int> m = { 1: 0 }
m[0] = 1;
print(m[0]);
print(m[1]);
print(m[2]);`

	expected := `@m = global ptr null
define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_map_create_int()
  call void @sydney_map_set_int(ptr %t0, i64 1, i64 0)
  store ptr %t0, ptr @m
  call void @sydney_gc_add_global_root(ptr @m)
  %t1 = load ptr, ptr @m
  call void @sydney_map_set_int(ptr %t1, i64 0, i64 1)
  %t2 = load ptr, ptr @m
  %t3 = call i64 @sydney_map_get_int(ptr %t2, i64 0)
  call void @sydney_print_int(i64 %t3)
  call void @sydney_print_newline()
  %t4 = load ptr, ptr @m
  %t5 = call i64 @sydney_map_get_int(ptr %t4, i64 1)
  call void @sydney_print_int(i64 %t5)
  call void @sydney_print_newline()
  %t6 = load ptr, ptr @m
  %t7 = call i64 @sydney_map_get_int(ptr %t6, i64 2)
  call void @sydney_print_int(i64 %t7)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestNestedCollections(t *testing.T) {
	source := `const map<string, array<int>> M = { "hello": [1, 2, 3], "world": [3, 4, 5] };
print(M["hello"][0]);
M["world"][0] = 6;
print(M["world"][0]);`

	expected := `@.str.0 = private unnamed_addr constant [6 x i8] c"hello\00"
@.str.1 = private unnamed_addr constant [6 x i8] c"world\00"
@M = global ptr null
define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_map_create_string()
  %t1 = call ptr @sydney_gc_alloc(i64 24)
  %t2 = getelementptr i64, ptr %t1, i32 0
  store i64 1, ptr %t2
  %t3 = getelementptr i64, ptr %t1, i32 1
  store i64 2, ptr %t3
  %t4 = getelementptr i64, ptr %t1, i32 2
  store i64 3, ptr %t4
  %t5 = call ptr @sydney_gc_alloc(i64 16)
  %t6 = getelementptr { i64, ptr }, ptr %t5, i32 0, i32 0
  store i64 3, ptr %t6
  %t7 = getelementptr { i64, ptr }, ptr %t5, i32 0, i32 1
  store ptr %t1, ptr %t7
  %t8 = ptrtoint ptr %t5 to i64
  call void @sydney_map_set_str(ptr %t0, ptr @.str.0, i64 %t8)
  %t9 = call ptr @sydney_gc_alloc(i64 24)
  %t10 = getelementptr i64, ptr %t9, i32 0
  store i64 3, ptr %t10
  %t11 = getelementptr i64, ptr %t9, i32 1
  store i64 4, ptr %t11
  %t12 = getelementptr i64, ptr %t9, i32 2
  store i64 5, ptr %t12
  %t13 = call ptr @sydney_gc_alloc(i64 16)
  %t14 = getelementptr { i64, ptr }, ptr %t13, i32 0, i32 0
  store i64 3, ptr %t14
  %t15 = getelementptr { i64, ptr }, ptr %t13, i32 0, i32 1
  store ptr %t9, ptr %t15
  %t16 = ptrtoint ptr %t13 to i64
  call void @sydney_map_set_str(ptr %t0, ptr @.str.1, i64 %t16)
  store ptr %t0, ptr @M
  call void @sydney_gc_add_global_root(ptr @M)
  %t17 = load ptr, ptr @M
  %t18 = call i64 @sydney_map_get_str(ptr %t17, ptr @.str.0)
  %t19 = inttoptr i64 %t18 to ptr
  %t20 = getelementptr { i64, ptr }, ptr %t19, i32 0, i32 1
  %t21 = load ptr, ptr %t20
  %t22 = getelementptr i64, ptr %t21, i64 0
  %t23 = load i64, ptr %t22
  call void @sydney_print_int(i64 %t23)
  call void @sydney_print_newline()
  %t24 = load ptr, ptr @M
  %t25 = call i64 @sydney_map_get_str(ptr %t24, ptr @.str.1)
  %t26 = inttoptr i64 %t25 to ptr
  %t27 = getelementptr { i64, ptr }, ptr %t26, i32 0, i32 1
  %t28 = load ptr, ptr %t27
  %t29 = getelementptr i64, ptr %t28, i64 0
  store i64 6, ptr %t29
  %t30 = load ptr, ptr @M
  %t31 = call i64 @sydney_map_get_str(ptr %t30, ptr @.str.1)
  %t32 = inttoptr i64 %t31 to ptr
  %t33 = getelementptr { i64, ptr }, ptr %t32, i32 0, i32 1
  %t34 = load ptr, ptr %t33
  %t35 = getelementptr i64, ptr %t34, i64 0
  %t36 = load i64, ptr %t35
  call void @sydney_print_int(i64 %t36)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
`

	runEmitterTest(t, source, expected)
}

func TestRecursiveClosure(t *testing.T) {
	source := `const countDown = func(int x) -> int {
    if (x == 0) { return 0; }
    return countDown(x - 1);
};
print(countDown(5));`

	expected := `@countDown = global ptr null
define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = call ptr @sydney_gc_alloc(i64 16)
  %t1 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 0
  store ptr @anon.0, ptr %t1
  %t2 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 1
  store ptr null, ptr %t2
  store ptr %t0, ptr @countDown
  call void @sydney_gc_add_global_root(ptr @countDown)
  %t3 = load ptr, ptr @countDown
  %t4 = getelementptr { ptr, ptr }, ptr %t3, i32 0, i32 0
  %t5 = load ptr, ptr %t4
  %t6 = getelementptr { ptr, ptr }, ptr %t3, i32 0, i32 1
  %t7 = load ptr, ptr %t6
  %t8 = call i64 %t5(ptr %t7, i64 5)
  call void @sydney_print_int(i64 %t8)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
define i64 @anon.0(ptr %env, i64 %x) {
entry:
  %x.0.addr = alloca i64
  %self = alloca { ptr, ptr }
  store i64 %x, ptr %x.0.addr
%t1 = getelementptr { ptr, ptr }, ptr %self, i32 0, i32 0
store ptr @anon.0, ptr %t1
%t2 = getelementptr { ptr, ptr }, ptr %self, i32 0, i32 1
store ptr %env, ptr %t2
  %t3 = load i64, ptr %x.0.addr
  %t4 = icmp eq i64 %t3, 0
  br i1 %t4, label %then.0, label %merge.1
then.0:
    ret i64 0
merge.1:
  %t5 = getelementptr { ptr, ptr }, ptr %self, i32 0, i32 0
  %t6 = load ptr, ptr %t5
  %t7 = getelementptr { ptr, ptr }, ptr %self, i32 0, i32 1
  %t8 = load ptr, ptr %t7
  %t9 = load i64, ptr %x.0.addr
  %t10 = sub i64 %t9, 1
  %t11 = call i64 %t6(ptr %t8, i64 %t10)
  ret i64 %t11
}

`

	runEmitterTest(t, source, expected)
}

func TestLocalRecursiveClosure(t *testing.T) {
	source := `func callCountDown() -> int {
    const countDown = func(int x) -> int {
        print(x);
        if (x == 0) { return 0; }
        return countDown(x - 1);
    };
    return countDown(5);
}
print(callCountDown());`

	expected := `define i64 @callCountDown() {
entry:
  %countDown.3.addr = alloca ptr
  %t0 = call ptr @sydney_gc_alloc(i64 16)
  %t1 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 0
  store ptr @anon.0, ptr %t1
  %t2 = getelementptr { ptr, ptr }, ptr %t0, i32 0, i32 1
  store ptr null, ptr %t2
  store ptr %t0, ptr %countDown.3.addr
  %t4 = load ptr, ptr %countDown.3.addr
  %t5 = getelementptr { ptr, ptr }, ptr %t4, i32 0, i32 0
  %t6 = load ptr, ptr %t5
  %t7 = getelementptr { ptr, ptr }, ptr %t4, i32 0, i32 1
  %t8 = load ptr, ptr %t7
  %t9 = call i64 %t6(ptr %t8, i64 5)
  ret i64 %t9
  }


define i32 @main() {
entry:
  call void @sydney_gc_init()
  %t0 = call i64 @callCountDown()
  call void @sydney_print_int(i64 %t0)
  call void @sydney_print_newline()
  call void @sydney_join_all()
  call void @sydney_gc_shutdown()
  ret i32 0
}
define i64 @anon.0(ptr %env, i64 %x) {
entry:
  %x.0.addr = alloca i64
  %self = alloca { ptr, ptr }
  store i64 %x, ptr %x.0.addr
%t1 = getelementptr { ptr, ptr }, ptr %self, i32 0, i32 0
store ptr @anon.0, ptr %t1
%t2 = getelementptr { ptr, ptr }, ptr %self, i32 0, i32 1
store ptr %env, ptr %t2
  %t3 = load i64, ptr %x.0.addr
  call void @sydney_print_int(i64 %t3)
  call void @sydney_print_newline()
  %t4 = load i64, ptr %x.0.addr
  %t5 = icmp eq i64 %t4, 0
  br i1 %t5, label %then.0, label %merge.1
then.0:
    ret i64 0
merge.1:
  %t6 = getelementptr { ptr, ptr }, ptr %self, i32 0, i32 0
  %t7 = load ptr, ptr %t6
  %t8 = getelementptr { ptr, ptr }, ptr %self, i32 0, i32 1
  %t9 = load ptr, ptr %t8
  %t10 = load i64, ptr %x.0.addr
  %t11 = sub i64 %t10, 1
  %t12 = call i64 %t7(ptr %t9, i64 %t11)
  ret i64 %t12
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
	c.Check(program, nil)
	if len(c.Errors()) != 0 {
		t.Fatalf("typechecker errors not empty: %v", c.Errors())
	}
	e := New()
	err := e.Emit(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	actual := e.buf.String()

	expected = buildExpected(expected)
	if actual != expected {
		t.Fatalf("expected:\n%s\ngot:\n%s", expected, actual)
	}
}

// e2e tests: emit IR → llc → clang → run binary → check stdout

type e2eTestCase struct {
	source   string
	expected string
}

func runE2ETests(t *testing.T, tests []e2eTestCase) {
	t.Helper()

	// find project root (where sydney_rt lives)
	projectRoot, err := filepath.Abs(filepath.Join("..", "."))
	if err != nil {
		t.Fatalf("cannot find project root: %v", err)
	}
	rtLib := filepath.Join(projectRoot, "sydney_rt", "target", "release")
	if _, err := os.Stat(filepath.Join(rtLib, "libsydney_rt.a")); err != nil {
		t.Skip("libsydney_rt.a not found, skipping e2e test")
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			tmpDir := t.TempDir()

			// emit IR
			l := lexer.New(tt.source)
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}
			c := typechecker.New(nil)
			c.Check(program, nil)
			if len(c.Errors()) != 0 {
				t.Fatalf("typechecker errors: %v", c.Errors())
			}
			ast.FilterGenericTemplates(program)
			e := New()
			if err := e.Emit(program, nil); err != nil {
				t.Fatalf("emitter error: %v", err)
			}

			llFile := filepath.Join(tmpDir, "test.ll")
			objFile := filepath.Join(tmpDir, "test.o")
			binFile := filepath.Join(tmpDir, "test")

			os.WriteFile(llFile, []byte(e.buf.String()), 0644)

			// llc
			cmd := exec.Command("llc", "-filetype=obj", llFile, "-o", objFile)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("llc failed: %s\n%s", err, out)
			}

			// clang link
			cmd = exec.Command("clang", objFile, "-L"+rtLib, "-lsydney_rt", "-o", binFile)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("clang failed: %s\n%s", err, out)
			}

			// run
			cmd = exec.Command(binFile)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("binary failed: %s\n%s", err, out)
			}

			actual := string(out)
			if actual != tt.expected {
				t.Fatalf("expected:\n%s\ngot:\n%s", tt.expected, actual)
			}
		})
	}
}

func TestE2EForLoops(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `mut sum = 0; for (mut i = 0; i < 5; i = i + 1) { sum = sum + i; } print(sum);`,
			expected: "10\n",
		},
		{ // three-part for with break
			source:   `mut sum = 0; for (mut i = 0; i < 10; i = i + 1) { if (i == 3) { break; } sum = sum + i; } print(sum);`,
			expected: "3\n",
		},
		{ // continue skips even numbers
			source:   `mut sum = 0; for (mut i = 0; i < 6; i = i + 1) { if (i % 2 == 0) { continue; } sum = sum + i; } print(sum);`,
			expected: "9\n",
		},
		{ // reuse loop var name
			source:   `for (mut i = 0; i < 2; i = i + 1) { print(i); } for (mut i = 10; i < 12; i = i + 1) { print(i); }`,
			expected: "0\n1\n10\n11\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EStringEquality(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `print("hello" == "hello"); print("hello" == "world"); print("a" != "b");`,
			expected: "true\nfalse\ntrue\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EConversionBuiltins(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `print(int('a')); print(char(byte(72)));`,
			expected: "97\nH\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EModulo(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `print(10 % 3); print(15 % 5);`,
			expected: "1\n0\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EAppend(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `const array<int> a = [1, 2, 3]; const b = append(a, 4); print(len(b)); print(b[3]);`,
			expected: "4\n4\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EEscapeSequences(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `print("hello\tworld"); print("line1\nline2");`,
			expected: "hello\tworld\nline1\nline2\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EIfAsStatement(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `mut r = 0; for (mut i = 0; i < 4; i = i + 1) { if (i % 2 == 0) { r = r + 1; } else { r = r + 10; } } print(r);`,
			expected: "22\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EArraySlice(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `const a = [1, 2, 3, 4, 5]; const b = a[1:4]; print(len(b)); print(b[0]); print(b[1]); print(b[2]);`,
			expected: "3\n2\n3\n4\n",
		},
		{
			source:   `const a = [10, 20, 30]; const b = a[0:2]; print(len(b)); print(b[0]); print(b[1]);`,
			expected: "2\n10\n20\n",
		},
		{
			source:   `const a = [1, 2, 3, 4, 5]; const b = a[3:]; print(len(b)); print(b[0]); print(b[1]);`,
			expected: "2\n4\n5\n",
		},
		{
			source:   `const a = [1, 2, 3, 4, 5]; const b = a[:2]; print(len(b)); print(b[0]); print(b[1]);`,
			expected: "2\n1\n2\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EStringSlice(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `const s = "Hello, World!"; print(s[0:5]);`,
			expected: "Hello\n",
		},
		{
			source:   `const s = "Hello, World!"; print(s[7:12]);`,
			expected: "World\n",
		},
		{
			source:   `const s = "abcdef"; print(s[2:]);`,
			expected: "cdef\n",
		},
		{
			source:   `const s = "abcdef"; print(s[:3]);`,
			expected: "abc\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EGenericFunctions(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `func identity<T>(T x) -> T { x; }
			         print(identity<int>(42));`,
			expected: "42\n",
		},
		{
			source: `func identity<T>(T x) -> T { x; }
			         print(identity<string>("hello"));`,
			expected: "hello\n",
		},
		{
			source: `func first<T>(array<T> a) -> T { a[0]; }
			         print(first<int>([10, 20, 30]));`,
			expected: "10\n",
		},
		{
			source: `func sum<T>(array<T> vals) -> T {
				mut T acc = 0;
				for (mut i = 0; i < len(vals); i = i + 1) {
					acc = acc + vals[i];
				}
				acc;
			}
			print(sum<int>([1, 2, 3, 4, 5]));`,
			expected: "15\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EGenericStructs(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `define struct Box<T> { value T }
			         const b = Box<int> { value: 42 };
			         print(b.value);`,
			expected: "42\n",
		},
		{
			source: `define struct Box<T> { value T }
			         const b = Box<string> { value: "hello" };
			         print(b.value);`,
			expected: "hello\n",
		},
		{
			source: `define struct Box<T> { value T }
			         const a = Box<int> { value: 5 };
			         const b = Box<int> { value: 10 };
			         print(a.value + b.value);`,
			expected: "15\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EForInArray(t *testing.T) {
	tests := []e2eTestCase{
		// Basic value iteration
		{
			source:   `const a = [1, 2, 3]; for (v in a) { print(v); }`,
			expected: "1\n2\n3\n",
		},
		// Index and value iteration
		{
			source:   `const a = [10, 20, 30]; for (i, v in a) { print(i); print(v); }`,
			expected: "0\n10\n1\n20\n2\n30\n",
		},
		// Sum with accumulator
		{
			source:   `mut s = 0; const a = [1, 2, 3, 4]; for (v in a) { s = s + v; } print(s);`,
			expected: "10\n",
		},
		// Nested for-in
		{
			source: `const a = [1, 2];
			         const b = [10, 20];
			         for (x in a) { for (y in b) { print(x * y); } }`,
			expected: "10\n20\n20\n40\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EForInMap(t *testing.T) {
	tests := []e2eTestCase{
		// Map iteration - print values
		{
			source:   `mut s = 0; const m = {"a": 1, "b": 2, "c": 3}; for (k, v in m) { s = s + v; } print(s);`,
			expected: "6\n",
		},
		// Map iteration with int keys
		{
			source:   `mut s = 0; const m = {1: 10, 2: 20}; for (k, v in m) { s = s + k + v; } print(s);`,
			expected: "33\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EBufferedChannel(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `const ch = chan<int>(1); ch <- 42; const val = <- ch; print(val);`,
			expected: "42\n",
		},
	}
	runE2ETests(t, tests)
}

func TestE2ESpawnWithChannel(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `const ch = chan<int>();
			spawn func() {
				ch <- 99;
			}();
			const val = <- ch;
			print(val);`,
			expected: "99\n",
		},
	}
	runE2ETests(t, tests)
}
