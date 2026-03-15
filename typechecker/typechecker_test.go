package typechecker

import (
	"strings"
	"sydney/lexer"
	"sydney/parser"
	"testing"
)

type TypeErrorTest struct {
	input         string
	expectedError string
}

func TestValidTypeChecking(t *testing.T) {
	source := []string{
		"const int x = 5; x;",
		"define interface Area { area() -> float }",
		`define struct Point { x int, y int }
		define struct Circle { p Point, radius float }
		
		define interface Area { area() -> float }
		define implementation Circle -> Area
		func area(Circle c) -> float {
			const pi = 3.14;
			return c.radius * c.radius * pi;
		};`,

		`define struct Rect { w float, h float }
		define struct Point { x float, y float }
		define struct Circle { p Point, r float }
		
		define interface Area { area() -> float }
		
		define implementation Circle -> Area
		define implementation Rect -> Area
		
		func area(Circle c) -> float {
			const pi = 3.14;
			return c.r * c.r * pi;
		}
		
		func area(Rect r) -> float {
			return r.w * r.h;
		}`,
		`define struct Rect { w float, h float }
		
		define interface Area { area() -> float }
		
		define implementation Rect -> Area
		
		func area(Rect r) -> float {
			return r.w * r.h;
		}
		
		func getArea(Area a) -> float {
			return a.area();
		}
		
		const Rect r = Rect { w: 2.0, h: 2.0 };
		
		getArea(r);`,
		`func f() -> int { func f() -> int { return 0 }; return f(); }`,
		`define struct Dog { name string, bark string }
		define interface Pet {
			speak() -> string,
			isSame(Pet p) -> bool,
			name() -> string,
		}
		
		func speak(Dog d) -> string {
			return d.name + " says " + d.bark + "!";
		}
		
		func name(Dog d) -> string {
			return d.name;
		}
		
		func isSame(Dog d, Pet p) -> bool {
			return d.name == p.name();
		}
		
		define implementation Dog -> Pet
		
		const Dog fido = Dog { name: "Fido", bark: "Woof" };
		const Dog rover = Dog { name: "Rover", bark: "Awoo" };
		
		print("--- Are they the same? ---");
		print(fido.name + " and " + rover.name)
		print(fido.isSame(rover));
		`,
		`define struct Dog { name string, bark string }
		define interface Pet { name() -> string }
		define implementation Dog -> Pet
		func name(Dog d) -> string { return d.name }
		func getPet() -> Pet {
				return Dog { name: "Fido", bark: "Woof" };
		}`,
		`define struct Dog { name string, bark string }
		define struct Cat { name string, purr string }
		define interface Pet { name() -> string }
		define implementation Dog -> Pet
		define implementation Cat -> Pet
		
		func name(Cat c) -> string {
			return c.name;
		}
		
		func name(Dog d) -> string {
			return d.name;
		}
		
		const array<Pet> pets = [Dog { name: "A", bark: "B" }, Cat { name: "C", purr: "D" }];`,
		`const r = ok(5);`,
		`const result<int> e = err("some error")`,
		`mut result<int> x;`,
		`func f() -> result<int> { return err("some error") }
		const r = f();
		`,
		`func f() -> result<int> { return ok(5); }
		const r = f();
		const x = match r {
			ok(val) -> { val + 1; },
			err(msg) -> { 0; },
		};`,
		`func f() -> result<int> { return ok(5); }
		const r = f();
		match r {
			ok(val) -> { print(val); },
			err(msg) -> { print(msg); },
		};`,
		`func f() -> result<int> { return ok(5); }
		const r = f();
		const x = match r {
			err(msg) -> { 0; },
			ok(val) -> { val * 2; },
		};`,
		`const byte b = 'a';`,
		`'a' + 'b'`,
		`'a' - 'b'`,
	}

	for _, s := range source {

		l := lexer.New(s)
		p := parser.New(l)
		program := p.ParseProgram()
		c := New(nil)
		c.Check(program, nil)

		errors := c.Errors()
		if len(errors) != 0 {
			t.Fatalf("typechecker errors: %v", errors)
		}
	}

}

func TestFirstClassFunctions(t *testing.T) {
	source := `const returnsOne = func() -> int { 1; };
			const returnsOneReturner = func() -> fn<() -> int> { returnsOne; };
			returnsOneReturner()();`
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	c := New(nil)
	c.Check(program, nil)
	errors := c.Errors()
	if len(errors) != 0 {
		t.Fatalf("typechecker errors: %v", errors)
	}
}

func TestTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{ // var declaration
			"const int x = \"hello\";",
			"type mismatch: cannot assign string to variable x of type int",
		},
		{ // var assignment
			"mut int x = 5; x = false;",
			"type mismatch: cannot assign bool to variable x of type int",
		},
		{ // for loop
			"mut int x = 0; for (x - 5) { x = x + 1 }; x;",
			"cannot use expression of type int for loop condition",
		},
		{ // return statement
			"const f = func(int x) -> int { return false }",
			"cannot return bool from function expecting int",
		},
		{ // for loop
			"mut int x = 0; for (x < 5) { x = x + false; }; x;",
			"type mismatch: cannot add types int and bool",
		},
		{ // function symbol resolution
			"f(5);",
			"undefined identifier: f",
		},
		{ // calling non function
			"mut int x = 0; x();",
			"cannot call non-function int x",
		},
		{ // invalid arg type
			"const f = func(int x) -> int { return x * 2 }; f(false)",
			"type mismatch: got bool for arg 1 in function f call, expected int",
		},
		{ // scope nesting
			"mut int x = 5; if (x == 5) { mut string x = \"hello\"; x = 5; }",
			"type mismatch: cannot assign int to variable x of type string",
		},
		{ // if alternative
			"mut int x = 5; if (x == 5) { mut string x = \"hello\"; } else { x = \"world\"; }",
			"type mismatch: cannot assign string to variable x of type int",
		},
		{ // prefix expression !
			"mut int x = 5; !x;",
			"invalid operation: ! is not defined for int",
		},
		{ // prefix expression -
			"mut string x = \"hello\"; -x;",
			"invalid operation: - is not defined for string",
		},
		{
			"mut int x = 5; mut int x = 4;",
			"variable x already declared",
		},
		{
			"mut byte x = 0;",
			"type mismatch: cannot assign int to variable x of type byte",
		},
	}

	testTypeErrors(t, tests)
}

func TestInfixExpresionErrorTesting(t *testing.T) {
	tests := []TypeErrorTest{
		{
			"10 + true",
			"type mismatch: cannot add types int and bool",
		},
		{
			"5 == false",
			"type mismatch: cannot compare types int to bool",
		},
		{
			"5 >= false",
			"type mismatch: cannot compare types int to bool",
		},
		{
			"5 <= false",
			"type mismatch: cannot compare types int to bool",
		},
		{
			"5 != false",
			"type mismatch: cannot compare types int to bool",
		},
		{
			"5 > false",
			"type mismatch: cannot compare types int to bool",
		},
		{
			"5 > false",
			"type mismatch: cannot compare types int to bool",
		},
		{
			"5 && false",
			"type mismatch: cannot perform boolean operation on types int and bool",
		},
		{
			"5 || false",
			"type mismatch: cannot perform boolean operation on types int and bool",
		},
		{
			"'a' * 5",
			"type mismatch: cannot multiply types byte and int",
		},
	}

	testTypeErrors(t, tests)
}

func TestIndexExpressionsErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{ // index expr
			"mut map<int, string> x = { 0: \"hello\" }; x[false];",
			"index type for map map<int, string> must be int, got bool",
		},
		{
			"mut array<int> a = [5]; a[false];",
			"index type for array must be int, got bool",
		},
		{
			"const array<int> a = [1]; a[1] = false;",
			"type mismatch: cannot assign bool to element of array of type array<int>",
		},
		{
			"const array<int> a = [1]; a[false] = 1;",
			"index type for array must be int, got bool",
		},
	}

	testTypeErrors(t, tests)
}

func TestArrayTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			"mut array<int> a = [false];",
			"type mismatch: array element got bool, expected int",
		},
		{
			"mut array<int> a = [1, false, \"hello\"];",
			"type mismatch: array element got bool, expected int",
		},
		{
			"const array<array<int>> a = [[null]]",
			"type mismatch: array element got null, expected int",
		},
	}

	testTypeErrors(t, tests)
}

func TestMapTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			"mut map<int, int> a = { 1: 1 }; a = { 1: false };",
			"type mismatch: cannot assign map<int, bool> to variable a of type map<int, int>",
		},
		{
			"mut map <int, int> a = {}; a = { 1: false };",
			"type mismatch: cannot assign map<int, bool> to variable a of type map<int, int>",
		},
		{
			"const map<int, int> a = { 1: 0, false: 0};",
			"type mismatch: map key got bool, expected int",
		},
		{
			"const map<int, int> a = { 1: 0, 0: false};",
			"type mismatch: map value got int, expected bool",
		},
	}

	testTypeErrors(t, tests)
}

func TestIfExpressionErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			"mut int x = if (true) { 4 } else { false };",
			"consequence and alternative for if expression must result in same type",
		},
	}

	testTypeErrors(t, tests)
}

func TestFunctionDeclarationErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         "func f(int x) -> int { return false; };",
			expectedError: "cannot return bool from function expecting int",
		},
		{
			input:         "func f(int x) { x + 2; return x; };",
			expectedError: "cannot return int from function expecting unit",
		},
		{
			input:         "func f(int x) -> int { if (x == 0) { return false; } else { return x; };",
			expectedError: "cannot return bool from function expecting int",
		},
		{
			input:         "func f(int x) -> int { if (x == 0) { return x; } else { return false; };",
			expectedError: "cannot return bool from function expecting int",
		},
		{
			input:         "func f() -> int { if (x == 0) { return x; } else { return false; };",
			expectedError: "undefined identifier: x",
		},
		{
			input:         `func f() {}; func f() {};`,
			expectedError: "function f already declared",
		},
	}

	testTypeErrors(t, tests)
}

func TestStructLiteralTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input: `define struct Point { x int, y int }
					const p = Circle { x: 0, y: false };`,
			expectedError: "unknown type Circle",
		},
		{
			input: `define struct Point { x int, y int }
					const p = Point { x: 0, y: false };`,
			expectedError: "type mismatch for field y in struct Point: expected int, got bool",
		},
		{
			input: `define struct Point { x int, y int }
					const p = Point { x: 0 };`,
			expectedError: "missing field y in struct literal Point",
		},
		{
			input: `define struct Point { x int, y int }
					const p = Point { x: 0, y: 0, z: 0 };`,
			expectedError: "field z of struct type Point not found",
		},
	}

	testTypeErrors(t, tests)
}

func TestSelectorExpressionTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input: `define struct Point { x int, y int }
					const p = Point { x: 0, y: 0 };
					p.z;`,
			expectedError: "field z of struct type p not found",
		},
		{
			input:         "const x = 5; x.foo();",
			expectedError: "cannot access field of non-struct value x of type int",
		},
	}

	testTypeErrors(t, tests)
}

func TestNestedStructFieldTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input: `define struct Point { x int, y int }
define struct Circle { center Point, radius int }
const c = Circle { center: Point { x: "bad", y: 0 }, radius: 5 };`,
			expectedError: "type mismatch for field x in struct Point: expected int, got string",
		},
	}

	testTypeErrors(t, tests)
}

func TestBuiltinFunctionTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         "len(1)",
			expectedError: "invalid argument type int for len()",
		},
		{
			input:         "len([1], [2])",
			expectedError: "len() expects exactly 1 argument",
		},
		{
			input:         "len(3.14)",
			expectedError: "invalid argument type float for len()",
		},
		{
			input:         "append([false], 1)",
			expectedError: "type mismatch: got int for append() value",
		},
	}

	testTypeErrors(t, tests)
}

func TestInterfaceMethodArgumentTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input: `
define interface Pet { greet(string name) -> string }
func greet(Dog d, string name) -> string { return "" }
func test(Pet p) {
	p.greet(123);
}`,
			expectedError: "type mismatch: got int for arg 1 in function p.greet call, expected string",
		},
	}

	testTypeErrors(t, tests)
}

func TestInterfaceImplementationTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input: `define interface Sized { size(int unit) -> int }
define struct Box { w int }
func size(Box b, string unit) -> int { b.w }
define implementation Box -> Sized`,
			expectedError: "struct Box does not satisfy interface Sized, wrong signature for method size. got func<(Box, string) -> int>, want func<(int) -> int>",
		},
	}

	testTypeErrors(t, tests)
}

func TestConstantReassingTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         "const x = 0; x = 1;",
			expectedError: "cannot assign to constant variable x",
		},
	}

	testTypeErrors(t, tests)
}

func TestNestedArrays(t *testing.T) {
	s := `const array<array<int>> nested = [[1, 2, 3], [4, 5, 6]];
print(nested[0][0]);
nested[0][1] = 10;
`

	l := lexer.New(s)
	p := parser.New(l)
	program := p.ParseProgram()
	c := New(nil)
	c.Check(program, nil)

	errors := c.Errors()
	if len(errors) != 0 {
		t.Fatalf("typechecker errors: %v", errors)
	}
}

func TestErrConstructorTypeErrorChecking(t *testing.T) {
	tt := []TypeErrorTest{
		{
			input:         "const r = err(5)",
			expectedError: "invalid argument type int for err()",
		},
		{
			input:         "mut r = err(\"some error\")",
			expectedError: "cannot infer result type for err()",
		},
	}

	testTypeErrors(t, tt)
}

