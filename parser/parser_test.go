package parser

import (
	"fmt"
	"strconv"
	"strings"
	"sydney/ast"
	"sydney/lexer"
	"sydney/types"
	"testing"
)

func TestVarDeclarationStmts(t *testing.T) {

	tests := []struct {
		source        string
		expectedIdent string
		expectedValue interface{}
		expectedType  types.Type
		isConst       bool
	}{
		{"mut int x = 5;", "x", 5, types.Int, false},
		{"const bool y = true;", "y", true, types.Bool, true},
		{"mut foo = y;", "foo", "y", nil, false},
		{"mut x = null", "x", "null", nil, false},
		{"mut int x;", "x", nil, types.Int, false},
		{"mut map<string, int> m;", "m", nil, types.MapType{KeyType: types.String, ValueType: types.Int}, false},
		{"mut array<int> a;", "a", nil, types.ArrayType{ElemType: types.Int}, false},
		{"mut fn<(int) -> int> f;", "f", nil, types.FunctionType{Params: []types.Type{types.Int}, Return: types.Int}, false},
	}

	for _, tt := range tests {
		l := lexer.New(tt.source)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Stmts) != 1 {
			t.Fatalf("program.Stmts doe snot contain 1 statement. got=%d", len(program.Stmts))
		}

		stmt := program.Stmts[0]
		if !testVarDeclarationStmt(t, stmt, tt.expectedIdent, tt.isConst) {
			return
		}

		val := stmt.(*ast.VarDeclarationStmt).Value

		if !strings.Contains(tt.source, "=") {
			if val != tt.expectedValue {
				t.Fatalf("expecting value %s, got %s", tt.expectedValue, val)
			} else {
				return
			}
		}

		str := val.String()
		if str == "null" {
			if !testNullLiteral(t, val) {
				return
			}
		} else {
			if !testLiteralExpr(t, val, tt.expectedValue) {
				return
			}
		}
	}
}

func TestReturnStatements(t *testing.T) {
	source := `
	return 5;
	return 10;
	return 15;
	return null;
	`

	l := lexer.New(source)

	parser := New(l)

	program := parser.ParseProgram()
	checkParserErrors(t, parser)

	if len(program.Stmts) != 4 {
		t.Fatalf("program.Stmts does not contain 4 statements. got=%d", len(program.Stmts))
	}

	for _, stmt := range program.Stmts {
		returnStmt, ok := stmt.(*ast.ReturnStmt)

		if !ok {
			t.Errorf("stmt not *ast.ReturnStmt. got=%T", stmt)
			continue
		}

		if returnStmt.TokenLiteral() != "return" {
			t.Errorf("returnStmt.TokenLiteral not 'return'. got=%q", returnStmt.TokenLiteral())
		}
	}
}

func TestIdentifierExpr(t *testing.T) {
	source := "myVar"

	l := lexer.New(source)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program has not enough statements. got=%d", len(program.Stmts))
	}
	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)

	if !ok {
		t.Fatalf("program.Stmts[0] not ast.ExpressionStmt. got=%T", stmt)
	}

	if !testIdentifier(t, stmt.Expr, "myVar") {
		return
	}

}

func TestIntegerLiteralExpr(t *testing.T) {
	source := "5"

	l := lexer.New(source)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program has not enough statements. got=%d", len(program.Stmts))
	}
	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)

	if !ok {
		t.Fatalf("program.Stmts[0] not ast.ExpressionStmt. got=%T", stmt)
	}

	if !testIntegerLiteral(t, stmt.Expr, 5) {
		return
	}
}

func TestParsingPrefixExpr(t *testing.T) {
	prefixTests := []struct {
		input    string
		operator string
		value    interface{}
	}{
		{"!5", "!", 5},
		{"-15", "-", 15},
		{"!true", "!", true},
		{"!false", "!", false},
	}

	for _, tt := range prefixTests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Stmts) != 1 {
			t.Fatalf("program.Stmts does not contain %d statements. got=%d\n", 1, len(program.Stmts))
		}

		stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
		if !ok {
			t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
		}

		expr, ok := stmt.Expr.(*ast.PrefixExpr)
		if !ok {
			t.Fatalf("stmt is not *ast.PrefixExpr. got=%T", stmt.Expr)
		}

		if expr.Operator != tt.operator {
			t.Fatalf("expr.Operator is not '%s'. got=%s", tt.operator, expr.Operator)
		}
		if !testLiteralExpr(t, expr.Right, tt.value) {
			return
		}
	}
}

