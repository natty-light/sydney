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
	c := New()
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
	}

	testTypeErrors(t, tests)
}

func TestArrayTypeErrorChecking(t *testing.T) {
	tests := []TypeErrorTest{
		{
			"mut array<int> a = [false];",
			"type mismatch: cannot assign array<bool> to variable a of type array<int>",
		},
		//{
		//	"mut array<null> a = []; a = [0];",
		//	"",
		//},
	}

	testTypeErrors(t, tests)
}

func testTypeErrors(t *testing.T, tests []TypeErrorTest) {
	for _, tt := range tests {

		l := lexer.New(tt.input)
		p := parser.New(l)
		program := p.ParseProgram()
		c := New()
		errors := c.Check(program)
		if len(c.Errors()) == 0 {
			t.Fatalf("input %q expected error but got none", tt.input)
		}

		if errors[0] != tt.expectedError {
			t.Fatalf("input %q expected error %q but got %q", tt.input, tt.expectedError, errors[0])
		}
	}
}
