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
		{"mut byte b = 'a';", "b", byte('a'), types.Byte, false},
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
			}
			continue
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

func TestParsingSelectorAssignments(t *testing.T) {
	source := "a.x = 1;"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.SelectorAssignmentStmt)
	if !ok {
		t.Fatalf("stmt is not *ast.SelectorAssignmentStmt")
	}

	testIntegerLiteral(t, stmt.Value, 1)
	if !testIdentifier(t, stmt.Left.Left, "a") {
		t.Fatalf("stmt.Left.Left is not a.")
	}

	if !testIdentifier(t, stmt.Left.Value, "x") {
		t.Fatalf("stmt.Left.Value is not x")
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

func TestStructDefinition(t *testing.T) {
	source := "define struct Person { name string, age int }"
	expectedType := types.StructType{Name: "Person", Fields: []string{"name", "age"}, Types: []types.Type{types.String, types.Int}}
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	if len(program.Stmts) != 1 {
		t.Fatalf("program.Body does not contain %d statements. got=%d\n", 1, len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.StructDefinitionStmt)
	if !ok {
		t.Fatalf("expected program.Stmts[0] to be *ast.StructDefinitionStmt. got=%T", program.Stmts[0])
	}

	if stmt.Name.String() != "Person" {
		t.Fatalf("stmt.Name.String() wrong. want %q, got=%q", "Person", stmt.Name.String())
	}

	if stmt.Type.Signature() != expectedType.Signature() {
		t.Fatalf("stmt.Type.Signature() wrong. want %q, got=%q", expectedType.Signature(), stmt.Type.Signature())
	}
}

func TestStructLiteral(t *testing.T) {
	source := "Person { name: \"Alice\", age: 42 };"
	expectedFields := []ExpectedStructField{
		{
			name:  "name",
			value: "Alice",
		},
		{
			name:  "age",
			value: 42,
		},
	}

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	if len(program.Stmts) != 1 {
		t.Fatalf("program.Body does not contain %d statements. got=%d\n", 1, len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("expected program.Stmts to be *ast.ExpressionStmt. got=%T", program.Stmts[0])
	}

	expr, ok := stmt.Expr.(*ast.StructLiteral)
	if !ok {
		t.Fatalf("expected stmt.Expr to be *ast.StructLiteral. got=%T", stmt.Expr)
	}

	testStructLiteral(t, expr, expectedFields, "Person")
}

type ExpectedStructField struct {
	name                       string
	value                      interface{}
	expectedEmbeddedStructName string
	expectedEmbeddedFields     []ExpectedStructField
}

func TestParseEmbeddedStructs(t *testing.T) {
	source := "Circle { center: Point { x: 0, y: 0 }, radius: 5 };"
	expectedFields := []ExpectedStructField{
		{
			name: "center",
			value: &ast.StructLiteral{
				Name:   "Person",
				Fields: []string{"x", "y"},
				Values: []ast.Expr{&ast.IntegerLiteral{Value: 0}, &ast.IntegerLiteral{Value: 0}},
			},
			expectedEmbeddedStructName: "Person",
			expectedEmbeddedFields: []ExpectedStructField{
				{
					name:  "x",
					value: 0,
				},
				{
					name:  "y",
					value: 0,
				},
			},
		}, {
			name:  "radius",
			value: 5,
		},
	}
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Body does not contain %d statements. got=%d\n", 1, len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("expected program.Stmts to be *ast.ExpressionStmt. got=%T", program.Stmts[0])
	}

	expr, ok := stmt.Expr.(*ast.StructLiteral)
	if !ok {
		t.Fatalf("expected stmt.Expr to be *ast.StructLiteral. got=%T", stmt.Expr)
	}

	testStructLiteral(t, expr, expectedFields, "Circle")
}

func TestParseInterfaceDefinition(t *testing.T) {
	source := "define interface Pointer { getX() -> int, setX(int x) }"
	expectedType := types.InterfaceType{
		Name:    "Pointer",
		Methods: []string{"getX", "setX"},
		Types: []types.Type{
			types.FunctionType{
				Params: []types.Type{},
				Return: types.Int,
			},
			types.FunctionType{
				Params: []types.Type{types.Int},
				Return: types.Unit,
			},
		},
	}
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	if len(program.Stmts) != 1 {
		t.Fatalf("program.Body does not contain %d statements. got=%d\n", 1, len(program.Stmts))
	}
	stmt, ok := program.Stmts[0].(*ast.InterfaceDefinitionStmt)
	if !ok {
		t.Fatalf("expected program.Stmts to be *ast.InterfaceDefinitionStmt. got=%T", program.Stmts[0])
	}
	testInterfaceDefinition(t, stmt, expectedType)
}

func TestParseInterfaceImplementation(t *testing.T) {
	source := "define implementation Circle -> Diameter, Area"
	expected := []string{"Diameter", "Area"}

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	if len(program.Stmts) != 1 {
		t.Fatalf("program.Body does not contain %d statements. got=%d\n", 1, len(program.Stmts))
	}
	stmt, ok := program.Stmts[0].(*ast.InterfaceImplementationStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.InterfaceImplementationStmt. got=%T", program.Stmts[0])
	}

	testIdentifier(t, stmt.StructName, "Circle")
	for i, e := range expected {
		testIdentifier(t, stmt.InterfaceNames[i], e)
	}
}

func testStructLiteral(t *testing.T, expr *ast.StructLiteral, expectedFields []ExpectedStructField, expectedName string) {
	if expr.Name != expectedName {
		t.Fatalf("expr.Name wrong. want %q, got=%q", "Person", expr.Name)
	}
	if len(expr.Fields) != len(expectedFields) {
		t.Fatalf("got wrong number of fields, want=%d, got=%d", len(expectedFields), len(expr.Fields))
	}
	if len(expr.Values) != len(expectedFields) {
		t.Fatalf("got wrong number of values, want=%d, got=%d", len(expectedFields), len(expr.Values))
	}
	for i, field := range expectedFields {
		if field.name != expr.Fields[i] {
			t.Fatalf("wrong field %d, got %s expected %s", i, field, expr.Fields[i])
		}

		embeddedStruct, ok := field.value.(*ast.StructLiteral)
		if ok {
			testStructLiteral(t, embeddedStruct, field.expectedEmbeddedFields, field.expectedEmbeddedStructName)
			continue
		}

		switch field.value.(type) {
		case string:
			str, ok := expr.Values[i].(*ast.StringLiteral)
			if !ok {
				t.Fatalf("expected stmt.Values[%d] to be *ast.StringLiteral. got=%T", i, expr.Values[i])
			}
			if str.Value != field.value {
				t.Fatalf("wrong value for field %s, got %s expected %s", field.name, str.Value, field.value)
			}
		case int:
			num, ok := expr.Values[i].(*ast.IntegerLiteral)
			if !ok {
				t.Fatalf("expected stmt.Values[%d] to be *ast.IntegerLiteral. got=%T", i, expr.Values[i])
			}

			if int(num.Value) != field.value {
				t.Fatalf("wrong value for field %s, got %d expected %v", field.name, num.Value, field.value)
			}
		default:
			t.Fatalf("wrong type for field %d, got %T", i, field)
		}
	}
}

func TestSelectorExpr(t *testing.T) {
	source := "p.x"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	expr, ok := stmt.Expr.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("expr not *ast.SelectorExpr. got=%T", stmt.Expr)
	}

	testSelectorExpr(t, expr, "p", "x")
}

func TestMultiParamInterfaceMethods(t *testing.T) {
	source := "define interface Comparable { compare(Comparable other, int mode) -> int }"
	expectedType := types.InterfaceType{
		Name:    "Comparable",
		Methods: []string{"compare"},
		Types: []types.Type{
			types.FunctionType{
				Params: []types.Type{
					types.InterfaceType{Name: "Comparable"},
					types.Int,
				},
				Return: types.Int,
			},
		},
	}

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	stmt, ok := program.Stmts[0].(*ast.InterfaceDefinitionStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.InterfaceDefinitionStmt. got=%T", program.Stmts[0])
	}

	testInterfaceDefinition(t, stmt, expectedType)
}

func testInterfaceDefinition(t *testing.T, stmt *ast.InterfaceDefinitionStmt, expectedType types.InterfaceType) {
	if stmt.Name.String() != expectedType.Name {
		t.Fatalf("stmt.Name.String() wrong. want %q, got=%q", "Pointer", stmt.Name.String())
	}
	tt := stmt.Type
	if len(tt.Methods) != len(expectedType.Methods) {
		t.Fatalf("wrong number of methods. want=%d, got=%d", len(expectedType.Methods), len(stmt.Type.Methods))
	}

	if len(tt.Types) != len(expectedType.Types) {
		t.Fatalf("wrong number of types. want=%d, got=%d", len(expectedType.Types), len(tt.Types))
	}

	for i, m := range tt.Methods {
		if m != expectedType.Methods[i] {
			t.Fatalf("wrong method. want=%q, got=%q", m, expectedType.Methods[i])
		}
	}

	for i, tt := range tt.Types {
		if tt.Signature() != expectedType.Types[i].Signature() {
			t.Fatalf("wrong signature. want=%q, got=%q", tt.Signature(), expectedType.Types[i].Signature())
		}
	}
}

func TestChainedSelectorExpr(t *testing.T) {
	source := "c.center.x"
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
	}
	expr, ok := stmt.Expr.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.SelectorExpr. got=%T", stmt.Expr)
	}

	left, ok := expr.Left.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("expr.Left is not *ast.SelectorExpr. got=%T", expr.Left)
	}
	testSelectorExpr(t, left, "c", "center")

	if !testIdentifier(t, expr.Value, "x") {
		t.Fatalf("expr.Value wrong, wanted ident x, got %q", expr.Value)
	}
}