func TestParsingInfixExpressions(t *testing.T) {
	infixTests := []struct {
		input      string
		leftValue  interface{}
		operator   string
		rightValue interface{}
	}{
		{"5 + 5;", 5, "+", 5},
		{"5 - 5;", 5, "-", 5},
		{"5 * 5;", 5, "*", 5},
		{"5 / 5;", 5, "/", 5},
		{"5 > 5;", 5, ">", 5},
		{"5 < 5;", 5, "<", 5},
		{"5 == 5;", 5, "==", 5},
		{"5 != 5;", 5, "!=", 5},
		{"true == true", true, "==", true},
		{"true && false", true, "&&", false},
		{"true != false", true, "!=", false},
		{"false == false", false, "==", false},
		{"true || false", true, "||", false},
	}
	for _, tt := range infixTests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Stmts) != 1 {
			t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
				1, len(program.Stmts))
		}

		stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
		if !ok {
			t.Fatalf("program.Statements[0] is not *ast.ExpressionStatement. got=%T",
				program.Stmts[0])
		}

		if !testInfixExpr(t, stmt.Expr, tt.leftValue, tt.operator, tt.rightValue) {
			return
		}
	}
}

func TestOperatorPrecedenceParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"true",
			"true",
		},
		{
			"false",
			"false",
		},
		{
			"3 > 5 == false",
			"((3 > 5) == false)",
		},
		{
			"3 < 5 == true",
			"((3 < 5) == true)",
		},
		{
			"true && false",
			"(true && false)",
		},
		{
			"true || false",
			"(true || false)",
		},
		{
			"-a * b",
			"((-a) * b)",
		},
		{
			"!-a",
			"(!(-a))",
		},
		{
			"a + b + c",
			"((a + b) + c)",
		},
		{
			"a + b - c",
			"((a + b) - c)",
		},
		{
			"a * b * c",
			"((a * b) * c)",
		},
		{
			"a * b / c",
			"((a * b) / c)",
		},
		{
			"a + b / c",
			"(a + (b / c))",
		},
		{
			"a + b * c + d / e - f",
			"(((a + (b * c)) + (d / e)) - f)",
		},
		{
			"3 + 4; -5 * 5",
			"(3 + 4)((-5) * 5)",
		},
		{
			"5 > 4 == 3 < 4",
			"((5 > 4) == (3 < 4))",
		}, {
			"5 < 4 != 3 > 4",
			"((5 < 4) != (3 > 4))",
		},
		{
			"3 + 4 * 5 == 3 * 1 + 4 * 5",
			"((3 + (4 * 5)) == ((3 * 1) + (4 * 5)))",
		},
		{
			"3 >= 4 * 5 == 5 <= 3 - 2",
			"((3 >= (4 * 5)) == (5 <= (3 - 2)))",
		},
		{
			"3 >= 4 * 5 != 5 < 3 / 2 - 4",
			"((3 >= (4 * 5)) != (5 < ((3 / 2) - 4)))",
		},
		{
			"4 > 5 || 2 < 3",
			"((4 > 5) || (2 < 3))",
		},
		{
			"4 > 5 && 2 < 3",
			"((4 > 5) && (2 < 3))",
		},
		{
			"4 > 5 || 2 < 3 && 2 + 4 * 3 / 7",
			"(((4 > 5) || (2 < 3)) && (2 + ((4 * 3) / 7)))",
		},
		{
			"1 + (2 + 3) + 4",
			"((1 + (2 + 3)) + 4)",
		},
		{
			"(5 + 5) * 2",
			"((5 + 5) * 2)",
		},
		{
			"2 / (5 + 5)",
			"(2 / (5 + 5))",
		},
		{
			"-(5 + 5)",
			"(-(5 + 5))",
		},
		{
			"!(true == true)",
			"(!(true == true))",
		},
		{
			"a + f(b * c) + d",
			"((a + f((b * c))) + d)",
		},
		{
			"add(a, b, 1, 2 * 3, 4 + 5, add(6, 7 * 8))",
			"add(a, b, 1, (2 * 3), (4 + 5), add(6, (7 * 8)))",
		},
		{
			"add(a + b + c * d / f + g)",
			"add((((a + b) + ((c * d) / f)) + g))",
		},
		{
			"a * [1, 2, 3, 4][b * c] * d",
			"((a * ([1, 2, 3, 4][(b * c)])) * d)",
		},
		{
			"add(a * b[2], b[1], 2 * [1, 2][1])",
			"add((a * (b[2])), (b[1]), (2 * ([1, 2][1])))",
		},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)
		actual := program.String()
		if actual != tt.expected {
			t.Errorf("expected=%q, got=%q", tt.expected, actual)
		}
	}
}

