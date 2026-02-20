package lexer

import (
	"sydney/token"
	"testing"
)

func TestNextToken(t *testing.T) {
	source := `mut five = 5;
	mut ten = 10;
	mut add = func (x, y) {
		 x + y;
	};
	mut result = add(five, ten);
	!-/*5;
	5 < 10 > 5;
	if (5 < 10) {
		return true;
	} else {
		return false;
	}
	10 == 10;
	10 != 9;
	5 >= 10;
	7 <= 6;
	true && false;
	true || false;
	"foobar";
	"foo bar";
	[1, 2];
	for (x < 10) { x }
	{"foo": "bar" }
	5.2;
	macro(x, y) { x + y; };
	define struct Person { age int, name string }
	define interface Pointer { getX() -> int, setX(int x) }
	define implementation Point -> Pointer
	`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.Mut, "mut"},
		{token.Identifier, "five"},
		{token.Assign, "="},
		{token.Integer, "5"},
		{token.Semicolon, ";"},

		{token.Mut, "mut"},
		{token.Identifier, "ten"},
		{token.Assign, "="},
		{token.Integer, "10"},
		{token.Semicolon, ";"},

		{token.Mut, "mut"},
		{token.Identifier, "add"},
		{token.Assign, "="},
		{token.Func, "func"},
		{token.LeftParen, "("},
		{token.Identifier, "x"},
		{token.Comma, ","},
		{token.Identifier, "y"},
		{token.RightParen, ")"},
		{token.LeftCurlyBracket, "{"},

		{token.Identifier, "x"},
		{token.Plus, "+"},
		{token.Identifier, "y"},
		{token.Semicolon, ";"},

		{token.RightCurlyBracket, "}"},
		{token.Semicolon, ";"},

		{token.Mut, "mut"},
		{token.Identifier, "result"},
		{token.Assign, "="},
		{token.Identifier, "add"},
		{token.LeftParen, "("},
		{token.Identifier, "five"},
		{token.Comma, ","},
		{token.Identifier, "ten"},
		{token.RightParen, ")"},
		{token.Semicolon, ";"},

		{token.Bang, "!"},
		{token.Minus, "-"},
		{token.Slash, "/"},
		{token.Star, "*"},
		{token.Integer, "5"},
		{token.Semicolon, ";"},

		{token.Integer, "5"},
		{token.LessThan, "<"},
		{token.Integer, "10"},
		{token.GreaterThan, ">"},
		{token.Integer, "5"},
		{token.Semicolon, ";"},

		{token.If, "if"},
		{token.LeftParen, "("},
		{token.Integer, "5"},
		{token.LessThan, "<"},
		{token.Integer, "10"},
		{token.RightParen, ")"},
		{token.LeftCurlyBracket, "{"},

		{token.Return, "return"},
		{token.True, "true"},
		{token.Semicolon, ";"},

		{token.RightCurlyBracket, "}"},
		{token.Else, "else"},
		{token.LeftCurlyBracket, "{"},

		{token.Return, "return"},
		{token.False, "false"},
		{token.Semicolon, ";"},

		{token.RightCurlyBracket, "}"},

		{token.Integer, "10"},
		{token.EqualTo, "=="},
		{token.Integer, "10"},
		{token.Semicolon, ";"},

		{token.Integer, "10"},
		{token.NotEqualTo, "!="},
		{token.Integer, "9"},
		{token.Semicolon, ";"},

		{token.Integer, "5"},
		{token.GreaterThanEqualTo, ">="},
		{token.Integer, "10"},
		{token.Semicolon, ";"},

		{token.Integer, "7"},
		{token.LessThanEqualTo, "<="},
		{token.Integer, "6"},
		{token.Semicolon, ";"},

		{token.True, "true"},
		{token.And, "&&"},
		{token.False, "false"},
		{token.Semicolon, ";"},

		{token.True, "true"},
		{token.Or, "||"},
		{token.False, "false"},
		{token.Semicolon, ";"},

		{token.String, "foobar"},
		{token.Semicolon, ";"},

		{token.String, "foo bar"},
		{token.Semicolon, ";"},

		{token.LeftSquareBracket, "["},
		{token.Integer, "1"},
		{token.Comma, ","},
		{token.Integer, "2"},
		{token.RightSquareBracket, "]"},
		{token.Semicolon, ";"},

		{token.For, "for"},
		{token.LeftParen, "("},
		{token.Identifier, "x"},
		{token.LessThan, "<"},
		{token.Integer, "10"},
		{token.RightParen, ")"},
		{token.LeftCurlyBracket, "{"},
		{token.Identifier, "x"},
		{token.RightCurlyBracket, "}"},

		{token.LeftCurlyBracket, "{"},
		{token.String, "foo"},
		{token.Colon, ":"},
		{token.String, "bar"},
		{token.RightCurlyBracket, "}"},

		{token.Float, "5.2"},
		{token.Semicolon, ";"},

		{token.Macro, "macro"},
		{token.LeftParen, "("},
		{token.Identifier, "x"},
		{token.Comma, ","},
		{token.Identifier, "y"},
		{token.RightParen, ")"},
		{token.LeftCurlyBracket, "{"},
		{token.Identifier, "x"},
		{token.Plus, "+"},
		{token.Identifier, "y"},
		{token.Semicolon, ";"},
		{token.RightCurlyBracket, "}"},
		{token.Semicolon, ";"},

		{token.Define, "define"},
		{token.Struct, "struct"},
		{token.Identifier, "Person"},
		{token.LeftCurlyBracket, "{"},
		{token.Identifier, "age"},
		{token.IntType, "int"},
		{token.Comma, ","},
		{token.Identifier, "name"},
		{token.StringType, "string"},
		{token.RightCurlyBracket, "}"},

		{token.Define, "define"},
		{token.Interface, "interface"},
		{token.Identifier, "Pointer"},
		{token.LeftCurlyBracket, "{"},
		{token.Identifier, "getX"},
		{token.LeftParen, "("},
		{token.RightParen, ")"},
		{token.Arrow, "->"},
		{token.IntType, "int"},
		{token.Comma, ","},
		{token.Identifier, "setX"},
		{token.LeftParen, "("},
		{token.IntType, "int"},
		{token.Identifier, "x"},
		{token.RightParen, ")"},
		{token.RightCurlyBracket, "}"},

		{token.Define, "define"},
		{token.Implementation, "implementation"},
		{token.Identifier, "Point"},
		{token.Arrow, "->"},
		{token.Identifier, "Pointer"},

		{token.EOF, ""},
	}

	lexer := New(source)

	for i, tt := range tests {
		tok := lexer.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestTypeLexing(t *testing.T) {
	source := `
	mut int x = 5;
	mut bool x;
	mut string x = "foo";
	mut map<int, string> m;
	const array<int> a;
	const fn<(int) -> int> f;
	`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.Mut, "mut"},
		{token.IntType, "int"},
		{token.Identifier, "x"},
		{token.Assign, "="},
		{token.Integer, "5"},
		{token.Semicolon, ";"},

		{token.Mut, "mut"},
		{token.BoolType, "bool"},
		{token.Identifier, "x"},
		{token.Semicolon, ";"},

		{token.Mut, "mut"},
		{token.StringType, "string"},
		{token.Identifier, "x"},
		{token.Assign, "="},
		{token.String, "foo"},
		{token.Semicolon, ";"},

		{token.Mut, "mut"},
		{token.MapType, "map"},
		{token.LessThan, "<"},
		{token.IntType, "int"},
		{token.Comma, ","},
		{token.StringType, "string"},
		{token.GreaterThan, ">"},
		{token.Identifier, "m"},
		{token.Semicolon, ";"},

		{token.Const, "const"},
		{token.ArrayType, "array"},
		{token.LessThan, "<"},
		{token.IntType, "int"},
		{token.GreaterThan, ">"},
		{token.Identifier, "a"},
		{token.Semicolon, ";"},

		{token.Const, "const"},
		{token.FunctionType, "fn"},
		{token.LessThan, "<"},
		{token.LeftParen, "("},
		{token.IntType, "int"},
		{token.RightParen, ")"},
		{token.Arrow, "->"},
		{token.IntType, "int"},
		{token.GreaterThan, ">"},
		{token.Identifier, "f"},
		{token.Semicolon, ";"},

		{token.EOF, ""},
	}
	lexer := New(source)

	for i, tt := range tests {
		tok := lexer.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}