func TestSelectorCallReceiver(t *testing.T) {
	source := "c.center.distance(origin)"
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
	}
	expr, ok := stmt.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.CallExpr. got=%T", stmt.Expr)
	}
	callee, ok := expr.Function.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.SelectorExpr. got=%T", stmt.Expr)
	}
	left, ok := callee.Left.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("left is not *ast.SelectorExpr. got=%T", callee.Left)
	}
	testSelectorExpr(t, left, "c", "center")

	if !testIdentifier(t, callee.Value, "distance") {
		t.Fatalf("callee.Value wrong, wanted ident x, got %q", callee.Value)
	}
	if len(expr.Arguments) != 1 {
		t.Fatalf("len(expr.Arguments) wrong, wanted 1, got %d", len(expr.Arguments))
	}

	testIdentifier(t, expr.Arguments[0], "origin")
}

func testSelectorExpr(t *testing.T, expr *ast.SelectorExpr, expectedIdent string, expectedValue string) {
	if !testIdentifier(t, expr.Left, expectedIdent) {
		t.Fatal("SelectorExpr expr.Left wrong")
	}
	if !testIdentifier(t, expr.Value, expectedValue) {
		t.Fatal("SelectorExpr expr.Value wrong")
	}
}

func TestCollectionTypeParsing(t *testing.T) {
	tests := []struct {
		source       string
		expectedType types.Type
		expectedName string
	}{
		{
			source: "mut map<int, int> m;",
			expectedType: types.MapType{
				KeyType:   types.Int,
				ValueType: types.Int,
			},
			expectedName: "m",
		},
		{
			source: "mut array<int> a;",
			expectedType: types.ArrayType{
				ElemType: types.Int,
			},
			expectedName: "a",
		},
		{
			source: "mut map<string, array<int>> a;",
			expectedType: types.MapType{
				KeyType: types.String,
				ValueType: types.ArrayType{
					ElemType: types.Int,
				},
			},
			expectedName: "a",
		},
	}

	for _, tt := range tests {
		l := lexer.New(tt.source)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		stmt, ok := program.Stmts[0].(*ast.VarDeclarationStmt)
		if !ok {
			t.Fatalf("")
		}

		ident := stmt.Name
		if !testIdentifier(t, ident, tt.expectedName) {
			t.Fatalf("stmt.Name wrong. want %q, got=%q", tt.expectedName, ident)
		}

		typ := stmt.Type
		testType(t, typ, tt.expectedType)
	}
}

