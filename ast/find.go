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
	case *PubStatement:
		return FindAt(node.Stmt, line, col)
	case *BlockStmt:
		log.Print("FindAt *BlockStmt, scope==nil", node.Scope == nil)
		for _, stmt := range node.Stmts {
			if found, scope := FindAt(stmt, line, col); found != nil {
				if scope != nil {
					return found, scope
				}
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
		for _, param := range node.Params {
			if found, _ := FindAt(param, line, col); found != nil {
				return found, node.Body.Scope
			}
		}
		return FindAt(node.Body, line, col)
	case *FunctionLiteral:
		return FindAt(node.Body, line, col)
	case *ReturnStmt:
		return FindAt(node.ReturnValue, line, col)
	case *ForStmt:
		if node.Init != nil {
			if found, _ := FindAt(node.Init, line, col); found != nil {
				return found, node.Body.Scope
			}
		}
		if found, _ := FindAt(node.Condition, line, col); found != nil {
			return found, node.Body.Scope
		}
		if found, scope := FindAt(node.Body, line, col); found != nil {
			return found, scope
		}
		if node.Post != nil {
			if found, _ := FindAt(node.Post, line, col); found != nil {
				return found, node.Body.Scope
			}
		}
	case *ForInStmt:
		if found, _ := FindAt(node.Iterable, line, col); found != nil {
			return found, node.Body.Scope
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
	case *InterfaceDefinitionStmt:
		return FindAt(node.Name, line, col)
	case *StructDefinitionStmt:
		return FindAt(node.Name, line, col)
	case *InterfaceImplementationStmt:
		if found, scope := FindAt(node.StructName, line, col); found != nil {
			return found, scope
		}

		for _, iface := range node.InterfaceNames {
			if found, scope := FindAt(iface, line, col); found != nil {
				return found, scope
			}
		}
	case *MatchExpr:
		if found, scope := FindAt(node.Subject, line, col); found != nil {
			return found, scope
		}

		if node.SomeArm != nil {
			if found, _ := FindAt(node.SomeArm.Pattern.Binding, line, col); found != nil {
				return found, node.SomeArm.Body.Scope
			}

			if found, scope := FindAt(node.SomeArm.Body, line, col); found != nil {
				return found, scope
			}
		}
		if node.NoneArm != nil {
			if found, scope := FindAt(node.NoneArm.Body, line, col); found != nil {
				return found, scope
			}
		}
		if node.OkArm != nil {
			if found, _ := FindAt(node.OkArm.Pattern.Binding, line, col); found != nil {
				return found, node.OkArm.Body.Scope
			}

			if found, scope := FindAt(node.OkArm.Body, line, col); found != nil {
				return found, scope
			}
		}
		if node.ErrArm != nil {
			if found, _ := FindAt(node.ErrArm.Pattern.Binding, line, col); found != nil {
				return found, node.ErrArm.Body.Scope
			}

			if found, scope := FindAt(node.ErrArm.Body, line, col); found != nil {
				return found, scope
			}
		}
	}

	return nil, nil
}

func FindSelectorAt(node Node, line, col int) *SelectorExpr {
	switch node := node.(type) {
	case *SelectorExpr:
		if ident, ok := node.Value.(*Identifier); ok {
			if matches(ident, line, col) {
				return node
			}
		}
		return FindSelectorAt(node.Left, line, col)
	case *Program:
		for _, stmt := range node.Stmts {
			if found := FindSelectorAt(stmt, line, col); found != nil {
				return found
			}
		}
	case *PubStatement:
		return FindSelectorAt(node.Stmt, line, col)
	case *BlockStmt:
		for _, stmt := range node.Stmts {
			if found := FindSelectorAt(stmt, line, col); found != nil {
				return found
			}
		}
	case *ExpressionStmt:
		return FindSelectorAt(node.Expr, line, col)
	case *VarDeclarationStmt:
		if node.Value != nil {
			return FindSelectorAt(node.Value, line, col)
		}
	case *VarAssignmentStmt:
		return FindSelectorAt(node.Value, line, col)
	case *ReturnStmt:
		if node.ReturnValue != nil {
			return FindSelectorAt(node.ReturnValue, line, col)
		}
	case *CallExpr:
		if found := FindSelectorAt(node.Function, line, col); found != nil {
			return found
		}
		for _, arg := range node.Arguments {
			if found := FindSelectorAt(arg, line, col); found != nil {
				return found
			}
		}
	case *InfixExpr:
		if found := FindSelectorAt(node.Left, line, col); found != nil {
			return found
		}
		return FindSelectorAt(node.Right, line, col)
	case *FunctionDeclarationStmt:
		return FindSelectorAt(node.Body, line, col)
	case *FunctionLiteral:
		return FindSelectorAt(node.Body, line, col)
	case *ForStmt:
		if node.Init != nil {
			if found := FindSelectorAt(node.Init, line, col); found != nil {
				return found
			}
		}
		return FindSelectorAt(node.Body, line, col)
	case *ForInStmt:
		return FindSelectorAt(node.Body, line, col)
	case *IfExpr:
		if found := FindSelectorAt(node.Consequence, line, col); found != nil {
			return found
		}
		if node.Alternative != nil {
			return FindSelectorAt(node.Alternative, line, col)
		}
	case *MatchExpr:
		if node.OkArm != nil {
			if found := FindSelectorAt(node.OkArm.Body, line, col); found != nil {
				return found
			}
		}
		if node.ErrArm != nil {
			if found := FindSelectorAt(node.ErrArm.Body, line, col); found != nil {
				return found
			}
		}
		if node.SomeArm != nil {
			if found := FindSelectorAt(node.SomeArm.Body, line, col); found != nil {
				return found
			}
		}
		if node.NoneArm != nil {
			if found := FindSelectorAt(node.NoneArm.Body, line, col); found != nil {
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
