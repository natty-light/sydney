package typechecker

import (
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
	}

	for _, s := range source {

		l := lexer.New(s)
		p := parser.New(l)
		program := p.ParseProgram()
		c := New(nil)
		c.Check(program)

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
	c.Check(program)
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
			"type mismatch: cannot assign bool to element of array a of type array<int>",
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

func testTypeErrors(t *testing.T, tests []TypeErrorTest) {
	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := parser.New(l)
		program := p.ParseProgram()
		c := New(nil)
		errors := c.Check(program)
		if len(c.Errors()) == 0 {
			t.Fatalf("input %q expected error but got none", tt.input)
		}

		if errors[0] != tt.expectedError {
			t.Fatalf("input %q expected error %q but got %q", tt.input, tt.expectedError, errors[0])
		}
	}
}