func testType(t *testing.T, actual types.Type, expected types.Type) {
	if actual.Signature() != expected.Signature() {
		t.Fatalf("Signature wrong. want=%q, got=%q", expected.Signature(), actual.Signature())
	}
}

func TestStructLiteralsAsArguments(t *testing.T) {
	source := `define struct Point { x int, y int } printPoint(Point { x: 0, y: 0 });`
	expectedFields := []ExpectedStructField{
		{
			name:  "x",
			value: 0,
		},
		{
			name:  "y",
			value: 0,
		},
	}

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	if len(program.Stmts) != 2 {
		t.Fatalf("len(program.Stmts) wrong, wanted 2, got %d", len(program.Stmts))
	}

	_, ok := program.Stmts[0].(*ast.StructDefinitionStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.StructDefinitionStmt. got=%T", program.Stmts[0])
	}

	stmt, ok := program.Stmts[1].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Stmts[1] is not *ast.ExpressionStmt. got=%T", stmt.Expr)
	}

	expr, ok := stmt.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.CallExpr. got=%T", stmt.Expr)
	}

	if len(expr.Arguments) != 1 {
		t.Fatalf("len(expr.Arguments) wrong, wanted 1, got %d", len(expr.Arguments))
	}
	st, ok := expr.Arguments[0].(*ast.StructLiteral)
	if !ok {
		t.Fatalf("expr.Arguments[0] is not *ast.StructLiteral. got=%T", stmt.Expr)
	}

	testStructLiteral(t, st, expectedFields, "Point")
}

