package compiler

import (
	"fmt"
	"sydney/ast"
	"sydney/code"
	"sydney/lexer"
	"sydney/object"
	"sydney/parser"
	"sydney/typechecker"
	"sydney/types"
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
			mut int x;
			x;
			`,
			expectedConstants: []interface{}{0},
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
			source: "func() -> int { return 5 + 10 }",
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
			source: "func() -> int { 5 + 10 }",
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
			source: "func() -> int { 1; 2 }",
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
			source: `func() -> int { 24 }()`,
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
			source: `const noArg = func() -> int { 24 }; noArg();`,
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
			source: `const oneArg = func(int a) -> int { a }; oneArg(24);`,
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
			source: `const manyArg = func(int a, int b, int c) -> int { a; b; c; };
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
		func(int a) {
			func(int b) {
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
			func(int a) {
				func(int b) {
					func(int c) {
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
			const countDown = func(int x) { countDown(x - 1); };
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
				const countDown = func(int x) { countDown(x - 1); };
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

func TestIndexAssignment(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `const array<int> a = [1]; a[0] = 2; a[0];`,
			expectedConstants: []interface{}{
				1,
				0,
				2,
				0,
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),           // push [1]
				code.Make(code.OpArray, 1),              // allocate array
				code.Make(code.OpSetImmutableGlobal, 0), // store in a
				code.Make(code.OpGetGlobal, 0),          // push a for assignment
				code.Make(code.OpConstant, 1),           // push index 0
				code.Make(code.OpConstant, 2),           // push value 2
				code.Make(code.OpIndexSet),              // perform assignment
				code.Make(code.OpGetGlobal, 0),          // push a for access
				code.Make(code.OpConstant, 3),           // push index 0
				code.Make(code.OpIndex),                 // perform index
				code.Make(code.OpPop),
			},
		},
		{
			source: `const map<int, int> m = {1: 1}; m[1] = 2; m[1];`,
			expectedConstants: []interface{}{
				1,
				1,
				1,
				2,
				1,
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),           // push key 1
				code.Make(code.OpConstant, 1),           // push value 1
				code.Make(code.OpHash, 2),               // allocate map
				code.Make(code.OpSetImmutableGlobal, 0), // store in a
				code.Make(code.OpGetGlobal, 0),          // push a for assignment
				code.Make(code.OpConstant, 2),           // push index 0
				code.Make(code.OpConstant, 3),           // push value 2
				code.Make(code.OpIndexSet),              // perform assignment
				code.Make(code.OpGetGlobal, 0),          // push a for access
				code.Make(code.OpConstant, 4),           // push index 0
				code.Make(code.OpIndex),                 // perform index
				code.Make(code.OpPop),
			},
		},
		{
			source: `const map<int, int> m = {}; m[1] = 2; m[1];`,
			expectedConstants: []interface{}{
				1,
				2,
				1,
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpHash, 0),               // allocate map
				code.Make(code.OpSetImmutableGlobal, 0), // store in a
				code.Make(code.OpGetGlobal, 0),          // push a for assignment
				code.Make(code.OpConstant, 0),           // push index 0
				code.Make(code.OpConstant, 1),           // push value 2
				code.Make(code.OpIndexSet),              // perform assignment
				code.Make(code.OpGetGlobal, 0),          // push a for access
				code.Make(code.OpConstant, 2),           // push index 0
				code.Make(code.OpIndex),                 // perform index
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestStructs(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `define struct Point { x int, y int }
					const Point p = Point { x: 0, y: 0 };
					p.x;
					p.y;
					p.x = 1;`,
			expectedConstants: []interface{}{
				0,
				0,
				&object.TypeObject{
					T: types.StructType{
						Fields: []string{"x", "y"},
						Types:  []types.Type{types.Int, types.Int},
						Name:   "Point",
					},
				},
				1,
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpStruct, 2, 2),

				code.Make(code.OpSetImmutableGlobal, 0),

				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpGetField, 0),
				code.Make(code.OpPop),

				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpGetField, 1),
				code.Make(code.OpPop),

				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpSetField, 0),
			},
		},
		{
			source: `define struct Point { x int, y int }
					const Point p = Point { x: 0, y: 0 };
					
					func printPoint(Point p) {
						print(p)
					};
					
					printPoint(p);`,
			expectedConstants: []interface{}{
				0,
				0,
				&object.TypeObject{
					T: types.StructType{
						Fields: []string{"x", "y"},
						Types:  []types.Type{types.Int, types.Int},
						Name:   "Point",
					},
				},
				[]code.Instructions{
					code.Make(code.OpGetBuiltIn, 1),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpCall, 1),
					code.Make(code.OpReturnValue),
					code.Make(code.OpReturn),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
				code.Make(code.OpStruct, 2, 2),

				code.Make(code.OpSetImmutableGlobal, 1),

				code.Make(code.OpClosure, 3, 0),
				code.Make(code.OpSetImmutableGlobal, 0),

				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpCall, 1),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestInterfaces(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `define struct Circle { radius float }
					define interface Area { area() -> float }
					define implementation Circle -> Area
					const Circle c = Circle { radius: 2.0 };

					func area(Circle c) -> float {
						const pi = 3.14;
						return c.radius * c.radius * pi;
					}

					c.area();`,
			expectedConstants: []interface{}{
				&object.Itab{
					InterfaceName:  "Area",
					ConcreteName:   "Circle",
					MethodsIndices: []int{1},
				},
				2.0,
				&object.TypeObject{
					T: types.StructType{
						Fields: []string{"radius"},
						Types:  []types.Type{types.Float},
						Name:   "Circle",
					},
				},
				3.14,
				[]code.Instructions{
					code.Make(code.OpConstant, 3),
					code.Make(code.OpSetImmutableLocal, 1),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpGetField, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpGetField, 0),
					code.Make(code.OpMul),
					code.Make(code.OpGetLocal, 1),
					code.Make(code.OpMul),
					code.Make(code.OpReturnValue),
					code.Make(code.OpReturn),
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 1),
				code.Make(code.OpStruct, 2, 1),
				code.Make(code.OpSetImmutableGlobal, 1),
				code.Make(code.OpClosure, 4),
				code.Make(code.OpSetImmutableGlobal, 0),
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpCall, 1),
				code.Make(code.OpPop),
			},
		},
	}

	runCompilerTests(t, tests)
}

