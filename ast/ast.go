package ast

import (
	"bytes"
	"fmt"
	"strings"
	"sydney/token"
	"sydney/types"
)

// Interfaces

type (
	Node interface {
		TokenLiteral() string
		String() string
	}

	Stmt interface {
		Node
		statementNode()
	}

	Expr interface {
		Node
		expressionNode()
	}
)

// Node
type (
	Program struct {
		Stmts []Stmt
	}
)

// Statements
type (
	VarDeclarationStmt struct {
		Token    token.Token // token.Mut or token.Const
		Name     *Identifier
		Value    Expr
		Constant bool
		Type     types.Type
	}

	ReturnStmt struct {
		Token       token.Token
		ReturnValue Expr
	}

	ExpressionStmt struct {
		Token token.Token
		Expr  Expr
	}

	BlockStmt struct {
		Token token.Token
		Stmts []Stmt
	}

	VarAssignmentStmt struct {
		Token      token.Token
		Identifier *Identifier
		Value      Expr
	}

	ForStmt struct {
		Token     token.Token
		Condition Expr
		Body      *BlockStmt
	}
)

// Expressions and literals
type (
	// Literals
	IntegerLiteral struct {
		Token token.Token
		Value int64
	}

	BooleanLiteral struct {
		Token token.Token
		Value bool
	}

	FunctionLiteral struct {
		Token      token.Token
		Parameters []*Identifier
		Body       *BlockStmt
		Name       string
		Type       types.FunctionType
	}

	StringLiteral struct {
		Token token.Token
		Value string
	}

	ArrayLiteral struct {
		Token    token.Token
		Elements []Expr
	}

	NullLiteral struct {
		Token token.Token
	}

	HashLiteral struct {
		Token token.Token
		Pairs map[Expr]Expr
	}

	FloatLiteral struct {
		Token token.Token
		Value float64
	}

	MacroLiteral struct {
		Token      token.Token
		Parameters []*Identifier
		Body       *BlockStmt
	}

	// Expressions
	Identifier struct {
		Token token.Token // token.Ident
		Value string
	}

	PrefixExpr struct {
		Token    token.Token
		Operator string
		Right    Expr
	}

	InfixExpr struct {
		Token    token.Token
		Left     Expr
		Operator string
		Right    Expr
	}

	IfExpr struct {
		Token       token.Token
		Condition   Expr
		Consequence *BlockStmt
		Alternative *BlockStmt
	}

	CallExpr struct {
		Token     token.Token
		Function  Expr
		Arguments []Expr
	}

	IndexExpr struct {
		Token token.Token
		Left  Expr
		Index Expr
	}

	IndexAssignmentStmt struct {
		Token token.Token
		Left  *IndexExpr
		Value Expr
	}
)

// Node interfaces
func (p *Program) TokenLiteral() string {
	if len(p.Stmts) > 0 {
		return p.Stmts[0].TokenLiteral()
	} else {
		return ""
	}
}

func (v *VarDeclarationStmt) TokenLiteral() string {
	return v.Token.Literal
}

func (r *ReturnStmt) TokenLiteral() string {
	return r.Token.Literal
}

func (e *ExpressionStmt) TokenLiteral() string {
	return e.Token.Literal
}

func (i *Identifier) TokenLiteral() string {
	return i.Token.Literal
}

func (i *IntegerLiteral) TokenLiteral() string {
	return i.Token.Literal
}

func (p *PrefixExpr) TokenLiteral() string {
	return p.Token.Literal
}

func (i *InfixExpr) TokenLiteral() string {
	return i.Token.Literal
}

func (b *BooleanLiteral) TokenLiteral() string {
	return b.Token.Literal
}

func (i *IfExpr) TokenLiteral() string {
	return i.Token.Literal
}

func (b *BlockStmt) TokenLiteral() string {
	return b.Token.Literal
}

func (f *FunctionLiteral) TokenLiteral() string {
	return f.Token.Literal
}

func (c *CallExpr) TokenLiteral() string {
	return c.Token.Literal
}

func (v *VarAssignmentStmt) TokenLiteral() string {
	return v.Token.Literal
}

func (s *StringLiteral) TokenLiteral() string {
	return s.Token.Literal
}

func (a *ArrayLiteral) TokenLiteral() string {
	return a.Token.Literal
}

func (i *IndexExpr) TokenLiteral() string {
	return i.Token.Literal
}

func (n *NullLiteral) TokenLiteral() string {
	return n.Token.Literal
}

func (f *ForStmt) TokenLiteral() string {
	return f.Token.Literal
}

func (h *HashLiteral) TokenLiteral() string {
	return h.Token.Literal
}

func (f *FloatLiteral) TokenLiteral() string {
	return f.Token.Literal
}

func (m *MacroLiteral) TokenLiteral() string {
	return m.Token.Literal
}

func (i *IndexAssignmentStmt) TokenLiteral() string {
	return i.Token.Literal
}

// Statements
func (p *Program) String() string {
	var out bytes.Buffer

	for _, stmt := range p.Stmts {
		out.WriteString(stmt.String())
	}

	return out.String()
}

func (v *VarDeclarationStmt) String() string {
	var out bytes.Buffer

	out.WriteString(v.TokenLiteral() + " ")
	out.WriteString(v.Name.String())
	out.WriteString(" = ")

	if v.Value != nil {
		out.WriteString(v.Value.String())
	}

	out.WriteString(";")

	return out.String()
}