func TestStructFieldOrder(t *testing.T) {
	source := `define struct Rect { x int, y int, w int, h int }
const r = Rect { w: 10, h: 20, x: 0, y: 0 };`
	expectedFields := []ExpectedStructField{
		{
			name:  "w",
			value: 10,
		},
		{
			name:  "h",
			value: 20,
		},
		{
			name:  "x",
			value: 0,
		},
		{
			name:  "y",
			value: 0,
		},
	}
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 2 {
		t.Fatalf("len(program.Stmts) wrong, wanted 2, got %d", len(program.Stmts))
	}
	_, ok := program.Stmts[0].(*ast.StructDefinitionStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.StructDefinitionStmt. got=%T", program.Stmts[0])
	}
	stmt, ok := program.Stmts[1].(*ast.VarDeclarationStmt)
	if !ok {
		t.Fatalf("program.Stmts[1] is not *ast.VarDeclarationStmt. got=%T", program.Stmts[1])
	}
	ident := stmt.Name
	if !testIdentifier(t, ident, "r") {
		t.Fatalf("stmt.Name wrong. want %q, got=%q", ident, "r")
	}

	lit := stmt.Value
	stLit, ok := lit.(*ast.StructLiteral)
	if !ok {
		t.Fatalf("lit is not *ast.StructLiteral. got=%T", lit)
	}

	testStructLiteral(t, stLit, expectedFields, "Rect")
}

func TestModuleDeclaration(t *testing.T) {
	source := "module \"math\""

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("len(program.Stmts) wrong, wanted 1, got %d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ModuleDeclarationStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ModuleDeclarationStmt. got=%T", stmt)
	}

	if stmt.Name.Value != "math" {
		t.Fatalf("stmt.Name wrong. want math, got=%q", stmt.Name.Value)
	}
}

func TestImportStatement(t *testing.T) {
	source := "import \"math\""

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	if len(program.Stmts) != 1 {
		t.Fatalf("len(program.Stmts) wrong, wanted 1, got %d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ImportStatement)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ImportStmt. got=%T", stmt)
	}

	if stmt.Name.Value != "math" {
		t.Fatalf("stmt.Name wrong. want math, got=%q", stmt.Name.Value)
	}
}

func TestScopeAccessExpression(t *testing.T) {
	source := "math:sqrt"
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	if len(program.Stmts) != 1 {
		t.Fatalf("len(program.Stmts) wrong, wanted 1, got %d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt. got=%T", stmt)
	}

	expr, ok := stmt.Expr.(*ast.ScopeAccessExpr)
	if !ok {
		t.Fatalf("stmt.Expression is not *ast.ScopeAccessExpr. got=%T", stmt.Expr)
	}

	if expr.Module.Value != "math" {
		t.Fatalf("expr.Module wrong. want math, got=%q", expr.Module.Value)
	}

	if expr.Member.Value != "sqrt" {
		t.Fatalf("expr.Member wrong. want sqrt, got=%q", expr.Member.Value)
	}
}

func TestResultTypeParsing(t *testing.T) {
	source := "mut result<int> x;"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	if len(program.Stmts) != 1 {
		t.Fatalf("len(program.Stmts) wrong, wanted 1, got %d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.VarDeclarationStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.VarDeclarationStmt. got=%T", stmt)
	}

	typ := stmt.Type
	rTyp, ok := typ.(types.ResultType)
	if !ok {
		t.Fatalf("stmt.Type is not types.ResultType. got=%T", typ)
	}
	if rTyp.Signature() != "result<int>" {
		t.Fatalf("stmt.Type wrong. want result<int>, got=%q", rTyp.Signature())
	}

	testVarDeclarationStmt(t, stmt, "x", false)
}

func TestMatchExpr(t *testing.T) {
	source := `match x {
		ok(val) -> { val * 2; },
		err(msg) -> { 0; },
	}`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("len(program.Stmts) wrong, wanted 1, got %d", len(program.Stmts))
	}
	stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt. got=%T", program.Stmts[0])
	}
	expr, ok := stmt.Expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.MatchExpr. got=%T", stmt.Expr)
	}

	testIdentifier(t, expr.Subject, "x")
	testMatchArm(t, expr.OkArm, "val", true)
	testMatchArm(t, expr.ErrArm, "msg", false)

	// check ok arm body: val * 2
	if len(expr.OkArm.Body.Stmts) != 1 {
		t.Fatalf("ok arm body should have 1 statement, got %d", len(expr.OkArm.Body.Stmts))
	}
	okBody, ok := expr.OkArm.Body.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("ok arm body stmt is not *ast.ExpressionStmt. got=%T", expr.OkArm.Body.Stmts[0])
	}
	testInfixExpr(t, okBody.Expr, "val", "*", 2)

	// check err arm body: 0
	if len(expr.ErrArm.Body.Stmts) != 1 {
		t.Fatalf("err arm body should have 1 statement, got %d", len(expr.ErrArm.Body.Stmts))
	}
	errBody, ok := expr.ErrArm.Body.Stmts[0].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("err arm body stmt is not *ast.ExpressionStmt. got=%T", expr.ErrArm.Body.Stmts[0])
	}
	testIntegerLiteral(t, errBody.Expr, 0)
}

