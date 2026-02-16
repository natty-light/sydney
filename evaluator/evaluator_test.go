package evaluator

import (
	"fmt"
	"sydney/lexer"
	"sydney/object"
	"sydney/parser"
	"testing"
)

func TestEvalIntegerExpr(t *testing.T) {
	tests := []struct {
		source   string
		expected int64
	}{
		{"5", 5},
		{"10", 10},
		{"-5", -5},
		{"-10", -10},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"-50 + 100 + -50", 0},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"20 + 2 * -10", 0},
		{"50 / 2 * 2 + 10", 60},
		{"2 * (5 + 10)", 30},
		{"3 * 3 * 3 + 10", 37},
		{"3 * (3 * 3) + 10", 37},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestEvalBooleanExpr(t *testing.T) {
	tests := []struct {
		source   string
		expected bool
	}{
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
		{"(1 < 2) == true", true},
		{"(1 < 2) == false", false},
		{"(1 > 2) == true", false},
		{"(1 > 2) == false", true},
		{"true && false", false},
		{"true || false", true},
		{"(1 > 2) || (3 + 2 ==5)", true},
		{"3 > 2 && 4 > 3", true},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestBangOperator(t *testing.T) {
	tests := []struct {
		source   string
		expected bool
	}{
		{"!true", false},
		{"!false", true},
		{"!5", false},
		{"!!true", true},
		{"!!false", false},
		{"!!5", true},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestIfElseExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"if (true) { 10 }", 10},
		{"if (false) { 10 }", nil},
		{"if (1) { 10 }", 10},
		{"if (1 < 2) { 10 }", 10},
		{"if (1 > 2) { 10 }", nil},
		{"if (1 > 2) { 10 } else { 20 }", 20},
		{"if (1 < 2) { 10 } else { 20 }", 10},
	}
	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func TestReturnStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"return 10;", 10},
		{"return 10; 9;", 10},
		{"return 2 * 5; 9;", 10},
		{"9; return 2 * 5; 9;", 10},
		{"if (10 > 1) { if (10 > 1) { return 10 } return 1 }", 10},
	}
	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		{
			"5 + true;",
			"type mismatch: Integer + Boolean",
		},
		{
			"5 + true; 5;",
			"type mismatch: Integer + Boolean",
		},
		{
			"-true",
			"unknown operation - for type Boolean",
		},
		{
			"true + false;",
			"unknown operator: Boolean + Boolean",
		},
		{
			"5; true + false; 5",
			"unknown operator: Boolean + Boolean",
		},
		{
			"if (10 > 1) { true + false; }",
			"unknown operator: Boolean + Boolean",
		},
		{
			` if (10 > 1) {
		  		if (10 > 1) {
					return true + false;
				}
				return 1;
			}`,
			"unknown operator: Boolean + Boolean",
		},
		{"foobar", "identifier not found: foobar"},
		{`"Hello" - "World"`, "unknown operator: String - String"},
		{
			`{"name": "QuonkScript"}[func(int x) { x }];`,
			"unusable as hash key: Function",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)

		errObj, ok := evaluated.(*object.Error)
		if !ok {
			fmt.Println(tt)
			t.Errorf("no error object returned. got=%T (%+v)", evaluated, evaluated)
			continue
		}

		if errObj.Message != tt.expectedMessage {
			t.Errorf("wrong error message. expected=%q, got=%q", tt.expectedMessage, errObj.Message)
		}
	}
}

