package parser

import (
	"fmt"
	"strconv"
	"sydney/ast"
	"sydney/lexer"
	"sydney/token"
	"sydney/types"
)

type (
	prefixParseFn func() ast.Expr
	infixParseFn  func(ast.Expr) ast.Expr
)

type Precedence int

const (
	LOWEST Precedence = iota + 1
	ANDOR             // I think this is right
	EQUALS
	LESSGREATEREQUAL
	LESSGREATER
	SUM
	PRODUCT
	PREFIX
	CALL
	INDEX
)

var precedences = map[token.TokenType]Precedence{
	token.And:                ANDOR,
	token.Or:                 ANDOR,
	token.EqualTo:            EQUALS,
	token.NotEqualTo:         EQUALS,
	token.LessThan:           LESSGREATER,
	token.GreaterThan:        LESSGREATER,
	token.GreaterThanEqualTo: LESSGREATEREQUAL,
	token.LessThanEqualTo:    LESSGREATEREQUAL,
	token.Plus:               SUM,
	token.Minus:              SUM,
	token.Slash:              PRODUCT,
	token.Star:               PRODUCT,
	token.Modulo:             PRODUCT,
	token.LeftParen:          CALL,
	token.LeftSquareBracket:  INDEX,
}

type Parser struct {
	lexer *lexer.Lexer

	currToken token.Token
	peekToken token.Token

	errors []string

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{lexer: l, errors: make([]string, 0)}

	// peekToken and currToken are initialized to the zero value of token.Token, so we advance twice
	p.nextToken() // set peek
	p.nextToken() // set curr and peek

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)

	p.registerPrefix(token.Identifier, p.parseIdentifier)
	p.registerPrefix(token.Integer, p.parseIntegerLiteral)
	p.registerPrefix(token.Bang, p.parsePrefixExpr)
	p.registerPrefix(token.Minus, p.parsePrefixExpr)
	p.registerPrefix(token.True, p.parseBooleanLiteral)
	p.registerPrefix(token.False, p.parseBooleanLiteral)
	p.registerPrefix(token.LeftParen, p.parseGroupedExpr)
	p.registerPrefix(token.If, p.parseIfExpr)
	p.registerPrefix(token.Func, p.parseFunctionLiteral)
	p.registerPrefix(token.String, p.parseStringLiteral)
	p.registerPrefix(token.LeftSquareBracket, p.parseArrayLiteral)
	p.registerPrefix(token.Null, p.parseNullLiteral)
	p.registerPrefix(token.LeftCurlyBracket, p.parseHashLiteral)
	p.registerPrefix(token.Float, p.parseFloatLiteral)
	p.registerPrefix(token.Macro, p.parseMacroLiteral)

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.Plus, p.parseInfixExpr)
	p.registerInfix(token.Minus, p.parseInfixExpr)
	p.registerInfix(token.Slash, p.parseInfixExpr)
	p.registerInfix(token.Star, p.parseInfixExpr)
	p.registerInfix(token.Modulo, p.parseInfixExpr)
	p.registerInfix(token.EqualTo, p.parseInfixExpr)
	p.registerInfix(token.NotEqualTo, p.parseInfixExpr)
	p.registerInfix(token.GreaterThanEqualTo, p.parseInfixExpr)
	p.registerInfix(token.LessThanEqualTo, p.parseInfixExpr)
	p.registerInfix(token.GreaterThan, p.parseInfixExpr)
	p.registerInfix(token.LessThan, p.parseInfixExpr)
	p.registerInfix(token.And, p.parseInfixExpr)
	p.registerInfix(token.Or, p.parseInfixExpr)
	p.registerInfix(token.LeftParen, p.parseCallExpr)
	p.registerInfix(token.LeftSquareBracket, p.parseIndexExpr)
	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

// advances current and peek by one
func (p *Parser) nextToken() {
	p.currToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

// Checks whether current token matches given type
func (p *Parser) currTokenIs(t token.TokenType) bool {
	return p.currToken.Type == t
}

// checks whether peek token matches given type
func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

// Checks if peek token matches given type, advances tokens if true
func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken() // eats
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead",
		t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

func (p *Parser) peekPrecedence() Precedence {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) currPrecedence() Precedence {
	if p, ok := precedences[p.currToken.Type]; ok {
		return p
	}
	return LOWEST
}

// Parsing methods
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Stmts = make([]ast.Stmt, 0)

	for !p.currTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Stmts = append(program.Stmts, stmt)
		}
		p.nextToken() // advance past semis?
	}
	return program
}

