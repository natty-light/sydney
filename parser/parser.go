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
	SCOPEACCESS
	PREFIX
	CALL
	INDEX
	SELECTOR
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
	token.Dot:                SELECTOR,
	token.Colon:              SCOPEACCESS,
}

type Parser struct {
	lexer *lexer.Lexer

	currToken     token.Token
	peekToken     token.Token
	peekPeekToken token.Token

	errors []string

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn

	definedStructs    map[string]types.Type
	definedInterfaces map[string]types.Type
	doneImports       bool

	genericNames   map[string]bool
	typeParameters map[string]bool

	suppressColon bool
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{lexer: l, errors: make([]string, 0)}

	// peekToken and currToken are initialized to the zero value of token.Token, so we advance twice
	p.nextToken() // set peek
	p.nextToken() // set curr and peek
	p.nextToken() // set curr and peek and peek peek

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)

	p.registerPrefix(token.Identifier, p.parseIdentifierOrStructLiteral)
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
	p.registerPrefix(token.Match, p.parseMatchExpr)
	p.registerPrefix(token.Byte, p.parseByteLiteral)
	p.registerPrefix(token.InvArrow, p.parseReceiveExpr)
	p.registerPrefix(token.ChannelType, p.parseChannelConstructor)

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
	p.registerInfix(token.Dot, p.parseSelectorExpr)
	p.registerInfix(token.Colon, p.parseScopeAccessExpr)
	p.registerPrefix(token.IntType, p.parseTypeCast)
	p.registerPrefix(token.ByteType, p.parseTypeCast)
	p.registerPrefix(token.FloatType, p.parseTypeCast)

	p.definedStructs = make(map[string]types.Type)
	p.definedInterfaces = make(map[string]types.Type)

	p.genericNames = make(map[string]bool)
	p.typeParameters = make(map[string]bool)
	return p
}

func NewWithGenericNames(l *lexer.Lexer, genericNames map[string]bool) *Parser {
	p := New(l)
	p.genericNames = genericNames
	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) ParseDefinitions() {
	for !p.currTokenIs(token.EOF) {
		if p.currTokenIs(token.Public) && p.peekTokenIs(token.Define) {
			p.nextToken()
		}
		if p.currTokenIs(token.Define) {
			if p.peekTokenIs(token.Struct) {
				p.nextToken()
				p.parseStructDefinitionStmt()
				continue
			}
			if p.peekTokenIs(token.Interface) {
				p.nextToken()
				p.parseInterfaceDefinitionStmt()
				continue
			}
		}
		p.nextToken()
	}
}

func (p *Parser) DefinedStructs() map[string]types.Type {
	return p.definedStructs
}

func (p *Parser) DefinedInterfaces() map[string]types.Type {
	return p.definedInterfaces
}

func (p *Parser) SetDefinedTypes(structs map[string]types.Type, interfaces map[string]types.Type) {
	for k, v := range structs {
		p.definedStructs[k] = v
	}
	for k, v := range interfaces {
		p.definedInterfaces[k] = v
	}
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func (p *Parser) noPrefixParseFnError(t token.Token) {
	msg := fmt.Sprintf("%d:%d no prefix parse function for %s found", t.Line, t.Column, t.Type)
	p.errors = append(p.errors, msg)
}

// advances current and peek by one
func (p *Parser) nextToken() {
	p.currToken = p.peekToken
	p.peekToken = p.peekPeekToken
	p.peekPeekToken = p.lexer.NextToken()
}

// Checks whether current token matches given type
func (p *Parser) currTokenIs(t token.TokenType) bool {
	return p.currToken.Type == t
}

// checks whether peek token matches given type
func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) peekPeekTokenIs(t token.TokenType) bool {
	return p.peekPeekToken.Type == t
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
	msg := fmt.Sprintf("%d:%d expected next token to be %s, got %s instead",
		p.peekToken.Line, p.peekToken.Column, t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

func (p *Parser) peekPrecedence() Precedence {
	if p.suppressColon && p.peekToken.Type == token.Colon {
		return LOWEST
	}
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
			_, ok := stmt.(*ast.ImportStatement)
			program.Stmts = append(program.Stmts, stmt)
			if ok && p.doneImports {
				p.errors = append(p.errors, "import statements must be top level")
			}

			if _, isMod := stmt.(*ast.ModuleDeclarationStmt); !isMod && !ok {
				p.doneImports = true
			}

		}
		p.nextToken()
	}
	return program
}