func (r *ReturnStmt) String() string {
	var out bytes.Buffer

	out.WriteString(r.TokenLiteral() + " ")

	if r.ReturnValue != nil {
		out.WriteString(r.ReturnValue.String())
	}

	out.WriteString(";")

	return out.String()
}

func (e *ExpressionStmt) String() string {
	if e.Expr != nil {
		return e.Expr.String()
	}
	return ""
}

func (b *BlockStmt) String() string {
	var out bytes.Buffer

	for _, stmt := range b.Stmts {
		out.WriteString(stmt.String())
	}

	return out.String()
}

func (f *ForStmt) String() string {
	var out bytes.Buffer

	out.WriteString("for (")
	out.WriteString(f.Condition.String())
	out.WriteString(") {")
	out.WriteString(f.Body.String())
	out.WriteString("}")

	return out.String()
}

func (i *IndexAssignmentStmt) String() string {
	var out bytes.Buffer
	out.WriteString(i.Left.String())
	out.WriteString(" = ")
	out.WriteString(i.Value.String())

	return out.String()
}

// Expressions
func (i *Identifier) String() string {
	return i.Value
}

func (p *PrefixExpr) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(p.Operator)
	out.WriteString(p.Right.String())
	out.WriteString(")")

	return out.String()
}

func (i *InfixExpr) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(i.Left.String())
	out.WriteString(" " + i.Operator + " ")
	out.WriteString(i.Right.String())
	out.WriteString(")")

	return out.String()
}

func (c *CallExpr) String() string {
	var out bytes.Buffer
	args := make([]string, 0)
	for _, arg := range c.Arguments {
		args = append(args, arg.String())
	}

	out.WriteString(c.Function.String())
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")

	return out.String()
}

func (v *VarAssignmentStmt) String() string {
	var out bytes.Buffer

	out.WriteString(v.Identifier.String())
	out.WriteString(" = ")
	out.WriteString(v.Value.String())

	return out.String()
}

func (i *IndexExpr) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(i.Left.String())
	out.WriteString("[")
	out.WriteString(i.Index.String())
	out.WriteString("])")

	return out.String()
}

func (i *IfExpr) String() string {
	var out bytes.Buffer

	out.WriteString("if")
	out.WriteString(i.Condition.String())
	out.WriteString(" ")
	out.WriteString(i.Consequence.String())

	if i.Alternative != nil {
		out.WriteString("else ")
		out.WriteString(i.Alternative.String())
	}

	return out.String()
}

// Literals
func (i *IntegerLiteral) String() string {
	return i.Token.Literal
}

func (s *StringLiteral) String() string {
	return s.Token.Literal
}

func (a *ArrayLiteral) String() string {
	var out bytes.Buffer

	elements := []string{}

	for _, el := range a.Elements {
		elements = append(elements, el.String())
	}
	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")

	return out.String()
}

func (n *NullLiteral) String() string {
	return "null"
}

func (b *BooleanLiteral) String() string {
	return b.Token.Literal
}

func (f *FunctionLiteral) String() string {
	var out bytes.Buffer

	params := make([]string, 0)

	for _, p := range f.Parameters {
		params = append(params, p.String())
	}

	out.WriteString(f.TokenLiteral())
	if f.Name != "" {
		out.WriteString(fmt.Sprintf("<%s>", f.Name))
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	out.WriteString(f.Body.String())

	return out.String()
}

func (h *HashLiteral) String() string {
	var out bytes.Buffer

	pairs := []string{}
	for key, val := range h.Pairs {
		pairs = append(pairs, key.String()+":"+val.String())
	}
	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")

	return out.String()
}

func (f *FloatLiteral) String() string {
	return f.Token.Literal
}

func (m *MacroLiteral) String() string {
	var out bytes.Buffer

	params := make([]string, 0)

	for _, p := range m.Parameters {
		params = append(params, p.String())
	}

	out.WriteString(m.TokenLiteral())
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	out.WriteString(m.Body.String())

	return out.String()
}

// Statements
func (v *VarDeclarationStmt) statementNode()  {}
func (r *ReturnStmt) statementNode()          {}
func (e *ExpressionStmt) statementNode()      {}
func (b *BlockStmt) statementNode()           {}
func (v *VarAssignmentStmt) statementNode()   {}
func (f *ForStmt) statementNode()             {}
func (i *IndexAssignmentStmt) statementNode() {}

// Expressions
func (i *Identifier) expressionNode()      {}
func (i *IntegerLiteral) expressionNode()  {}
func (p *PrefixExpr) expressionNode()      {}
func (i *InfixExpr) expressionNode()       {}
func (b *BooleanLiteral) expressionNode()  {}
func (i *IfExpr) expressionNode()          {}
func (f *FunctionLiteral) expressionNode() {}
func (c *CallExpr) expressionNode()        {}
func (s *StringLiteral) expressionNode()   {}
func (a *ArrayLiteral) expressionNode()    {}
func (i *IndexExpr) expressionNode()       {}
func (n *NullLiteral) expressionNode()     {}
func (h *HashLiteral) expressionNode()     {}
func (f *FloatLiteral) expressionNode()    {}
func (m *MacroLiteral) expressionNode()    {}