func TestBooleanExpr(t *testing.T) {
	tests := []struct {
		input           string
		expectedBoolean bool
	}{
		{"true;", true},
		{"false;", false},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Stmts) != 1 {
			t.Fatalf("program.Statements does not contain %d statements. got=%d", 1, len(program.Stmts))
		}

		stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
		if !ok {
			t.Fatalf("program.Statements[0] is not *ast.ExpressionStatement. got=%T",
				program.Stmts[0])
		}

		if !testLiteralExpr(t, stmt.Expr, tt.expectedBoolean) {
			return
		}
	}
}

func TestIfExpr(t *testing.T) {
	source := `if (x < y) { x }`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts does not contain %d statements. got=%d", 1, len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
	}

	expr, ok := stmt.Expr.(*ast.IfExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.IfExpr. got=%T", stmt.Expr)
	}

	if !testInfixExpr(t, expr.Condition, "x", "<", "y") {
		return
	}

	if len(expr.Consequence.Stmts) != 1 {
		t.Errorf("consequence is not %d statements. got=%d\n", 1, len(expr.Consequence.Stmts))
	}

	consequence, ok := expr.Consequence.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("expr.Consequence.Stmts[0] is not *ast.ExpressionStmt. got=%T", expr.Consequence.Stmts[0])
	}

	if !testIdentifier(t, consequence.Expr, "x") {
		return
	}

	if expr.Alternative != nil {
		t.Errorf("expr.Alternative.Statments was not nil. got=%+v", expr.Alternative)
	}
}

func TestIfElseExpr(t *testing.T) {
	source := `if (x < y) { x } else { y }`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts does not contain %d statements. got=%d", 1, len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
	}

	expr, ok := stmt.Expr.(*ast.IfExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.IfExpr. got=%T", stmt.Expr)
	}

	if !testInfixExpr(t, expr.Condition, "x", "<", "y") {
		return
	}

	if len(expr.Consequence.Stmts) != 1 {
		t.Errorf("consequence is not %d statements. got=%d\n", 1, len(expr.Consequence.Stmts))
	}

	consequence, ok := expr.Consequence.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("expr.Consequence.Stmts[0] is not *ast.ExpressionStmt. got=%T", expr.Consequence.Stmts[0])
	}

	if !testIdentifier(t, consequence.Expr, "x") {
		return
	}

	if len(expr.Alternative.Stmts) == 0 {
		t.Errorf("expr.Alternative.Statments is not %d statements. got=%d", 1, len(expr.Alternative.Stmts))
	}

	alternative, ok := expr.Alternative.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("expr.Alternative.Stmts[0] is not *ast.ExpressionStmt. got=%T", expr.Alternative.Stmts[0])
	}

	if !testIdentifier(t, alternative.Expr, "y") {
		return
	}
}

func TestFunctionLiteralParsing(t *testing.T) {
	source := "func(int x, int y) -> int { x + y; }"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts does not contain %d statements. got=%d\n", 1, len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Statements[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
	}

	function, ok := stmt.Expr.(*ast.FunctionLiteral)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.FunctionLiteral. got=%T", stmt.Expr)
	}

	if len(function.Parameters) != 2 {
		t.Fatalf("function literal parameters wrong. want 2, got=%d\n", len(function.Parameters))
	}

	testLiteralExpr(t, function.Parameters[0], "x")
	testLiteralExpr(t, function.Parameters[1], "y")

	if len(function.Body.Stmts) != 1 {
		t.Fatalf("function.Body.Stmts does not contain %d statements. got=%d\n", 1, len(function.Body.Stmts))
	}

	bodyStmt, ok := function.Body.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("function body stmt is not *ast.ExpressionStmt. got=%T", function.Body.Stmts[0])
	}

	testInfixExpr(t, bodyStmt.Expr, "x", "+", "y")
}

func TestFunctionParameterParsing(t *testing.T) {
	tests := []struct {
		input          string
		expectedParams []string
		expectedTypes  []string
		expectedReturn interface{}
	}{
		{"func() {}", []string{}, []string{}, nil},
		{input: "func(int x) -> int {}", expectedParams: []string{"x"}, expectedTypes: []string{"int"}, expectedReturn: "int"},
		{input: "func(int x, int y, bool z) -> bool {}", expectedParams: []string{"x", "y", "z"}, expectedTypes: []string{"int", "int", "bool"}, expectedReturn: "bool"},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		stmt := program.Stmts[0].(*ast.ExpressionStmt)
		function := stmt.Expr.(*ast.FunctionLiteral)

		if len(function.Parameters) != len(tt.expectedParams) {
			t.Errorf("parameter length wrong. want %d. got=%d", len(tt.expectedParams), len(function.Parameters))
		}

		testFunctionType(t, function.Type, tt.expectedTypes, tt.expectedReturn)

		for i, ident := range tt.expectedParams {
			testLiteralExpr(t, function.Parameters[i], ident)
		}
	}
}

