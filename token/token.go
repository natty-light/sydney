package token

type TokenType string

const (
	// Literals
	Null       TokenType = "Null"
	Identifier TokenType = "Identifier"
	Integer    TokenType = "Number"
	String     TokenType = "String"
	Float      TokenType = "Float"

	// Keywords
	Mut            TokenType = "Mut"
	Const          TokenType = "Const"
	True           TokenType = "True"
	False          TokenType = "False"
	If             TokenType = "If"
	Else           TokenType = "Else"
	Elseif         TokenType = "Elseif"
	Func           TokenType = "Func"
	Return         TokenType = "Return"
	For            TokenType = "For"
	Macro          TokenType = "Macro"
	Define         TokenType = "Define"
	Struct         TokenType = "Struct"
	Interface      TokenType = "Interface"
	Implementation TokenType = "Implementation"

	// Types
	IntType    TokenType = "IntegerType"
	StringType TokenType = "StringType"
	FloatType  TokenType = "FloatType"
	BoolType   TokenType = "BoolType"
	//NullType     TokenType = "NullType"
	ArrayType    TokenType = "ArrayType"
	MapType      TokenType = "MapType"
	FunctionType TokenType = "FunctionType"

	// Grouping
	LeftParen          TokenType = "LeftParen"
	RightParen         TokenType = "RightParen"
	LeftCurlyBracket   TokenType = "LeftCurlyBracket"
	RightCurlyBracket  TokenType = "RightCurlyBracket"
	LeftSquareBracket  TokenType = "LeftSquareBracket"
	RightSquareBracket TokenType = "RightSquareBracket"
	Semicolon          TokenType = "Semicolon"
	Comma              TokenType = "Comma"
	Colon              TokenType = "Colon"
	Dot                TokenType = "Dot"

	// Symbols
	Plus        TokenType = "Plus"
	Minus       TokenType = "Minus"
	Slash       TokenType = "Slash"
	Star        TokenType = "Star"
	Modulo      TokenType = "Modulus"
	Assign      TokenType = "Assign"
	GreaterThan TokenType = "GreaterThan"
	LessThan    TokenType = "LessThan"
	Bang        TokenType = "Bang"

	// Multi char symbols
	EqualTo            TokenType = "Equality"
	GreaterThanEqualTo TokenType = "GreaterThanEqualTo"
	LessThanEqualTo    TokenType = "LessThanEqualTo"
	NotEqualTo         TokenType = "NotEqual"
	And                TokenType = "And"
	Or                 TokenType = "Or"
	Arrow              TokenType = "Arrow"

	EOF     TokenType = "EOF" // End of File
	Illegal TokenType = "Illegal"
)

type Token struct {
	Literal string
	Type    TokenType
}

func MakeToken(Type TokenType, char byte) Token {
	return Token{Type: Type, Literal: string(char)}
}
