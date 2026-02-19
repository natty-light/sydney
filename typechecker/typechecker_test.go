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
	source := "const int x = 5; x;"

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
			"unresolved symbol: f",
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
			"mut int x = 5; if (x == 5) { mut string x = \"hello\"; } else { mut string x = 5; }",
			"type mismatch: cannot assign int to variable x of type string",
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
			"type mismatch: cannot assign array<bool> to variable a of type array<int>",
		},
		{
			"mut array<null> a = []; a = [0];",
			"type mismatch: cannot assign array<int> to variable a of type array<null>",
		},
		{
			"mut array<int> a = [1, false, \"hello\"];",
			"type mismatch: array element got bool, expected int",
		},
		{
			"const array<array<int>> a = [[null]]",
			"type mismatch: cannot assign array<array<null>> to variable a of type array<array<int>>",
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