func TestCallExprParsing(t *testing.T) {
	source := "add(1, 2 * 3, 4 + 5)"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts does not contain %d statements. got=%d", 1, len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
	}

	expr, ok := stmt.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.CallExpr. got=%T", stmt.Expr)
	}

	if !testIdentifier(t, expr.Function, "add") {
		return
	}

	if len(expr.Arguments) != 3 {
		t.Fatalf("wrong length of args. wanted=%d. got=%d", 3, len(expr.Arguments))
	}

	testLiteralExpr(t, expr.Arguments[0], 1)
	testInfixExpr(t, expr.Arguments[1], 2, "*", 3)
	testInfixExpr(t, expr.Arguments[2], 4, "+", 5)
}

func TestCallExprArgsParsing(t *testing.T) {
	tests := []struct {
		source       string
		expectedArgs []string
	}{
		{source: "f()", expectedArgs: []string{}},
		{source: "f(x)", expectedArgs: []string{"x"}},
		{source: "f(x, y + z, w * v)", expectedArgs: []string{"x", "(y + z)", "(w * v)"}},
	}

	for _, tt := range tests {
		l := lexer.New(tt.source)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Stmts) != 1 {
			t.Fatalf("program.Stmts does not contain %d statements. got=%d", 1, len(program.Stmts))
		}

		stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
		if !ok {
			t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
		}

		expr, ok := stmt.Expr.(*ast.CallExpr)
		if !ok {
			t.Fatalf("stmt.Expr is not *ast.CallExpr. got=%T", stmt.Expr)
		}

		if !testIdentifier(t, expr.Function, "f") {
			return
		}

		if len(expr.Arguments) != len(tt.expectedArgs) {
			t.Fatalf("wrong length of args. wanted=%d. got=%d", len(tt.expectedArgs), len(expr.Arguments))
		}

		for i, arg := range tt.expectedArgs {
			if expr.Arguments[i].String() != arg {
				t.Fatalf("argument %d wrong. wanted=%s. got=%s", i, arg, expr.Arguments[i].String())
			}
		}
	}
}

func TestVarAssignmentStmt(t *testing.T) {
	tests := []struct {
		source        string
		expectedIdent string
		expectedValue interface{}
	}{
		{"x = 5;", "x", 5},
		{"x = true;", "x", true},
		{"x = y;", "x", "y"},
	}

	for _, tt := range tests {
		l := lexer.New(tt.source)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Stmts) != 1 {
			t.Fatalf("program.Stmts does not contain 1 statement. got=%d", len(program.Stmts))
		}

		expr, ok := program.Stmts[0].(*ast.VarAssignmentStmt)

		if !ok {
			t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
		}

		if !testVarAssignmentStmt(t, expr, tt.expectedIdent) {
			return
		}

		if !testLiteralExpr(t, expr.Value, tt.expectedValue) {
			return
		}
	}
}

func TestStringLiteralExpr(t *testing.T) {
	source := `"Hello, World!`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt := program.Stmts[0].(*ast.ExpressionStmt)
	literal, ok := stmt.Expr.(*ast.StringLiteral)
	if !ok {
		t.Fatalf("expr not *ast.StringLiteral. got=%T", stmt.Expr)
	}

	if literal.Value != "Hello, World!" {
		t.Errorf("literal.HashValue not %q. got=%q", "Hello, World!", literal.Value)
	}
}

func TestParsingArrayLiterals(t *testing.T) {
	source := "[1, 2 * 2, 3 + 3]"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	array, ok := stmt.Expr.(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("expr is not *ast.ArrayLiteral. got=%T", stmt.Expr)
	}

	if len(array.Elements) != 3 {
		t.Fatalf("array.Elements does not have 3 elements. got=%d", len(array.Elements))
	}

	testIntegerLiteral(t, array.Elements[0], 1)
	testInfixExpr(t, array.Elements[1], 2, "*", 2)
	testInfixExpr(t, array.Elements[2], 3, "+", 3)
}