func TestThreePartForLoop(t *testing.T) {
	source := "for (mut i = 0; i < 10; i = i + 1) { print(i); }"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts has wrong length. want=1, got=%d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ForStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ForStmt. got=%T", program.Stmts[0])
	}

	if stmt.Init == nil {
		t.Fatal("for loop Init is nil")
	}
	if _, ok := stmt.Init.(*ast.VarDeclarationStmt); !ok {
		t.Fatalf("for loop Init is not *ast.VarDeclarationStmt. got=%T", stmt.Init)
	}

	if stmt.Condition == nil {
		t.Fatal("for loop Condition is nil")
	}

	if stmt.Post == nil {
		t.Fatal("for loop Post is nil")
	}
	if _, ok := stmt.Post.(*ast.VarAssignmentStmt); !ok {
		t.Fatalf("for loop Post is not *ast.VarAssignmentStmt. got=%T", stmt.Post)
	}

	if len(stmt.Body.Stmts) != 1 {
		t.Fatalf("for loop body should have 1 statement, got %d", len(stmt.Body.Stmts))
	}
}

func TestForRangeArray(t *testing.T) {
	source := "for (x in arr) { print(x); }"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts has wrong length. want=1, got=%d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ForInStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ForRangeStmt. got=%T", program.Stmts[0])
	}

	if stmt.Value.Value != "x" {
		t.Fatalf("stmt.Value wrong. want=x, got=%s", stmt.Value.Value)
	}

	if stmt.Key != nil {
		t.Fatalf("stmt.Index should be nil for single-binding form")
	}

	testIdentifier(t, stmt.Iterable, "arr")

	if len(stmt.Body.Stmts) != 1 {
		t.Fatalf("body should have 1 statement, got %d", len(stmt.Body.Stmts))
	}
}

func TestForRangeWithIndex(t *testing.T) {
	source := "for (i, v in arr) { print(i); print(v); }"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts has wrong length. want=1, got=%d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ForInStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ForRangeStmt. got=%T", program.Stmts[0])
	}

	if stmt.Key.Value != "i" {
		t.Fatalf("stmt.Index wrong. want=i, got=%s", stmt.Key.Value)
	}

	if stmt.Value.Value != "v" {
		t.Fatalf("stmt.Value wrong. want=v, got=%s", stmt.Value.Value)
	}

	testIdentifier(t, stmt.Iterable, "arr")

	if len(stmt.Body.Stmts) != 2 {
		t.Fatalf("body should have 2 statements, got %d", len(stmt.Body.Stmts))
	}
}

func TestForRangeMap(t *testing.T) {
	source := `for (k, v in m) { print(k); }`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts has wrong length. want=1, got=%d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.ForInStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.ForRangeStmt. got=%T", program.Stmts[0])
	}

	if stmt.Key.Value != "k" {
		t.Fatalf("stmt.Index wrong. want=k, got=%s", stmt.Key.Value)
	}

	if stmt.Value.Value != "v" {
		t.Fatalf("stmt.Value wrong. want=v, got=%s", stmt.Value.Value)
	}

	testIdentifier(t, stmt.Iterable, "m")
}

func TestBreakContinueStatements(t *testing.T) {
	source := `for (x < 10) { break; continue; }`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt := program.Stmts[0].(*ast.ForStmt)
	if len(stmt.Body.Stmts) != 2 {
		t.Fatalf("body should have 2 statements, got %d", len(stmt.Body.Stmts))
	}

	if _, ok := stmt.Body.Stmts[0].(*ast.BreakStmt); !ok {
		t.Fatalf("stmt 0 is not *ast.BreakStmt. got=%T", stmt.Body.Stmts[0])
	}
	if _, ok := stmt.Body.Stmts[1].(*ast.ContinueStmt); !ok {
		t.Fatalf("stmt 1 is not *ast.ContinueStmt. got=%T", stmt.Body.Stmts[1])
	}
}

func TestMatchOnScopeAccess(t *testing.T) {
	source := `match foo:bar(x) {
		ok(v) -> { v; },
		err(e) -> { 0; },
	}`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt := program.Stmts[0].(*ast.ExpressionStmt)
	expr, ok := stmt.Expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.MatchExpr. got=%T", stmt.Expr)
	}

	call, ok := expr.Subject.(*ast.CallExpr)
	if !ok {
		t.Fatalf("match subject is not *ast.CallExpr. got=%T", expr.Subject)
	}

	scope, ok := call.Function.(*ast.ScopeAccessExpr)
	if !ok {
		t.Fatalf("call function is not *ast.ScopeAccessExpr. got=%T", call.Function)
	}

	if scope.Module.Value != "foo" {
		t.Fatalf("scope module wrong. want=foo, got=%s", scope.Module.Value)
	}
	if scope.Member.Value != "bar" {
		t.Fatalf("scope member wrong. want=bar, got=%s", scope.Member.Value)
	}
}

