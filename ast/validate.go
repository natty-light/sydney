package ast

import (
	"fmt"
	"runtime/debug"
	"strings"
)

func AssertPostMonomorphization(program *Program) {
	for _, stmt := range program.Stmts {
		assertStmt(stmt)
	}
}

func assertStmt(stmt Stmt) {
	switch s := stmt.(type) {
	case *FunctionDeclarationStmt:
		if len(s.TypeParams) > 0 {
			panic(fmt.Sprintf("invariant violation: function %q has TypeParams after monomorphization\n%s",
				s.Name.Value, debug.Stack()))
		}
		assertBlock(s.Body)
	case *StructDefinitionStmt:
		if len(s.Type.TypeParams) > 0 {
			panic(fmt.Sprintf("invariant violation: struct %q has TypeParams after monomorphization\n%s",
				s.Name.Value, debug.Stack()))
		}
	case *PubStatement:
		assertStmt(s.Stmt)
	case *ExpressionStmt:
		assertExpr(s.Expr)
	case *VarDeclarationStmt:
		if s.Value != nil {
			assertExpr(s.Value)
		}
	case *ReturnStmt:
		if s.ReturnValue != nil {
			assertExpr(s.ReturnValue)
		}
	case *VarAssignmentStmt:
		assertExpr(s.Value)
	case *ForStmt:
		if s.Init != nil {
			assertStmt(s.Init)
		}
		if s.Condition != nil {
			assertExpr(s.Condition)
		}
		if s.Post != nil {
			assertStmt(s.Post)
		}
		assertBlock(s.Body)
	case *ForInStmt:
		assertExpr(s.Iterable)
		assertBlock(s.Body)
	case *BlockStmt:
		assertBlock(s)
	}
}

func assertBlock(block *BlockStmt) {
	if block == nil {
		return
	}
	for _, stmt := range block.Stmts {
		assertStmt(stmt)
	}
}

func assertExpr(expr Expr) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *CallExpr:
		if e.MangledName != "" && !strings.Contains(e.MangledName, "__") && !strings.Contains(e.MangledName, ".") {
			panic(fmt.Sprintf("invariant violation: CallExpr has MangledName %q which is not mangled\n%s",
				e.MangledName, debug.Stack()))
		}
		assertExpr(e.Function)
		for _, arg := range e.Arguments {
			assertExpr(arg)
		}
	case *StructLiteral:
		if len(e.TypeArgs) > 0 && !strings.Contains(e.Name, "__") {
			panic(fmt.Sprintf("invariant violation: StructLiteral %q has TypeArgs but name is not mangled\n%s",
				e.Name, debug.Stack()))
		}
		for _, v := range e.Values {
			assertExpr(v)
		}
	case *InfixExpr:
		assertExpr(e.Left)
		assertExpr(e.Right)
	case *PrefixExpr:
		assertExpr(e.Right)
	case *IfExpr:
		assertExpr(e.Condition)
		assertBlock(e.Consequence)
		if e.Alternative != nil {
			assertBlock(e.Alternative)
		}
	case *FunctionLiteral:
		assertBlock(e.Body)
	case *IndexExpr:
		assertExpr(e.Left)
		assertExpr(e.Index)
	case *SelectorExpr:
		assertExpr(e.Left)
	case *ArrayLiteral:
		for _, el := range e.Elements {
			assertExpr(el)
		}
	case *HashLiteral:
		for k, v := range e.Pairs {
			assertExpr(k)
			assertExpr(v)
		}
	case *MatchExpr:
		assertExpr(e.Subject)
		if e.OkArm != nil {
			assertBlock(e.OkArm.Body)
		}
		if e.ErrArm != nil {
			assertBlock(e.ErrArm.Body)
		}
		if e.SomeArm != nil {
			assertBlock(e.SomeArm.Body)
		}
		if e.NoneArm != nil {
			assertBlock(e.NoneArm.Body)
		}
	case *MatchTypeExpr:
		assertExpr(e.Subject)
		for _, arm := range e.Arms {
			assertBlock(arm.Body)
		}
		if e.Default != nil {
			assertBlock(e.Default)
		}
	}
}