func TestParsingIndexExpr(t *testing.T) {
	source := "arr[1 + 1]"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	expr, ok := stmt.Expr.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("expr not *ast.IndexExpr. got=%T", stmt.Expr)
	}

	if !testIdentifier(t, expr.Left, "arr") {
		return
	}
	if !testInfixExpr(t, expr.Index, 1, "+", 1) {
		return
	}
}

func TestParsingIndexAssignments(t *testing.T) {
	source := "a[0] = 1;"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.IndexAssignmentStmt)
	if !ok {
		t.Fatalf("stmt is not *ast.IndexAssignmentStmt")
	}

	testIntegerLiteral(t, stmt.Value, 1)
	if !testIdentifier(t, stmt.Left.Left, "a") {
		t.Fatalf("stmt.Left.Left is not *ast.IndexExpr.")
	}

}

func TestNullLiteral(t *testing.T) {
	source := "null;"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	expr, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("expr is not *ast.ExpressionStmt. got=%T", expr)
	}
	null, ok := expr.Expr.(*ast.NullLiteral)
	if !ok {
		t.Fatalf("expr.Expr is not *ast.NullLiteral. got=%T", expr.Expr)
	}

	if null.String() != "null" {
		t.Fatalf("null has wrong value. got=%s", null.String())
	}

}

func TestParsingForStmts(t *testing.T) {
	source := "for (x < 10) { x }"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts has too many elements. want=1, got=%d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ForStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ForStmt. got=%T", program.Stmts[0])
	}

	condition := stmt.Condition
	if !testInfixExpr(t, condition, "x", "<", 10) {
		return
	}

	body := stmt.Body

	if len(body.Stmts) != 1 {
		t.Fatalf("body.Stmts has wrong number of eleents. want=1, got=%d", len(body.Stmts))
	}

	expr, ok := body.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("body.Stmts[0] not *ast.ExpressionStmt. got=%T", body.Stmts[0])
	}

	ident := expr.Expr.(*ast.Identifier)
	if !testIdentifier(t, ident, "x") {
		return
	}
}

func TestParsingHashLiteralsStringKeys(t *testing.T) {
	source := `{"one": 1, "two": 2, "three": 3}`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt := program.Stmts[0].(*ast.ExpressionStmt)
	hash, ok := stmt.Expr.(*ast.HashLiteral)
	if !ok {
		t.Fatalf("expr is not *ast.HashLiteral. got=%T", stmt.Expr)
	}

	if len(hash.Pairs) != 3 {
		t.Errorf("hash.Pairs has wrong length. want=3, got=%d", len(hash.Pairs))
	}

	expected := map[string]int64{
		"one":   1,
		"two":   2,
		"three": 3,
	}

	for key, val := range hash.Pairs {
		literal, ok := key.(*ast.StringLiteral)
		if !ok {
			t.Errorf("key is not *ast.StringLiteral. got=%T", key)
		}

		expectedVal := expected[literal.String()]

		testIntegerLiteral(t, val, expectedVal)
	}
}

func TestParsingHashLiteralsBooleanKeys(t *testing.T) {
	source := `{true: 1, false: 2}`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt := program.Stmts[0].(*ast.ExpressionStmt)
	hash, ok := stmt.Expr.(*ast.HashLiteral)
	if !ok {
		t.Fatalf("expr is not *ast.HashLiteral. got=%T", stmt.Expr)
	}

	if len(hash.Pairs) != 2 {
		t.Errorf("hash.Pairs has wrong length. want=2, got=%d", len(hash.Pairs))
	}

	expected := map[string]int64{
		"true":  1,
		"false": 2,
	}

	for key, val := range hash.Pairs {
		literal, ok := key.(*ast.BooleanLiteral)
		if !ok {
			t.Errorf("key is not *ast.StringLiteral. got=%T", key)
		}

		expectedVal := expected[literal.String()]

		testIntegerLiteral(t, val, expectedVal)
	}
}

func TestParsingHashLiteralsIntegerKeys(t *testing.T) {
	source := `{1: 1, 2: 2, 3: 3}`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt := program.Stmts[0].(*ast.ExpressionStmt)
	hash, ok := stmt.Expr.(*ast.HashLiteral)
	if !ok {
		t.Fatalf("expr is not *ast.HashLiteral. got=%T", stmt.Expr)
	}

	if len(hash.Pairs) != 3 {
		t.Errorf("hash.Pairs has wrong length. want=3, got=%d", len(hash.Pairs))
	}

	expected := map[string]int64{
		"1": 1,
		"2": 2,
		"3": 3,
	}

	for key, val := range hash.Pairs {
		literal, ok := key.(*ast.IntegerLiteral)
		if !ok {
			t.Errorf("key is not *ast.StringLiteral. got=%T", key)
		}

		expectedVal := expected[literal.String()]

		testIntegerLiteral(t, val, expectedVal)
	}
}