// Statements
func (p *Parser) parseStatement() ast.Stmt {
	if p.currTokenIs(token.AnnotationStart) {
		annotation := p.parseAnnotation()
		stmt := p.parseStatement()
		switch s := stmt.(type) {
		case *ast.StructDefinitionStmt:
			stmt.SetAnnotations([]*ast.Annotation{annotation})
		case *ast.PubStatement:
			if inner, ok := s.Stmt.(*ast.StructDefinitionStmt); ok {
				inner.SetAnnotations([]*ast.Annotation{annotation})
			} else {
				p.errors = append(p.errors, "can only provide annotations for struct definitions")
				return nil
			}
		default:
			p.errors = append(p.errors, "can only provide annotations for struct definitions")
			return nil
		}
		return stmt
	}

	switch p.currToken.Type {
	case token.Mut:
		fallthrough
	case token.Const:
		return p.parseVarDeclarationStmt()
	case token.Public:
		pubStmt := &ast.PubStatement{Token: p.currToken}
		p.nextToken()
		switch p.currToken.Type {
		case token.Mut:
			fallthrough
		case token.Const:
			pubStmt.Stmt = p.parseVarDeclarationStmt()
			break
		case token.Func:
			pubStmt.Stmt = p.parseFunctionDeclarationStmt(false)
			break
		case token.Extern:
			if !p.expectPeek(token.Func) {
				p.errors = append(p.errors, "expected function declaration")
			}
			pubStmt.Stmt = p.parseFunctionDeclarationStmt(true)
		case token.Define:
			p.nextToken()
			if p.currTokenIs(token.Struct) {
				pubStmt.Stmt = p.parseStructDefinitionStmt()
				break
			} else if p.currTokenIs(token.Interface) {
				pubStmt.Stmt = p.parseInterfaceDefinitionStmt()
				break
			}
		default:
			p.errors = append(p.errors, fmt.Sprintf("%d:%d cannot define pub for token: %s", p.currToken.Line, p.currToken.Column, p.currToken.Type))
			return nil
		}
		return pubStmt
	case token.Return:
		return p.parseReturnStmt()
	case token.For:
		return p.parseForStmt()
	case token.Extern:
		p.errors = append(p.errors, fmt.Sprintf("%d:%d cannot extern for token: %s", p.currToken.Line, p.currToken.Column, p.currToken.Type))
		return nil
	case token.Func:
		if p.peekTokenIs(token.Identifier) {
			return p.parseFunctionDeclarationStmt(false)
		}

		return p.parseExpressionOrAssignmentStmt()
	case token.Define:
		p.nextToken() // move past define
		if p.currTokenIs(token.Struct) {
			return p.parseStructDefinitionStmt()
		} else if p.currTokenIs(token.Interface) {
			return p.parseInterfaceDefinitionStmt()
		} else if p.currTokenIs(token.Implementation) {
			return p.parseInterfaceImplementationStmt()
		}
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected interface or struct, got %s instead", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return nil
	case token.Import:
		return p.parseImportStatement()
	case token.Module:
		return p.parseModuleStatement()
	case token.Break:
		stmt := &ast.BreakStmt{Token: p.currToken}
		if p.peekTokenIs(token.Semicolon) {
			p.nextToken()
		}
		return stmt
	case token.Continue:
		stmt := &ast.ContinueStmt{Token: p.currToken}
		if p.peekTokenIs(token.Semicolon) {
			p.nextToken()
		}
		return stmt
	case token.Spawn:
		stmt := &ast.SpawnStmt{Token: p.currToken}
		p.nextToken()
		expr := p.parseExpression(LOWEST)
		stmt.CallExpr = expr
		if p.peekTokenIs(token.Semicolon) {
			p.nextToken()
		}
		return stmt
	default:
		return p.parseExpressionOrAssignmentStmt()
	}
}

func (p *Parser) parseImportStatement() *ast.ImportStatement {
	p.nextToken()
	name := &ast.StringLiteral{Token: p.currToken, Value: p.currToken.Literal}
	return &ast.ImportStatement{
		Token: p.currToken,
		Name:  name,
	}
}

func (p *Parser) parseModuleStatement() *ast.ModuleDeclarationStmt {
	p.nextToken()
	name := &ast.StringLiteral{Token: p.currToken, Value: p.currToken.Literal}
	return &ast.ModuleDeclarationStmt{
		Token: p.currToken,
		Name:  name,
	}
}

