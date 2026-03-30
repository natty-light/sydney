package vm

import (
	"fmt"
	"sydney/ast"
	"sydney/compiler"
	"sydney/lexer"
	"sydney/object"
	"sydney/parser"
	"sydney/typechecker"
	"testing"
)

type vmTestCase struct {
	source   string
	expected interface{}
}

func TestIntegerArithmetic(t *testing.T) {
	tests := []vmTestCase{
		{"1", 1},
		{"2", 2},
		{"1 + 2", 3},
		{"1 * 2", 2},
		{"4 / 2", 2},
		{"50 / 2 * 2 + 10 - 5", 55},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"5 * (2 + 10)", 60},
		{"-5", -5},
		{"-10", -10},
		{"-50 + 100 + -50", 0},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	runVmTests(t, tests)
}

func TestFLoatArithmetic(t *testing.T) {
	tests := []vmTestCase{
		{"1.0", 1.0},
		{"2.0", 2.0},
		{"1.0 + 2.0", 3.0},
		{"1.0 * 2.0", 2.0},
		{"4.0 / 2.0", 2.0},
		{"50.0 / 2.0 * 2.0 + 10.0 - 5.0", 55.0},
		{"5.0 + 5.0 + 5.0 + 5.0 - 10.0", 10.0},
		{"2.0 * 2.0 * 2.0 * 2.0 * 2.0", 32.0},
		{"5.0 * 2.0 + 10.0", 20.0},
		{"5.0 + 2.0 * 10.0", 25.0},
		{"5.0 * (2.0 + 10.0)", 60.0},
		{"-5.0", -5.0},
		{"-10.0", -10.0},
		{"-50.0 + 100.0 + -50.0", 0.0},
		{"(5.0 + 10.0 * 2.0 + 15.0 / 3.0) * 2.0 + -10.0", 50.0},
	}

	runVmTests(t, tests)
}

func TestBooleanExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 < 1", false},
		{"1 > 1", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 2", false},
		{"1 != 2", true},
		{"true == true", true},
		{"false == false", true},
		{"true == false", false},
		{"true != false", true},
		{"false != true", true},
		{"1 < 2 == true", true},
		{"1 < 2 == false", false},
		{"1 > 2 == true", false},
		{"1 > 2 == false", true},
		{"1 < 2 == 2 > 1", true},
		{"1 < 2 == 2 < 1", false},
		{"1 > 2 == 2 > 1", false},
		{"1 > 2 == 2 < 1", true},
		{"(1 < 2) != true", false},
		{"(1 < 2) != false", true},
		{"(1 > 2) != true", true},
		{"(1 > 2) != false", false},
		{"(1 < 2) != (2 > 1)", false},
		{"(1 < 2) != (2 < 1)", true},
		{"(1 > 2) != (2 > 1)", true},
		{"(1 > 2) != (2 < 1)", false},
		{"true && true", true},
		{"true && false", false},
		{"false && true", false},
		{"false && false", false},
		{"true || true", true},
		{"true || false", true},
		{"false || true", true},
		{"false || false", false},
		{"1 > 2 && 1 < 2", false},
		{"1 > 2 || 1 < 2", true},
		{"1 < 2 && 1 < 2", true},
		{"true == true && true == true", true},
		{"true == true && true == false", false},
		{"true == true || true == false", true},
		{"true == false || true == false", false},
		{"true != true && true != true", false},
		{"true != true && true != false", false},
		{"true != true || true != false", true},
		{"true != false || true != false", true},
		{"!true", false},
		{"!false", true},
		{"!!true", true},
		{"!!false", false},
	}

	runVmTests(t, tests)
}

func TestConditionals(t *testing.T) {
	tests := []vmTestCase{
		{"if (true) { 10 }", 10},
		{"if (true) { 10 } else { 20 }", 10},
		{"if (false) { 10 } else { 20 }", 20},
		{"if (1 < 2) { 10 }", 10},
		{"if (1 < 2) { 10 } else { 20 }", 10},
		{"if (1 > 2) { 10 } else { 20 }", 20},
		{"if (1 > 2) { 10 }", Null},
		{"if (false) { 10 }", Null},
	}

	runVmTests(t, tests)
}

func TestGlobalVarDeclarationStatements(t *testing.T) {
	tests := []vmTestCase{
		{"mut one = 1; one", 1},
		{"mut one = 1; const two = 2; one + two", 3},
		{"mut one = 1; mut two = one + one; one + two", 3},
	}

	runVmTests(t, tests)
}