func TestParsingEmptyHashLiteral(t *testing.T) {
	source := "{}"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt := program.Stmts[0].(*ast.ExpressionStmt)
	hash, ok := stmt.Expr.(*ast.HashLiteral)
	if !ok {
		t.Fatalf("expr is not *ast.HashLiteral. got=%T", stmt.Expr)
	}

	if len(hash.Pairs) != 0 {
		t.Errorf("hash.Pairs has wrong length. want=0, got=%d", len(hash.Pairs))
	}
}

func TestParsingHashLiteralWithExpressions(t *testing.T) {
	source := `{"one": 0 + 1, "two": 10 - 8, "three": 15 / 5 }`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt := program.Stmts[0].(*ast.ExpressionStmt)
	hash, ok := stmt.Expr.(*ast.HashLiteral)
	if !ok {
		t.Fatalf("exp is not ast.HashLiteral. got=%T", stmt.Expr)
	}
	if len(hash.Pairs) != 3 {
		t.Errorf("hash.Pairs has wrong length. got=%d", len(hash.Pairs))
	}
	tests := map[string]func(expr ast.Expr){
		"one": func(e ast.Expr) {
			testInfixExpr(t, e, 0, "+", 1)
		},
		"two": func(e ast.Expr) {
			testInfixExpr(t, e, 10, "-", 8)
		},
		"three": func(e ast.Expr) {
			testInfixExpr(t, e, 15, "/", 5)
		},
	}
	for key, value := range hash.Pairs {
		literal, ok := key.(*ast.StringLiteral)
		if !ok {
			t.Errorf("key is not ast.StringLiteral. got=%T", key)
			continue
		}
		testFunc, ok := tests[literal.String()]
		if !ok {
			t.Errorf("No test function for key %q found", literal.String())
			continue
		}
		testFunc(value)
	}
}

func TestFloatLiteralExpr(t *testing.T) {
	source := "5.2"

	l := lexer.New(source)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program has not enough statements. got=%d", len(program.Stmts))
	}
	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)

	if !ok {
		t.Fatalf("program.Stmts[0] not ast.ExpressionStmt. got=%T", stmt)
	}

	if !testFloatLiteral(t, stmt.Expr, 5.2) {
		return
	}
}

func TestMacroLiteralParsing(t *testing.T) {
	source := `macro(x, y) { x + y; }`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts does not contain %d statements. got=%d\n", 1, len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Statements[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
	}

	macro, ok := stmt.Expr.(*ast.MacroLiteral)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.MacroLiteral. got=%T", stmt.Expr)
	}

	if len(macro.Parameters) != 2 {
		t.Fatalf("macro literal parameters wrong. want 2, got=%d\n", len(macro.Parameters))
	}

	testLiteralExpr(t, macro.Parameters[0], "x")
	testLiteralExpr(t, macro.Parameters[1], "y")

	if len(macro.Body.Stmts) != 1 {
		t.Fatalf("macro.Body.Stmts does not contain %d statements. got=%d\n", 1, len(macro.Body.Stmts))
	}

	bodyStmt, ok := macro.Body.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("macro body stmt is not *ast.ExpressionStmt. got=%T", macro.Body.Stmts[0])
	}

	testInfixExpr(t, bodyStmt.Expr, "x", "+", "y")
}

func TestFunctionLiteralWithName(t *testing.T) {
	source := `const myFunc = func() { }`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("progream.Body does not contain %d stmts. got=%d\n", 1, len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.VarDeclarationStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.VarDeclarationStmt. got=%T", program.Stmts[0])
	}

	fn, ok := stmt.Value.(*ast.FunctionLiteral)
	if !ok {
		t.Fatalf("stmt.Value is not *ast.FunctionLiteral. got=%T", stmt.Value)
	}

	if fn.Name != "myFunc" {
		t.Fatalf("function literal name wrong. want `myFunc`, got=%q\n", fn.Name)
	}
}

