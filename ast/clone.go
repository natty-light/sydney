package ast

func Clone(n Node) *Program {
	nn := &Program{Stmts: make([]Stmt, 0)}

	switch node := n.(type) {
	case *Program:
		for _, stmt := range node.Stmts {
			nn.Stmts = append(nn.Stmts, cloneStmt(stmt))
		}
	}

	return nn
}

func cloneStmt(stmt Stmt) Stmt {
	switch stmt := stmt.(type) {
	case *ExpressionStmt:
		cloned := *stmt
		expr := cloneExpr(stmt.Expr)
		cloned.Expr = expr
		return &cloned
	case *ReturnStmt:
		cloned := *stmt
		expr := cloneExpr(stmt.ReturnValue)
		cloned.ReturnValue = expr
		return &cloned
	case *BlockStmt:
		return cloneBlockStmt(stmt)
	case *VarDeclarationStmt:
		cloned := *stmt
		expr := cloneExpr(stmt.Value)
		cloned.Value = expr
		return &cloned
	case *VarAssignmentStmt:
		cloned := *stmt
		cloned.Value = cloneExpr(stmt.Value)
		cloned.Identifier = cloneExpr(stmt.Identifier).(*Identifier)
		return &cloned
	case *StructDefinitionStmt:
		cloned := *stmt
		return &cloned
	case *FunctionDeclarationStmt:
		cloned := *stmt
		cloned.Name = cloneIdentifier(stmt.Name)
		cloned.Params = make([]*Identifier, len(stmt.Params))
		for i, p := range stmt.Params {
			cloned.Params[i] = cloneIdentifier(p)
		}
		cloned.Body = cloneStmt(stmt.Body).(*BlockStmt)
		return &cloned
	case *ForStmt:
		cloned := *stmt
		if stmt.Init != nil {
			cloned.Init = cloneStmt(stmt.Init)
		}
		cloned.Condition = cloneExpr(stmt.Condition)
		if stmt.Post != nil {
			cloned.Post = cloneStmt(stmt.Post)
		}
		cloned.Body = cloneBlockStmt(stmt.Body)
		return &cloned
	case *IndexAssignmentStmt:
		cloned := *stmt
		cloned.Left = cloneIndexExpr(stmt.Left)
		cloned.Value = cloneExpr(stmt.Value)
		return &cloned
	case *SelectorAssignmentStmt:
		cloned := *stmt
		cloned.Left = cloneSelectorExpr(stmt.Left)
		cloned.Value = cloneExpr(stmt.Value)
		return &cloned
	case *BreakStmt:
		cloned := *stmt
		return &cloned
	case *ContinueStmt:
		cloned := *stmt
		return &cloned
	}
	return nil
}

func cloneBlockStmt(stmt *BlockStmt) *BlockStmt {
	cloned := *stmt
	cloned.Stmts = make([]Stmt, len(stmt.Stmts))
	for i, s := range stmt.Stmts {
		cloned.Stmts[i] = cloneStmt(s)
	}
	cloned.Scope = stmt.Scope

	return &cloned
}

func cloneIdentifier(expr *Identifier) *Identifier {
	cloned := *expr
	return &cloned
}

func cloneIndexExpr(expr *IndexExpr) *IndexExpr {
	cloned := *expr
	cloned.Left = cloneExpr(expr.Left)
	cloned.Index = cloneExpr(expr.Index)
	return &cloned
}

func cloneSelectorExpr(expr *SelectorExpr) *SelectorExpr {
	cloned := *expr
	cloned.Left = cloneExpr(expr.Left)
	cloned.Value = cloneExpr(expr.Value)
	return &cloned
}

