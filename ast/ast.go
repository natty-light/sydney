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

	Resolvable interface {
		SetResolvedType(t types.Type)
		GetResolvedType() types.Type
	}

	Castable interface {
		SetCastTo(it *types.InterfaceType)
		GetCastTo() *types.InterfaceType
	}

	Expr interface {
		Node
		Resolvable
		Castable
		expressionNode()
	}
)

// Node
type (
	Program struct {
		Stmts []Stmt
	}
)

type castable struct{ CastTo *types.InterfaceType }

func (c *castable) SetCastTo(it *types.InterfaceType) { c.CastTo = it }
func (c *castable) GetCastTo() *types.InterfaceType   { return c.CastTo }

type noCast struct{}

func (n *noCast) SetCastTo(_ *types.InterfaceType) {}
func (n *noCast) GetCastTo() *types.InterfaceType  { return nil }

type resolvable struct{ ResolvedType types.Type }

func (r *resolvable) GetResolvedType() types.Type  { return r.ResolvedType }
func (r *resolvable) SetResolvedType(t types.Type) { r.ResolvedType = t }

type MatchArm struct {
	Pattern *MatchPattern
	Body    *BlockStmt
}

type MatchPattern struct {
	IsOk    bool
	Binding *Identifier
}

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
		Init      Stmt
		Condition Expr
		Post      Stmt
		Body      *BlockStmt
	}

	IndexAssignmentStmt struct {
		Token token.Token
		Left  *IndexExpr
		Value Expr
	}

	FunctionDeclarationStmt struct {
		Token       token.Token
		Name        *Identifier
		Params      []*Identifier
		Body        *BlockStmt
		Type        types.Type
		MangledName string
		IsExtern    bool
	}

	StructDefinitionStmt struct {
		Token token.Token
		Name  *Identifier
		Type  types.StructType
	}

	SelectorAssignmentStmt struct {
		Token token.Token
		Left  *SelectorExpr
		Value Expr
	}

	InterfaceDefinitionStmt struct {
		Token token.Token
		Name  *Identifier
		Type  types.InterfaceType
	}

	InterfaceImplementationStmt struct {
		Token          token.Token
		StructName     *Identifier
		InterfaceNames []*Identifier
	}

	ImportStatement struct {
		Token token.Token
		Name  *StringLiteral
	}

	ModuleDeclarationStmt struct {
		Token token.Token
		Name  *StringLiteral
	}

	PubStatement struct {
		Token token.Token
		Stmt  Stmt
	}

	BreakStmt struct {
		Token token.Token
	}

	ContinueStmt struct {
		Token token.Token
	}
)