func (p *Parser) parseVarDeclarationStmt() *ast.VarDeclarationStmt {

	// To be here, currToken is either Mut or Const
	isConst := p.currToken.Type == token.Const

	stmt := &ast.VarDeclarationStmt{Token: p.currToken, Constant: isConst}

	// parse type. constant variables do not need a type annotation, mutable variables that are uninitialized do
	if p.isPeekTokenType() {
		p.nextToken()
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

	if !p.currTokenIs(token.Semicolon) {
		stmt.ReturnValue = p.parseExpression(LOWEST)
	}

	if p.peekTokenIs(token.Semicolon) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpressionOrAssignmentStmt() ast.Stmt {
	exprTok := p.currToken
	expr := p.parseExpression(LOWEST)

	if p.peekTokenIs(token.Assign) {
		p.nextToken()

		assignmentTok := p.currToken
		p.nextToken()

		value := p.parseExpression(LOWEST)

		if p.peekTokenIs(token.Semicolon) {
			p.nextToken()
		}

		if idxExpr, ok := expr.(*ast.IndexExpr); ok {
			return &ast.IndexAssignmentStmt{
				Token: assignmentTok,
				Left:  idxExpr,
				Value: value,
			}
		}

		if selectorExpr, ok := expr.(*ast.SelectorExpr); ok {
			return &ast.SelectorAssignmentStmt{
				Token: assignmentTok,
				Left:  selectorExpr,
				Value: value,
			}
		}

		if ident, ok := expr.(*ast.Identifier); ok {
			return &ast.VarAssignmentStmt{
				Identifier: ident,
				Token:      assignmentTok,
				Value:      value,
			}
		}
	}

	if p.peekTokenIs(token.InvArrow) {
		return p.parseSendStmt(expr)
	}

	if p.peekTokenIs(token.Semicolon) {
		p.nextToken()
	}

	return &ast.ExpressionStmt{Token: exprTok, Expr: expr}
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

// Expressions
func (p *Parser) parseExpression(precedence Precedence) ast.Expr {
	prefix := p.prefixParseFns[p.currToken.Type] // look for prefix function for p.currToken
	if prefix == nil {
		p.noPrefixParseFnError(p.currToken)
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

// this is an prefixParseFn, so it will not call p.nextToken() at the end. It als
func (p *Parser) parseIdentifier() ast.Expr {
	ident := &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
	if p.genericNames[ident.Value] && p.peekTokenIs(token.LessThan) {
		p.nextToken()
		p.nextToken()
		typeArgs := p.parseTypeArgs()

		if p.peekTokenIs(token.LeftParen) {
			p.nextToken()
			args := p.parseExpressionList(token.RightParen)
			return &ast.CallExpr{
				Token:     p.currToken,
				Function:  ident,
				Arguments: args,
				TypeArgs:  typeArgs,
			}
		}

		if p.peekTokenIs(token.LeftCurlyBracket) {
			expr := p.parseStructLiteral(ident.Token)
			if expr != nil {
				expr.TypeArgs = typeArgs
			}

			return expr
		}

	}

	return ident
}

// this is an prefixParseFn, so it will not call p.nextToken() at the end
func (p *Parser) parseIntegerLiteral() ast.Expr {
	literal := &ast.IntegerLiteral{Token: p.currToken}

	value, err := strconv.ParseInt(p.currToken.Literal, 0, 64)

	if err != nil {
		msg := fmt.Sprintf("%d:%d could not parse %q as integer", p.currToken.Line, p.currToken.Column, p.currToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	literal.Value = value

	return literal
}

func (p *Parser) parseByteLiteral() ast.Expr {
	literal := &ast.ByteLiteral{Token: p.currToken}
	val := p.currToken.Literal[0]
	literal.Value = val
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

	params, ts := p.parseFunctionParameters()
	function.Parameters = params

	fType := types.FunctionType{Params: ts, Return: nil}

	if p.peekTokenIs(token.Arrow) {
		p.nextToken()
		p.nextToken() // parseType requires us to be on the type token
		fType.Return = p.parseType()
	}

	if !p.expectPeek(token.LeftCurlyBracket) {
		return nil
	}

	function.Body = p.parseBlockStmt()

	function.Type = fType

	return function
}

func (p *Parser) parseFunctionParameters() ([]*ast.Identifier, []types.Type) {
	idents := make([]*ast.Identifier, 0)
	tt := make([]types.Type, 0)

	if p.peekTokenIs(token.RightParen) {
		p.nextToken()
		return idents, tt
	}

	p.nextToken()
	t := p.parseType()
	tt = append(tt, t)
	p.nextToken()

	ident := &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
	idents = append(idents, ident)

	// This loop will start with currToken equal to an ident
	for p.peekTokenIs(token.Comma) {
		p.nextToken() // advance comma to currToken
		p.nextToken() // advance past comma to next type
		t := p.parseType()
		tt = append(tt, t)
		p.nextToken() // move past type
		ident := &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
		idents = append(idents, ident)
	}

	if !p.expectPeek(token.RightParen) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d missing closing parenthesis", p.peekToken.Line, p.peekToken.Column))
		return nil, nil
	}

	return idents, tt
}

func (p *Parser) parseFunctionDeclarationStmt(extern bool) ast.Stmt {
	stmt := &ast.FunctionDeclarationStmt{Token: p.currToken}

	if !p.expectPeek(token.Identifier) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if p.peekTokenIs(token.LessThan) {
		p.nextToken()
		stmt.TypeParams = p.parseTypeParamList()
		p.genericNames[stmt.Name.Value] = true
		p.typeParameters = make(map[string]bool)
		for _, tp := range stmt.TypeParams {
			p.typeParameters[tp.Name] = true
		}
	}

	if !p.expectPeek(token.LeftParen) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d missing opening parenthesis for function declaration", p.peekToken.Line, p.peekToken.Column))
		return nil
	}

	params, pTypes := p.parseFunctionParameters()
	stmt.Params = params

	fType := types.FunctionType{Params: pTypes, Return: nil}
	if p.peekTokenIs(token.Arrow) {
		p.nextToken()
		p.nextToken()

		fType.Return = p.parseType()
	} else {
		fType.Return = types.Unit
	}

	stmt.Type = fType
	if extern {
		stmt.IsExtern = true
		if p.peekTokenIs(token.Semicolon) {
			p.nextToken()
		}
		return stmt
	}

	if !p.expectPeek(token.LeftCurlyBracket) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected body for function %s declaration", p.peekToken.Line, p.peekToken.Column, stmt.Name.String()))
		return nil
	}

	stmt.Body = p.parseBlockStmt()

	p.typeParameters = make(map[string]bool)

	if p.peekTokenIs(token.Semicolon) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseMacroParameters() []*ast.Identifier {
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
		p.nextToken() // advance to ident
		ident := &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
		idents = append(idents, ident)
	}

	if !p.expectPeek(token.RightParen) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d missing closing parenthesis", p.peekToken.Line, p.peekToken.Column))
		return nil
	}

	return idents
}

func (p *Parser) parseCallExpr(function ast.Expr) ast.Expr {
	expr := &ast.CallExpr{Token: p.currToken, Function: function}
	if p.peekTokenIs(token.LessThan) {
		expr.TypeArgs = p.parseTypeArgs()
	}
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
	if p.peekPeekTokenIs(token.Colon) || p.peekTokenIs(token.Colon) {
		return p.parseSliceExpr(left)
	}

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

func (p *Parser) parseForStmt() ast.Stmt {
	forStmt := &ast.ForStmt{Token: p.currToken}
	if !p.expectPeek(token.LeftParen) {
		return nil
	}
	p.nextToken() // advance past (
	if p.currTokenIs(token.Mut) || p.currTokenIs(token.Const) {
		p.parseDeclaredThreePartForStmt(forStmt)
	} else {
		expr := p.parseExpression(LOWEST)

		if p.peekTokenIs(token.Assign) {
			p.parseAssignedThreePartForStmt(expr, forStmt)
		} else if p.peekTokenIs(token.In) || p.peekTokenIs(token.Comma) {
			return p.parseForInStmt(forStmt.Token)
		} else {
			forStmt.Condition = expr
			if !p.expectPeek(token.RightParen) {
				p.errors = append(p.errors, fmt.Sprintf("%d:%d expected ) after condition, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
				return nil
			}
		}
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
		p.suppressColon = true
		key := p.parseExpression(LOWEST)
		p.suppressColon = false
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
		msg := fmt.Sprintf("%d:%d could not parse %q as float", p.currToken.Line, p.currToken.Column, p.currToken.Literal)
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

	macro.Parameters = p.parseMacroParameters()

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
	token.Null:       types.Null,
	token.BoolType:   types.Bool,
	token.ByteType:   types.Byte,
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
	case token.Null:
		fallthrough
	case token.FunctionType:
		fallthrough
	case token.MapType:
		fallthrough
	case token.ResultType:
		fallthrough
	case token.OptionType:
		fallthrough
	case token.ByteType:
		fallthrough
	case token.ChannelType:
		fallthrough
	case token.ArrayType:
		return true
	}

	if p.peekTokenIs(token.Identifier) && p.peekPeekTokenIs(token.Colon) {
		return true
	}

	_, ok := p.definedStructs[p.peekToken.Literal]
	if ok {
		return true
	}
	_, ok = p.typeParameters[p.peekToken.Literal]

	return ok
}

func (p *Parser) parseType() types.Type {
	switch p.currToken.Type {
	case token.IntType:
		return typeMap[p.currToken.Type]
	case token.FloatType:
		return typeMap[p.currToken.Type]
	case token.StringType:
		return typeMap[p.currToken.Type]
	case token.BoolType:
		return typeMap[p.currToken.Type]
	case token.Null:
		return typeMap[p.currToken.Type]
	case token.ByteType:
		return typeMap[p.currToken.Type]
	case token.MapType:
		return p.parseMapType()
	case token.ArrayType:
		return p.parseArrayType()
	case token.FunctionType:
		return p.parseFunctionType()
	case token.ResultType:
		return p.parseResultType()
	case token.OptionType:
		return p.parseOptionType()
	case token.ChannelType:
		return p.parseChannelType()
	case token.Identifier:
		var t types.Type = nil
		var ok bool

		if p.typeParameters != nil && p.typeParameters[p.currToken.Literal] {
			return &types.TypeParamRef{Name: p.currToken.Literal}
		}

		if p.genericNames[p.currToken.Literal] && p.peekTokenIs(token.LessThan) {
			name := p.currToken.Literal
			templateType, ok := p.definedStructs[name]
			if !ok {
				p.errors = append(p.errors, fmt.Sprintf("%d:%d unknown generic struct %s", p.currToken.Line, p.currToken.Column, name))
				return nil
			}

			template := templateType.(types.StructType)
			p.nextToken()
			p.nextToken()
			typeArgs := p.parseTypeArgs()

			if len(typeArgs) != len(template.TypeParams) {
				p.errors = append(p.errors, fmt.Sprintf("%d:%d %s expects exactly %d type arguments", p.currToken.Line, p.currToken.Column, name, len(template.TypeArgs)))
				return nil
			}

			subs := make(map[string]types.Type)
			for i, tp := range template.TypeParams {
				subs[tp.Name] = typeArgs[i]
			}

			result := types.SubstituteTypeParams(template, subs).(types.StructType)
			result.TypeArgs = typeArgs
			result.TypeParams = nil

			return result
		}

		if p.peekTokenIs(token.Colon) {
			module := p.currToken.Literal
			p.nextToken()
			p.nextToken()
			name := p.currToken.Literal

			return types.ScopeType{Module: module, Name: name}
		}

		if t, ok = p.definedStructs[p.currToken.Literal]; ok {
			return t
		}

		if t, ok = p.definedInterfaces[p.currToken.Literal]; ok {
			return t
		}

	}

	p.errors = append(p.errors, fmt.Sprintf("%d:%d unknown type %q", p.peekToken.Line, p.peekToken.Column, p.peekToken.Type))
	return nil
}

func (p *Parser) parseInterfaceMethod() types.Type {
	_, ts := p.parseFunctionParameters()

	fType := types.FunctionType{Params: ts, Return: types.Unit}

	if p.peekTokenIs(token.Arrow) {
		p.nextToken()
		p.nextToken() // parseType requires us to be on the type token
		fType.Return = p.parseType()
	}

	return fType
}

func (p *Parser) parseFunctionType() types.Type {
	if !p.expectPeek(token.LessThan) {
		p.errors = append(p.errors, getTypeParseError("function", token.LessThan, p.peekToken.Type))
		return nil
	}
	if !p.expectPeek(token.LeftParen) {
		p.errors = append(p.errors, getTypeParseError("function", token.LeftParen, p.peekToken.Type))
		return nil
	}
	params := make([]types.Type, 0)

	for !p.peekTokenIs(token.RightParen) {
		p.nextToken()
		t := p.parseType()
		if p.peekTokenIs(token.RightParen) {
			params = append(params, t)
			break
		}

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
	p.nextToken()
	r := p.parseType()
	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, getTypeParseError("function", token.GreaterThan, p.peekToken.Type))
	}

	return types.FunctionType{Params: params, Return: r}
}

func (p *Parser) parseResultType() types.Type {
	if !p.expectPeek(token.LessThan) {
		p.errors = append(p.errors, getTypeParseError("result", token.LessThan, p.peekToken.Type))
		return nil
	}
	p.nextToken()
	t := p.parseType()

	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, getTypeParseError("result", token.GreaterThan, p.peekToken.Type))
		return nil
	}

	return types.ResultType{T: t}
}

func (p *Parser) parseOptionType() types.Type {
	if !p.expectPeek(token.LessThan) {
		p.errors = append(p.errors, getTypeParseError("option", token.LessThan, p.peekToken.Type))
		return nil
	}
	p.nextToken()
	t := p.parseType()

	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, getTypeParseError("option", token.GreaterThan, p.peekToken.Type))
		return nil
	}

	return types.OptionType{T: t}
}

func (p *Parser) parseChannelType() types.Type {
	if !p.expectPeek(token.LessThan) {
		p.errors = append(p.errors, getTypeParseError("chan", token.LessThan, p.peekToken.Type))
		return nil
	}
	p.nextToken()
	t := p.parseType()

	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, getTypeParseError("chan", token.GreaterThan, p.peekToken.Type))
		return nil
	}

	return types.ChannelType{ElemType: t}
}

