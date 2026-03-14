package lexer

import (
	"sydney/token"
	"sydney/utils"
)

const (
	eqSym = '='

	leftParen          = '('
	rightParen         = ')'
	leftSquareBracket  = '['
	rightSquareBracket = ']'
	leftCurlyBracket   = '{'
	rightCurlyBracket  = '}'

	semi        = ';'
	comma       = ','
	colon       = ':'
	dot         = '.'
	quote       = '"'
	singleQuote = '\''
	backSlash   = '\\'

	plus   = '+'
	star   = '*'
	slash  = '/'
	minus  = '-'
	modulo = '%'

	greaterThan = '>'
	lessThan    = '<'
	bang        = '!'
	ampersand   = '&'
	pipe        = '|'
)

type Lexer struct {
	source       string
	position     int
	readPosition int
	char         byte

	line   int
	column int
}

func New(source string) *Lexer {
	lexer := &Lexer{source: source, line: 1}
	lexer.readChar() // set up lexer
	return lexer
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.source) {
		l.char = 0
	} else {
		l.char = l.source[l.readPosition]
	}

	l.position = l.readPosition
	l.readPosition += 1

	if l.char == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column += 1
	}
}

func (l *Lexer) readIdentifer() string {
	position := l.position

	for utils.IsAlpha(string(l.char)) {
		l.readChar() // Advances the position pointer
	}
	return l.source[position:l.position]
}

func (l *Lexer) readNumber() (string, bool) {
	position := l.position
	encounteredDecimal := false
	for utils.IsNumeric(string(l.char)) || (!encounteredDecimal && l.char == dot) {
		if l.char == dot {
			encounteredDecimal = true
		}
		l.readChar() // This just advances the position pointer
	}
	return l.source[position:l.position], encounteredDecimal
}

func (l *Lexer) readString() string {
	var result []byte
	for {
		l.readChar()
		if l.char == quote || l.char == 0 {
			break
		}

		if l.char == '\\' {
			l.readChar()
			switch l.char {
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case 'r':
				result = append(result, '\r')
			case '\\':
				result = append(result, '\\')
			case '"':
				result = append(result, '"')
			case '0':
				result = append(result, 0)
			default:
				result = append(result, '\\', l.char)
			}
		} else {
			result = append(result, l.char)
		}
	}
	return string(result)
}