func TestGenericFunctionParsing(t *testing.T) {
	source := `func f<T>(T x) { }`
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.FunctionDeclarationStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.FunctionDeclarationStmt, got=%T", program.Stmts[0])
	}

	if len(stmt.TypeParams) != 1 {
		t.Fatalf("len(stmt.TypeParams) is not 1, got=%d", len(stmt.TypeParams))
	}

	tp := stmt.TypeParams[0]
	if tp.Name != "T" {
		t.Fatalf("tp.Name wrong, want T, got=%s", tp.Name)
	}

	if tp.Constraint != nil {
		t.Fatalf("expected no constraint, got=%s", tp.Constraint.Signature())
	}

	// Verify param type is TypeParamRef
	fnType := stmt.Type.(types.FunctionType)
	paramType, ok := fnType.Params[0].(*types.TypeParamRef)
	if !ok {
		t.Fatalf("param type is not TypeParamRef, got=%T", fnType.Params[0])
	}
	if paramType.Name != "T" {
		t.Fatalf("param type name wrong, want T, got=%s", paramType.Name)
	}
}

func TestGenericFunctionMultipleTypeParams(t *testing.T) {
	source := `func f<T, U>(T x, U y) -> T { }`
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.FunctionDeclarationStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.FunctionDeclarationStmt, got=%T", program.Stmts[0])
	}

	if len(stmt.TypeParams) != 2 {
		t.Fatalf("len(stmt.TypeParams) is not 2, got=%d", len(stmt.TypeParams))
	}

	if stmt.TypeParams[0].Name != "T" {
		t.Fatalf("TypeParams[0].Name wrong, want T, got=%s", stmt.TypeParams[0].Name)
	}
	if stmt.TypeParams[1].Name != "U" {
		t.Fatalf("TypeParams[1].Name wrong, want U, got=%s", stmt.TypeParams[1].Name)
	}

	// Verify return type is TypeParamRef
	fnType := stmt.Type.(types.FunctionType)
	retType, ok := fnType.Return.(*types.TypeParamRef)
	if !ok {
		t.Fatalf("return type is not TypeParamRef, got=%T", fnType.Return)
	}
	if retType.Name != "T" {
		t.Fatalf("return type name wrong, want T, got=%s", retType.Name)
	}
}

func TestGenericFunctionWithConstraint(t *testing.T) {
	source := `func f<T: Shape>(T x) { }`
	l := lexer.New(source)
	p := New(l)
	// Register Shape as a known interface so parseType can resolve it
	p.definedInterfaces["Shape"] = types.InterfaceType{Name: "Shape"}
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.FunctionDeclarationStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.FunctionDeclarationStmt, got=%T", program.Stmts[0])
	}

	tp := stmt.TypeParams[0]
	if tp.Name != "T" {
		t.Fatalf("tp.Name wrong, want T, got=%s", tp.Name)
	}
	if tp.Constraint == nil {
		t.Fatalf("expected constraint, got nil")
	}
	if tp.Constraint.Signature() != "Shape" {
		t.Fatalf("constraint wrong, want Shape, got=%s", tp.Constraint.Signature())
	}
}

func TestGenericTypeArgParsing(t *testing.T) {
	source := `
func f<T, U>(T t, U u) -> bool {}
f<int, bool>();`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[1].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Stmts[1] not *ast.ExpressionStmt got=%T", program.Stmts[1])
	}

	expr, ok := stmt.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.CallExpr, got=%T", stmt.Expr)
	}

	// Verify function name
	fn, ok := expr.Function.(*ast.Identifier)
	if !ok {
		t.Fatalf("expr.Function is not *ast.Identifier, got=%T", expr.Function)
	}
	if fn.Value != "f" {
		t.Fatalf("function name wrong, want f, got=%s", fn.Value)
	}

	// Verify type args
	if len(expr.TypeArgs) != 2 {
		t.Fatalf("len(expr.TypeArgs) not 2, got=%d", len(expr.TypeArgs))
	}
	if expr.TypeArgs[0].Signature() != "int" {
		t.Fatalf("TypeArgs[0] wrong, want int, got=%s", expr.TypeArgs[0].Signature())
	}
	if expr.TypeArgs[1].Signature() != "bool" {
		t.Fatalf("TypeArgs[1] wrong, want bool, got=%s", expr.TypeArgs[1].Signature())
	}

	// Verify no value args
	if len(expr.Arguments) != 0 {
		t.Fatalf("len(expr.Arguments) not 0, got=%d", len(expr.Arguments))
	}
}

