package ast

import "log"

func FindAt(node Node, line, col int) (*Identifier, Scope) {
	switch node := node.(type) {
	case *Identifier:
		if matches(node, line, col) {
			return node, nil
		}
	case *Program:
		for _, stmt := range node.Stmts {
			if found, scope := FindAt(stmt, line, col); found != nil {
				return found, scope
			}
		}
	case *BlockStmt:
		log.Printf("FindAt *BlockStmt, scope==nil", node.Scope == nil)
		for _, stmt := range node.Stmts {
			if found, _ := FindAt(stmt, line, col); found != nil {
				return found, node.Scope
			}
		}
	case *ExpressionStmt:
		return FindAt(node.Expr, line, col)
	case *VarDeclarationStmt:
		if matches(node.Name, line, col) {
			return node.Name, nil
		}
		if found, scope := FindAt(node.Value, line, col); found != nil {
			return found, scope
		}
	case *VarAssignmentStmt:
		if matches(node.Identifier, line, col) {
			return node.Identifier, nil
		}
		return FindAt(node.Value, line, col)
	case *InfixExpr:
		if found, scope := FindAt(node.Left, line, col); found != nil {
			return found, scope
		}
		return FindAt(node.Right, line, col)
	case *CallExpr:
		if found, scope := FindAt(node.Function, line, col); found != nil {
			return found, scope
		}
		for _, arg := range node.Arguments {
			if found, scope := FindAt(arg, line, col); found != nil {
				return found, scope
			}
		}
	case *SelectorExpr:
		if found, scope := FindAt(node.Left, line, col); found != nil {
			return found, scope
		}
		return FindAt(node.Value, line, col)
	case *ScopeAccessExpr:
		return FindAt(node.Member, line, col)
	case *FunctionDeclarationStmt:
		if found, scope := FindAt(node.Name, line, col); found != nil {
			return found, scope
		}
		return FindAt(node.Body, line, col)
	case *FunctionLiteral:
		return FindAt(node.Body, line, col)
	case *ReturnStmt:
		return FindAt(node.ReturnValue, line, col)
	case *ForStmt:
		if node.Init != nil {
			if found, scope := FindAt(node.Init, line, col); found != nil {
				return found, scope
			}
		}
		if found, scope := FindAt(node.Condition, line, col); found != nil {
			return found, scope
		}
		if found, scope := FindAt(node.Body, line, col); found != nil {
			return found, scope
		}
		if node.Post != nil {
			return FindAt(node.Post, line, col)
		}
	case *ForInStmt:
		if found, scope := FindAt(node.Iterable, line, col); found != nil {
			return found, scope
		}
		return FindAt(node.Body, line, col)
	case *IfExpr:
		if found, scope := FindAt(node.Condition, line, col); found != nil {
			return found, scope
		}
		if found, scope := FindAt(node.Consequence, line, col); found != nil {
			return found, scope
		}

		if node.Alternative != nil {
			return FindAt(node.Alternative, line, col)
		}
	}

	return nil, nil
}

func matches(ident *Identifier, line, col int) bool {
	l, c := ident.Pos()
	log.Printf("hover: ast_line=%d ast_col=%d, ide_line=%d id_col=%d", l, c, line, col)
	return l == line && col >= c && col < c+len(ident.Value)
}