func TestStringExpressions(t *testing.T) {
	tests := []vmTestCase{
		{
			`"quonk"`, "quonk",
		},
		{
			`"quonk" + "script"`, "quonkscript",
		},
		{
			`"quonk" + " " + "script"`, "quonk script",
		},
	}

	runVmTests(t, tests)
}

func TestArrayLiterals(t *testing.T) {
	tests := []vmTestCase{
		{
			`const array<int> a = []; a;`, []int{},
		},
		{
			`[1, 2, 3]`, []int{1, 2, 3},
		},
		{
			`[1 + 2, 3 * 4, 5 + 6]`, []int{3, 12, 11},
		},
	}

	runVmTests(t, tests)
}

func TestHashLiterals(t *testing.T) {
	tests := []vmTestCase{
		{
			`{}`, map[object.HashKey]int64{},
		},
		{
			`{1: 2, 2: 3}`, map[object.HashKey]int64{
				(&object.Integer{Value: 1}).HashKey(): 2,
				(&object.Integer{Value: 2}).HashKey(): 3,
			},
		},
		{
			`{1 + 1: 2 * 2, 3 + 3: 4 * 4}`, map[object.HashKey]int64{
				(&object.Integer{Value: 2}).HashKey(): 4,
				(&object.Integer{Value: 6}).HashKey(): 16,
			},
		},
	}

	runVmTests(t, tests)
}

func TestIndexExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"[1, 2, 3][1]", 2},
		{"[1, 2, 3][0 + 2]", 3},
		{"[[1, 1, 1]][0][0]", 1},
		{"[[1, 1, 1]][0][0] + 1", 2},
		{"[1, 2, 3][1 + 1]", 3},
		{"const i = 0; [1][i]", 1},
		{`const r = {1: 1, 2: 2}[1]; match r { some(v) -> { v; }, none -> { 0; }, };`, 1},
		{`const r = {1: 1, 2: 2}[2]; match r { some(v) -> { v; }, none -> { 0; }, };`, 2},
		{`const r = {1: 1}[0]; match r { some(v) -> { v; }, none -> { -1; }, };`, -1},
	}

	runVmTests(t, tests)
}

func TestArrayIndexOutOfBounds(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `[][0]`,
			expected: "array index out of bounds: index 0 but length is 0",
		},
		{
			source:   `[1, 2, 3][99]`,
			expected: "array index out of bounds: index 99 but length is 3",
		},
		{
			source:   `[1][-1]`,
			expected: "array index out of bounds: index -1 but length is 1",
		},
	}

	for _, tt := range tests {
		program := parse(tt.source)

		c := typechecker.New(nil)
		c.Check(program, nil)
		ast.FilterGenericTemplates(program)

		comp := compiler.New()
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}

		vm := New(comp.Bytecode())
		err = vm.Run()
		if err == nil {
			t.Fatalf("expected vm error, but got nil")
		}

		if err.Error() != tt.expected {
			t.Fatalf("wrong error message. want=%q, got=%q", tt.expected, err.Error())
		}
	}
}

func TestCallingFunctionsWithoutArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `const fivePlusTen = func() -> int { 5 + 10; }; fivePlusTen();`,
			expected: 15,
		},
		{
			source:   `const one = func() -> int { 1; }; const two = func() -> int { 2; }; one() + two();`,
			expected: 3,
		},
		{
			source: `
			const a = func() -> int { 1; };
			const b = func() -> int { a() + 1; };
			const c = func() -> int { b() + 1; };
			c();`,
			expected: 3,
		},
	}

	runVmTests(t, tests)
}

func TestFunctionsWithReturnStatements(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `const earlyExit = func() -> int { return 99; 100; }; earlyExit();`,
			expected: 99,
		},
		{
			source:   `const earlyExit = func() -> int { return 99; return 100; }; earlyExit();`,
			expected: 99,
		},
	}

	runVmTests(t, tests)
}

func TestFunctionsWithoutReturnValue(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `const noReturn = func() { }; noReturn();`,
			expected: Null,
		},
		{
			source: `const noReturn = func() { };
			const noReturnTwo = func() { noReturn(); };
			noReturn();
			noReturnTwo();`,
			expected: Null,
		},
	}

	runVmTests(t, tests)
}