func TestVarDeclarationStmts(t *testing.T) {
	tests := []struct {
		source   string
		expected interface{}
	}{
		{"mut a = 5; a;", 5},
		{"const a = 5 * 5; a;", 25},
		{"mut a = 5; mut b = a; b;", 5},
		{"const a = 5; mut b = 5; const c = a + b + 5; c;", 15},
		{"mut x = null; x;", NULL},
		{"mut int x; x;", 0},
		{"mut bool x; x;", false},
		{"mut array<int> x; x", nil},
		{"mut map<int, string> x; x;", nil},
		{"mut string x; x;", ""},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)
		if integer, ok := tt.expected.(int); ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else if tt.expected == nil {
			if evaluated == nil {
				return
			}
		} else if boolean, ok := tt.expected.(bool); ok {
			testBooleanObject(t, evaluated, boolean)
		} else if str, ok := tt.expected.(string); ok {
			testStringObject(t, evaluated, str)
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func TestFunctionObject(t *testing.T) {
	input := "func(int x) { x + 2; };"
	evaluated := testEval(t, input)
	fn, ok := evaluated.(*object.Function)
	if !ok {
		t.Fatalf("object is not Function. got=%T (%+v)", evaluated, evaluated)
	}

	if len(fn.Parameters) != 1 {
		t.Fatalf("function has wrong parameters. Parameters=%+v", fn.Parameters)
	}

	if fn.Parameters[0].String() != "x" {
		t.Fatalf("parameter is not 'x'. got=%q", fn.Parameters[0])
	}
	expectedBody := "(x + 2)"
	if fn.Body.String() != expectedBody {
		t.Fatalf("body is not %q. got=%q", expectedBody, fn.Body.String())
	}
}

func TestFunctionApplication(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"const identity = func(int x) { x; }; identity(5);", 5},
		{"const identity = func(int x) { return x; }; identity(5);", 5},
		{"const double = func(int x) { x * 2; }; double(5);", 10},
		{"const add = func(int x, int y) { x + y; }; add(5, 5);", 10},
		{"const add = func(int x, int y) { x + y; }; add(5 + 5, add(5, 5));", 20},
		{"func(int x) { x; }(5)", 5},
	}
	for _, tt := range tests {
		testIntegerObject(t, testEval(t, tt.input), tt.expected)
	}
}

func TestClosures(t *testing.T) {
	input := `
   const newAdder = func(int x) {
     func(int y) { x + y };
};
   const addTwo = newAdder(2);
   addTwo(2);`
	testIntegerObject(t, testEval(t, input), 4)
}

func TestVariableAssignment(t *testing.T) {
	tests := []struct {
		source   string
		expected interface{}
	}{
		{"mut y = 5; y = 6; y;", 6},
		{
			`
			mut x = 5;
			const f = func () { x = 7; };
			f();
			x;
			`,
			7,
		},
		{
			"mut x = 5; x = null; x;", nil,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)
		if integer, ok := tt.expected.(int); ok {
			if !testIntegerObject(t, evaluated, int64(integer)) {
				return
			}
		} else {
			if !testNullObject(t, evaluated) {
				return
			}
		}
	}
}

func TestStringLiteral(t *testing.T) {
	source := `"Hello, World!"`
	evaluated := testEval(t, source)

	str, ok := evaluated.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", evaluated, evaluated)
	}

	if str.Value != "Hello, World!" {
		t.Errorf("String has wrong value. got=%q", str.Value)
	}
}

func TestStringConcatenation(t *testing.T) {
	source := `"Hello, " + "World!"`
	evaluated := testEval(t, source)
	str, ok := evaluated.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", evaluated, evaluated)
	}

	if str.Value != "Hello, World!" {
		t.Errorf("String has wrong value. got=%q", str.Value)
	}
}

func TestBuiltInFunction(t *testing.T) {
	tests := []struct {
		source   string
		expected interface{}
	}{
		{`len("")`, 0},
		{`len("four")`, 4},
		{`len("hello world")`, 11},
		{`len(1)`, "argument to `len` of wrong type. got=Integer"},
		{`len("one", "two")`, "`len` expects one argument"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)

		switch expected := tt.expected.(type) {
		case int:
			testIntegerObject(t, evaluated, int64(expected))
		case string:
			errObj, ok := evaluated.(*object.Error)
			if !ok {
				t.Errorf("object is not Error. got=%T (%+v)", evaluated, evaluated)
				continue
			}
			if errObj.Message != expected {
				t.Errorf("wrong error message. expected=%q, got=%q", expected, errObj.Message)
			}
		}
	}
}