func (l *Lexer) skipWhitespace() {
	for l.char == ' ' || l.char == '\t' || l.char == '\n' || l.char == '\r' {
		l.readChar()
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.source) {
		return 0
	} else {
		return l.source[l.readPosition]
	}
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token
	l.skipWhitespace()
	tok.Line = l.line
	tok.Column = l.column
	switch l.char {
	// grouping
	case leftParen:
		tok = l.makeToken(token.LeftParen, l.char)
	case rightParen:
		tok = l.makeToken(token.RightParen, l.char)
	case leftCurlyBracket:
		tok = l.makeToken(token.LeftCurlyBracket, l.char)
	case rightCurlyBracket:
		tok = l.makeToken(token.RightCurlyBracket, l.char)
	case leftSquareBracket:
		tok = l.makeToken(token.LeftSquareBracket, l.char)
	case rightSquareBracket:
		tok = l.makeToken(token.RightSquareBracket, l.char)

	// Punctuation
	case semi:
		tok = l.makeToken(token.Semicolon, l.char)
	case comma:
		tok = l.makeToken(token.Comma, l.char)
	case colon:
		tok = l.makeToken(token.Colon, l.char)
	case dot:
		tok = l.makeToken(token.Dot, l.char)
	case quote:
		tok.Type = token.String
		tok.Literal = l.readString()
	case singleQuote:
		l.readChar()
		if l.char == backSlash {
			l.readChar() // advance past \\
			switch l.char {
			case 'n':
				tok = l.makeToken(token.Byte, '\n')
			case 't':
				tok = l.makeToken(token.Byte, '\t')
			case 'r':
				tok = l.makeToken(token.Byte, '\r')
			case backSlash:
				tok = l.makeToken(token.Byte, l.char)
			case '\'':
				tok = l.makeToken(token.Byte, '\'')
			case '0':
				tok = l.makeToken(token.Byte, '0')
			default:
				tok = l.makeToken(token.Byte, l.char)
			}
		} else {
			tok = l.makeToken(token.Byte, l.char)
		}
		l.readChar()
	// Symbols
	case eqSym:
		if l.peekChar() == eqSym {
			char := l.char
			l.readChar() // advance past first equals
			literal := string(char) + string(l.char)
			tok = token.Token{Type: token.EqualTo, Literal: literal}
		} else {
			tok = l.makeToken(token.Assign, l.char)
		}
	case plus:
		tok = l.makeToken(token.Plus, l.char)
	case minus:
		if l.peekChar() == greaterThan {
			char := l.char
			l.readChar()
			literal := string(char) + string(l.char)
			tok = token.Token{Type: token.Arrow, Literal: literal}
		} else {
			tok = l.makeToken(token.Minus, l.char)
		}
	case star:
		tok = l.makeToken(token.Star, l.char)
	case slash:
		if l.peekChar() == slash {
			l.readChar()
			for l.char != '\n' && l.char != 0 {
				l.readChar()
			}
			return l.NextToken()
		}
		tok = l.makeToken(token.Slash, l.char)
	case modulo:
		tok = l.makeToken(token.Modulo, l.char)
	case greaterThan:
		if l.peekChar() == eqSym {
			char := l.char
			l.readChar() // advance past first equals
			literal := string(char) + string(l.char)
			tok = token.Token{Type: token.GreaterThanEqualTo, Literal: literal}
		} else {
			tok = l.makeToken(token.GreaterThan, l.char)
		}
	case lessThan:
		if l.peekChar() == eqSym {
			char := l.char
			l.readChar() // advance past first equals
			literal := string(char) + string(l.char)
			tok = token.Token{Type: token.LessThanEqualTo, Literal: literal}
		} else {
			tok = l.makeToken(token.LessThan, l.char)
		}
	case bang:
		if l.peekChar() == eqSym {
			char := l.char
			l.readChar() // advance past first equals
			literal := string(char) + string(l.char)
			tok = token.Token{Type: token.NotEqualTo, Literal: literal}
		} else {
			tok = l.makeToken(token.Bang, l.char)
		}
	case ampersand:
		if l.peekChar() == ampersand {
			char := l.char
			l.readChar()
			literal := string(char) + string(l.char)
			tok = token.Token{Type: token.And, Literal: literal}
		} else {
			// Single & is an illegal char
			tok = l.makeToken(token.Illegal, l.char)
		}
	case pipe:
		if l.peekChar() == pipe {
			char := l.char
			l.readChar()
			literal := string(char) + string(l.char)
			tok = l.makeTokenStr(token.Or, literal)
		} else {
			// Single & is an illegal char
			tok = l.makeToken(token.Illegal, l.char)
		}
	case 0:
		tok.Literal = ""
		tok.Type = "EOF"

	default:
		if utils.IsAlpha(string(l.char)) {
			tok.Literal = l.readIdentifer()
			tok.Type = LookupIdent(tok.Literal)
			tok.Line = l.line
			tok.Column = l.column
			return tok // This is to avoid the l.readChar() call before this functions return
		} else if utils.IsNumeric(string(l.char)) {

			literal, decimal := l.readNumber()
			if decimal {
				tok.Type = token.Float
			} else {
				tok.Type = token.Integer
			}
			tok.Literal = literal
			tok.Line = l.line
			tok.Column = l.column
			return tok // This is to avoid the l.readChar() call before this functions return
		} else {
			tok = l.makeToken(token.Illegal, l.char)
		}
	}

	l.readChar()
	return tok
}

var keywords = map[string]token.TokenType{
	"mut":            token.Mut,
	"const":          token.Const,
	"null":           token.Null,
	"true":           token.True,
	"false":          token.False,
	"if":             token.If,
	"else":           token.Else,
	"func":           token.Func,
	"return":         token.Return,
	"for":            token.For,
	"macro":          token.Macro,
	"array":          token.ArrayType,
	"map":            token.MapType,
	"struct":         token.Struct,
	"define":         token.Define,
	"interface":      token.Interface,
	"implementation": token.Implementation,
	"pub":            token.Public,
	"module":         token.Module,
	"import":         token.Import,
	"match":          token.Match,
	"extern":         token.Extern,
	"break":          token.Break,
	"continue":       token.Continue,
}

var types = map[string]token.TokenType{
	"string": token.StringType,
	"int":    token.IntType,
	"bool":   token.BoolType,
	"float":  token.FloatType,
	"array":  token.ArrayType,
	"map":    token.MapType,
	"fn":     token.FunctionType,
	"result": token.ResultType,
	"byte":   token.ByteType,
}

func LookupIdent(ident string) token.TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}

	if tok, ok := types[ident]; ok {
		return tok
	}

	return token.Identifier
}

func (l *Lexer) makeToken(t token.TokenType, char byte) token.Token {
	tok := token.MakeToken(t, char)
	tok.Line = l.line
	tok.Column = l.column
	return tok
}

func (l *Lexer) makeTokenStr(t token.TokenType, char string) token.Token {
	tok := token.Token{Type: t, Literal: char}
	tok.Line = l.line
	tok.Column = l.column
	return tok
}