func TestFirstClassFunctions(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `const returnsOne = func() -> int { 1; };
			const returnsOneReturner = func() -> fn<() -> int> { returnsOne; };
			returnsOneReturner()();`,
			expected: 1,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `const one = func() -> int { const one = 1; one; }; one()`,
			expected: 1,
		},
		{
			source: `const oneAndTwo = func() -> int { const one = 1; const two = 2; one + two; };
			oneAndTwo();`,
			expected: 3,
		},
		{
			source: `
			const oneAndTwo = func() -> int {const one = 1; const two = 2; one + two; };
			const threeAndFour = func() -> int { const three = 3; const four = 4; three + four; };
			oneAndTwo() + threeAndFour();
			`,
			expected: 10,
		},
		{
			source: `
			const firstFunc = func() -> int { const x = 50; x };
			const secondFunc = func() -> int { const x = 100; x };
			firstFunc() + secondFunc();
			`,
			expected: 150,
		},
		{
			source: `
			mut int globalNum = 50;
			const minusOne = func() -> int { const num = 1; globalNum - num; };
			const minusTwo = func() -> int { const num = 2; globalNum - num; };
			minusOne() + minusTwo();
			`,
			expected: 97,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithArgumentsAndBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `const identity = func(int a) -> int { a; }; identity(4);`,
			expected: 4,
		},
		{
			source:   `const sum = func(int a, int b) -> int { a + b; }; sum(1, 2);`,
			expected: 3,
		},
		{
			source:   `const sum = func(int a, int b) -> int { const c = a + b; c; }; sum(1, 2);`,
			expected: 3,
		},
		{
			source: `
			const sum = func(int a, int b) -> int {
				const c = a + b;
				c;
			};
			const outer = func() {
				sum(1, 2) + sum(3, 4);
			}
			outer();
			`,
			expected: 10,
		},
		{
			source: `
			const globalNum = 10;
			const sum = func(int a, int b) -> int {
				const c = a + b;
				c + globalNum;	
			}
			const outer = func() -> int {
				sum(1, 2) + sum(3, 4) + globalNum;
			};
			outer() + globalNum;
			`,
			expected: 50,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithWrongArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `func() -> int { 1; }(1);`,
			expected: "wrong number of arguments. want=0, got=1",
		},
		{
			source:   `func(int a) -> int { a; }();`,
			expected: "wrong number of arguments. want=1, got=0",
		},
		{
			source:   `func(int a, int b) { a + b; }(1);`,
			expected: "wrong number of arguments. want=2, got=1",
		},
	}

	for _, tt := range tests {
		program := parse(tt.source)

		comp := compiler.New()
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}

		vm := New(comp.Bytecode())
		err = vm.Run()
		if err == nil {
			t.Fatalf("expected vm error, but got nil")
		}

		if err.Error() != tt.expected {
			t.Fatalf("wrong error message. want=%q, got=%q", tt.expected, err.Error())
		}
	}
}

func TestBuiltinFunctions(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `len("")`,
			expected: 0,
		},
		{
			source:   `len("four")`,
			expected: 4,
		},
		{
			source:   `len("hello world")`,
			expected: 11,
		},
		{
			source:   `const array<int> a = []; len(a)`,
			expected: 0,
		},
		{
			source:   `len([1, 2, 3])`,
			expected: 3,
		},
		{
			source:   `const array<int> a = []; append(a, 1)`,
			expected: []int{1},
		},
	}

	runVmTests(t, tests)
}

func TestClosures(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `
			const newClosure = func(int a) -> fn<() -> int> {
				func() -> int { a; };
			}
			const closure = newClosure(99);
			closure();
			`,
			expected: 99,
		},
	}

	runVmTests(t, tests)
}

func TestRecursiveFunctions(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `
			const countDown = func(int x) -> int {
				if (x == 0) {
					return 0;
				} else {
					countDown(x - 1);
				}
			}
			countDown(1);
			`,
			expected: 0,
		},
		{
			source: `const countDown = func(int x) -> int {
				if (x == 0) {
					return 0;
				}

				return countDown(x - 1);
			};
			const wrapper = func() -> int {
				countDown(1);
			};
			wrapper();
			`,
			expected: 0,
		},
		{
			`
			const wrapper = func() -> int {
				const countDown = func(int x) -> int {
					if (x == 0) {
						return 0;
					} else {
						countDown(x - 1);
					}

				};
				countDown(1);
			};
			wrapper();
			`,
			0,
		},
	}

	runVmTests(t, tests)
}

func TestRecursiveFibonacci(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `
			const fib = func(int x) -> int {
				if (x == 0) {
					return 0;
				} else {
					if (x == 1) {
						return 1;
					} else {
						fib(x - 1) + fib(x - 2);
					}
				}
			}
			fib(15);
			`,
			expected: 610,
		},
	}

	runVmTests(t, tests)
}

