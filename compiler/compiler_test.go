package compiler

import (
	"fmt"
	"sydney/ast"
	"sydney/code"
	"sydney/lexer"
	"sydney/object"
	"sydney/parser"
	"testing"
)

type compilerTestCase struct {
	source               string
	expectedConstants    []interface{}
	expectedInstructions []code.Instructions
}

func TestIntegerArithmetic(t *testing.T) {
	tests := []compilerTestCase{
		{
			source:            "1 + 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpAdd),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "1; 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpPop),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "1 - 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpSub),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "1 * 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpMul),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "2 / 1",
			expectedConstants: []interface{}{2, 1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpDiv),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "-1",
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpMinus),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestFloatArithmetic(t *testing.T) {
	tests := []compilerTestCase{
		{
			source:            "1.1 + 2.2",
			expectedConstants: []interface{}{1.1, 2.2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpAdd),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "-1.0",
			expectedConstants: []interface{}{1.0},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpMinus),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestBooleanExpressions(t *testing.T) {
	tests := []compilerTestCase{
		{
			source:            "true",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "false",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpFalse),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "1 > 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGt),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "1 < 2",
			expectedConstants: []interface{}{2, 1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGt),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "1 >= 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGte),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "1 <= 2",
			expectedConstants: []interface{}{2, 1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpGte),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "1 == 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpEqual),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "1 != 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpNotEqual),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "true == false",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpEqual),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "true != false",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpNotEqual),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "true && false",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpAnd),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "true || false",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpFalse),
				code.Make(code.OpOr),
				code.Make(code.OpPop),
			},
		},
		{
			source:            "!true",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpTrue),
				code.Make(code.OpBang),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestConditionals(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `
			if (true) { 10 }; 3333;
			`,
			expectedConstants: []interface{}{10, 3333},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpTrue),
				// 0001
				code.Make(code.OpJumpNotTruthy, 10),
				// 0004
				code.Make(code.OpConstant, 0),
				// 0007
				code.Make(code.OpJump, 11),
				// 0010
				code.Make(code.OpNull),
				// 0011
				code.Make(code.OpPop),
				// 0012
				code.Make(code.OpConstant, 1),
				// 0015
				code.Make(code.OpPop),
			},
		},
		{
			source: `
			if (true) { 10 } else { 20 }; 3333;
			`,
			expectedConstants: []interface{}{10, 20, 3333},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpTrue),
				// 0001
				code.Make(code.OpJumpNotTruthy, 10),
				// 0004
				code.Make(code.OpConstant, 0),
				// 0007
				code.Make(code.OpJump, 13),
				// 0010
				code.Make(code.OpConstant, 1),
				// 0013
				code.Make(code.OpPop),
				// 0014
				code.Make(code.OpConstant, 2),
				// 0017
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestGlobalVarDeclarationStatements(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `
			mut x = 1;
			const y = 2;
			`,
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpSetMutableGlobal, 0),
				// 0006
				code.Make(code.OpConstant, 1),
				// 0009
				code.Make(code.OpSetImmutableGlobal, 1),
			},
		},
		{
			source: `
			mut x = 1;
			x;
			`,
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpSetMutableGlobal, 0),
				// 0006
				code.Make(code.OpGetGlobal, 0),
				// 0009
				code.Make(code.OpPop),
			},
		},
		{
			source: `
			mut x = 1;
			mut y = x;
			y;
			`,
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpSetMutableGlobal, 0),
				// 0006
				code.Make(code.OpGetGlobal, 0),
				// 0009
				code.Make(code.OpSetMutableGlobal, 1),
				// 0012
				code.Make(code.OpGetGlobal, 1),
				// 0015
				code.Make(code.OpPop),
			},
		},
		{
			source: `
			mut x;
			x;
			`,
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpNull),
				// 0003
				code.Make(code.OpSetMutableGlobal, 0),
				// 0006
				code.Make(code.OpGetGlobal, 0),
				// 0009
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestVariableAssignment(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `
			mut x = 2;
			x = 3;
			x;
			`,
			expectedConstants: []interface{}{2, 3},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpSetMutableGlobal, 0),
				// 0006
				code.Make(code.OpConstant, 1),
				// 0009
				code.Make(code.OpSetMutableGlobal, 0),
				// 0012
				code.Make(code.OpGetGlobal, 0),
				// 0015
				code.Make(code.OpPop),
			},
		},
		{
			source: `
			mut x = 2;
			mut y = 3;
			x = y;
			y;
			`,
			expectedConstants: []interface{}{2, 3},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpSetMutableGlobal, 0),
				// 0006
				code.Make(code.OpConstant, 1),
				// 0009
				code.Make(code.OpSetMutableGlobal, 1),
				// 0012
				code.Make(code.OpGetGlobal, 1),
				// 0015
				code.Make(code.OpSetMutableGlobal, 0),
				// 0018
				code.Make(code.OpGetGlobal, 1),
				// 0021
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestStringExpressions(t *testing.T) {
	tests := []compilerTestCase{
		{
			source:            `"quonkscript"`,
			expectedConstants: []interface{}{"quonkscript"},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpPop),
			},
		},
		{
			source:            `"quonk" + "script"`,
			expectedConstants: []interface{}{"quonk", "script"},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpConstant, 1),
				// 0006
				code.Make(code.OpAdd),
				// 0007
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestArrayLiterals(t *testing.T) {
	tests := []compilerTestCase{
		{
			source:            "[]",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpArray, 0),
				// 0003
				code.Make(code.OpPop),
			},
		},
		{
			source:            "[1, 2, 3]",
			expectedConstants: []interface{}{1, 2, 3},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpConstant, 1),
				// 0006
				code.Make(code.OpConstant, 2),
				// 0009
				code.Make(code.OpArray, 3),
				// 0012
				code.Make(code.OpPop),
			},
		},
		{
			source:            "[1 + 2, 3 - 4, 5 * 6]",
			expectedConstants: []interface{}{1, 2, 3, 4, 5, 6},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpConstant, 1),
				// 0006
				code.Make(code.OpAdd),
				// 0007
				code.Make(code.OpConstant, 2),
				// 0010
				code.Make(code.OpConstant, 3),
				// 0013
				code.Make(code.OpSub),
				// 0014
				code.Make(code.OpConstant, 4),
				// 0017
				code.Make(code.OpConstant, 5),
				// 0020
				code.Make(code.OpMul),
				// 0021
				code.Make(code.OpArray, 3),
				// 0024
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestHashLiterals(t *testing.T) {
	tests := []compilerTestCase{
		{
			source:            "{}",
			expectedConstants: []interface{}{},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpHash, 0),
				// 0003
				code.Make(code.OpPop),
			},
		},
		{
			source:            "{1: 2, 3: 4, 5: 6}",
			expectedConstants: []interface{}{1, 2, 3, 4, 5, 6},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpConstant, 1),
				// 0006
				code.Make(code.OpConstant, 2),
				// 0009
				code.Make(code.OpConstant, 3),
				// 0012
				code.Make(code.OpConstant, 4),
				// 0015
				code.Make(code.OpConstant, 5),
				// 0018
				code.Make(code.OpHash, 6),
				// 0021
				code.Make(code.OpPop),
			},
		},
		{
			source:            "{1 + 1: 2 * 2, 3 - 3: 4 / 4, 5 * 5: 6 + 6}",
			expectedConstants: []interface{}{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpConstant, 1),
				// 0006
				code.Make(code.OpAdd),
				// 0007
				code.Make(code.OpConstant, 2),
				// 0010
				code.Make(code.OpConstant, 3),
				// 0013
				code.Make(code.OpMul),
				// 0014
				code.Make(code.OpConstant, 4),
				// 0017
				code.Make(code.OpConstant, 5),
				// 0020
				code.Make(code.OpSub),
				// 0021
				code.Make(code.OpConstant, 6),
				// 0024
				code.Make(code.OpConstant, 7),
				// 0027
				code.Make(code.OpDiv),
				// 0028
				code.Make(code.OpConstant, 8),
				// 0031
				code.Make(code.OpConstant, 9),
				// 0034
				code.Make(code.OpMul),
				// 0035
				code.Make(code.OpConstant, 10),
				// 0038
				code.Make(code.OpConstant, 11),
				// 0041
				code.Make(code.OpAdd),
				// 0042
				code.Make(code.OpHash, 6),
				// 0045
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestIndexExpressions(t *testing.T) {
	tests := []compilerTestCase{
		{
			source:            "[1, 2, 3][1 + 1]",
			expectedConstants: []interface{}{1, 2, 3, 1, 1},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpConstant, 1),
				// 0006
				code.Make(code.OpConstant, 2),
				// 0009
				code.Make(code.OpArray, 3),
				// 0012
				code.Make(code.OpConstant, 3),
				// 0015
				code.Make(code.OpConstant, 4),
				// 0018
				code.Make(code.OpAdd),
				// 0019
				code.Make(code.OpIndex),
				// 0020
				code.Make(code.OpPop),
			},
		},
		{
			source:            "{1: 2}[2 - 1]",
			expectedConstants: []interface{}{1, 2, 2, 1},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpConstant, 1),
				// 0006
				code.Make(code.OpHash, 2),
				// 0009
				code.Make(code.OpConstant, 2),
				// 0012
				code.Make(code.OpConstant, 3),
				// 0015
				code.Make(code.OpSub),
				// 0016
				code.Make(code.OpIndex),
				// 0017
				code.Make(code.OpPop),
			},
		},
		{
			source:            "[[1, 2, 3]][0][0] + 1",
			expectedConstants: []interface{}{1, 2, 3, 0, 0, 1},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpConstant, 1),
				// 0006
				code.Make(code.OpConstant, 2),
				// 0009
				code.Make(code.OpArray, 3),
				// 0012
				code.Make(code.OpArray, 1),
				// 0015
				code.Make(code.OpConstant, 3),
				// 0018
				code.Make(code.OpIndex),
				// 0019
				code.Make(code.OpConstant, 4),
				// 0022
				code.Make(code.OpIndex),
				// 0023
				code.Make(code.OpConstant, 5),
				// 0026
				code.Make(code.OpAdd),
				// 0027
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestFunctions(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: "func() { return 5 + 10 }",
			expectedConstants: []interface{}{5, 10,
				[]code.Instructions{
					// 0000
					code.Make(code.OpConstant, 0),
					// 0003
					code.Make(code.OpConstant, 1),
					// 0006
					code.Make(code.OpAdd),
					// 0007
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpClosure, 2, 0),
				// 0003
				code.Make(code.OpPop),
			},
		},
		{
			source: "func() { 5 + 10 }",
			expectedConstants: []interface{}{5, 10,
				[]code.Instructions{
					// 0000
					code.Make(code.OpConstant, 0),
					// 0003
					code.Make(code.OpConstant, 1),
					// 0006
					code.Make(code.OpAdd),
					// 0007
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpClosure, 2, 0),
				// 0003
				code.Make(code.OpPop),
			},
		},
		{
			source: "func() { 1; 2 }",
			expectedConstants: []interface{}{1, 2,
				[]code.Instructions{
					// 0000
					code.Make(code.OpConstant, 0),
					// 0003
					code.Make(code.OpPop),
					// 0004
					code.Make(code.OpConstant, 1),
					// 0007
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpClosure, 2, 0),
				// 0003
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestFunctionsWithoutReturnValue(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: "func() { }",
			expectedConstants: []interface{}{
				[]code.Instructions{
					// 0000
					code.Make(code.OpReturn),
				},
			},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpClosure, 0, 0),
				// 0003
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestFunctionCalls(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `func() { 24 }()`,
			expectedConstants: []interface{}{24,
				[]code.Instructions{
					// 0000
					code.Make(code.OpConstant, 0),
					// 0003
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			source: `const noArg = func() { 24 }; noArg();`,
			expectedConstants: []interface{}{24,
				[]code.Instructions{
					// 0000
					code.Make(code.OpConstant, 0),
					// 0003
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpSetImmutableGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
		{
			source: `const oneArg = func(a) { a }; oneArg(24);`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpReturnValue),
				},
				24,
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 0, 0),
				code.Make(code.OpSetImmutableGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpCall, 1),
				code.Make(code.OpPop),
			},
		},
		{
			source: `const manyArg = func(a, b, c) { a; b; c; };
			manyArg(1, 2, 3);`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpPop),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpPop),
					code.Make(code.OpGetLocal, 2),
					code.Make(code.OpReturnValue),
				},
				1,
				2,
				3,
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 0, 0),
				code.Make(code.OpSetImmutableGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpCall, 3),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestCompilerScopes(t *testing.T) {
	compiler := New()

	if compiler.scopeIndex != 0 {
		t.Errorf("scopeIndex wrong. got=%d, want=%d", compiler.scopeIndex, 0)
	}
	globalSymbolTable := compiler.symbolTable

	compiler.emit(code.OpMul)

	compiler.enterScope()
	if compiler.scopeIndex != 1 {
		t.Errorf("scopeIndex wrong. got=%d, want=%d", compiler.scopeIndex, 1)
	}

	compiler.emit(code.OpSub)

	if len(compiler.scopes[compiler.scopeIndex].instructions) != 1 {
		t.Errorf("instructions length wrong. got=%d",
			len(compiler.scopes[compiler.scopeIndex].instructions))
	}

	last := compiler.scopes[compiler.scopeIndex].lastInstruction
	if last.Opcode != code.OpSub {
		t.Errorf("lastInstruction.OpCode wrong. got=%d, want=%d", last.Opcode, code.OpSub)
	}

	if compiler.symbolTable.Outer != globalSymbolTable {
		t.Errorf("compiler did not enclose symbolTable")
	}

	compiler.leaveScope()
	if compiler.scopeIndex != 0 {
		t.Errorf("scopeIndex wrong. got=%d, want=%d", compiler.scopeIndex, 0)
	}

	if compiler.symbolTable != globalSymbolTable {
		t.Errorf("compiler did not restore global symbol table")
	}

	if compiler.symbolTable.Outer != nil {
		t.Errorf("compiler did not restore global symbol table")
	}

	compiler.emit(code.OpAdd)

	if len(compiler.scopes[compiler.scopeIndex].instructions) != 2 {
		t.Errorf("instructions length wrong. got=%d",
			len(compiler.scopes[compiler.scopeIndex].instructions))
	}

	last = compiler.scopes[compiler.scopeIndex].lastInstruction
	if last.Opcode != code.OpAdd {
		t.Errorf("lastInstruction.OpCode wrong. got=%d, want=%d", last.Opcode, code.OpAdd)
	}

	previous := compiler.scopes[compiler.scopeIndex].previousInstruction
	if previous.Opcode != code.OpMul {
		t.Errorf("previousInstruction.OpCode wrong. got=%d, want=%d", previous.Opcode, code.OpMul)
	}
}