func TestFunctionDeclaration(t *testing.T) {
	tests := []struct {
		input              string
		expectedName       string
		expectedParams     []string
		expectedParamTypes []string
		expectedReturnType string
	}{
		{
			input:              "func f(int x) -> int { return x * 2; };",
			expectedName:       "f",
			expectedParams:     []string{"x"},
			expectedParamTypes: []string{"int"},
			expectedReturnType: "int",
		},
		{
			input:              "func f(int x, bool y) -> int { if (y) { return x; } else { return x - 2 } };",
			expectedName:       "f",
			expectedParams:     []string{"x", "y"},
			expectedParamTypes: []string{"int", "bool"},
			expectedReturnType: "int",
		},
		{
			input:              "func f(int x) { print(x); };",
			expectedName:       "f",
			expectedParamTypes: []string{"int"},
			expectedReturnType: "unit",
			expectedParams:     []string{"x"},
		},
		{
			input:              "func f(int x) { print(x); };",
			expectedName:       "f",
			expectedParamTypes: []string{"int"},
			expectedReturnType: "unit",
			expectedParams:     []string{"x"},
		},
		{
			input:              "func f() { };",
			expectedName:       "f",
			expectedParamTypes: []string{},
			expectedReturnType: "unit",
			expectedParams:     []string{},
		},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)
		if len(program.Stmts) != 1 {
			t.Fatalf("program.Stmts does not contain %d statements. got=%d\n", 1, len(program.Stmts))
		}

		stmt, ok := program.Stmts[0].(*ast.FunctionDeclarationStmt)
		if !ok {
			t.Fatalf("program.Stmts[0] is not *ast.FunctionDeclarationStmt. got=%T", program.Stmts[0])
		}

		if stmt.Name.String() != tt.expectedName {
			t.Fatalf("stmt.Name.String() wrong. want %q, got=%q", tt.expectedName, stmt.Name.String())
		}
		if len(stmt.Params) != len(tt.expectedParams) {
			t.Fatalf("got wrong number of parameters. want=%d, got=%d", len(tt.expectedParams), len(stmt.Params))
		}

		for i, param := range tt.expectedParams {
			if stmt.Params[i].String() != param {
				t.Fatalf("wrong argument %d, got %s expected %s", i, stmt.Params[i], param)
			}
		}

		fType, ok := stmt.Type.(types.FunctionType)
		if !ok {
			t.Fatalf("stmt.Type is not *types.FunctionType. got=%T", stmt.Type)
		}

		if len(fType.Params) != len(tt.expectedParamTypes) {
			t.Fatalf("got wrong number of parameters. want=%d, got=%d", len(tt.expectedParams), len(stmt.Params))
		}

		for i, param := range tt.expectedParamTypes {
			if fType.Params[i].Signature() != param {
				t.Fatalf("wrong argument type %d, got %s expected %s", i, fType.Params[i].Signature(), param)
			}
		}

		if fType.Return.Signature() != tt.expectedReturnType {
			t.Fatalf("wrong return type, got %s expected %s", fType.Return.Signature(), tt.expectedReturnType)
		}
	}
}

// Utilities

func checkParserErrors(t *testing.T, p *Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}

func testVarDeclarationStmt(t *testing.T, s ast.Stmt, name string, isConst bool) bool {
	if s.TokenLiteral() != "mut" && s.TokenLiteral() != "const" {
		t.Errorf("s.TokenLiteral not `mut` or `const`. got=%q", s.TokenLiteral())
		return false
	}

	varDeclStmt, ok := s.(*ast.VarDeclarationStmt)
	if !ok {
		t.Errorf("s not *ast.VarDeclarationStmt. got=%T", s)
		return false
	}

	if varDeclStmt.Name.Value != name {
		t.Errorf("varDeclStmt.Name.HashValue not '%s'. got=%s", name, varDeclStmt.Name.Value)
		return false
	}

	if varDeclStmt.Name.TokenLiteral() != name {
		t.Errorf("varDeclStmt.Name.TokenLiteral() not '%s'. got=%s", name, varDeclStmt.Name.TokenLiteral())
		return false
	}

	if varDeclStmt.Constant != isConst {
		t.Errorf("varDeclStmt.Name.Constant not %t. got=%t", isConst, varDeclStmt.Constant)
	}

	return true
}

func testVarAssignmentStmt(t *testing.T, s ast.Stmt, ident string) bool {

	varAssignExpr, ok := s.(*ast.VarAssignmentStmt)

	if !ok {
		t.Errorf("e not *ast.VarAssignmentExpr. got=%T", s)
		return false
	}

	if varAssignExpr.Identifier.TokenLiteral() != ident {
		t.Errorf("varAssignExpr.Identifier.TokenLiteral() not '%s'. got=%s", ident, varAssignExpr.Identifier.TokenLiteral())
		return false
	}

	if !testIdentifier(t, varAssignExpr.Identifier, ident) {
		return false
	}

	return true
}