// Statements
func (p *Parser) parseStatement() ast.Stmt {
	switch p.currToken.Type {
	case token.Mut:
		fallthrough
	case token.Const:
		return p.parseVarDeclarationStmt()
	case token.Return:
		return p.parseReturnStmt()
	case token.Identifier:
		if p.peekTokenIs(token.Assign) {
			return p.parseAssignmentStmt()
		} else {
			return p.parseExpressionStmt()
		}
	case token.For:
		return p.parseForStmt()
	default:
		return p.parseExpressionStmt()
	}
}

func (p *Parser) parseVarDeclarationStmt() *ast.VarDeclarationStmt {

	// To be here, currToken is either Mut or Const
	isConst := p.currToken.Type == token.Const

	stmt := &ast.VarDeclarationStmt{Token: p.currToken, Constant: isConst}

	// parse type. constant variables do not need a type annotation, mutable variables that are uninitialized do
	if p.isPeekTokenType() {
		stmt.Type = p.parseType()
	}

	// expectPeek eats?
	if !p.expectPeek(token.Identifier) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if p.peekTokenIs(token.Semicolon) {
		if isConst {
			p.errors = append(p.errors, "const variable must be initialized")
			return nil
		} else if stmt.Type == nil {
			p.errors = append(p.errors, "uninitialized mut variable must have a type")
			return nil
		} else {
			p.nextToken() // advance past ;
			return stmt
		}
	}

	if !p.expectPeek(token.Assign) {
		return nil
	}

	p.nextToken() // advance past =
	stmt.Value = p.parseExpression(LOWEST)

	if fl, ok := stmt.Value.(*ast.FunctionLiteral); ok {
		fl.Name = stmt.Name.Value
	}

	if p.peekTokenIs(token.Semicolon) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseReturnStmt() *ast.ReturnStmt {
	stmt := &ast.ReturnStmt{Token: p.currToken}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.Semicolon) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpressionStmt() *ast.ExpressionStmt {
	stmt := &ast.ExpressionStmt{Token: p.currToken}

	stmt.Expr = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.Semicolon) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseBlockStmt() *ast.BlockStmt {
	block := &ast.BlockStmt{Token: p.currToken}
	block.Stmts = make([]ast.Stmt, 0)

	p.nextToken() // advance past {

	for !p.currTokenIs(token.RightCurlyBracket) && !p.currTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
		p.nextToken() // what is this advance for
	}
	return block
}

func (p *Parser) parseAssignmentStmt() ast.Stmt {

	ident := &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if !p.expectPeek(token.Assign) {
		return nil
	}
	p.nextToken() // advance past =

	stmt := &ast.VarAssignmentStmt{Identifier: ident, Token: ident.Token}
	val := p.parseExpression(LOWEST) // Maybe this should be LOWEST, but since the only thing lower than ASSIGNMENT is LOWEST i think we are ok
	stmt.Value = val

	if !p.expectPeek(token.Semicolon) {
		return nil
	}

	return stmt
}

// Expressions
func (p *Parser) parseExpression(precedence Precedence) ast.Expr {
	prefix := p.prefixParseFns[p.currToken.Type] // look for prefix function for p.currToken
	if prefix == nil {
		p.noPrefixParseFnError(p.currToken.Type)
		return nil
	}

	left := prefix() // call prefix function

	// if the statement has not ended and the passed in precedence is lower than the precedence of the next token
	// if the precedence of the next token is higher, then we need to parse it as an infix expression because it is higher priority
	// otherwise we return the expression as parsed by the prefix
	for !p.peekTokenIs(token.Semicolon) && precedence < p.peekPrecedence() {
		// look for an infix parse fn
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return left
		}

		p.nextToken()

		// we bind left to the infix expression
		left = infix(left)
	}

	return left
}

// prefix and infix functions

// this is an prefixParseFn, so it will not call p.nextToken() at the end
func (p *Parser) parseIdentifier() ast.Expr {
	return &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
}

// this is an prefixParseFn, so it will not call p.nextToken() at the end
func (p *Parser) parseIntegerLiteral() ast.Expr {
	literal := &ast.IntegerLiteral{Token: p.currToken}

	value, err := strconv.ParseInt(p.currToken.Literal, 0, 64)

	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.currToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	literal.Value = value

	return literal
}

// this is an prefixParseFn, so it will not call p.nextToken() at the end
func (p *Parser) parsePrefixExpr() ast.Expr {
	expr := &ast.PrefixExpr{Token: p.currToken, Operator: p.currToken.Literal}

	p.nextToken() // advance past operator
	expr.Right = p.parseExpression(PREFIX)

	return expr
}

// this is an infixParseFn, so it will not call p.nextToken() at the end
func (p *Parser) parseInfixExpr(left ast.Expr) ast.Expr {
	expr := &ast.InfixExpr{Token: p.currToken, Operator: p.currToken.Literal, Left: left}

	precedence := p.currPrecedence()
	p.nextToken()
	expr.Right = p.parseExpression(precedence)

	return expr
}

