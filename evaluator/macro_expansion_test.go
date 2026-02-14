package evaluator

import (
	"sydney/ast"
	"sydney/lexer"
	"sydney/object"
	"sydney/parser"
	"testing"
)

func TestExpandMacros(t *testing.T) {
	tests := []struct {
		source   string
		expected string
	}{
		{
			`mut infixExpr = macro() { quote(1 + 2); };
				    infixExpr();`,
			`(1 + 2)`,
		},
		{
			`mut reverse = macro(a, b) { quote(unquote(b) - unquote(a)); };
				    reverse(2 + 2, 10 - 5);`,
			`(10 - 5) - (2 + 2)`,
		},
		{
			`mut unless = macro(condition, consequence, alternative) {
						quote( if (!(unquote(condition))) {
							unquote(consequence);
						} else {
							unquote(alternative);
						});
					}
					unless(10 > 5, print("not greater"), print("greater"));`,
			`if (!(10 > 5)) { print("not greater") } else { print("greater") }`,
		},
	}

	for _, tt := range tests {
		expected := testParseProgram(tt.expected)
		program := testParseProgram(tt.source)

		scope := object.NewScope()
		DefineMacros(program, scope)
		expanded := ExpandMacros(program, scope)

		if expanded.String() != expected.String() {
			t.Errorf("not equal. got=%q, want=%q", expanded.String(), expected.String())
		}
	}
}

func testParseProgram(input string) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)
	return p.ParseProgram()
}