func TestArrayLiterals(t *testing.T) {
	input := "[1, 2 * 2, 3 + 3]"
	evaluated := testEval(t, input)
	result, ok := evaluated.(*object.Array)
	if !ok {
		t.Fatalf("object is not Array. got=%T (%+v)", evaluated, evaluated)
	}
	if len(result.Elements) != 3 {
		t.Fatalf("array has wrong num of elements. got=%d",
			len(result.Elements))
	}
	testIntegerObject(t, result.Elements[0], 1)
	testIntegerObject(t, result.Elements[1], 4)
	testIntegerObject(t, result.Elements[2], 6)
}

func TestArrayIndexExpressions(t *testing.T) {
	tests := []struct {
		source               string
		expected             interface{}
		expectedErrorMessage interface{}
	}{
		{
			"[1, 2, 3][0]",
			1,
			nil,
		},
		{
			"[1, 2, 3][1]",
			2,
			nil,
		},
		{
			"[1, 2, 3][2]",
			3,
			nil,
		},
		{
			"const myArray = [1, 2, 3]; myArray[2];",
			3,
			nil,
		},
		{
			"const myArray = [1, 2, 3]; myArray[0] + myArray[1] + myArray[2];",
			6,
			nil,
		},
		{
			"const myArray = [1, 2, 3]; const i = myArray[0]; myArray[i]",
			2,
			nil,
		},
		{
			"[1, 2, 3][3]",
			nil,
			"array index out of bounds",
		},
		{
			"[1, 2, 3][-1]",
			3,
			nil,
		},
		{
			"[1, 2, 3][-2]",
			2,
			nil,
		},
		{
			"[1, 2, 3][-3]",
			1,
			nil,
		},
		{
			"[1, 2, 3][-4]",
			nil,
			"array index out of bounds",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			err := evaluated.(*object.Error)
			if err.Message != tt.expectedErrorMessage {
				t.Fatalf("error has wrong message. expected=%q. got=%q", tt.expectedErrorMessage, err.Message)
			}
		}
	}
}

func TestHashLiterals(t *testing.T) {
	source := `mut two = "two";
	{
		"one": 10 - 9,
		two: 1 + 1,
		"thr"+"ee": 6 / 2,
		4: 4,
		true: 5,
		false: 6,
	}`

	evaluated := testEval(t, source)
	result, ok := evaluated.(*object.Hash)
	if !ok {
		t.Fatalf("Eval didn't return hash. got=%T (%+v)", evaluated, evaluated)
	}

	expected := map[object.HashKey]int64{
		(&object.String{Value: "one"}).HashKey():   1,
		(&object.String{Value: "two"}).HashKey():   2,
		(&object.String{Value: "three"}).HashKey(): 3,
		(&object.Integer{Value: 4}).HashKey():      4,
		TRUE.HashKey():                             5,
		FALSE.HashKey():                            6,
	}

	if len(result.Pairs) != len(expected) {
		t.Fatalf("Hash has wrong num of pairs. want=%d, got=%d", len(expected), len(result.Pairs))
	}

	for expectedKey, expectedValue := range expected {
		pair, ok := result.Pairs[expectedKey]
		if !ok {
			t.Errorf("no pair for given key in Pairs")
		}
		testIntegerObject(t, pair.Value, expectedValue)
	}
}