func TestIndexAssingment(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `const array<int> a = [0]; a[0] = 2; a[0];`,
			expected: 2,
		},
		{
			source:   `const map<int, int> m = { 0: 1 }; m[0] = 2; match m[0] { some(v) -> { v; }, none -> { 0; }, };`,
			expected: 2,
		},
		{
			source:   `const map<int, int> m = {}; m[0] = 2; match m[0] { some(v) -> { v; }, none -> { 0; }, };`,
			expected: 2,
		},
	}

	runVmTests(t, tests)
}

func TestStructs(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `define struct Point { x int, y int }
					const Point p = Point { x: 0, y: 0 };
					p.y = 1;
					p.x = 1;
					p.x + p.y;`,
			expected: 2,
		},
	}

	runVmTests(t, tests)
}

func TestInterfaces(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `
		define struct Rect { w int, h int }
		define struct Point { x int, y int }
		define struct Circle { p Point, r int }

		define interface Area { area() -> int }


		func area(Circle c) -> int {
			const pi = 3;
			return c.r * c.r * pi;
		}

		func area(Rect r) -> int {
			return r.w * r.h;
		}

		const Rect r = Rect { w: 2, h: 2 }
		r.area();
		`,
			expected: 4,
		},
	}

	runVmTests(t, tests)
}

func TestInterfacesAsParams(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `define struct Rect { w float, h float }
		define interface Area { area() -> float }

		func area(Rect r) -> float {
			return r.w * r.h;
		}
		
		func getArea(Area a) -> float {
			return a.area();
		}
		
		const Rect r = Rect { w: 2.0, h: 2.0 };

		getArea(r);
`,
			expected: 4.0,
		},
	}

	runVmTests(t, tests)
}

func TestMatchExpression(t *testing.T) {
	tests := []vmTestCase{
		{ // match ok arm
			source: `func f() -> result<int> { return ok(42); }
			const r = f();
			match r {
				ok(val) -> { val + 1; },
				err(msg) -> { 0; },
			};`,
			expected: 43,
		},
		{ // match err arm
			source: `func f() -> result<int> { return err("bad"); }
			const r = f();
			match r {
				ok(val) -> { val + 1; },
				err(msg) -> { 0; },
			};`,
			expected: 0,
		},
		{ // match as value in variable
			source: `func f() -> result<int> { return ok(10); }
			const r = f();
			const x = match r {
				ok(val) -> { val * 2; },
				err(msg) -> { 0; },
			};
			x;`,
			expected: 20,
		},
		{ // match err arm with string
			source: `func f() -> result<int> { return err("oops"); }
			const r = f();
			match r {
				ok(val) -> { "success"; },
				err(msg) -> { msg; },
			};`,
			expected: "oops",
		},
		{ // match with err first
			source: `func f() -> result<int> { return ok(5); }
			const r = f();
			match r {
				err(msg) -> { 0; },
				ok(val) -> { val + 5; },
			};`,
			expected: 10,
		},
	}

	runVmTests(t, tests)
}

func TestOptionMatch(t *testing.T) {
	tests := []vmTestCase{
		{ // match some arm
			source: `const option<int> x = some(42);
			match x {
				some(val) -> { val + 1; },
				none -> { 0; },
			};`,
			expected: 43,
		},
		{ // match none arm
			source: `const option<int> x = none();
			match x {
				some(val) -> { val + 1; },
				none -> { 0; },
			};`,
			expected: 0,
		},
		{ // match as value in variable
			source: `const option<int> x = some(10);
			const y = match x {
				some(val) -> { val * 2; },
				none -> { 0; },
			};
			y;`,
			expected: 20,
		},
		{ // none first
			source: `const option<int> x = some(5);
			match x {
				none -> { 0; },
				some(val) -> { val + 5; },
			};`,
			expected: 10,
		},
	}

	runVmTests(t, tests)
}

func TestThreePartForLoops(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `mut sum = 0; for (mut i = 0; i < 5; i = i + 1) { sum = sum + i; } sum;`,
			expected: 10,
		},
		{
			source:   `mut s = ""; for (mut i = 0; i < 3; i = i + 1) { s = s + "x"; } s;`,
			expected: "xxx",
		},
		{ // init variable doesn't leak — reuse i
			source:   `mut r = 0; for (mut i = 0; i < 3; i = i + 1) { r = r + i; } for (mut i = 0; i < 3; i = i + 1) { r = r + i; } r;`,
			expected: 6,
		},
	}
	runVmTests(t, tests)
}

func TestBreakStatement(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `mut sum = 0; for (mut i = 0; i < 10; i = i + 1) { if (i == 3) { break; } sum = sum + i; } sum;`,
			expected: 3,
		},
		{
			source:   `mut r = 0; for (r < 100) { r = r + 1; if (r == 5) { break; } } r;`,
			expected: 5,
		},
	}
	runVmTests(t, tests)
}