func TestGenericCallWithArguments(t *testing.T) {
	source := `
func identity<T>(T x) -> T {}
identity<int>(42);`

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[1].(*ast.ExpressionStmt)
	if !ok {
		t.Fatalf("program.Stmts[1] not *ast.ExpressionStmt got=%T", program.Stmts[1])
	}

	expr, ok := stmt.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("stmt.Expr is not *ast.CallExpr, got=%T", stmt.Expr)
	}

	if len(expr.TypeArgs) != 1 {
		t.Fatalf("len(expr.TypeArgs) not 1, got=%d", len(expr.TypeArgs))
	}
	if expr.TypeArgs[0].Signature() != "int" {
		t.Fatalf("TypeArgs[0] wrong, want int, got=%s", expr.TypeArgs[0].Signature())
	}

	if len(expr.Arguments) != 1 {
		t.Fatalf("len(expr.Arguments) not 1, got=%d", len(expr.Arguments))
	}
	testIntegerLiteral(t, expr.Arguments[0], 42)
}

func TestLessThanNotConfusedWithGenericCall(t *testing.T) {
	source := `const x = a < b;`
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.VarDeclarationStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.VarDeclarationStmt, got=%T", program.Stmts[0])
	}

	infix, ok := stmt.Value.(*ast.InfixExpr)
	if !ok {
		t.Fatalf("stmt.Value is not *ast.InfixExpr, got=%T", stmt.Value)
	}
	if infix.Operator != "<" {
		t.Fatalf("operator wrong, want <, got=%s", infix.Operator)
	}
}

func TestGenericStructDefinition(t *testing.T) {
	source := "define struct Pair<T, U> { first T, second U }"
	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	stmt, ok := program.Stmts[0].(*ast.StructDefinitionStmt)
	if !ok {
		t.Fatalf("program.Stmts[1] not *ast.StructDefinitionStmt, got=%T", program.Stmts[1])
	}

	if len(stmt.Type.TypeParams) != 2 {
		t.Fatalf("len(stmt.TypeParams) not 2, got=%d", len(stmt.Type.TypeParams))
	}

	if stmt.Type.TypeParams[0].Signature() != "T" {
		t.Fatalf("TypeParams[0] wrong, want T, got=%s", stmt.Type.TypeParams[0].Signature())
	}
	if stmt.Type.TypeParams[1].Signature() != "U" {
		t.Fatalf("TypeParams[1] wrong, want U, got=%s", stmt.Type.TypeParams[1].Signature())
	}

	tpr, ok := stmt.Type.Types[0].(*types.TypeParamRef)
	if !ok {
		t.Fatalf("stmt.Type.Types[0] is not *types.TypeParamRef, got=%T", stmt.Type.Types[0])
	}

	if tpr.Name != "T" {
		t.Fatalf("tpr.Name not T, got=%s", tpr.Name)
	}

	tpr, ok = stmt.Type.Types[1].(*types.TypeParamRef)
	if !ok {
		t.Fatalf("stmt.Type.Types[1] is not *types.TypeParamRef, got=%T", stmt.Type.Types[1])
	}

	if tpr.Name != "U" {
		t.Fatalf("tpr.Name not U, got=%s", tpr.Name)
	}
}

func TestGenericStructLiteral(t *testing.T) {
	source := "const p = Pair<int, bool> { first: 0, second: true };"
	l := lexer.New(source)
	p := New(l)
	p.definedStructs["Pair"] = types.StructType{Name: "Pair"}
	p.genericNames["Pair"] = true
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Stmts[0].(*ast.VarDeclarationStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.VarDeclarationStmt, got=%T", program.Stmts[0])
	}

	val, ok := stmt.Value.(*ast.StructLiteral)
	if !ok {
		t.Fatalf("stmt.Value is not *ast.StructLiteral, got=%T", stmt.Value)
	}

	if val.Name != "Pair" {
		t.Fatalf("struct name wrong, want Pair, got=%s", val.Name)
	}

	if len(val.TypeArgs) != 2 {
		t.Fatalf("len(val.TypeArgs) not 2, got=%d", len(val.TypeArgs))
	}
	if val.TypeArgs[0].Signature() != "int" {
		t.Fatalf("TypeArgs[0] wrong, want int, got=%s", val.TypeArgs[0].Signature())
	}
	if val.TypeArgs[1].Signature() != "bool" {
		t.Fatalf("TypeArgs[1] wrong, want bool, got=%s", val.TypeArgs[1].Signature())
	}

	if len(val.Fields) != 2 {
		t.Fatalf("len(val.Fields) not 2, got=%d", len(val.Fields))
	}
	if val.Fields[0] != "first" {
		t.Fatalf("Fields[0] wrong, want first, got=%s", val.Fields[0])
	}
	if val.Fields[1] != "second" {
		t.Fatalf("Fields[1] wrong, want second, got=%s", val.Fields[1])
	}

	testIntegerLiteral(t, val.Values[0], 0)
	testBooleanLiteral(t, val.Values[1], true)
}