func cloneExpr(e Expr) Expr {
	switch expr := e.(type) {
	case *Identifier:
		cloned := *expr
		return &cloned
	case *IntegerLiteral:
		cloned := *expr
		return &cloned
	case *FloatLiteral:
		cloned := *expr
		return &cloned
	case *StringLiteral:
		cloned := *expr
		return &cloned
	case *BooleanLiteral:
		cloned := *expr
		return &cloned
	case *NullLiteral:
		cloned := *expr
		return &cloned
	case *ByteLiteral:
		cloned := *expr
		return &cloned
	case *PrefixExpr:
		cloned := *expr
		cloned.Right = cloneExpr(expr.Right)
		return &cloned
	case *InfixExpr:
		cloned := *expr
		cloned.Right = cloneExpr(expr.Right)
		cloned.Left = cloneExpr(expr.Left)
		return &cloned
	case *IfExpr:
		cloned := *expr
		cloned.Condition = cloneExpr(expr.Condition)
		cloned.Consequence = cloneBlockStmt(expr.Consequence)
		if expr.Alternative != nil {
			cloned.Alternative = cloneBlockStmt(expr.Alternative)
		}
		return &cloned
	case *CallExpr:
		cloned := *expr
		cloned.Arguments = make([]Expr, len(expr.Arguments))
		for i, a := range expr.Arguments {
			cloned.Arguments[i] = cloneExpr(a)
		}
		cloned.Function = cloneExpr(expr.Function)
		return &cloned
	case *IndexExpr:
		cloned := *expr
		cloned.Left = cloneExpr(expr.Left)
		cloned.Index = cloneExpr(expr.Index)
		return &cloned
	case *SelectorExpr:
		return cloneSelectorExpr(expr)
	case *FunctionLiteral:
		cloned := *expr
		cloned.Parameters = make([]*Identifier, len(expr.Parameters))
		for i, p := range expr.Parameters {
			cloned.Parameters[i] = cloneIdentifier(p)
		}
		cloned.Body = cloneBlockStmt(expr.Body)
		return &cloned
	case *ArrayLiteral:
		cloned := *expr
		cloned.Elements = make([]Expr, len(expr.Elements))
		for i, e := range expr.Elements {
			cloned.Elements[i] = cloneExpr(e)
		}
		return &cloned
	case *HashLiteral:
		cloned := *expr
		cloned.Pairs = make(map[Expr]Expr)
		for k, v := range expr.Pairs {
			ck := cloneExpr(k)
			cv := cloneExpr(v)
			cloned.Pairs[ck] = cv
		}
		return &cloned
	case *StructLiteral:
		cloned := *expr
		cloned.Values = make([]Expr, len(expr.Values))
		for i, v := range expr.Values {
			cloned.Values[i] = cloneExpr(v)
		}
		cloned.Fields = make([]string, len(expr.Fields))
		for i, f := range expr.Fields {
			cloned.Fields[i] = f
		}
		return &cloned
	case *ScopeAccessExpr:
		cloned := *expr
		cloned.Module = cloneIdentifier(expr.Module)
		cloned.Member = cloneIdentifier(expr.Member)
		return &cloned
	case *MatchExpr:
		cloned := *expr
		if expr.OkArm != nil {
			cok := *expr.OkArm
			cokp := *cok.Pattern
			if expr.OkArm.Pattern.Binding != nil {
				cokp.Binding = cloneIdentifier(expr.OkArm.Pattern.Binding)
			}
			cok.Pattern = &cokp
			cok.Body = cloneBlockStmt(expr.OkArm.Body)
			cloned.OkArm = &cok
		}

		if expr.ErrArm != nil {
			eok := *expr.ErrArm
			eokp := *eok.Pattern
			if expr.ErrArm.Pattern.Binding != nil {
				eokp.Binding = cloneIdentifier(expr.ErrArm.Pattern.Binding)
			}
			eok.Pattern = &eokp
			eok.Body = cloneBlockStmt(expr.ErrArm.Body)
			cloned.ErrArm = &eok
		}

		if expr.SomeArm != nil {
			csome := *expr.SomeArm
			csomep := *csome.Pattern
			if expr.SomeArm.Pattern.Binding != nil {
				csomep.Binding = cloneIdentifier(expr.SomeArm.Pattern.Binding)
			}
			csome.Pattern = &csomep
			csome.Body = cloneBlockStmt(expr.SomeArm.Body)
			cloned.SomeArm = &csome
		}

		if expr.NoneArm != nil {
			cnone := *expr.NoneArm
			cnonep := *cnone.Pattern
			cnone.Pattern = &cnonep
			cnone.Body = cloneBlockStmt(expr.NoneArm.Body)
			cloned.NoneArm = &cnone
		}

		return &cloned
	}
	return nil
}

func FilterGenericTemplates(program *Program) {
	var stmts []Stmt
	for _, stmt := range program.Stmts {
		switch s := stmt.(type) {
		case *FunctionDeclarationStmt:
			if len(s.TypeParams) > 0 {
				continue
			}
		case *StructDefinitionStmt:
			if len(s.Type.TypeParams) > 0 {
				continue
			}
		case *PubStatement:
			if fd, ok := s.Stmt.(*FunctionDeclarationStmt); ok && len(fd.TypeParams) > 0 {
				continue
			}
			if sd, ok := s.Stmt.(*StructDefinitionStmt); ok && len(sd.Type.TypeParams) > 0 {
				continue
			}
		}
		stmts = append(stmts, stmt)
	}
	program.Stmts = stmts
}