func TestMatchExprTypeErrorChecking(t *testing.T) {
	tt := []TypeErrorTest{
		{
			input: `const x = 5;
			match x {
				ok(val) -> { val; },
				err(msg) -> { 0; },
			};`,
			expectedError: "can only match on result type",
		},
		{
			input: `func f() -> result<int> { return ok(5); }
			const r = f();
			const x = match r {
				ok(val) -> { val + 1; },
				err(msg) -> { "error"; },
			};`,
			expectedError: `type mismatch: match arms must result in same type, got int and string`,
		},
	}

	testTypeErrors(t, tt)
}

func TestBreakContinueOutsideLoop(t *testing.T) {
	tests := []TypeErrorTest{
		{"break;", "break statement cannot be outside of loop"},
		{"continue;", "continue statement cannot be outside of loop"},
		{"func foo() -> int { break; }", "break statement cannot be outside of loop"},
		{"func foo() -> int { continue; }", "continue statement cannot be outside of loop"},
	}
	testTypeErrors(t, tests)
}

func TestBreakContinueInsideLoop(t *testing.T) {
	sources := []string{
		"for (mut i = 0; i < 10; i = i + 1) { break; }",
		"for (mut i = 0; i < 10; i = i + 1) { continue; }",
		"for (mut i = 0; i < 10; i = i + 1) { if (i == 5) { break; } }",
	}
	for _, src := range sources {
		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()
		c := New(nil)
		c.Check(program, nil)
		if len(c.Errors()) != 0 {
			t.Fatalf("input %q expected no errors, got %v", src, c.Errors())
		}
	}
}

func TestThreePartForLoopScoping(t *testing.T) {
	sources := []string{
		`for (mut i = 0; i < 10; i = i + 1) { print(i); }
		 for (mut i = 0; i < 5; i = i + 1) { print(i); }`,
	}
	for _, src := range sources {
		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()
		c := New(nil)
		c.Check(program, nil)
		if len(c.Errors()) != 0 {
			t.Fatalf("input %q expected no errors, got %v", src, c.Errors())
		}
	}
}

func TestConversionBuiltinErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{`const x = int("hello");`, `invalid argument type string for int(), expected byte`},
		{`const x = byte("hello");`, `invalid argument type string for byte(), expected int`},
		{`const x = char(5);`, `invalid argument type int for char(), expected byte`},
	}
	testTypeErrors(t, tests)
}

func TestConversionBuiltins(t *testing.T) {
	sources := []string{
		`const x = int('a');`,
		`const x = byte(65);`,
		`const x = char(byte(65));`,
	}
	for _, src := range sources {
		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()
		c := New(nil)
		c.Check(program, nil)
		if len(c.Errors()) != 0 {
			t.Fatalf("input %q expected no errors, got %v", src, c.Errors())
		}
	}
}

func TestGenericFunctionMonomorphization(t *testing.T) {
	sources := []string{
		// Basic generic identity
		`func identity<T>(T x) -> T { return x; }
		const int r = identity<int>(42);`,

		// Multiple type params
		`func pair<T, U>(T a, U b) -> T { return a; }
		pair<int, string>(1, "hello");`,

		// Generic with array of type param
		`func first<T>(array<T> a) -> T { return a[0]; }
		const int x = first<int>([1, 2, 3]);`,
	}
	for _, src := range sources {
		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("parser errors: %v", p.Errors())
		}
		c := New(nil)
		c.Check(program, nil)
		if len(c.Errors()) != 0 {
			t.Fatalf("input %q expected no errors, got %v", src, c.Errors())
		}
	}
}

func TestGenericFunctionErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			`func identity<T>(T x) -> T { return x; }
			identity<int>("hello");`,
			`type mismatch: got string for arg 1 in function identity call, expected int`,
		},
		{
			`func identity<T>(T x) -> T { return x; }
			identity<int, string>(42);`,
			`identity expects exactly 1 type arguments`,
		},
	}
	testTypeErrors(t, tests)
}

func testTypeErrors(t *testing.T, tests []TypeErrorTest) {
	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := parser.New(l)
		program := p.ParseProgram()
		c := New(nil)
		errors := c.Check(program, nil)
		if len(c.Errors()) == 0 {
			t.Fatalf("input %q expected error but got none", tt.input)
		}

		if !strings.Contains(errors[0], tt.expectedError) {
			t.Fatalf("input %q expected error %q but got %q", tt.input, tt.expectedError, errors[0])
		}
	}
}