// Expressions and literals
type (
	// Literals
	IntegerLiteral struct {
		Token token.Token
		Value int64
		resolvable
		noCast
	}

	BooleanLiteral struct {
		Token token.Token
		Value bool
		resolvable
		noCast
	}

	FunctionLiteral struct {
		Token      token.Token
		Parameters []*Identifier
		Body       *BlockStmt
		Name       string
		Type       types.FunctionType
		resolvable
		noCast
	}

	StringLiteral struct {
		Token token.Token
		Value string
		resolvable
		noCast
	}

	ArrayLiteral struct {
		Token    token.Token
		Elements []Expr
		resolvable
		noCast
	}

	NullLiteral struct {
		Token token.Token
		resolvable
		noCast
	}

	HashLiteral struct {
		Token token.Token
		Pairs map[Expr]Expr
		noCast
		resolvable
	}

	FloatLiteral struct {
		Token token.Token
		Value float64
		resolvable
		noCast
	}

	MacroLiteral struct {
		Token      token.Token
		Parameters []*Identifier
		Body       *BlockStmt
		resolvable
		noCast
	}

	StructLiteral struct {
		Token  token.Token
		Name   string
		Fields []string
		Values []Expr
		Module string
		resolvable
		castable
	}

	ByteLiteral struct {
		Token token.Token
		Value byte
		resolvable
		noCast
	}

	// Expressions
	Identifier struct {
		Token token.Token // token.Ident
		Value string
		resolvable
		castable
	}

	PrefixExpr struct {
		Token    token.Token
		Operator string
		Right    Expr
		resolvable
		noCast
	}

	InfixExpr struct {
		Token    token.Token
		Left     Expr
		Operator string
		Right    Expr
		resolvable
		noCast
	}

	IfExpr struct {
		Token       token.Token
		Condition   Expr
		Consequence *BlockStmt
		Alternative *BlockStmt
		resolvable
		castable
	}

	CallExpr struct {
		Token       token.Token
		Function    Expr
		Arguments   []Expr
		MangledName string
		resolvable
		castable
	}

	IndexExpr struct {
		Token         token.Token
		Left          Expr
		Index         Expr
		ContainerType types.Type
		resolvable
		castable
	}

	SelectorExpr struct {
		Token token.Token
		Left  Expr
		Value Expr
		resolvable
		castable
	}

	ScopeAccessExpr struct {
		Token  token.Token
		Module *Identifier
		Member *Identifier
		resolvable
		noCast
	}

	MatchExpr struct {
		Token       token.Token
		Subject     Expr
		OkArm       *MatchArm
		ErrArm      *MatchArm
		SubjectType types.Type
		noCast
		resolvable
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

func (f *FunctionDeclarationStmt) TokenLiteral() string {
	return f.Token.Literal
}

func (s *StructLiteral) TokenLiteral() string {
	return s.Token.Literal
}

func (s *StructDefinitionStmt) TokenLiteral() string {
	return s.Token.Literal
}

func (s *SelectorExpr) TokenLiteral() string {
	return s.Token.Literal
}

func (s *SelectorAssignmentStmt) TokenLiteral() string {
	return s.Token.Literal
}

func (i *InterfaceDefinitionStmt) TokenLiteral() string {
	return i.Token.Literal
}

func (i *InterfaceImplementationStmt) TokenLiteral() string {
	return i.Token.Literal
}

func (i *ImportStatement) TokenLiteral() string {
	return i.Token.Literal
}

func (m *ModuleDeclarationStmt) TokenLiteral() string {
	return m.Token.Literal
}

func (p *PubStatement) TokenLiteral() string {
	return p.Token.Literal
}

func (s *ScopeAccessExpr) TokenLiteral() string {
	return s.Token.Literal
}

func (m *MatchExpr) TokenLiteral() string {
	return m.Token.Literal
}

func (b *ByteLiteral) TokenLiteral() string {
	return b.Token.Literal
}

func (b *BreakStmt) TokenLiteral() string {
	return b.Token.Literal
}

func (c *ContinueStmt) TokenLiteral() string {
	return c.Token.Literal
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

func (f *FunctionDeclarationStmt) String() string {
	var out bytes.Buffer

	params := make([]string, 0)

	for _, p := range f.Params {
		params = append(params, p.String())
	}

	out.WriteString(f.TokenLiteral())
	if f.Name != nil {
		out.WriteString(fmt.Sprintf(" %s ", f.Name.String()))
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	out.WriteString(f.Body.String())

	return out.String()
}

func (s *StructDefinitionStmt) String() string {
	var out bytes.Buffer
	out.WriteString("define struct")
	out.WriteString(s.Name.String())
	out.WriteString(" { ")
	for i, f := range s.Type.Fields {
		out.WriteString(f)
		out.WriteString(" ")
		out.WriteString(s.Type.Types[i].Signature())
		if i < len(s.Type.Fields)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(" }")
	return out.String()
}

func (s *SelectorAssignmentStmt) String() string {
	var out bytes.Buffer
	out.WriteString(s.Left.String())
	out.WriteString(" = ")
	out.WriteString(s.Value.String())

	return out.String()
}

func (i *InterfaceDefinitionStmt) String() string {
	var out bytes.Buffer
	out.WriteString("define interface")
	out.WriteString(i.Type.Signature())

	return out.String()
}

func (i *InterfaceImplementationStmt) String() string {
	var out bytes.Buffer
	out.WriteString("define implementation ")
	out.WriteString(i.StructName.String())
	out.WriteString(" -> ")
	if len(i.InterfaceNames) > 1 {
		out.WriteString("(")
	}

	for idx, n := range i.InterfaceNames {
		out.WriteString(n.String())
		if idx < len(i.InterfaceNames)-1 {
			out.WriteString(", ")
		}
	}

	if len(i.InterfaceNames) > 1 {
		out.WriteString(")")
	}

	return out.String()
}

func (m *ModuleDeclarationStmt) String() string {
	return fmt.Sprintf("module %s", m.Name.String())
}

func (i *ImportStatement) String() string {
	return fmt.Sprintf("import %s", i.Name.String())
}

func (p *PubStatement) String() string {
	var out bytes.Buffer
	out.WriteString("pub ")
	out.WriteString(p.Stmt.String())
	out.WriteString(";")
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

func (s *SelectorExpr) String() string {
	var out bytes.Buffer
	out.WriteString(s.Left.String())
	out.WriteString(".")
	out.WriteString(s.Value.String())

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

func (s *StructLiteral) String() string {
	var out bytes.Buffer
	out.WriteString(s.Name)
	out.WriteString(" { ")
	for i, f := range s.Fields {
		out.WriteString(f)
		out.WriteString(" ")
		out.WriteString(s.Values[i].String())
		if i < len(s.Fields)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString(" }")
	return out.String()
}

func (s *ScopeAccessExpr) String() string {
	var out bytes.Buffer
	out.WriteString(s.Module.String())
	out.WriteString(":")
	out.WriteString(s.Member.String())
	return out.String()
}

func (m *MatchExpr) String() string {
	var out bytes.Buffer
	out.WriteString("match")
	out.WriteString(m.Subject.String())
	out.WriteString(" {\n")
	out.WriteString("\t")
	if m.OkArm.Pattern.IsOk {
		out.WriteString("ok(")
		out.WriteString(m.OkArm.Pattern.Binding.String())
		out.WriteString(")")
		out.WriteString(" -> ")
		out.WriteString(m.OkArm.Body.String())
		out.WriteString(",\n")
	}
	if m.ErrArm.Pattern.IsOk {
		out.WriteString("err(")
		out.WriteString(m.ErrArm.Pattern.Binding.String())
		out.WriteString(")")
		out.WriteString(" -> ")
		out.WriteString(m.ErrArm.Body.String())
		out.WriteString(",\n")
	}
	out.WriteString("}")

	return out.String()
}

func (b *ByteLiteral) String() string {
	return b.Token.Literal
}

func (b *BreakStmt) String() string {
	return b.Token.Literal
}

func (c *ContinueStmt) String() string {
	return c.Token.Literal
}

// Statements
func (v *VarDeclarationStmt) statementNode()          {}
func (r *ReturnStmt) statementNode()                  {}
func (e *ExpressionStmt) statementNode()              {}
func (b *BlockStmt) statementNode()                   {}
func (v *VarAssignmentStmt) statementNode()           {}
func (f *ForStmt) statementNode()                     {}
func (i *IndexAssignmentStmt) statementNode()         {}
func (f *FunctionDeclarationStmt) statementNode()     {}
func (s *StructDefinitionStmt) statementNode()        {}
func (s *SelectorAssignmentStmt) statementNode()      {}
func (i *InterfaceDefinitionStmt) statementNode()     {}
func (i *InterfaceImplementationStmt) statementNode() {}
func (m *ModuleDeclarationStmt) statementNode()       {}
func (i *ImportStatement) statementNode()             {}
func (p *PubStatement) statementNode()                {}
func (c *ContinueStmt) statementNode()                {}
func (b *BreakStmt) statementNode()                   {}

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
func (s *StructLiteral) expressionNode()   {}
func (s *SelectorExpr) expressionNode()    {}
func (s *ScopeAccessExpr) expressionNode() {}
func (m *MatchExpr) expressionNode()       {}
func (b *ByteLiteral) expressionNode()     {}

func Dump(node Node, indent int) {
	prefix := func(label string) {
		fmt.Println(withIdent(label, indent))
	}
	field := func(label string, val string) {
		fmt.Println(withIdent(label+" "+val, indent+2))
	}
	child := func(label string, n Node) {
		fmt.Println(withIdent(label, indent+2))
		if n != nil {
			Dump(n, indent+4)
		}
	}

	switch node := node.(type) {
	case *Program:
		prefix("Program")
		for _, stmt := range node.Stmts {
			Dump(stmt, indent+2)
		}

	// Statements
	case *ExpressionStmt:
		prefix("ExpressionStmt")
		child("Expr:", node.Expr)
	case *ReturnStmt:
		prefix("ReturnStmt")
		child("Value:", node.ReturnValue)
	case *BlockStmt:
		prefix("BlockStmt")
		for _, stmt := range node.Stmts {
			Dump(stmt, indent+2)
		}
	case *VarDeclarationStmt:
		prefix("VarDeclarationStmt")
		field("Name:", node.Name.Value)
		field("Constant:", fmt.Sprintf("%v", node.Constant))
		if node.Type != nil {
			field("Type:", node.Type.Signature())
		}
		child("Value:", node.Value)
	case *VarAssignmentStmt:
		prefix("VarAssignmentStmt")
		field("Name:", node.Identifier.Value)
		child("Value:", node.Value)
	case *ForStmt:
		prefix("ForStmt")
		child("Condition:", node.Condition)
		child("Body:", node.Body)
	case *IndexAssignmentStmt:
		prefix("IndexAssignmentStmt")
		child("Left:", node.Left)
		child("Value:", node.Value)
	case *FunctionDeclarationStmt:
		prefix("FunctionDeclarationStmt")
		field("Name:", node.Name.Value)
		if node.MangledName != "" {
			field("MangledName:", node.MangledName)
		}
		if node.IsExtern {
			field("Extern:", "true")
		}
		if node.Type != nil {
			field("Type:", node.Type.Signature())
		}
		for _, p := range node.Params {
			child("Param:", p)
		}
		if node.Body != nil {
			child("Body:", node.Body)
		}
	case *StructDefinitionStmt:
		prefix("StructDefinitionStmt")
		field("Name:", node.Name.Value)
		for i, f := range node.Type.Fields {
			field("Field:", f+" "+node.Type.Types[i].Signature())
		}
	case *SelectorAssignmentStmt:
		prefix("SelectorAssignmentStmt")
		child("Left:", node.Left)
		child("Value:", node.Value)
	case *InterfaceDefinitionStmt:
		prefix("InterfaceDefinitionStmt")
		field("Name:", node.Name.Value)
		for i, m := range node.Type.Methods {
			field("Method:", m+" "+node.Type.Types[i].Signature())
		}
	case *InterfaceImplementationStmt:
		prefix("InterfaceImplementationStmt")
		field("Struct:", node.StructName.Value)
		for _, n := range node.InterfaceNames {
			field("Interface:", n.Value)
		}
	case *ImportStatement:
		prefix("ImportStatement")
		field("Name:", node.Name.Value)
	case *ModuleDeclarationStmt:
		prefix("ModuleDeclarationStmt")
		field("Name:", node.Name.Value)
	case *PubStatement:
		prefix("PubStatement")
		Dump(node.Stmt, indent+2)

	// Expressions
	case *Identifier:
		prefix("Identifier")
		field("Value:", node.Value)
		if node.GetResolvedType() != nil {
			field("ResolvedType:", node.GetResolvedType().Signature())
		}
	case *IntegerLiteral:
		prefix(fmt.Sprintf("IntegerLiteral(%d)", node.Value))
	case *FloatLiteral:
		prefix(fmt.Sprintf("FloatLiteral(%g)", node.Value))
	case *StringLiteral:
		prefix(fmt.Sprintf("StringLiteral(%q)", node.Value))
	case *BooleanLiteral:
		prefix(fmt.Sprintf("BooleanLiteral(%v)", node.Value))
	case *NullLiteral:
		prefix("NullLiteral")
	case *ArrayLiteral:
		prefix("ArrayLiteral")
		for i, el := range node.Elements {
			child(fmt.Sprintf("[%d]:", i), el)
		}
	case *HashLiteral:
		prefix("HashLiteral")
		for k, v := range node.Pairs {
			child("Key:", k)
			child("Value:", v)
		}
	case *FunctionLiteral:
		prefix("FunctionLiteral")
		if node.Name != "" {
			field("Name:", node.Name)
		}
		field("Type:", node.Type.Signature())
		for _, p := range node.Parameters {
			child("Param:", p)
		}
		child("Body:", node.Body)
	case *StructLiteral:
		prefix("StructLiteral")
		field("Name:", node.Name)
		for i, f := range node.Fields {
			fmt.Println(withIdent("Field: "+f, indent+2))
			Dump(node.Values[i], indent+4)
		}
	case *MacroLiteral:
		prefix("MacroLiteral")
		for _, p := range node.Parameters {
			child("Param:", p)
		}
		child("Body:", node.Body)
	case *PrefixExpr:
		prefix("PrefixExpr")
		field("Op:", node.Operator)
		child("Right:", node.Right)
	case *InfixExpr:
		prefix("InfixExpr")
		field("Op:", node.Operator)
		child("Left:", node.Left)
		child("Right:", node.Right)
	case *IfExpr:
		prefix("IfExpr")
		child("Condition:", node.Condition)
		child("Consequence:", node.Consequence)
		if node.Alternative != nil {
			child("Alternative:", node.Alternative)
		}
	case *CallExpr:
		prefix("CallExpr")
		if node.MangledName != "" {
			field("MangledName:", node.MangledName)
		}
		child("Function:", node.Function)
		for i, arg := range node.Arguments {
			child(fmt.Sprintf("Arg[%d]:", i), arg)
		}
	case *IndexExpr:
		prefix("IndexExpr")
		if node.ContainerType != nil {
			field("ContainerType:", node.ContainerType.Signature())
		}
		child("Left:", node.Left)
		child("Index:", node.Index)
	case *SelectorExpr:
		prefix("SelectorExpr")
		child("Left:", node.Left)
		child("Field:", node.Value)
	case *ScopeAccessExpr:
		prefix("ScopeAccessExpr")
		field("Module:", node.Module.Value)
		field("Member:", node.Member.Value)
	case *MatchExpr:
		prefix("MatchExpr")
		if node.SubjectType != nil {
			field("SubjectType:", node.SubjectType.Signature())
		}
		child("Subject:", node.Subject)
		if node.OkArm != nil {
			fmt.Println(withIdent("OkArm:", indent+2))
			if node.OkArm.Pattern.Binding != nil {
				fmt.Println(withIdent("Binding: "+node.OkArm.Pattern.Binding.Value, indent+4))
			}
			Dump(node.OkArm.Body, indent+4)
		}
		if node.ErrArm != nil {
			fmt.Println(withIdent("ErrArm:", indent+2))
			if node.ErrArm.Pattern.Binding != nil {
				fmt.Println(withIdent("Binding: "+node.ErrArm.Pattern.Binding.Value, indent+4))
			}
			Dump(node.ErrArm.Body, indent+4)
		}
	case *ByteLiteral:
		prefix(fmt.Sprintf("ByteLiteral(%d)", node.Value))

	default:
		prefix(fmt.Sprintf("<%T>", node))
	}
}

func withIdent(text string, space int) string {
	return strings.Repeat(" ", space) + text
}