func (p *Parser) parseBooleanLiteral() ast.Expr {
	return &ast.BooleanLiteral{Token: p.currToken, Value: p.currTokenIs(token.True)}
}

// this is a prefixParseFn
func (p *Parser) parseGroupedExpr() ast.Expr {
	p.nextToken() // advance past (

	expr := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RightParen) {
		return nil
	}

	return expr
}

func (p *Parser) parseIfExpr() ast.Expr {
	expr := &ast.IfExpr{Token: p.currToken}

	if !p.expectPeek(token.LeftParen) {
		return nil
	}
	p.nextToken() // advance past (
	expr.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RightParen) {
		return nil
	}
	if !p.expectPeek(token.LeftCurlyBracket) {
		return nil
	}

	expr.Consequence = p.parseBlockStmt()

	// For else if, probably want to set the alternative to another ast.IfExpr, maybe by calling this method recursively
	if p.peekTokenIs(token.Else) {
		p.nextToken() // advance past else
		if !p.expectPeek(token.LeftCurlyBracket) {
			return nil
		}

		expr.Alternative = p.parseBlockStmt()
	}

	return expr
}

func (p *Parser) parseFunctionLiteral() ast.Expr {
	function := &ast.FunctionLiteral{Token: p.currToken}

	if !p.expectPeek(token.LeftParen) {
		return nil
	}

	function.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.LeftCurlyBracket) {
		return nil
	}

	function.Body = p.parseBlockStmt()

	return function
}

func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	idents := make([]*ast.Identifier, 0)

	if p.peekTokenIs(token.RightParen) {
		p.nextToken()
		return idents
	}

	p.nextToken()

	ident := &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
	idents = append(idents, ident)

	// This loop will start with currToken equal to an ident
	for p.peekTokenIs(token.Comma) {
		p.nextToken() // advance comma to currToken
		p.nextToken() // advance past comma to next ident
		ident := &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
		idents = append(idents, ident)
	}

	if !p.expectPeek(token.RightParen) {
		return nil
	}

	return idents
}

func (p *Parser) parseCallExpr(function ast.Expr) ast.Expr {
	expr := &ast.CallExpr{Token: p.currToken, Function: function}
	expr.Arguments = p.parseExpressionList(token.RightParen)
	return expr
}

