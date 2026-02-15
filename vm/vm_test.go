package vm

import (
	"fmt"
	"sydney/ast"
	"sydney/compiler"
	"sydney/lexer"
	"sydney/object"
	"sydney/parser"
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
		{"!1", false},
		{"!!1", true},
		{"!0", true},
		{"!!0", false},
	}

	runVmTests(t, tests)
}

func TestConditionals(t *testing.T) {
	tests := []vmTestCase{
		{"if (true) { 10 }", 10},
		{"if (true) { 10 } else { 20 }", 10},
		{"if (false) { 10 } else { 20 }", 20},
		{"if (1) { 10 }", 10},
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
			`[]`, []int{},
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
		{"[][0]", Null},
		{"[1, 2, 3][99]", Null},
		{"[1][-1]", Null},
		{"{1: 1, 2: 2}[1]", 1},
		{"{1: 1, 2: 2}[2]", 2},
		{"{1: 1}[0]", Null},
		{"{}[0]", Null},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithoutArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `const fivePlusTen = func() { 5 + 10; }; fivePlusTen();`,
			expected: 15,
		},
		{
			source:   `const one = func() { 1; }; const two = func() { 2; }; one() + two();`,
			expected: 3,
		},
		{
			source: `
			const a = func() { 1; };
			const b = func() { a() + 1; };
			const c = func() { b() + 1; };
			c();`,
			expected: 3,
		},
	}

	runVmTests(t, tests)
}

func TestFunctionsWithReturnStatements(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `const earlyExit = func() { return 99; 100; }; earlyExit();`,
			expected: 99,
		},
		{
			source:   `const earlyExit = func() { return 99; return 100; }; earlyExit();`,
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
			source: `const returnsOne = func() { 1; };
			const returnsOneReturner = func() { returnsOne; };
			returnsOneReturner()();`,
			expected: 1,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			source:   `const one = func() { const one = 1; one; }; one()`,
			expected: 1,
		},
		{
			source: `const oneAndTwo = func() { const one = 1; const two = 2; one + two; };
			oneAndTwo();`,
			expected: 3,
		},
		{
			source: `
			const oneAndTwo = func() {const one = 1; const two = 2; one + two; };
			const threeAndFour = func() { const three = 3; const four = 4; three + four; };
			oneAndTwo() + threeAndFour();
			`,
			expected: 10,
		},
		{
			source: `
			const firstFunc = func() { const x = 50; x };
			const secondFunc = func() { const x = 100; x };
			firstFunc() + secondFunc();
			`,
			expected: 150,
		},
		{
			source: `
			mut globalNum = 50;
			const minusOne = func() { const num = 1; globalNum - num; };
			const minusTwo = func() { const num = 2; globalNum - num; };
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
			source:   `len([])`,
			expected: 0,
		},
		{
			source:   `len([1, 2, 3])`,
			expected: 3,
		},
		{
			source:   `len(1)`,
			expected: &object.Error{Message: "argument to `len` of wrong type. got=Integer"},
		},
		{
			source:   `len("one", "two")`,
			expected: &object.Error{Message: "`len` expects one argument"},
		},
		{
			source:   `first([1, 2, 3])`,
			expected: 1,
		},
		{
			source:   `first([])`,
			expected: Null,
		},
		{
			source:   `first(1)`,
			expected: &object.Error{Message: "argument to `first` must be array type"},
		},
		{
			source:   `last([1, 2, 3])`,
			expected: 3,
		},
		{
			source:   `last([])`,
			expected: Null,
		},
		{
			source:   `last(1)`,
			expected: &object.Error{Message: "argument to `last` must be array type"},
		},
		{
			source:   `rest([1, 2, 3])`,
			expected: []int{2, 3},
		},
		{
			source:   `rest([])`,
			expected: Null,
		},
		{
			source:   `append([], 1)`,
			expected: []int{1},
		},
		{
			source:   `append(1, 1)`,
			expected: &object.Error{Message: "first argument to `append` must be array type"},
		},
	}

	runVmTests(t, tests)
}

func TestClosures(t *testing.T) {
	tests := []vmTestCase{
		{
			source: `
			const newClosure = func(int a) {
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

func runVmTests(t *testing.T, tests []vmTestCase) {
	t.Helper()

	for _, tt := range tests {
		program := parse(tt.source)

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