func TestContinueStatement(t *testing.T) {
	tests := []vmTestCase{
		{ // skip even numbers
			source:   `mut sum = 0; for (mut i = 0; i < 6; i = i + 1) { if (i % 2 == 0) { continue; } sum = sum + i; } sum;`,
			expected: 9,
		},
	}
	runVmTests(t, tests)
}

func TestModulo(t *testing.T) {
	tests := []vmTestCase{
		{source: `10 % 3;`, expected: 1},
		{source: `15 % 5;`, expected: 0},
		{source: `7 % 2;`, expected: 1},
	}
	runVmTests(t, tests)
}

func TestConversionBuiltins(t *testing.T) {
	tests := []vmTestCase{
		{source: `int('a');`, expected: 97},
		{source: `char(byte(72));`, expected: "H"},
	}
	runVmTests(t, tests)
}

func TestIfExpressionAsStatement(t *testing.T) {
	tests := []vmTestCase{
		{ // if-as-statement in loop shouldn't corrupt stack
			source:   `mut r = 0; for (mut i = 0; i < 3; i = i + 1) { if (i == 1) { r = 10; } } r;`,
			expected: 10,
		},
		{ // if-else as statement in loop
			source:   `mut r = 0; for (mut i = 0; i < 4; i = i + 1) { if (i % 2 == 0) { r = r + 1; } else { r = r + 10; } } r;`,
			expected: 22,
		},
	}
	runVmTests(t, tests)
}

func TestSliceExpressions(t *testing.T) {
	tests := []vmTestCase{
		// Array slicing with both bounds
		{
			source:   `const a = [1, 2, 3, 4, 5]; a[1:4];`,
			expected: []int{2, 3, 4},
		},
		// Array slicing from start
		{
			source:   `const a = [10, 20, 30, 40]; a[0:2];`,
			expected: []int{10, 20},
		},
		// Array slicing with omitted end
		{
			source:   `const a = [1, 2, 3, 4, 5]; a[2:-1];`,
			expected: []int{3, 4, 5},
		},
		// String slicing with both bounds
		{
			source:   `const s = "Hello, World!"; s[0:5];`,
			expected: "Hello",
		},
		// String slicing middle
		{
			source:   `const s = "Hello, World!"; s[7:12];`,
			expected: "World",
		},
		// Single element array slice
		{
			source:   `const a = [1, 2, 3]; a[1:2];`,
			expected: []int{2},
		},
	}
	runVmTests(t, tests)
}

func TestGenericFunctions(t *testing.T) {
	tests := []vmTestCase{
		// Generic identity function
		{
			source:   `func identity<T>(T x) -> T { x; } identity<int>(42);`,
			expected: 42,
		},
		// Generic identity with string
		{
			source:   `func identity<T>(T x) -> T { x; } identity<string>("hello");`,
			expected: "hello",
		},
		// Generic function with array parameter
		{
			source:   `func first<T>(array<T> a) -> T { a[0]; } first<int>([10, 20, 30]);`,
			expected: 10,
		},
		// Generic function called with different types
		{
			source: `func identity<T>(T x) -> T { x; }
			         const a = identity<int>(5);
			         const b = identity<int>(10);
			         a + b;`,
			expected: 15,
		},
		// Generic function with type param in body
		{
			source: `func sum<T>(array<T> vals) -> T {
				mut T acc = 0;
				for (mut i = 0; i < len(vals); i = i + 1) {
					acc = acc + vals[i];
				}
				acc;
			}
			sum<int>([1, 2, 3, 4, 5]);`,
			expected: 15,
		},
	}
	runVmTests(t, tests)
}

func TestGenericStructs(t *testing.T) {
	tests := []vmTestCase{
		// Generic struct field access
		{
			source: `define struct Box<T> { value T }
			         const b = Box<int> { value: 42 };
			         b.value;`,
			expected: 42,
		},
		// Generic struct with string
		{
			source: `define struct Box<T> { value T }
			         const b = Box<string> { value: "hello" };
			         b.value;`,
			expected: "hello",
		},
		// Multiple instantiations
		{
			source: `define struct Box<T> { value T }
			         const a = Box<int> { value: 5 };
			         const b = Box<int> { value: 10 };
			         a.value + b.value;`,
			expected: 15,
		},
	}
	runVmTests(t, tests)
}

func TestBufferedChannel(t *testing.T) {
	tests := []vmTestCase{
		{
			`const ch = chan<int>(1);
			ch <- 99;
			<- ch;`,
			99,
		},
	}

	runVmTests(t, tests)
}