func TestGenericStructMonomorphization(t *testing.T) {
	sources := []string{
		// Basic generic struct
		`define struct Box<T> { value T }
		const b = Box<int> { value: 42 };`,

		// Two type params
		`define struct Pair<T, U> { first T, second U }
		const p = Pair<int, string> { first: 1, second: "hello" };`,

		// Field access on generic struct
		`define struct Box<T> { value T }
		const b = Box<int> { value: 42 };
		const int v = b.value;`,

		// Multiple instantiations of same generic struct
		`define struct Box<T> { value T }
		const a = Box<int> { value: 1 };
		const b = Box<string> { value: "hi" };`,

		// Generic struct with array field
		`define struct Container<T> { items array<T> }
		const c = Container<int> { items: [1, 2, 3] };`,
	}
	for _, src := range sources {
		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("parser errors: %v", p.Errors())
		}
		c := New(nil)
		c.Check(program, nil)
		if len(c.Errors()) != 0 {
			t.Fatalf("input %q expected no errors, got %v", src, c.Errors())
		}
	}
}

func TestGenericStructAsArgument(t *testing.T) {
	sources := []string{
		// Generic struct as function parameter
		`define struct Box<T> { value T }
		func unbox<T>(Box<T> b) -> T { return b.value; }
		const int r = unbox<int>(Box<int> { value: 42 });`,

		// Two type params in struct, used in function param
		`define struct Pair<T, U> { first T, second U }
		func getFirst<T, U>(Pair<T, U> p) -> T { return p.first; }
		const int r = getFirst<int, string>(Pair<int, string> { first: 1, second: "hi" });`,

		// Generic struct param with implicit return
		`define struct Box<T> { value T }
		func unbox<T>(Box<T> b) -> T { b.value; }
		const int r = unbox<int>(Box<int> { value: 10 });`,
	}
	for _, src := range sources {
		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("parser errors: %v", p.Errors())
		}
		c := New(nil)
		c.Check(program, nil)
		if len(c.Errors()) != 0 {
			t.Fatalf("input %q expected no errors, got %v", src, c.Errors())
		}
	}
}

func TestGenericStructAsArgumentErrors(t *testing.T) {
	tests := []TypeErrorTest{
		// Mismatched argument type
		{
			`define struct Box<T> { value T }
			func unbox<T>(Box<T> b) -> T { return b.value; }
			unbox<int>(Box<string> { value: "hi" });`,
			`type mismatch: got Box__string for arg 1 in function unbox call, expected Box__int`,
		},
	}
	testTypeErrors(t, tests)
}

func TestMissingReturnOnAllPaths(t *testing.T) {
	tests := []TypeErrorTest{
		// if without else
		{
			`func f(int x) -> int {
				if (x > 0) { return x; }
			}`,
			`function f missing return on all paths`,
		},
		// last statement is not an expression or return
		{
			`func f(int x) -> int {
				mut int y = x + 1;
			}`,
			`function f missing return on all paths`,
		},
		// empty body
		{
			`func f() -> int {}`,
			`function f missing return on all paths`,
		},
	}
	testTypeErrors(t, tests)
}

func TestGenericStructErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			`define struct Box<T> { value T }
			Box<int> { value: "hello" };`,
			`type mismatch for field value in struct Box__int: expected int, got string`,
		},
		{
			`define struct Pair<T, U> { first T, second U }
			Pair<int> { first: 1 };`,
			`Pair expects exactly 2 type arguments`,
		},
		{
			`define struct Box<T> { value T }
			Box<int> { wrong: 42 };`,
			`missing field value in struct literal Box__int`,
		},
	}
	testTypeErrors(t, tests)
}

func TestSliceExpr(t *testing.T) {
	sources := []string{
		`mut s = "Hello, World!"; s[1:3]`,
		`mut s = "Hello, World!"; s[:3]`,
		`mut s = "Hello, World!"; s[1]`,
		`mut a = [1, 2, 3, 4, 5]; a[1:3]`,
	}

	for _, src := range sources {
		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("input %q expected no errors, got %v", src, p.Errors())
		}
		c := New(nil)
		c.Check(program, nil)
		if len(c.Errors()) != 0 {
			t.Fatalf("input %q expected no errors, got %v", src, c.Errors())
		}
	}
}

func TestSliceExprErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `mut s = "Hello, World!"; s[:]`,
			expectedError: "must provide start or end for slice expression",
		},
		{
			input:         `mut m = { 1: 0, 2: 1, 3: 1 }; m[2:3]`,
			expectedError: "unsupported slice type map<int, int>",
		},
		{
			input:         `mut i = 5; i[1:]`,
			expectedError: "unsupported slice type int",
		},
	}
	testTypeErrors(t, tests)
}
