package ast

import "log"

func FindAt(node Node, line, col int) *Identifier {
	switch node := node.(type) {
	case *Identifier:
		if matches(node, line, col) {
			return node
		}
	case *Program:
		for _, stmt := range node.Stmts {
			if found := FindAt(stmt, line, col); found != nil {
				return found
			}
		}
	case *ExpressionStmt:
		return FindAt(node.Expr, line, col)
	case *VarDeclarationStmt:
		if matches(node.Name, line, col) {
			return node.Name
		}
		if found := FindAt(node.Value, line, col); found != nil {
			return found
		}
	case *VarAssignmentStmt:
		if matches(node.Identifier, line, col) {
			return node.Identifier
		}
		return FindAt(node.Value, line, col)
	case *InfixExpr:
		if found := FindAt(node.Left, line, col); found != nil {
			return found
		}
		return FindAt(node.Right, line, col)
	case *CallExpr:
		if found := FindAt(node.Function, line, col); found != nil {
			return found
		}
		for _, arg := range node.Arguments {
			if found := FindAt(arg, line, col); found != nil {
				return found
			}
		}
	}

	return nil
}

func matches(ident *Identifier, line, col int) bool {
	l, c := ident.Pos()
	log.Printf("hover: ast_line=%d ast_col=%d, ide_line=%d id_col=%d", l, c, line, col)
	return l == line && col >= c && col < c+len(ident.Value)
}