func (p *Parser) parseCallArguments() []ast.Expr {
	args := make([]ast.Expr, 0)

	if p.peekTokenIs(token.RightParen) {
		p.nextToken()
		return args
	}

	p.nextToken()                                  // advance past openParen
	args = append(args, p.parseExpression(LOWEST)) // parse first arg

	for p.peekTokenIs(token.Comma) {
		p.nextToken() // advance comma into currToken
		p.nextToken() // advance past comma
		args = append(args, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(token.RightParen) {
		return nil
	}

	return args
}

func (p *Parser) parseStringLiteral() ast.Expr {
	return &ast.StringLiteral{Token: p.currToken, Value: p.currToken.Literal}
}

func (p *Parser) parseArrayLiteral() ast.Expr {
	array := &ast.ArrayLiteral{Token: p.currToken}

	array.Elements = p.parseExpressionList(token.RightSquareBracket)

	return array
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expr {
	list := []ast.Expr{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}
	p.nextToken() // advance past opening toke

	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.Comma) {
		p.nextToken() // advance to commma
		p.nextToken() // advance past comma
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseIndexExpr(left ast.Expr) ast.Expr {
	expr := &ast.IndexExpr{Token: p.currToken, Left: left}

	p.nextToken() // advance past [

	expr.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RightSquareBracket) {
		return nil
	}

	return expr
}

func (p *Parser) parseNullLiteral() ast.Expr {
	return &ast.NullLiteral{Token: p.currToken}
}

func (p *Parser) parseForStmt() *ast.ForStmt {
	forStmt := &ast.ForStmt{Token: p.currToken}
	if !p.expectPeek(token.LeftParen) {
		return nil
	}
	p.nextToken() // advance past for

	forStmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RightParen) {
		return nil
	}
	if !p.expectPeek(token.LeftCurlyBracket) {
		return nil
	}

	forStmt.Body = p.parseBlockStmt()

	return forStmt
}

func (p *Parser) parseHashLiteral() ast.Expr {
	hash := &ast.HashLiteral{Token: p.currToken}
	hash.Pairs = make(map[ast.Expr]ast.Expr)

	for !p.peekTokenIs(token.RightCurlyBracket) {
		p.nextToken()
		key := p.parseExpression(LOWEST)
		if !p.expectPeek(token.Colon) {
			return nil
		}
		p.nextToken()
		val := p.parseExpression(LOWEST)

		hash.Pairs[key] = val

		if !p.peekTokenIs(token.RightCurlyBracket) && !p.expectPeek(token.Comma) {
			return nil
		}
	}

	if !p.expectPeek(token.RightCurlyBracket) {
		return nil
	}

	return hash
}

func (p *Parser) parseFloatLiteral() ast.Expr {
	literal := &ast.FloatLiteral{Token: p.currToken}

	value, err := strconv.ParseFloat(p.currToken.Literal, 64)

	if err != nil {
		msg := fmt.Sprintf("could not parse %q as float", p.currToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	literal.Value = value

	return literal
}

func (p *Parser) parseMacroLiteral() ast.Expr {
	macro := &ast.MacroLiteral{Token: p.currToken}

	if !p.expectPeek(token.LeftParen) {
		return nil
	}

	macro.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.LeftCurlyBracket) {
		return nil
	}

	macro.Body = p.parseBlockStmt()

	return macro
}

var typeMap = map[token.TokenType]types.Type{
	token.IntType:    types.Int,
	token.FloatType:  types.Float,
	token.StringType: types.String,
	token.NullType:   types.Null,
	token.BoolType:   types.Bool,
}

func (p *Parser) isPeekTokenType() bool {
	switch p.peekToken.Type {
	case token.IntType:
		fallthrough
	case token.FloatType:
		fallthrough
	case token.StringType:
		fallthrough
	case token.BoolType:
		fallthrough
	case token.NullType:
		fallthrough
	case token.FunctionType:
		fallthrough
	case token.MapType:
		fallthrough
	case token.ArrayType:
		return true
	default:
		return false
	}
}

func (p *Parser) parseType() types.Type {
	p.nextToken() // advance curr token to type token
	switch p.currToken.Type {
	case token.IntType:
		fallthrough
	case token.FloatType:
		fallthrough
	case token.StringType:
		fallthrough
	case token.BoolType:
		fallthrough
	case token.NullType:
		return typeMap[p.currToken.Type]
	case token.MapType:
		return p.parseMapType()
	case token.ArrayType:
		return p.parseArrayType()
	case token.FunctionType:
		return p.parseFunctionType()
	}

	p.errors = append(p.errors, fmt.Sprintf("unknown type %q", p.peekToken.Type))
	return nil
}

func (p *Parser) parseFunctionType() types.Type {
	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, getTypeParseError("function", token.GreaterThan, p.peekToken.Type))
		return nil
	}
	if !p.expectPeek(token.LeftParen) {
		p.errors = append(p.errors, getTypeParseError("function", token.LeftParen, p.peekToken.Type))
		return nil
	}

	params := make([]types.Type, 0)

	for !p.peekTokenIs(token.RightParen) {
		t := p.parseType()
		if !p.expectPeek(token.Comma) {
			p.errors = append(p.errors, getTypeParseError("function", token.Comma, p.peekToken.Type))
			return nil
		}
		params = append(params, t)
	}

	if !p.expectPeek(token.RightParen) {
		p.errors = append(p.errors, getTypeParseError("function", token.RightParen, p.peekToken.Type))
		return nil
	}

	if !p.expectPeek(token.Arrow) {
		p.errors = append(p.errors, getTypeParseError("function", token.Arrow, p.peekToken.Type))
		return nil
	}

	r := p.parseType()

	return types.FunctionType{Params: params, Return: r}

}

func (p *Parser) parseArrayType() types.Type {
	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, getTypeParseError("array", token.GreaterThan, p.peekToken.Type))
		return nil
	}

	t := p.parseType() // recursively get type for array

	if !p.expectPeek(token.LessThan) {
		p.errors = append(p.errors, getTypeParseError("array", token.LessThan, p.peekToken.Type))
		return nil
	}

	return types.ArrayType{ElemType: t}
}

func (p *Parser) parseMapType() types.Type {
	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, getTypeParseError("map", token.GreaterThan, p.peekToken.Type))
		return nil
	}

	k := p.parseType() // recursively get type for key type

	if !p.expectPeek(token.Comma) {
		p.errors = append(p.errors, getTypeParseError("map", token.Comma, p.peekToken.Type))
		return nil
	}

	if !p.expectPeek(token.LessThan) {
		p.errors = append(p.errors, getTypeParseError("map", token.LessThan, p.peekToken.Type))
		return nil
	}

	v := p.parseType()

	return types.MapType{KeyType: k, ValueType: v}
}

func getTypeParseError(name string, expected token.TokenType, got token.TokenType) string {
	return fmt.Sprintf("expected %q for %s type annotation, got %q", expected, name, got)
}