func (p *Parser) parseArrayType() types.Type {
	if !p.expectPeek(token.LessThan) {
		p.errors = append(p.errors, getTypeParseError("array", token.GreaterThan, p.peekToken.Type))
		return nil
	}
	p.nextToken()
	t := p.parseType() // recursively get type for array

	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, getTypeParseError("array", token.LessThan, p.peekToken.Type))
		return nil
	}

	return types.ArrayType{ElemType: t}
}

func (p *Parser) parseMapType() types.Type {
	if !p.expectPeek(token.LessThan) {
		p.errors = append(p.errors, getTypeParseError("map", token.GreaterThan, p.peekToken.Type))
		return nil
	}
	p.nextToken()
	k := p.parseType() // recursively get type for key type

	if !p.expectPeek(token.Comma) {
		p.errors = append(p.errors, getTypeParseError("map", token.Comma, p.peekToken.Type))
		return nil
	}

	p.nextToken()
	v := p.parseType()

	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, getTypeParseError("map", token.LessThan, p.peekToken.Type))
		return nil
	}

	return types.MapType{KeyType: k, ValueType: v}
}

func (p *Parser) parseStructDefinitionStmt() ast.Stmt {
	stmt := &ast.StructDefinitionStmt{}
	if !p.expectPeek(token.Identifier) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected identifier, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return nil
	}
	name := &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
	stmt.Name = name

	typeParams := make([]*types.TypeParam, 0)
	if p.peekTokenIs(token.LessThan) {
		p.nextToken()
		typeParams = p.parseTypeParamList()
		p.typeParameters = make(map[string]bool)
		for _, t := range typeParams {
			p.typeParameters[t.Name] = true
		}
		p.genericNames[stmt.Name.Value] = true
	}

	if !p.expectPeek(token.LeftCurlyBracket) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected {, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return nil
	}

	fields := make([]string, 0)
	tt := make([]types.Type, 0)

	for !p.peekTokenIs(token.RightCurlyBracket) {
		p.nextToken() // move to ident
		if !p.currTokenIs(token.Identifier) {
			p.errors = append(p.errors, fmt.Sprintf("%d:%d expected identifier, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
			return nil
		}
		fields = append(fields, p.currToken.Literal)
		p.nextToken() // we don't have a way to peek for a set of types so advance and let the type parser catch it
		tt = append(tt, p.parseType())

		if !p.peekTokenIs(token.RightCurlyBracket) && !p.expectPeek(token.Comma) {
			p.errors = append(p.errors, fmt.Sprintf("%d:%d expected , or } got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
			return nil
		}
	}
	if !p.expectPeek(token.RightCurlyBracket) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected }, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return nil
	}
	t := types.StructType{
		Fields:              fields,
		Types:               tt,
		Name:                stmt.Name.Value,
		Interfaces:          make([]types.Type, 0),
		SatisfiedInterfaces: make([]string, 0),
		TypeParams:          typeParams,
	}

	stmt.Type = t
	p.definedStructs[stmt.Name.Value] = t
	p.typeParameters = nil

	return stmt
}

func (p *Parser) parseInterfaceDefinitionStmt() ast.Stmt {
	stmt := &ast.InterfaceDefinitionStmt{}
	if !p.expectPeek(token.Identifier) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected identifier, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return nil
	}

	name := &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
	stmt.Name = name

	t := types.InterfaceType{Methods: make([]string, 0), Types: make([]types.Type, 0), Name: stmt.Name.Value}
	p.definedInterfaces[stmt.Name.Value] = t

	if !p.expectPeek(token.LeftCurlyBracket) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected {, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}

	methods := make([]string, 0)
	tt := make([]types.Type, 0)

	for !p.peekTokenIs(token.RightCurlyBracket) {
		p.nextToken() // move to ident
		if !p.currTokenIs(token.Identifier) {
			p.errors = append(p.errors, fmt.Sprintf("%d:%d expected identifier, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
			return nil
		}
		methods = append(methods, p.currToken.Literal)
		p.nextToken() // we don't have a way to peek for a set of types so advance and let the type parser catch it
		tt = append(tt, p.parseInterfaceMethod())

		if !p.peekTokenIs(token.RightCurlyBracket) && !p.expectPeek(token.Comma) {
			p.errors = append(p.errors, fmt.Sprintf("%d:%d expected , or } got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
			return nil
		}
	}
	if !p.expectPeek(token.RightCurlyBracket) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected }, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}
	t.Methods = methods
	t.Types = tt
	t.MethodIndices = make(map[string]int)
	for i, mn := range methods {
		t.MethodIndices[mn] = i
	}
	p.definedInterfaces[stmt.Name.Value] = t

	stmt.Type = t

	return stmt
}

func (p *Parser) parseInterfaceImplementationStmt() ast.Stmt {
	stmt := &ast.InterfaceImplementationStmt{}
	if !p.expectPeek(token.Identifier) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected identifier, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}
	stmt.StructName = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
	if !p.expectPeek(token.Arrow) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected ->, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}
	p.nextToken() // move past ->

	interfaceNames := make([]ast.Expr, 0)
	var left ast.Expr = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
	if p.peekTokenIs(token.Colon) {
		p.nextToken()
		left = p.parseScopeAccessExpr(left)
	}

	interfaceNames = append(interfaceNames, left)
	for p.peekTokenIs(token.Comma) {
		p.nextToken()
		if !p.expectPeek(token.Identifier) {
			p.errors = append(p.errors, fmt.Sprintf("%d:%d expected identifier, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
			return nil
		}
		left = &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
		if p.peekTokenIs(token.Colon) {
			p.nextToken()
			left = p.parseScopeAccessExpr(left)
		}
		interfaceNames = append(interfaceNames, left)
	}

	stmt.InterfaceNames = interfaceNames

	return stmt
}

func (p *Parser) parseStructLiteral(tok token.Token) *ast.StructLiteral {
	expr := &ast.StructLiteral{Token: tok, Name: tok.Literal, Fields: make([]string, 0), Values: make([]ast.Expr, 0)}
	if !p.expectPeek(token.LeftCurlyBracket) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected {, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}

	for !p.peekTokenIs(token.RightCurlyBracket) {
		p.nextToken() // advance to field name
		if !p.currTokenIs(token.Identifier) {
			p.errors = append(p.errors, fmt.Sprintf("%d:%d expected identifier, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
			return nil
		}
		expr.Fields = append(expr.Fields, p.currToken.Literal)
		if !p.expectPeek(token.Colon) {
			p.errors = append(p.errors, fmt.Sprintf("%d:%d expected :, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
			return nil
		}

		p.nextToken() // move to value expr
		expr.Values = append(expr.Values, p.parseExpression(LOWEST))

		if !p.peekTokenIs(token.RightCurlyBracket) && !p.expectPeek(token.Comma) {
			p.errors = append(p.errors, fmt.Sprintf("%d:%d expected , or } got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
			return nil
		}
	}

	if !p.expectPeek(token.RightCurlyBracket) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected }, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}
	return expr
}

func (p *Parser) parseIdentifierOrStructLiteral() ast.Expr {
	if p.peekTokenIs(token.LeftCurlyBracket) {
		return p.parseStructLiteral(p.currToken)
	}

	return p.parseIdentifier()
}

func (p *Parser) parseSelectorExpr(left ast.Expr) ast.Expr {
	expr := &ast.SelectorExpr{Token: p.currToken, Left: left}
	if !p.expectPeek(token.Identifier) {
		return nil
	}
	expr.Value = p.parseIdentifier()

	return expr
}

func (p *Parser) parseScopeAccessExpr(left ast.Expr) ast.Expr {
	ident, ok := left.(*ast.Identifier)
	if !ok {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected identifier, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return nil
	}
	p.nextToken()
	member := &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}

	if p.peekTokenIs(token.LeftCurlyBracket) {
		expr := p.parseStructLiteral(p.currToken)
		if expr != nil {
			expr.Module = ident.Value
		}
		return expr
	}

	// Check for generic call: module:func<Type>(args)
	if p.peekTokenIs(token.LessThan) {
		p.nextToken() // consume <
		p.nextToken() // move to first type arg
		typeArgs := p.parseTypeArgs()
		if p.peekTokenIs(token.LeftParen) {
			p.nextToken() // consume (
			args := p.parseExpressionList(token.RightParen)
			return &ast.CallExpr{
				Token:     p.currToken,
				Function:  &ast.ScopeAccessExpr{Member: member, Module: ident},
				Arguments: args,
				TypeArgs:  typeArgs,
			}
		}
	}

	return &ast.ScopeAccessExpr{Member: member, Module: ident}
}

func (p *Parser) parseMatchExpr() ast.Expr {
	m := &ast.MatchExpr{Token: p.currToken}
	if !p.expectPeek(token.Identifier) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected identifier, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}

	subject := p.parseIdentifier()
	if p.peekTokenIs(token.Colon) {
		p.nextToken()
		subject = p.parseScopeAccessExpr(subject)
	}
	if p.peekTokenIs(token.LeftParen) {
		p.nextToken()
		subject = p.parseCallExpr(subject)
	}
	if p.peekTokenIs(token.LeftSquareBracket) {
		p.nextToken()
		subject = p.parseIndexExpr(subject)
	}
	m.Subject = subject
	if !p.expectPeek(token.LeftCurlyBracket) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected {, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}
	if !p.expectPeek(token.Identifier) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected ok, err, some, or none, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}

	switch p.currToken.Literal {
	case "ok", "err":
		if err := p.parseResultMatch(m, p.currToken.Literal); err != nil {
			p.errors = append(p.errors, err.Error())
			return nil
		}
	case "some", "none":
		if err := p.parseOptionMatch(m, p.currToken.Literal); err != nil {
			p.errors = append(p.errors, err.Error())
			return nil
		}
	default:
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected ok, err, some, or none, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return nil
	}

	if !p.expectPeek(token.RightCurlyBracket) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected }, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}

	return m
}

func (p *Parser) parseMatchArmWithBinding(a *ast.MatchArm, isOk bool, isSome bool) error {
	pattern := &ast.MatchPattern{IsOk: isOk, IsSome: isSome}
	if !p.expectPeek(token.LeftParen) {
		return fmt.Errorf("%d:%d expected (, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal)
	}
	if !p.expectPeek(token.Identifier) {
		return fmt.Errorf("%d:%d expected identifier, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal)
	}
	binding := p.parseIdentifier()
	pattern.Binding = binding.(*ast.Identifier)
	a.Pattern = pattern
	if !p.expectPeek(token.RightParen) {
		return fmt.Errorf("%d:%d expected ), got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal)
	}
	if !p.expectPeek(token.Arrow) {
		return fmt.Errorf("%d:%d expected ->, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal)
	}
	p.nextToken()
	a.Body = p.parseBlockStmt()
	if !p.expectPeek(token.Comma) {
		return fmt.Errorf("%d:%d expected , got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal)
	}
	return nil
}

func (p *Parser) parseResultArmPair(okArm *ast.MatchArm, errArm *ast.MatchArm, first string) error {
	if first == "ok" {
		if err := p.parseMatchArmWithBinding(okArm, true, false); err != nil {
			return err
		}
		if !p.expectPeek(token.Identifier) {
			return fmt.Errorf("%d:%d expected Identifer, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Type)
		}
		if p.currToken.Literal != "err" {
			return fmt.Errorf("%d:%d expected err, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal)
		}
		return p.parseMatchArmWithBinding(errArm, false, false)
	}

	if err := p.parseMatchArmWithBinding(errArm, false, false); err != nil {
		return err
	}
	if !p.expectPeek(token.Identifier) {
		return fmt.Errorf("%d:%d expected Identifier got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Type)
	}
	if p.currToken.Literal != "ok" {
		return fmt.Errorf("%d:%d expected ok, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal)
	}
	return p.parseMatchArmWithBinding(okArm, true, false)
}

func (p *Parser) parseResultMatch(m *ast.MatchExpr, first string) error {
	okArm := &ast.MatchArm{}
	errArm := &ast.MatchArm{}
	if err := p.parseResultArmPair(okArm, errArm, first); err != nil {
		return err
	}
	m.OkArm = okArm
	m.ErrArm = errArm
	return nil
}

func (p *Parser) parseOptionMatch(m *ast.MatchExpr, first string) error {
	someArm := &ast.MatchArm{}
	noneArm := &ast.MatchArm{}

	if first == "some" {
		if err := p.parseMatchArmWithBinding(someArm, false, true); err != nil {
			return err
		}
		if !p.expectPeek(token.Identifier) {
			return fmt.Errorf("%d:%d expected none, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal)
		}
		if p.currToken.Literal != "none" {
			return fmt.Errorf("%d:%d expected none, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal)
		}
		if err := p.parseMatchArmNone(noneArm); err != nil {
			return err
		}
	} else {
		if err := p.parseMatchArmNone(noneArm); err != nil {
			return err
		}
		if !p.expectPeek(token.Identifier) {
			return fmt.Errorf("%d:%d expected some, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal)
		}
		if p.currToken.Literal != "some" {
			return fmt.Errorf("%d:%d expected some, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal)
		}
		if err := p.parseMatchArmWithBinding(someArm, false, true); err != nil {
			return err
		}
	}

	m.SomeArm = someArm
	m.NoneArm = noneArm
	return nil
}

func (p *Parser) parseMatchArmNone(a *ast.MatchArm) error {
	a.Pattern = &ast.MatchPattern{IsSome: false}
	if !p.expectPeek(token.Arrow) {
		return fmt.Errorf("%d:%d expected ->, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal)
	}
	p.nextToken()
	a.Body = p.parseBlockStmt()
	if !p.expectPeek(token.Comma) {
		return fmt.Errorf("%d:%d expected , got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal)
	}
	return nil
}

func (p *Parser) parseTypeCast() ast.Expr {
	return &ast.Identifier{Token: p.currToken, Value: p.currToken.Literal}
}

func (p *Parser) parseGenericType() *types.TypeParam {
	if !p.expectPeek(token.Identifier) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected ident, got %s", p.peekToken.Line, p.peekToken.Column, p.peekToken.Literal))
		return nil
	}
	ident := p.parseIdentifier().(*ast.Identifier)
	tp := &types.TypeParam{Name: ident.Value}
	if p.peekTokenIs(token.Colon) {
		p.nextToken()
		p.nextToken()
		c := p.parseType()
		tp.Constraint = c
	}

	return tp
}

func (p *Parser) parseTypeParamList() []*types.TypeParam {
	tpa := make([]*types.TypeParam, 0)
	tp := p.parseGenericType()
	tpa = append(tpa, tp)
	for p.peekTokenIs(token.Comma) && !p.peekTokenIs(token.GreaterThan) {
		p.nextToken()
		tp := p.parseGenericType()
		if tp != nil {
			tpa = append(tpa, tp)

		}
	}

	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, fmt.Sprintf("expected >, got %s", p.currToken.Literal))
		return nil
	}

	return tpa
}

func (p *Parser) parseTypeArgs() []types.Type {
	ta := make([]types.Type, 0)
	t := p.parseType()
	ta = append(ta, t)

	for p.peekTokenIs(token.Comma) {
		p.nextToken()
		p.nextToken()
		tt := p.parseType()
		ta = append(ta, tt)
	}

	if !p.expectPeek(token.GreaterThan) {
		p.errors = append(p.errors, fmt.Sprintf("expected < after type arg list, got %s", p.peekToken.Literal))
		return nil
	}

	return ta
}

func (p *Parser) parseSliceExpr(left ast.Expr) ast.Expr {
	expr := &ast.SliceExpr{Token: p.currToken, Left: left}
	if !p.peekTokenIs(token.Colon) {
		p.nextToken()
		p.suppressColon = true
		expr.Start = p.parseExpression(LOWEST)
		p.suppressColon = false
	}
	if !p.expectPeek(token.Colon) {
		return nil
	}

	if !p.peekTokenIs(token.RightSquareBracket) {
		p.nextToken()
		expr.End = p.parseExpression(LOWEST)
	}
	if !p.expectPeek(token.RightSquareBracket) {
		return nil
	}
	return expr
}

func getTypeParseError(name string, expected token.TokenType, got token.TokenType) string {
	return fmt.Sprintf("expected %q for %s type annotation, got %q", expected, name, got)
}

func (p *Parser) parseDeclaredThreePartForStmt(stmt *ast.ForStmt) {
	stmt.Init = p.parseVarDeclarationStmt()
	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)
	if !p.expectPeek(token.Semicolon) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected ; after condition, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return
	}

	p.nextToken()
	stmt.Post = p.parseExpressionOrAssignmentStmt()
	if !p.expectPeek(token.RightParen) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected ) after post, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return
	}
}

func (p *Parser) parseAssignedThreePartForStmt(expr ast.Expr, stmt *ast.ForStmt) {
	p.nextToken()
	assignTok := p.currToken
	p.nextToken()
	value := p.parseExpression(LOWEST)
	if ident, ok := expr.(*ast.Identifier); ok {
		stmt.Init = &ast.VarAssignmentStmt{
			Identifier: ident,
			Value:      value,
			Token:      assignTok,
		}
	}

	if !p.expectPeek(token.Semicolon) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected ; after init, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return
	}
	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)
	if !p.expectPeek(token.Semicolon) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected ; after condition, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return
	}
	p.nextToken()
	stmt.Post = p.parseExpressionOrAssignmentStmt()
	if !p.expectPeek(token.RightParen) {
		p.errors = append(p.errors, fmt.Sprintf("%d:%d expected ) after post, got %s", p.currToken.Line, p.currToken.Column, p.currToken.Literal))
		return
	}
}

func (p *Parser) parseForInStmt(tok token.Token) ast.Stmt {
	stmt := &ast.ForInStmt{Token: tok}

	if p.peekTokenIs(token.Comma) {
		stmt.Key = p.parseIdentifier().(*ast.Identifier)
		p.nextToken()
		p.nextToken()
		stmt.Value = p.parseIdentifier().(*ast.Identifier)
	} else {
		stmt.Value = p.parseIdentifier().(*ast.Identifier)
	}
	if !p.expectPeek(token.In) {
		return nil
	}
	p.nextToken()
	stmt.Iterable = p.parseExpression(LOWEST)
	if !p.expectPeek(token.RightParen) {
		return nil
	}
	if !p.expectPeek(token.LeftCurlyBracket) {
		return nil
	}
	stmt.Body = p.parseBlockStmt()

	return stmt
}

func (p *Parser) parseSendStmt(ch ast.Expr) ast.Stmt {
	stmt := &ast.SendStmt{
		Chan: ch,
	}
	p.nextToken() // advance past ident
	p.nextToken() // advance past <-

	stmt.Value = p.parseExpression(LOWEST)
	if !p.expectPeek(token.Semicolon) {
		return nil
	}
	return stmt
}

func (p *Parser) parseReceiveExpr() ast.Expr {
	expr := &ast.ReceiveExpr{
		Token: p.currToken,
	}
	p.nextToken()
	expr.Chan = p.parseExpression(LOWEST)
	if p.peekTokenIs(token.Semicolon) {
		p.nextToken()
	}
	return expr
}

func (p *Parser) parseChannelConstructor() ast.Expr {
	typ := p.parseChannelType()

	if !p.expectPeek(token.LeftParen) {
		return nil
	}

	var capacity ast.Expr
	if !p.peekTokenIs(token.RightParen) {
		p.nextToken()
		capacity = p.parseExpression(LOWEST)
	}

	if !p.expectPeek(token.RightParen) {
		return nil
	}

	return &ast.ChannelConstructorExpr{Type: typ, Capacity: capacity}
}

func (p *Parser) parseAnnotation() *ast.Annotation {
	p.nextToken()
	name := p.parseIdentifier().(*ast.Identifier).Value
	if !p.expectPeek(token.LeftParen) {
		return nil
	}
	p.nextToken()
	args := make([]string, 0)
	arg := p.parseIdentifier().(*ast.Identifier).Value
	args = append(args, arg)
	for p.peekTokenIs(token.Comma) {
		p.nextToken()
		p.nextToken()
		arg := p.parseIdentifier().(*ast.Identifier).Value
		args = append(args, arg)
	}
	if !p.expectPeek(token.RightParen) {
		return nil
	}
	if !p.expectPeek(token.RightSquareBracket) {
		return nil
	}
	p.nextToken()
	return &ast.Annotation{
		Name: name,
		Args: args,
	}
}