func TestHashIndexExpressions(t *testing.T) {
	tests := []struct {
		source   string
		expected interface{}
	}{
		{
			`{"foo": 5}["foo"]`,
			5,
		},
		{
			`{"foo": 5}["bar"]`,
			nil,
		},
		{
			`mut key = "foo"; {"foo": 5}[key]`,
			5,
		},
		{
			`{}["foo"]`,
			nil,
		},
		{
			`{true: 5}[true]`,
			5,
		},
		{
			`{5: 5}[5]`,
			5,
		},
		{
			`{false: 5}[false]`,
			5,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func TestFloatExpressions(t *testing.T) {
	tests := []struct {
		source   string
		expected float64
	}{
		{
			`5.2 + 2.5`,
			7.7,
		},
		{
			`2.2 * 3.3`,
			7.26,
		},
		{
			`2.2 - 1.1`,
			1.1,
		},
		{
			`2.0 + 4.2`,
			6.2,
		},
		{
			`4.0 * 1.1`,
			4.4,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)
		testFloatObject(t, evaluated, tt.expected)
	}
}

// Utils

func testNullObject(t *testing.T, obj object.Object) bool {
	if obj != NULL {
		t.Errorf("object is not NULL. got=%T (%+v)", obj, obj)
		return false
	}
	return true
}

func testEval(t *testing.T, source string) object.Object {
	l := lexer.New(source)
	p := parser.New(l)

	program := p.ParseProgram()
	scope := object.NewScope()
	if len(p.Errors()) != 0 {
		for _, msg := range p.Errors() {
			fmt.Println(msg)
			t.Fatalf("parser errors not empty")
		}
	}
	return Eval(program, scope)
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) bool {
	result, ok := obj.(*object.Integer)
	if !ok {
		t.Errorf("object is not *object.Integer. got=%T (%+v)", obj, obj)
		return false
	}

	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d", result.Value, expected)
		return false
	}

	return true
}

func testBooleanObject(t *testing.T, obj object.Object, expected bool) bool {
	result, ok := obj.(*object.Boolean)
	if !ok {
		t.Errorf("object is not *object.Boolean. got=%T (%+v)", obj, obj)
		return false
	}

	if result.Value != expected {
		t.Errorf("object has wrong value. got=%t, want=%t", result.Value, expected)
		return false
	}

	return true
}

func testStringObject(t *testing.T, obj object.Object, expected string) bool {
	result, ok := obj.(*object.String)
	if !ok {
		t.Errorf("object is not *object.String. got=%T (%+v)", obj, obj)
		return false
	}

	if result.Value != expected {
		t.Errorf("object has wrong value. got=%q, want=%q", result.Value, expected)
		return false
	}

	return true
}

func testFloatObject(t *testing.T, obj object.Object, expected float64) bool {
	result, ok := obj.(*object.Float)
	if !ok {
		t.Errorf("object is not *object.Float. got=%T (%+v)", obj, obj)
		return false
	}

	if result.Value != expected {
		t.Errorf("object has wrong value. got=%f, want=%f", result.Value, expected)
		return false
	}

	return true
}

func TestQuote(t *testing.T) {
	tests := []struct {
		source   string
		expected string
	}{
		{
			`quote(5)`,
			`5`,
		},
		{
			`quote(5 + 8)`,
			`(5 + 8)`,
		},
		{
			`quote(foobar)`,
			`foobar`,
		},
		{
			`quote(foo + bar)`,
			`(foo + bar)`,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)
		quote, ok := evaluated.(*object.Quote)
		if !ok {
			t.Fatalf("expected *object.Quote. got=%T (%+v)", evaluated, evaluated)
		}

		if quote.Node == nil {
			t.Fatalf("quote.Node is nil")
		}

		if quote.Node.String() != tt.expected {
			t.Errorf("not equal. got=%q, want=%q", quote.Node.String(), tt.expected)
		}
	}
}

func TestQuoteUnquote(t *testing.T) {
	tests := []struct {
		source   string
		expected string
	}{
		{
			`quote(unquote(4))`,
			`4`,
		},
		{
			`quote(unquote(4 + 4))`,
			`8`,
		},
		{
			`quote(unquote(4 + 4) + 8)`,
			`(8 + 8)`,
		},
		{
			`quote(unquote(2.5 + 1.5))`,
			`4.000000`,
		},
		{
			`mut foo = 8;
					quote(foo)`,
			`foo`,
		},
		{
			`mut foo = 8;
  					quote(unquote(foo))`,
			`8`,
		},
		{
			`quote(unquote(true))`,
			`true`,
		},
		{
			`quote(unquote(true == false))`,
			`false`,
		},
		{
			`quote(unquote(true || false))`,
			`true`,
		},
		{
			`quote(unquote(true && false))`,
			`false`,
		},
		{
			`quote(unquote(quote(4 + 4)))`,
			`(4 + 4)`,
		},
		{
			`mut quotedInfixExpr = quote(4 + 4)
			quote(unquote(4 + 4) + unquote(quotedInfixExpr))`,
			`(8 + (4 + 4))`,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.source)
		quote, ok := evaluated.(*object.Quote)
		if !ok {
			t.Fatalf("expected *object.Quote. got=%T (%+v)", evaluated, evaluated)
		}

		if quote.Node == nil {
			t.Fatalf("quote.Node is nil")
		}

		if quote.Node.String() != tt.expected {
			t.Errorf("not equal. got=%q, want=%q", quote.Node.String(), tt.expected)
		}
	}
}

func TestDefineMacros(t *testing.T) {
	source := `mut number = 1;
	           mut f = func(int x, int y) { x + y; };
               mut m = macro(x, y) { x + y; };
			   mut int anotherM;
			   anotherM = macro(x, y) { x - y; };
		       `

	scope := object.NewScope()
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()

	DefineMacros(program, scope)

	if len(program.Stmts) != 3 {
		t.Fatalf("program.Statements does not contain 3 statements. got=%d", len(program.Stmts))
	}

	_, _, ok := scope.Get("number")
	if ok {
		t.Fatalf("number should not be defined")
	}

	_, _, ok = scope.Get("f")
	if ok {
		t.Fatalf("f should not be defined")
	}

	obj, _, ok := scope.Get("m")
	if !ok {
		t.Fatalf("macro not in scope")
	}

	macro, ok := obj.Value.(*object.Macro)
	if !ok {
		t.Fatalf("object is not Macro. got=%T (%+v)", obj.Value, obj.Value)
	}

	if len(macro.Parameters) != 2 {
		t.Fatalf("macro parameters wrong. want=2, got=%d", len(macro.Parameters))
	}

	if macro.Parameters[0].String() != "x" {
		t.Fatalf("parameter is not 'x'. got=%q", macro.Parameters[0])
	}

	if macro.Parameters[1].String() != "y" {
		t.Fatalf("parameter is not 'y'. got=%q", macro.Parameters[1])
	}

	expectedBody := "(x + y)"
	if macro.Body.String() != expectedBody {
		t.Fatalf("body is not %q. got=%q", expectedBody, macro.Body.String())
	}

	anotherObj, _, ok := scope.Get("anotherM")
	if !ok {
		t.Fatalf("macro not in scope")
	}

	anotherMacro, ok := anotherObj.Value.(*object.Macro)
	if !ok {
		t.Fatalf("object is not Macro. got=%T (%+v)", anotherObj.Value, anotherObj.Value)
	}

	if len(anotherMacro.Parameters) != 2 {
		t.Fatalf("macro parameters wrong. want=2, got=%d", len(anotherMacro.Parameters))
	}

	if anotherMacro.Parameters[0].String() != "x" {
		t.Fatalf("parameter is not 'x'. got=%q", anotherMacro.Parameters[0])
	}

	if anotherMacro.Parameters[1].String() != "y" {
		t.Fatalf("parameter is not 'y'. got=%q", anotherMacro.Parameters[1])
	}

	anotherExpectedBody := "(x - y)"
	if anotherMacro.Body.String() != anotherExpectedBody {
		t.Fatalf("body is not %q. got=%q", anotherExpectedBody, anotherMacro.Body.String())
	}
}