func TestInterfacesAsArguments(t *testing.T) {
	tests := []compilerTestCase{
		{
			source: `define struct Rect { w float, h float }
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
			expectedConstants: []interface{}{
				&object.Itab{
					InterfaceName:  "Area",
					ConcreteName:   "Rect",
					MethodsIndices: []int{1},
				},
				[]code.Instructions{
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpGetField, 0),
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpGetField, 1),
					code.Make(code.OpMul),
					code.Make(code.OpReturnValue),
					code.Make(code.OpReturn),
				},
				[]code.Instructions{
					code.Make(code.OpGetLocal, 0),
					code.Make(code.OpCallInterface, 0, 0),
					code.Make(code.OpReturnValue),
					code.Make(code.OpReturn),
				},
				2.0,
				2.0,
				&object.TypeObject{
					T: types.StructType{
						Fields: []string{"w", "h"},
						Types:  []types.Type{types.Float, types.Float},
						Name:   "Rect",
					},
				},
			},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpClosure, 1),
				code.Make(code.OpSetImmutableGlobal, 0),
				code.Make(code.OpClosure, 2),
				code.Make(code.OpSetImmutableGlobal, 1),
				code.Make(code.OpConstant, 3),
				code.Make(code.OpConstant, 4),
				code.Make(code.OpStruct, 5, 2),
				code.Make(code.OpSetImmutableGlobal, 2),
				code.Make(code.OpGetGlobal, 1),
				code.Make(code.OpGetGlobal, 2),
				code.Make(code.OpBox, 0),
				code.Make(code.OpCall, 1),
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
	program := p.ParseProgram()
	c := typechecker.New(nil)
	c.Check(program)

	return program
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
		case object.TypeObject:
			err := testTypeObject(constant, actual[i])
			if err != nil {
				return fmt.Errorf("constant %d - testTypeObject failed: %s", i, err)
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

func testTypeObject(expected object.TypeObject, actual object.Object) error {
	result, ok := actual.(*object.TypeObject)
	if !ok {
		return fmt.Errorf("object is not TypeObject. got=%T (%+v)", actual, actual)
	}
	switch t := expected.T.(type) {
	case types.StructType:
		at, ok := result.T.(*types.StructType)
		if !ok {
			return fmt.Errorf("object is not TypeObject. got=%T (%+v)", actual, actual)
		}
		if at.Name != t.Name {
			return fmt.Errorf("object has wrong name. got=%s, want=%s", t.Name, at.Name)
		}

		for i, field := range t.Fields {
			if at.Fields[i] != field {
				return fmt.Errorf("object has wrong field. got=%s, want=%s", t.Fields[i], field)
			}
		}
		for i, tt := range t.Types {
			if at.Types[i].Signature() != tt.Signature() {
				return fmt.Errorf("object field %s has wrong type, got %s, want=%s", t.Fields[i], tt.Signature(), at.Types[i].Signature())
			}
		}
	}

	return nil
}