func TestUnbufferedChannel(t *testing.T) {
	tests := []vmTestCase{
		{
			`const ch = chan<int>();
			spawn func() {
				ch <- 42;
			}();
			<- ch;`,
			42,
		},
	}

	runVmTests(t, tests)
}

func runVmTests(t *testing.T, tests []vmTestCase) {
	t.Helper()

	for _, tt := range tests {
		program := parse(tt.source)

		c := typechecker.New(nil)
		errors := c.Check(program, nil)
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		ast.FilterGenericTemplates(program)

		comp := compiler.New()
		err := comp.Compile(program)

		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}

		vm := New(comp.Bytecode())
		err = vm.Run()
		if err != nil {
			t.Fatalf("vm error: %s", err)
		}

		stackElem := vm.LastPoppedStackElem()
		testExpectedObject(t, tt.expected, stackElem)
	}
}

func testExpectedObject(t *testing.T, expected interface{}, actual object.Object) {
	t.Helper()

	switch expected := expected.(type) {
	case int:
		err := testIntegerObject(int64(expected), actual)
		if err != nil {
			t.Errorf("testIntegerObject failed: %s", err)
		}
	case float64:
		err := testFloatObject(expected, actual)
		if err != nil {
			t.Errorf("testFloatObject failed: %s", err)
		}
	case float32:
		err := testFloatObject(float64(expected), actual)
		if err != nil {
			t.Errorf("testFloatObject failed: %s", err)
		}
	case bool:
		err := testBooleanObject(expected, actual)
		if err != nil {
			t.Errorf("testBooleanObject failed: %s", err)
		}
	case string:
		err := testStringObject(expected, actual)
		if err != nil {
			t.Errorf("testStringObject failed: %s", err)
		}
	case []int:
		array, ok := actual.(*object.Array)
		if !ok {
			t.Errorf("object is not Array. got=%T (%+v)", actual, actual)
			return
		}
		if len(array.Elements) != len(expected) {
			t.Errorf("wrong number of elements. want=%d, got=%d", len(expected), len(array.Elements))
			return
		}

		for i, expectedElem := range expected {
			err := testIntegerObject(int64(expectedElem), array.Elements[i])
			if err != nil {
				t.Errorf("testIntegerObject failed: %s", err)
			}
		}
	case map[object.HashKey]int64:
		hash, ok := actual.(*object.Hash)
		if !ok {
			t.Errorf("object is not Hash. got=%T (%+v)", actual, actual)
			return
		}

		if len(hash.Pairs) != len(expected) {
			t.Errorf("hash has wrong number of pairs. want=%d, got=%d", len(expected), len(hash.Pairs))
			return
		}

		for expectedKey, expectedValue := range expected {
			pair, ok := hash.Pairs[expectedKey]
			if !ok {
				t.Errorf("no pair for given key in Pairs")
				return
			}

			err := testIntegerObject(expectedValue, pair.Value)
			if err != nil {
				t.Errorf("testIntegerObject failed: %s", err)
			}
		}
	case *object.Null:
		if actual != Null {
			t.Errorf("object is not Null. got=%T (%+v)", actual, actual)
		}
	case *object.Error:
		errObj, ok := actual.(*object.Error)
		if !ok {
			t.Errorf("object is not Error. got=%T (%+v)", actual, actual)
			return
		}

		if errObj.Message != expected.Message {
			t.Errorf("wrong error message. want=%q, got=%q", expected.Message, errObj.Message)
		}
	}
}

func parse(source string) *ast.Program {
	l := lexer.New(source)
	p := parser.New(l)
	return p.ParseProgram()
}