func testIntegerLiteral(t *testing.T, i ast.Expr, value int64) bool {
	integ, ok := i.(*ast.IntegerLiteral)
	if !ok {
		t.Errorf("i not *ast.IntegerLiteral. got=%T", integ)
		return false
	}

	if integ.Value != value {
		t.Errorf("integ.HashValue not %d. got=%d", value, integ.Value)
		return false
	}

	if integ.TokenLiteral() != fmt.Sprintf("%d", value) {
		t.Errorf("integ.TokenLiteral not %d. got=%s", value, integ.TokenLiteral())
		return false
	}

	return true
}

func testIdentifier(t *testing.T, expr ast.Expr, value string) bool {
	ident, ok := expr.(*ast.Identifier)

	if !ok {
		t.Errorf("expr not *ast.Identifier. got=%T", expr)
		return false
	}

	if ident.Value != value {
		t.Errorf("ident.HashValue not %s. got=%s", value, ident.Value)
		return false
	}

	if ident.TokenLiteral() != value {
		t.Errorf("ident.TokenLiteral not %s. got=%s", value, ident.TokenLiteral())
		return false
	}

	return true
}

func testLiteralExpr(t *testing.T, expr ast.Expr, expected interface{}) bool {
	switch v := expected.(type) {
	case int:
		return testIntegerLiteral(t, expr, int64(v))
	case int64:
		return testIntegerLiteral(t, expr, v)
	case string:
		return testIdentifier(t, expr, v)
	case bool:
		return testBooleanLiteral(t, expr, v)
	}
	t.Errorf("type of expr not handled. got=%T", expr)
	return false
}

func testInfixExpr(t *testing.T, expr ast.Expr, left interface{}, operator string, right interface{}) bool {
	opExpr, ok := expr.(*ast.InfixExpr)
	if !ok {
		t.Errorf("expr is not *ast.InfixExpr. got=%T(%s)", expr, expr)
		return false
	}

	if !testLiteralExpr(t, opExpr.Left, left) {
		return false
	}

	if opExpr.Operator != operator {
		t.Errorf("expr.Operator is not '%s'. got=%s", operator, opExpr.Operator)
		return false
	}

	if !testLiteralExpr(t, opExpr.Right, right) {
		return false
	}

	return true
}

func testBooleanLiteral(t *testing.T, expr ast.Expr, value bool) bool {
	boolean, ok := expr.(*ast.BooleanLiteral)
	if !ok {
		t.Errorf("expr not *ast.BooleanLiteral. got=%T", expr)
		return false
	}

	if boolean.Value != value {
		t.Errorf("boolean.HashValue not %t. got=%t", value, boolean.Value)
		return false
	}

	if boolean.TokenLiteral() != fmt.Sprintf("%t", value) {
		t.Errorf("boolean.TokenLiteral not %t. got=%s", value, boolean.TokenLiteral())
		return false
	}

	return true
}

func testNullLiteral(t *testing.T, expr ast.Expr) bool {
	null, ok := expr.(*ast.NullLiteral)
	if !ok {
		t.Errorf("expr is not *ast.NullLiteral. got=%T", expr)
	}

	if null.TokenLiteral() != "null" {
		t.Errorf("null.TokenLiteral() not null. got=%s", null.TokenLiteral())
		return false
	}
	return true
}

func testFloatLiteral(t *testing.T, i ast.Expr, value float64) bool {
	float, ok := i.(*ast.FloatLiteral)
	if !ok {
		t.Errorf("i not *ast.FloatLiteral. got=%T", float)
		return false
	}

	if float.Value != value {
		t.Errorf("float.Value not %f. got=%f", value, float.Value)
		return false
	}

	valAsStr := strconv.FormatFloat(value, 'f', -1, 64)
	if float.TokenLiteral() != valAsStr {
		t.Errorf("float.TokenLiteral not %s. got=%s", valAsStr, float.TokenLiteral())
		return false
	}

	return true
}

func testFunctionType(t *testing.T, ty types.FunctionType, expectedParams []string, expectedReturn interface{}) {
	if len(ty.Params) != len(expectedParams) {
		t.Fatalf("len(params) not %d. got=%d", len(ty.Params), len(expectedParams))
	}

	for i, p := range ty.Params {
		if p.Signature() != expectedParams[i] {
			t.Fatalf("params[%d].Signature() not %s. got=%s", i, p.Signature(), expectedParams[i])
		}
	}

	if expectedReturn != nil {
		asStr, _ := expectedReturn.(string)
		if asStr != ty.Return.Signature() {
			t.Fatalf("Return.Signature() not %s. got=%s", ty.Return.Signature(), asStr)
		}
	}
}