func TestSliceExpr(t *testing.T) {
	tests := []struct {
		source        string
		expectedStart interface{}
		expectedEnd   interface{}
		expectedLeft  string
	}{
		{
			source:        "a[1:5]",
			expectedStart: int64(1),
			expectedEnd:   int64(5),
			expectedLeft:  "a",
		},
		{
			source:        "a[:5]",
			expectedStart: nil,
			expectedEnd:   int64(5),
			expectedLeft:  "a",
		},
		{
			source:        "a[3:]",
			expectedStart: int64(3),
			expectedEnd:   nil,
			expectedLeft:  "a",
		},
	}
	for _, tt := range tests {
		l := lexer.New(tt.source)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		stmt, ok := program.Stmts[0].(*ast.ExpressionStmt)
		if !ok {
			t.Fatalf("program.Stmts[0] is not *ast.ExpressionStmt, got=%T", program.Stmts[0])
		}

		expr, ok := stmt.Expr.(*ast.SliceExpr)
		if !ok {
			t.Fatalf("stmt.Expr is not *ast.SliceExpr, got=%T", stmt.Expr)
		}

		testIdentifier(t, expr.Left, tt.expectedLeft)

		if expr.Start != nil {
			if tt.expectedStart == nil {
				t.Fatalf("expr.Start is not nil, got=%T", expr.Start)
			}

			i, ok := expr.Start.(*ast.IntegerLiteral)
			if !ok {
				t.Fatalf("expr.Start is not IntegerLiteral, got=%T", expr.Start)
			}

			if i.Value != tt.expectedStart {
				t.Fatalf("expr.Start is not %v, got=%v", tt.expectedStart, i.Value)
			}
		} else if expr.Start == nil && tt.expectedStart != nil {
			t.Fatalf("expr.Start is nil, want=%T", expr.Start)
		}

		if expr.End != nil {
			if tt.expectedEnd == nil {
				t.Fatalf("expr.End is not nil, got=%T", expr.End)
			}

			i, ok := expr.End.(*ast.IntegerLiteral)
			if !ok {
				t.Fatalf("expr.End is not IntegerLiteral, got=%T", expr.End)
			}

			if i.Value != tt.expectedEnd {
				t.Fatalf("expr.End is not %v, got=%v", tt.expectedEnd, i.Value)
			}
		} else if expr.End == nil && tt.expectedEnd != nil {
			t.Fatalf("expr.End is nil, want=%T", expr.End)
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
	case byte:
		return testByteLiteral(t, expr, v)
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

func testByteLiteral(t *testing.T, expr ast.Expr, expected byte) bool {
	byt, ok := expr.(*ast.ByteLiteral)
	if !ok {
		t.Errorf("expr not *ast.ByteLiteral. got=%T", expr)
		return false
	}

	if byt.Value != expected {
		t.Errorf("byt.Value not %d. got=%q", expected, byt.Value)
		return false
	}

	if byt.TokenLiteral() != string(expected) {
		t.Errorf("byt.TokenLiteral not %s. got=%s", string(expected), byt.TokenLiteral())
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

func TestSpawnStmt(t *testing.T) {
	source := "spawn do_work(x, y);"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts has wrong length. want=1, got=%d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.SpawnStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.SpawnStmt. got=%T", program.Stmts[0])
	}

	callExpr, ok := stmt.CallExpr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("stmt.CallExpr is not *ast.CallExpr. got=%T", stmt.CallExpr)
	}

	testIdentifier(t, callExpr.Function, "do_work")

	if len(callExpr.Arguments) != 2 {
		t.Fatalf("wrong number of arguments. want=2, got=%d", len(callExpr.Arguments))
	}

	testIdentifier(t, callExpr.Arguments[0], "x")
	testIdentifier(t, callExpr.Arguments[1], "y")
}

func TestSpawnStmtNoArgs(t *testing.T) {
	source := "spawn do_work();"

	l := lexer.New(source)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Stmts) != 1 {
		t.Fatalf("program.Stmts has wrong length. want=1, got=%d", len(program.Stmts))
	}

	stmt, ok := program.Stmts[0].(*ast.SpawnStmt)
	if !ok {
		t.Fatalf("program.Stmts[0] is not *ast.SpawnStmt. got=%T", program.Stmts[0])
	}

	callExpr, ok := stmt.CallExpr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("stmt.CallExpr is not *ast.CallExpr. got=%T", stmt.CallExpr)
	}

	testIdentifier(t, callExpr.Function, "do_work")

	if len(callExpr.Arguments) != 0 {
		t.Fatalf("wrong number of arguments. want=0, got=%d", len(callExpr.Arguments))
	}
}

func testMatchArm(t *testing.T, arm *ast.MatchArm, binding string, isOk bool) bool {
	if arm.Pattern.IsOk != isOk {
		t.Errorf("arm.Pattern.IsOk wrong, want %t, got %t", isOk, arm.Pattern.IsOk)
		return false
	}
	if arm.Pattern.Binding.Value != binding {
		t.Errorf("arm.Pattern.Binding wrong, want %s, got %s", binding, arm.Pattern.Binding.Value)
		return false
	}
	return true
}