func testIntegerObject(expected int64, actual object.Object) error {
	result, ok := actual.(*object.Integer)
	if !ok {
		return fmt.Errorf("object is not Integer. got=%T (%+v)", actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%d, want=%d", result.Value, expected)
	}

	return nil
}

func testFloatObject(expected float64, actual object.Object) error {
	result, ok := actual.(*object.Float)
	if !ok {
		return fmt.Errorf("object is not Float. got=%T (%+v)", actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%f, want=%f", result.Value, expected)
	}

	return nil
}

func testBooleanObject(expected bool, actual object.Object) error {
	result, ok := actual.(*object.Boolean)
	if !ok {
		return fmt.Errorf("object is not Boolean. got=%T (%+v)", actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%t, want=%t", result.Value, expected)
	}

	return nil
}

func testStringObject(expected string, actual object.Object) error {
	result, ok := actual.(*object.String)
	if !ok {
		return fmt.Errorf("object is not String. got=%T (%+v)", actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%s, want=%s", result.Value, expected)
	}

	return nil
}

func TestForInArray(t *testing.T) {
	tests := []vmTestCase{
		// Basic value iteration
		{
			source:   `mut s = 0; const a = [1, 2, 3]; for (v in a) { s = s + v; } s;`,
			expected: 6,
		},
		// Index and value iteration
		{
			source:   `mut s = 0; const a = [10, 20, 30]; for (i, v in a) { s = s + i + v; } s;`,
			expected: 63, // (0+10) + (1+20) + (2+30)
		},
		// Empty array
		{
			source:   `mut s = 0; const array<int> a = []; for (v in a) { s = s + 1; } s;`,
			expected: 0,
		},
		// String array
		{
			source:   `mut s = ""; const a = ["a", "b", "c"]; for (v in a) { s = s + v; } s;`,
			expected: "abc",
		},
		// Nested for-in
		{
			source: `mut s = 0;
			         const a = [1, 2];
			         const b = [10, 20];
			         for (x in a) { for (y in b) { s = s + x * y; } }
			         s;`,
			expected: 90, // 1*10 + 1*20 + 2*10 + 2*20
		},
	}
	runVmTests(t, tests)
}

func TestForInMap(t *testing.T) {
	tests := []vmTestCase{
		// Basic map iteration - sum values
		{
			source:   `mut s = 0; const m = {"a": 1, "b": 2, "c": 3}; for (k, v in m) { s = s + v; } s;`,
			expected: 6,
		},
		// Map iteration with int keys
		{
			source:   `mut s = 0; const m = {1: 10, 2: 20}; for (k, v in m) { s = s + k + v; } s;`,
			expected: 33, // (1+10) + (2+20)
		},
	}
	runVmTests(t, tests)
}

func TestStructMethods(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `define struct Point { x int, y int }
			func getX(Point p) -> int { return p.x; }
			const Point p = Point { x: 42, y: 0 };
			p.getX();`,
			expected: 42,
		},
		{
			source: `define struct Counter { val int }
			func add(Counter c, int n) -> int { return c.val + n; }
			const Counter c = Counter { val: 10 };
			c.add(5);`,
			expected: 15,
		},
		{
			source: `define struct Greeter { name string }
			func greet(Greeter g, string prefix) -> string { return prefix + " " + g.name; }
			const Greeter g = Greeter { name: "world" };
			g.greet("hello");`,
			expected: "hello world",
		},
	}
	runVmTests(t, tests)
}

func TestMutableStructFields(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `define struct Point { x int, y int }
			mut Point p = Point { x: 1, y: 2 };
			p.x = 10;
			p.x;`,
			expected: 10,
		},
		{
			source: `define struct Config { name string }
			mut Config c = Config { name: "old" };
			c.name = "new";
			c.name;`,
			expected: "new",
		},
	}
	runVmTests(t, tests)
}

func TestStringConcatenation(t *testing.T) {
	tests := []vmTestCase{
		{`"hello" + " " + "world";`, "hello world"},
		{`const a = "foo"; const b = "bar"; a + b;`, "foobar"},
		{`const s = "abc"; s + s + s;`, "abcabcabc"},
	}
	runVmTests(t, tests)
}

func TestFunctionLiteralAsValue(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `const f = func(int x) -> int { x * 2; }; f(5);`,
			expected: 10,
		},
		{
			source:   `const f = func(int a, int b) -> int { a + b; }; f(3, 7);`,
			expected: 10,
		},
		{
			source: `func apply(fn<(int) -> int> f, int x) -> int { return f(x); }
			apply(func(int x) -> int { x + 1; }, 41);`,
			expected: 42,
		},
	}
	runVmTests(t, tests)
}

func TestByteOperations(t *testing.T) {
	tests := []vmTestCase{
		{`mut byte b = 'a'; int(b);`, 97},
		{`const byte b = byte(65); char(b);`, "A"},
		{`mut byte b = 'z'; b == 'z';`, true},
		{`mut byte b = 'a'; b == 'b';`, false},
	}
	runVmTests(t, tests)
}

func TestForInWithBreak(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `mut s = 0; const a = [1, 2, 3, 4, 5]; for (v in a) { if (v == 4) { break; } s = s + v; } s;`,
			expected: 6, // 1+2+3
		},
	}
	runVmTests(t, tests)
}

func TestForInWithContinue(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `mut s = 0; const a = [1, 2, 3, 4, 5]; for (v in a) { if (v == 3) { continue; } s = s + v; } s;`,
			expected: 12, // 1+2+4+5
		},
	}
	runVmTests(t, tests)
}

