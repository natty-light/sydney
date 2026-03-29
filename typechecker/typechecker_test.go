package typechecker

import (
	"strings"
	"sydney/ast"
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
			expectedError: "can only match on result or option type",
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

func TestOptionMatchTypeChecking(t *testing.T) {
	sources := []string{
		// basic option match
		`const option<int> x = some(5);
		const y = match x {
			some(val) -> { val + 1; },
			none -> { 0; },
		};`,
		// none first
		`const option<int> x = some(5);
		const y = match x {
			none -> { 0; },
			some(val) -> { val + 1; },
		};`,
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

func TestOptionMatchTypeErrorChecking(t *testing.T) {
	tt := []TypeErrorTest{
		{
			input: `const option<int> x = some(5);
			const y = match x {
				some(val) -> { val + 1; },
				none -> { "nope"; },
			};`,
			expectedError: "type mismatch: match arms must result in same type, got int and string",
		},
		{
			input: `const x = 5;
			match x {
				some(val) -> { val; },
				none -> { 0; },
			};`,
			expectedError: "can only match on result or option type",
		},
	}
	testTypeErrors(t, tt)
}

func TestSomeNoneBuiltInTypeChecking(t *testing.T) {
	sources := []string{
		`const option<int> x = some(5);`,
		`const option<string> x = some("hello");`,
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

func TestNoneBuiltInTypeErrorChecking(t *testing.T) {
	tt := []TypeErrorTest{
		{
			input:         "const x = none()",
			expectedError: "cannot infer option type for none()",
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
		{`const x = int("hello");`, `type mismatch: got string for arg 1 in function int call, expected byte`},
		{`const x = byte("hello");`, `type mismatch: got string for arg 1 in function byte call, expected int`},
		{`const x = char(5);`, `type mismatch: got int for arg 1 in function char call, expected byte`},
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

		if !strings.Contains(errors[0].Message, tt.expectedError) {
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

func TestForInStmt(t *testing.T) {
	tests := []string{
		`const m = { 0: 0 };
		for (k, v in m) {
			print(k);
			print(v);
		}`,
		`const a = [1, 2, 3];
		for (i, v in a) {
			print(v);
			print(i);
		}`,
		`const a = [1, 2, 3];
		for (v in a) {
			print(v);
		}`,
	}

	for _, src := range tests {
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

func TestSpawnStmt(t *testing.T) {
	tests := []string{
		`func work() { const x = 1; }
		spawn work();`,

		`func add(int a, int b) -> int { a + b; }
		spawn add(1, 2);`,
	}

	for _, src := range tests {
		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("input %q expected no parse errors, got %v", src, p.Errors())
		}
		c := New(nil)
		c.Check(program, nil)
		if len(c.Errors()) != 0 {
			t.Fatalf("input %q expected no errors, got %v", src, c.Errors())
		}
	}
}

func TestSpawnStmtTypeErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         "spawn 5;",
			expectedError: "must spawn function call",
		},
		{
			input:         `func work(int a) { const x = a; } spawn work("hello");`,
			expectedError: "type mismatch",
		},
	}

	testTypeErrors(t, tests)
}

func TestChannelTypeChecking(t *testing.T) {
	source := `const chan<int> ch = chan<int>();
ch <- 5;
const int x = <- ch;`
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("input %q expected no errors, got %v", source, p.Errors())
	}
	c := New(nil)
	c.Check(program, nil)
	if len(c.Errors()) != 0 {
		t.Fatalf("input %q expected no errors, got %v", source, c.Errors())
	}
}

func TestStructMethodCall(t *testing.T) {
	sources := []string{
		`define struct Point { x int, y int }
		func getX(Point p) -> int { return p.x; }
		const Point p = Point { x: 1, y: 2 };
		const int x = p.getX();`,

		`define struct Counter { val int }
		func increment(Counter c, int n) -> int { return c.val + n; }
		const Counter c = Counter { val: 10 };
		const int r = c.increment(5);`,
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

func TestPanicAsNeverInMatch(t *testing.T) {
	sources := []string{
		`func open() -> result<int> { ok(42); }
		const r = open();
		const val = match r {
			ok(v) -> { v; },
			err(msg) -> { panic(msg); },
		};`,

		`func find() -> option<int> { some(1); }
		const o = find();
		const val = match o {
			some(v) -> { v; },
			none -> { panic("not found"); },
		};`,
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

func TestForInTypeErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `for (x in 5) { print(x); }`,
			expectedError: "cannot iterate over value of type int",
		},
		{
			input:         `for (x in "hello") { print(x); }`,
			expectedError: "cannot iterate over value of type string",
		},
	}
	testTypeErrors(t, tests)
}

func TestChannelSendTypeError(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `const chan<int> ch = chan<int>(); ch <- "hello";`,
			expectedError: "type mismatch",
		},
	}
	testTypeErrors(t, tests)
}

func TestResultTypeReturn(t *testing.T) {
	sources := []string{
		`func foo() -> result<int> { ok(42); }
		const r = foo();
		const val = match r {
			ok(v) -> { v; },
			err(msg) -> { 0; },
		};`,

		`func bar() -> result<string> { return err("oops"); }
		const r = bar();`,
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

func TestStructMethodCallTypeErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input: `define struct Point { x int, y int }
			func getX(Point p) -> int { return p.x; }
			const Point p = Point { x: 1, y: 2 };
			const string s = p.getX();`,
			expectedError: "type mismatch",
		},
	}
	testTypeErrors(t, tests)
}

func TestOptionTypeReturn(t *testing.T) {
	sources := []string{
		`func find(int x) -> option<int> {
			if (x > 0) { return some(x); }
			return none();
		}
		const r = find(5);`,

		`func lookup(string key) -> option<string> {
			return some("found");
		}
		const option<string> r = lookup("a");`,
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

func TestOptionAsParameter(t *testing.T) {
	sources := []string{
		`func unwrap(option<int> o) -> int {
			const val = match o {
				some(v) -> { v; },
				none -> { 0; },
			};
			return val;
		}
		unwrap(some(42));`,
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

func TestOptionTypeErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `const option<int> x = some("hello");`,
			expectedError: "type mismatch",
		},
		{
			input: `func find() -> option<int> { return some(5); }
			const r = find();
			const val = match r {
				some(v) -> { v + 1; },
				none -> { "nope"; },
			};`,
			expectedError: "type mismatch: match arms must result in same type",
		},
	}
	testTypeErrors(t, tests)
}

func TestInterfaceMethodDispatch(t *testing.T) {
	sources := []string{
		`define struct Circle { r float }
		define interface Shape { area() -> float }
		define implementation Circle -> Shape
		func area(Circle c) -> float { return c.r * c.r * 3.14; }
		func getArea(Shape s) -> float { return s.area(); }
		const Circle c = Circle { r: 5.0 };
		const float a = getArea(c);`,

		`define struct Dog { name string }
		define struct Cat { name string }
		define interface Pet { speak() -> string }
		define implementation Dog -> Pet
		define implementation Cat -> Pet
		func speak(Dog d) -> string { return d.name; }
		func speak(Cat c) -> string { return c.name; }
		func greet(Pet p) -> string { return p.speak(); }
		greet(Dog { name: "Rex" });
		greet(Cat { name: "Whiskers" });`,
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

func TestClosureCapture(t *testing.T) {
	sources := []string{
		`const int x = 10;
		const f = func() -> int { x; };
		const int r = f();`,

		`const int n = 5;
		const adder = func(int x) -> int { x + n; };
		const int r = adder(3);`,

		`const string greeting = "hello";
		const f = func(string name) -> string { greeting + " " + name; };
		const string r = f("world");`,
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

func TestMutableStructFieldAssignment(t *testing.T) {
	sources := []string{
		`define struct Point { x int, y int }
		mut Point p = Point { x: 1, y: 2 };
		p.x = 10;`,

		`define struct Config { name string, val int }
		mut Config c = Config { name: "a", val: 0 };
		c.name = "b";
		c.val = 42;`,
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

func TestFunctionLiteralAsArgument(t *testing.T) {
	sources := []string{
		`func apply(fn<(int) -> int> f, int x) -> int { return f(x); }
		const int r = apply(func(int x) -> int { x * 2; }, 5);`,

		`func filter(array<int> a, fn<(int) -> bool> pred) -> int { return a[0]; }
		filter([1, 2, 3], func(int x) -> bool { x > 1; });`,
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

func TestRecursiveFunction(t *testing.T) {
	sources := []string{
		`func fact(int n) -> int {
			if (n <= 1) { return 1; }
			return n * fact(n - 1);
		}
		const int r = fact(5);`,

		`func fib(int n) -> int {
			if (n <= 1) { return n; }
			return fib(n - 1) + fib(n - 2);
		}
		fib(10);`,
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

func TestStringConcatenation(t *testing.T) {
	sources := []string{
		`const string s = "hello" + " " + "world";`,
		`const string a = "foo";
		const string b = a + "bar";`,
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

func TestStringConcatenationTypeError(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `const x = "hello" + 5;`,
			expectedError: "type mismatch",
		},
	}
	testTypeErrors(t, tests)
}

func TestBuiltinFirst(t *testing.T) {
	sources := []string{
		`const a = [1, 2, 3]; const int x = first(a);`,
		`const a = ["a", "b"]; const string s = first(a);`,
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

func TestBuiltinFirstErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `first(5);`,
			expectedError: "invalid argument type int for first()",
		},
	}
	testTypeErrors(t, tests)
}

func TestBuiltinLast(t *testing.T) {
	sources := []string{
		`const a = [1, 2, 3]; const int x = last(a);`,
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

func TestBuiltinLastErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `last("hello");`,
			expectedError: "invalid argument type string for last()",
		},
	}
	testTypeErrors(t, tests)
}

func TestBuiltinRest(t *testing.T) {
	sources := []string{
		`const a = [1, 2, 3]; const array<int> r = rest(a);`,
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

func TestBuiltinRestErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `rest(5);`,
			expectedError: "invalid argument type int for rest()",
		},
	}
	testTypeErrors(t, tests)
}

func TestBuiltinKeysValues(t *testing.T) {
	sources := []string{
		`const m = {"a": 1, "b": 2}; const array<string> k = keys(m);`,
		`const m = {"a": 1, "b": 2}; const array<int> v = values(m);`,
		`const m = {1: "x", 2: "y"}; const array<int> k = keys(m);`,
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

func TestBuiltinKeysValuesErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `keys([1, 2, 3]);`,
			expectedError: "invalid argument type",
		},
		{
			input:         `values("hello");`,
			expectedError: "invalid argument type",
		},
	}
	testTypeErrors(t, tests)
}

func TestBuiltinAppend(t *testing.T) {
	sources := []string{
		`const a = [1, 2]; const array<int> b = append(a, 3);`,
		`const a = ["x"]; const array<string> b = append(a, "y");`,
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

func TestBuiltinAppendErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `const a = [1, 2]; append(a, "hello");`,
			expectedError: "type mismatch",
		},
	}
	testTypeErrors(t, tests)
}

func TestScopeTypeInParams(t *testing.T) {
	sources := []string{
		`define struct Foo { x int }
		func bar(Foo f) -> int { return f.x; }
		const Foo f = Foo { x: 5 };
		const int r = bar(f);`,
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

func TestMatchExprAsValue(t *testing.T) {
	sources := []string{
		`func f() -> result<int> { ok(5); }
		const r = f();
		const x = match r { ok(v) -> { v * 2; }, err(msg) -> { 0; }, };
		const int y = x + 1;`,

		`func g() -> option<string> { some("hi"); }
		const o = g();
		const s = match o { some(v) -> { v; }, none -> { "default"; }, };`,
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

func TestIndexAssignmentTypeErrors(t *testing.T) {
	tests := []TypeErrorTest{
		{
			input:         `mut a = [1, 2, 3]; a[0] = "hello";`,
			expectedError: "type mismatch",
		},
		{
			input:         `mut m = {"a": 1}; m["a"] = "hello";`,
			expectedError: "type mismatch",
		},
	}
	testTypeErrors(t, tests)
}

func TestNestedClosureCapture(t *testing.T) {
	sources := []string{
		`const int x = 1;
		const f = func() -> int {
			const g = func() -> int { x + 1; };
			g();
		};
		const int r = f();`,
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

func TestVariableShadowing(t *testing.T) {
	sources := []string{
		`const int x = 5;
		func f() -> int {
			const int x = 10;
			return x;
		}
		f();`,
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

func TestTypeMatchExpr(t *testing.T) {
	sources := []string{
		`define struct Circle { radius float }
		define struct Rect { w float, h float }

		define interface Shape { area() -> float }
		define implementation Circle -> Shape
		define implementation Rect -> Shape

		func area(Circle c) -> float { c.radius * c.radius * 3.14; }
		func area(Rect r) -> float { r.w * r.h; }

		func describe(Shape s) -> float {
			match typeof s {
				Circle(c) -> { c.radius; },
				Rect(r) -> { r.w; },
				_ -> { 0.0; },
			};
		}`,
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

func TestTypeMatchExprErrors(t *testing.T) {
	tt := []TypeErrorTest{
		{
			input: `const x = 5;
			match typeof x {
				int(v) -> { v; },
				_ -> { 0; },
			};`,
			expectedError: "match typeof requires any or interface type",
		},
		{
			input: `define struct Circle { radius float }
			define struct Rect { w float, h float }

			define interface Shape { area() -> float }
			define implementation Circle -> Shape

			func area(Circle c) -> float { c.radius * c.radius * 3.14; }

			func describe(Shape s) -> float {
				match typeof s {
					Circle(c) -> { c.radius; },
					Rect(r) -> { r.w; },
					_ -> { 0.0; },
				};
			}`,
			expectedError: "does not satisfy interface",
		},
		{
			input: `define struct Circle { radius float }
			define struct Rect { w float, h float }

			define interface Shape { area() -> float }
			define implementation Circle -> Shape
			define implementation Rect -> Shape

			func area(Circle c) -> float { c.radius * c.radius * 3.14; }
			func area(Rect r) -> float { r.w * r.h; }

			func describe(Shape s) -> string {
				match typeof s {
					Circle(c) -> { c.radius; },
					Rect(r) -> { "rect"; },
					_ -> { 0.0; },
				};
			}`,
			expectedError: "all arms of type match must result in same type",
		},
	}

	testTypeErrors(t, tt)
}

func TestMonomorphizedFunctionInsertedBeforeCall(t *testing.T) {
	src := `func identity<T>(T x) -> T { return x; }
	const int r = identity<int>(42);`

	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	c := New(nil)
	c.Check(program, nil)
	if len(c.Errors()) != 0 {
		t.Fatalf("expected no errors, got %v", c.Errors())
	}

	ast.FilterGenericTemplates(program)

	callIdx := -1
	fnIdx := -1
	for i, stmt := range program.Stmts {
		switch s := stmt.(type) {
		case *ast.FunctionDeclarationStmt:
			if s.Name.Value == "identity__int" {
				fnIdx = i
			}
		case *ast.VarDeclarationStmt:
			if s.Name.Value == "r" {
				callIdx = i
			}
		}
	}

	if fnIdx == -1 {
		t.Fatal("monomorphized function identity__int not found in program")
	}
	if callIdx == -1 {
		t.Fatal("variable declaration r not found in program")
	}
	if fnIdx >= callIdx {
		t.Errorf("monomorphized function at index %d should appear before call site at index %d", fnIdx, callIdx)
	}
}
