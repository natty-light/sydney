package evaluator

import (
	"sydney/ast"
	"sydney/object"
)

func DefineMacros(program *ast.Program, scope *object.Scope) {
	definitions := []int{}

	for i, statement := range program.Stmts {
		if isMacroDefinition(statement) {
			addMacro(statement, scope)
			definitions = append(definitions, i)
		}
	}

	for i := len(definitions) - 1; i >= 0; i = i - 1 {
		definitionIndex := definitions[i]
		program.Stmts = append(program.Stmts[:definitionIndex], program.Stmts[definitionIndex+1:]...)
	}
}

// TODO: Update this to allow for things like
// let myMacro = macro(x) { x };
// let anotherName = myMacro;
func isMacroDefinition(node ast.Stmt) bool {
	switch node := node.(type) {
	case *ast.VarDeclarationStmt:
		_, ok := node.Value.(*ast.MacroLiteral)
		if !ok {
			return false
		}
		return true
	case *ast.VarAssignmentStmt:
		_, ok := node.Value.(*ast.MacroLiteral)
		if !ok {
			return false
		}
		return true
	default:
		return false
	}
}

func addMacro(stmt ast.Stmt, scope *object.Scope) {
	switch stmt := stmt.(type) {
	case *ast.VarDeclarationStmt:
		macro := stmt.Value.(*ast.MacroLiteral)
		macroObj := &object.Macro{Parameters: macro.Parameters, Body: macro.Body, Scope: scope}
		scope.Set(stmt.Name.Value, macroObj, stmt.Constant)
	case *ast.VarAssignmentStmt:
		macro := stmt.Value.(*ast.MacroLiteral)
		macroObj := &object.Macro{Parameters: macro.Parameters, Body: macro.Body, Scope: scope}
		scope.Set(stmt.Identifier.Value, macroObj, false)
	}
}

func ExpandMacros(program ast.Node, scope *object.Scope) ast.Node {
	return ast.Modify(program, func(node ast.Node) ast.Node {
		callExpr, ok := node.(*ast.CallExpr)
		if !ok {
			return node
		}

		macro, ok := isMacroCall(callExpr, scope)
		if !ok {
			return node
		}

		args := quoteArgs(callExpr)
		evalScope := extendMacroScope(macro, args)

		evaluated := Eval(macro.Body, evalScope)

		quote, ok := evaluated.(*object.Quote)
		if !ok {
			panic("we only support returning AST-nodes from macros")
		}

		return quote.Node
	})
}

func isMacroCall(expr *ast.CallExpr, scope *object.Scope) (*object.Macro, bool) {
	identifier, ok := expr.Function.(*ast.Identifier)
	if !ok {
		return nil, false
	}

	obj, _, ok := scope.Get(identifier.Value)
	if !ok {
		return nil, false
	}

	macro, ok := obj.Value.(*object.Macro)
	if !ok {
		return nil, false
	}

	return macro, true
}

func quoteArgs(expr *ast.CallExpr) []*object.Quote {
	args := []*object.Quote{}
	for _, a := range expr.Arguments {
		args = append(args, &object.Quote{Node: a})
	}
	return args
}

func extendMacroScope(macro *object.Macro, args []*object.Quote) *object.Scope {
	extended := object.NewEnclosedScope(macro.Scope)

	for paramIdx, param := range macro.Parameters {
		extended.Set(param.Value, args[paramIdx], false)
	}

	return extended
}