func TestPanicInMatchArm(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `func safe() -> result<int> { ok(42); }
			const r = safe();
			const val = match r {
				ok(v) -> { v; },
				err(msg) -> { panic(msg); },
			};
			val;`,
			expected: 42,
		},
	}
	runVmTests(t, tests)
}

func TestNestedMatchExpressions(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `func outer() -> result<int> { ok(10); }
			func inner(int x) -> option<int> { some(x * 2); }
			const r = outer();
			const val = match r {
				ok(v) -> {
					const o = inner(v);
					match o {
						some(x) -> { x; },
						none -> { 0; },
					};
				},
				err(msg) -> { 0; },
			};
			val;`,
			expected: 20,
		},
	}
	runVmTests(t, tests)
}

func TestTypeMatch(t *testing.T) {
	tests := []vmTestCase{
		{ // match first arm
			source: `
		define struct Circle { r int }
		define struct Rect { w int, h int }

		define interface Shape { area() -> int }

		func area(Circle c) -> int { c.r * c.r * 3; }
		func area(Rect r) -> int { r.w * r.h; }

		func describe(Shape s) -> int {
			match typeof s {
				Circle(c) -> { c.r; },
				Rect(r) -> { r.w + r.h; },
				_ -> { 0; },
			};
		}

		const Circle c = Circle { r: 5 };
		describe(c);
		`,
			expected: 5,
		},
		{ // match second arm
			source: `
		define struct Circle { r int }
		define struct Rect { w int, h int }

		define interface Shape { area() -> int }

		func area(Circle c) -> int { c.r * c.r * 3; }
		func area(Rect r) -> int { r.w * r.h; }

		func describe(Shape s) -> int {
			match typeof s {
				Circle(c) -> { c.r; },
				Rect(r) -> { r.w + r.h; },
				_ -> { 0; },
			};
		}

		const Rect r = Rect { w: 3, h: 4 };
		describe(r);
		`,
			expected: 7,
		},
		{ // match default arm
			source: `
		define struct Circle { r int }
		define struct Rect { w int, h int }
		define struct Tri { b int }

		define interface Shape { area() -> int }

		func area(Circle c) -> int { c.r * c.r * 3; }
		func area(Rect r) -> int { r.w * r.h; }
		func area(Tri t) -> int { t.b; }

		func describe(Shape s) -> int {
			match typeof s {
				Circle(c) -> { c.r; },
				Rect(r) -> { r.w + r.h; },
				_ -> { 99; },
			};
		}

		const Tri t = Tri { b: 10 };
		describe(t);
		`,
			expected: 99,
		},
		{ // type match as value in variable
			source: `
		define struct Circle { r int }
		define struct Rect { w int, h int }

		define interface Shape { area() -> int }

		func area(Circle c) -> int { c.r * c.r * 3; }
		func area(Rect r) -> int { r.w * r.h; }

		func describe(Shape s) -> int {
			const x = match typeof s {
				Circle(c) -> { c.r * 10; },
				Rect(r) -> { r.w * 10; },
				_ -> { 0; },
			};
			x;
		}

		const Circle c = Circle { r: 7 };
		describe(c);
		`,
			expected: 70,
		},
	}

	runVmTests(t, tests)
}

func TestAnyTypeMatch(t *testing.T) {
	tests := []vmTestCase{
		{
			`func check(any val) -> int {
				match typeof val {
					int(i) -> { i; },
					_ -> { 0; },
				};
			}
			check(42);`,
			42,
		},
		{
			`func check(any val) -> string {
				match typeof val {
					int(i) -> { "int"; },
					string(s) -> { s; },
					_ -> { "other"; },
				};
			}
			check("hello");`,
			"hello",
		},
		{
			`func check(any val) -> string {
				match typeof val {
					int(i) -> { "int"; },
					float(f) -> { "float"; },
					bool(b) -> { "bool"; },
					_ -> { "other"; },
				};
			}
			check(true);`,
			"bool",
		},
	}

	runVmTests(t, tests)
}

func TestAnyArrayTypeMatch(t *testing.T) {
	tests := []vmTestCase{
		{
			`func first_type(array<any> args) -> string {
				const any a = args[0];
				match typeof a {
					int(i) -> { "int"; },
					string(s) -> { "string"; },
					_ -> { "other"; },
				};
			}
			first_type([42]);`,
			"int",
		},
		{
			`func first_type(array<any> args) -> string {
				const any a = args[0];
				match typeof a {
					int(i) -> { "int"; },
					string(s) -> { "string"; },
					_ -> { "other"; },
				};
			}
			first_type(["hello"]);`,
			"string",
		},
	}

	runVmTests(t, tests)
}