func TestVarDeclarationStatementScopes(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `const num = 55;
			func() { num }`,
			expectedConstants: []interface{}{55,
				[]code.Instructions{
					// 0000
					code.Make(code.OpGetGlobal, 0),
					// 0003
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetImmutableGlobal, 0),
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpPop),
			},
		},
		{
			source: `mut num = 55;
			func() { num }`,
			expectedConstants: []interface{}{55,
				[]code.Instructions{
					code.Make(code.OpGetGlobal, 0),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpSetMutableGlobal, 0),
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpPop),
			},
		},
		{
			source: `func() {
				const a = 55;
				a;
			}`,
			expectedConstants: []interface{}{55,
				[]code.Instructions{
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetImmutableLocal, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpPop),
			},
		},
		{
			source: `func() {
				mut a = 55;
				a;
			}`,
			expectedConstants: []interface{}{55,
				[]code.Instructions{
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetMutableLocal, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpPop),
			},
		},
		{
			source: `func() {
				const a = 55;
				mut b = 77;
				a + b;
			}`,
			expectedConstants: []interface{}{
				55,
				77,
				[]code.Instructions{
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSetImmutableLocal, 0),
					code.Make(code.OpConstant, 1),
					code.Make(code.OpSetMutableLocal, 1),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestBuiltins(t *testing.T) {
	tests := []compilerTestCase{
		{
			source:            "len([]); append([], 1);",
			expectedConstants: []interface{}{1},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpGetBuiltIn, 0),
				code.Make(code.OpArray, 0),
				code.Make(code.OpCall, 1),
				code.Make(code.OpPop),
				code.Make(code.OpGetBuiltIn, 5),
				code.Make(code.OpArray, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpCall, 2),
				code.Make(code.OpPop),
			},
		},
		{
			source: "func() { len([]) }",
			expectedConstants: []interface{}{
				[]code.Instructions{
					code.Make(code.OpGetBuiltIn, 0),
					code.Make(code.OpArray, 0),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 0, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestForLoops(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `
			mut i = 0;
			for (i < 5) { i }
			`,
			expectedConstants: []interface{}{0, 5},
			expectedInstructions: []code.Instructions{
				// 0000
				code.Make(code.OpConstant, 0),
				// 0003
				code.Make(code.OpSetMutableGlobal, 0),
				// 0006
				code.Make(code.OpConstant, 1),
				// 0009
				code.Make(code.OpGetGlobal, 0),
				// 0012
				code.Make(code.OpGt),
				// 0013
				code.Make(code.OpJumpNotTruthy, 23),
				// 0016
				code.Make(code.OpGetGlobal, 0),
				// 0019
				code.Make(code.OpPop),
				// 0020
				code.Make(code.OpJump, 6),
				// 0023
				code.Make(code.OpNull),
				// 0024
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestClosures(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `
		func(a) {
			func(b) {
				a + b
			}
		}
		`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					code.Make(code.OpGetFree, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpClosure, 0, 1),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpPop),
			},
		},
		{
			source: `
			func(a) {
				func(b) {
					func(c) {
						a + b + c
					}
				}
			}
			`,
			expectedConstants: []interface{}{
				[]code.Instructions{
					code.Make(code.OpGetFree, 0), // a
					code.Make(code.OpGetFree, 1), // b
					code.Make(code.OpAdd),
					code.Make(code.OpGetLocal, 0), // c
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{
					code.Make(code.OpGetFree, 0),  // a
					code.Make(code.OpGetLocal, 0), // b
					code.Make(code.OpClosure, 0, 2),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpClosure, 1, 1),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 2, 0),
				code.Make(code.OpPop),
			},
		},
		{
			source: `
			const global = 55;

			func() {
				const a = 66;

				func() {
					const b = 77;

					func() {
						const c = 88;

						global + a + b + c;
					}
				}
			}
			`,
			expectedConstants: []interface{}{
				55,
				66,
				77,
				88,
				[]code.Instructions{
					code.Make(code.OpConstant, 3),          // 88
					code.Make(code.OpSetImmutableLocal, 0), // c
					code.Make(code.OpGetGlobal, 0),         // global
					code.Make(code.OpGetFree, 0),           // a,
					code.Make(code.OpAdd),
					code.Make(code.OpGetFree, 1), // b
					code.Make(code.OpAdd),
					code.Make(code.OpGetLocal, 0), // c
					code.Make(code.OpAdd),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{
					code.Make(code.OpConstant, 2),          // 77
					code.Make(code.OpSetImmutableLocal, 0), // b,
					code.Make(code.OpGetFree, 0),           // a
					code.Make(code.OpGetLocal, 0),          // b
					code.Make(code.OpClosure, 4, 2),
					code.Make(code.OpReturnValue),
				},
				[]code.Instructions{
					code.Make(code.OpConstant, 1),          // 66
					code.Make(code.OpSetImmutableLocal, 0), // a
					code.Make(code.OpGetLocal, 0),          // a
					code.Make(code.OpClosure, 5, 1),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),        // 55
				code.Make(code.OpSetImmutableGlobal), // global
				code.Make(code.OpClosure, 6, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestRecursiveFunctions(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `
			const countDown = func(x) { countDown(x - 1); };
			countDown(1);
			`,
			expectedConstants: []interface{}{
				1,
				[]code.Instructions{
					code.Make(code.OpCurrentClosure),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSub),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturnValue),
				},
				1,
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1, 0),
				code.Make(code.OpSetImmutableGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 2),
				code.Make(code.OpCall, 1),
				code.Make(code.OpPop),
			},
		},
		{
			source: `
			const wrapper = func() {
				const countDown = func(x) { countDown(x - 1); };
				countDown(1);
			};
			wrapper();`,
			expectedConstants: []interface{}{
				1,
				[]code.Instructions{
					code.Make(code.OpCurrentClosure),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 0),
					code.Make(code.OpSub),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturnValue),
				},
				1,
				[]code.Instructions{
					code.Make(code.OpClosure, 1, 0),
					code.Make(code.OpSetImmutableLocal, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpConstant, 2),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturnValue),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpSetImmutableGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpCall, 0),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func runCompilerTests(t *testing.T, tests []compilerTestCase) {
	t.Helper()

	for _, tt := range tests {
		program := parse(tt.source)

		compiler := New()
		err := compiler.Compile(program)
		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}

		bytecode := compiler.Bytecode()

		err = testInstructions(tt.expectedInstructions, bytecode.Instructions)
		if err != nil {
			t.Fatalf("testInstructions failed: %s", err)
		}

		err = testConstants(tt.expectedConstants, bytecode.Constants)
		if err != nil {
			t.Fatalf("testConstants failed: %s", err)
		}
	}
}

func parse(source string) *ast.Program {
	l := lexer.New(source)
	p := parser.New(l)
	return p.ParseProgram()
}

func testInstructions(expected []code.Instructions, actual code.Instructions) error {
	concatted := concatInstructions(expected)
	if len(actual) != len(concatted) {
		return fmt.Errorf("wrong instruction length.\nwant=%q\ngot=%q", concatted, actual)
	}

	for i, ins := range concatted {
		if actual[i] != ins {
			return fmt.Errorf("wrong instruction at %d.\nwant=%q\ngot=%q", i, concatted, actual)
		}
	}

	return nil
}

func concatInstructions(s []code.Instructions) code.Instructions {
	out := code.Instructions{}
	for _, ins := range s {
		out = append(out, ins...)
	}
	return out
}

func testConstants(expected []interface{}, actual []object.Object) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("wrong number of constants. got=%d, want=%d", len(actual), len(expected))
	}

	for i, constant := range expected {
		switch constant := constant.(type) {
		case int:
			err := testIntegerObject(int64(constant), actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testIntegerObject failed: %s", i, err)
			}
		case string:
			err := testStringObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testStringObject failed: %s", i, err)
			}
		case []code.Instructions:
			fn, ok := actual[i].(*object.CompiledFunction)
			if !ok {
				return fmt.Errorf("constant %d - not a function: %+v", i, actual[i])
			}

			err := testInstructions(constant, fn.Instructions)
			if err != nil {
				return fmt.Errorf("constant %d - testInstructions failed: %s", i, err)
			}
		}
	}

	return nil
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
