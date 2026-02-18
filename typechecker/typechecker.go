package typechecker

import (
	"fmt"
	"sydney/ast"
	"sydney/types"
)

type Checker struct {
	env               *TypeEnv
	errors            []string
	currentReturnType types.Type
}

func New(globalEnv *TypeEnv) *Checker {
	env := globalEnv
	if env == nil {
		env = &TypeEnv{
			store: make(map[string]types.Type),
			outer: nil,
		}
	}
	errors := make([]string, 0)

	return &Checker{
		env,
		errors,
		nil,
	}
}

func (c *Checker) Errors() []string {
	return c.errors
}

func (c *Checker) Check(node ast.Node) []string {
	c.check(node)
	return c.errors
}

func (c *Checker) check(node ast.Node) types.Type {
	switch node := node.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			c.check(stmt)
		}
		return types.Unit
	case *ast.ExpressionStmt:
		return c.typeOf(node.Expr)
	case *ast.ReturnStmt:
		if c.currentReturnType == nil {
			c.errors = append(c.errors, "return statement outside of function body")
		}

		valType := c.typeOf(node.ReturnValue)

		if !typesMatch(c.currentReturnType, valType) {
			c.errors = append(c.errors, fmt.Sprintf("cannot return %s from function expecting %s", valType.Signature(), c.currentReturnType.Signature()))
		}

		return valType
	case *ast.ForStmt:
		conditionType := c.typeOf(node.Condition)
		if conditionType != types.Bool {
			c.errors = append(c.errors, fmt.Sprintf("cannot use expression of type %s for loop condition", conditionType.Signature()))
		}
		return c.check(node.Body)
	case *ast.VarDeclarationStmt:
		valType := c.typeOf(node.Value)
		if node.Type != nil {
			if !typesMatch(node.Type, valType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot assign %s to variable %s of type %s", valType.Signature(), node.Name.String(), node.Type.Signature()))
			}
			c.env.Set(node.Name.Value, node.Type)
		} else {
			c.env.Set(node.Name.Value, valType)
		}
		return types.Unit
	case *ast.VarAssignmentStmt:
		valType := c.typeOf(node.Value)
		varType, ok := c.env.Get(node.Identifier.Value)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("cannot assign to undefined variable %s", node.Identifier.Value))
		}

		if !typesMatch(varType, valType) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot assign %s to variable %s of type %s", valType.Signature(), node.Identifier.Value, varType.Signature()))
		}

		return types.Unit
	case *ast.IndexAssignmentStmt:
		valType := c.typeOf(node.Value)

		ident, ok := node.Left.Left.(*ast.Identifier)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("cannot assign to %s", node.Left.Left.String()))
		}

		indexOrKeyType := c.typeOf(node.Left.Index)

		t, ok := c.env.Get(ident.Value)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("cannot assign to undefined variable %s", ident.Value))
		}

		switch colType := t.(type) {
		case types.ArrayType:
			if indexOrKeyType != types.Int {
				c.errors = append(c.errors, fmt.Sprintf("index must be type int, got %s", indexOrKeyType.Signature()))
			}

			if !typesMatch(valType, colType.ElemType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot assign %s to element of array %s of type %s", valType.Signature(), ident.Value, colType.Signature()))
			}
		case types.MapType:
			if !typesMatch(indexOrKeyType, colType.KeyType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: key for map %s of type %s must be %s, got %s", ident.Value, colType.Signature(), colType.KeyType.Signature(), indexOrKeyType.Signature()))
			}

			if !typesMatch(valType, colType.ValueType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot assign %s to entry of map %s of type %s", valType.Signature(), ident.Value, valType.Signature()))
			}
		}

		return types.Unit
	case *ast.BlockStmt:
		oldEnv := c.env
		c.env = NewTypeEnv(oldEnv)
		var lastType types.Type = types.Unit
		for _, stmt := range node.Stmts {
			lastType = c.check(stmt)
		}

		c.env = oldEnv

		return lastType
	}

	return types.Unit
}

func (c *Checker) typeOf(expr ast.Expr) types.Type {
	if expr == nil {
		return types.Null
	}

	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		return types.Int
	case *ast.StringLiteral:
		return types.String
	case *ast.FloatLiteral:
		return types.Float
	case *ast.BooleanLiteral:
		return types.Bool
	case *ast.NullLiteral:
		return types.Null
	case *ast.FunctionLiteral:
		oldReturnType := c.currentReturnType
		c.currentReturnType = expr.Type.Return
		oldEnv := c.env
		c.env = NewTypeEnv(oldEnv)

		for i, param := range expr.Parameters {
			c.env.Set(param.Value, expr.Type.Params[i])
		}

		c.check(expr.Body)

		c.env = oldEnv
		c.currentReturnType = oldReturnType

		return expr.Type
	case *ast.ArrayLiteral:
		var elemType types.Type
		for _, element := range expr.Elements {
			eType := c.typeOf(element)
			if eType != nil {
				elemType = eType
				continue
			}
			if !typesMatch(eType, elemType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: array element got %s, expected %s", eType.Signature(), elemType.Signature()))
			}
		}
		isEmpty := len(expr.Elements) == 0

		if elemType == nil {
			return types.ArrayType{ElemType: types.Null, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
		}

		return types.ArrayType{ElemType: elemType, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
	case *ast.HashLiteral:
		var keyType, valType types.Type

		for k, v := range expr.Pairs {
			kType := c.typeOf(k)
			vType := c.typeOf(v)

			if keyType == nil {
				keyType = kType
				valType = vType
				continue
			}

			if !typesMatch(kType, keyType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: map key got %s, expected %s", kType.Signature(), keyType.Signature()))
			}
			if !typesMatch(valType, vType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: map value got %s, expected %s", vType.Signature(), valType.Signature()))
			}
		}

		isEmpty := len(expr.Pairs) == 0
		if keyType == nil {
			return types.MapType{ValueType: types.Null, KeyType: types.Null, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
		}

		return types.MapType{KeyType: keyType, ValueType: valType, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
	case *ast.Identifier:
		t, ok := c.env.Get(expr.Value)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("undefined identifier: %s", expr.Value))
			return nil
		}

		return t
	case *ast.InfixExpr:
		lt := c.typeOf(expr.Left)
		rt := c.typeOf(expr.Right)
		return c.checkInfixExpr(expr.Operator, lt, rt)
	case *ast.PrefixExpr:
		t := c.typeOf(expr.Right)
		return c.checkPrefixExpr(expr.Operator, t)
	case *ast.IfExpr:
		t := c.typeOf(expr.Condition)
		if t != types.Bool {
			c.errors = append(c.errors, fmt.Sprintf("cannot use expression of type %s for if condition", t))
			return nil
		}
		cType := c.check(expr.Consequence)
		var aType types.Type
		if expr.Alternative != nil {
			aType = c.check(expr.Alternative)
		}
		if !typesMatch(cType, aType) {
			c.errors = append(c.errors, fmt.Sprintf("consequence and alternative for if expression must result in same type"))
			return types.Unit
		}
	case *ast.CallExpr:
		fn, ok := c.env.Get(expr.Function.String())
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("unresolved symbol: %s", expr.Function.String()))
			return nil
		}

		fnType, ok := fn.(types.FunctionType)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("cannot call non-function %s %s", fn.Signature(), expr.Function.String()))
			return nil
		}
		for i, arg := range expr.Arguments {
			argType := c.typeOf(arg)
			if !typesMatch(argType, fnType.Params[i]) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: got %s for arg %d in function %s call, expected %s", argType.Signature(), i+1, expr.Function.String(), fnType.Params[i].Signature()))
				return nil
			}
		}
		return fnType.Return
	case *ast.IndexExpr:
		lt := c.typeOf(expr.Left)
		idxT := c.typeOf(expr.Index)
		mt, mok := lt.(types.MapType)
		at, aok := lt.(types.ArrayType)

		if aok {
			if idxT != types.Int {
				c.errors = append(c.errors, fmt.Sprintf("index type for array must be int, got %s", idxT.Signature()))
				return nil
			}

			return at.ElemType
		} else if mok {
			if idxT != mt.KeyType {
				c.errors = append(c.errors, fmt.Sprintf("index type for map %s must be %s, got %s", mt.Signature(), mt.KeyType.Signature(), idxT.Signature()))
				return nil
			}

			return mt.ValueType
		}
		c.errors = append(c.errors, fmt.Sprintf("index operation undefined for type: %s", lt.Signature()))
		return nil
	}
	return nil
}

func (c *Checker) checkInfixExpr(operator string, lt types.Type, rt types.Type) types.Type {
	switch operator {
	case "==":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		return types.Bool
	case ">":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return types.Bool
	case ">=":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return types.Bool
	case "<":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return types.Bool
	case "!=":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		return lt
	case "<=":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		return types.Bool

	case "&&":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot perform boolean operation on types %s and %s", lt.Signature(), rt.Signature()))
		}
		if lt != types.Bool {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}
		return types.Bool
	case "||":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot perform boolean operation on types %s and %s", lt.Signature(), rt.Signature()))
		}
		if lt != types.Bool {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}
		return types.Bool
	case "+":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot add types %s and %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.String && lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return lt

	case "-":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot subtract types %s and %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return lt
	case "*":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot multiply types %s and %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return lt
	case "/":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot divide types %s and %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return lt
	case "%":
		if !typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot modulo types %s and %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return types.Int
	default:
		c.errors = append(c.errors, fmt.Sprintf("unknown operator %s", operator))
		return nil
	}
}

func (c *Checker) checkPrefixExpr(operator string, t types.Type) types.Type {
	if operator == "!" {
		if t != types.Bool {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for %s", operator, t.Signature()))
			return nil
		}
		return t
	} else if operator == "-" {
		if t != types.Float && t != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for %s", operator, t.Signature()))
			return nil
		}

		return t
	}

	c.errors = append(c.errors, fmt.Sprintf("unknown operator %s", operator))
	return nil
}

func typesMatch(actual, expected types.Type) bool {
	if actual == nil || expected == nil {
		return actual == expected
	}
	if _, ok := actual.(types.BasicType); ok {
		if _, ok := expected.(types.BasicType); ok {
			return actual == expected
		}
	}

	// Handle empty collections (e.g., [] matching array<int>)
	if actArr, ok := actual.(types.ArrayType); ok {
		if expectedArr, ok := expected.(types.ArrayType); ok {
			if actArr.IsEmpty {
				return true
			}

			return typesMatch(actArr.ElemType, expectedArr.ElemType)
		}
	}

	if expMap, ok := actual.(types.MapType); ok {
		if actMap, ok := expected.(types.MapType); ok {
			if actMap.IsEmpty {
				return true
			}

			return typesMatch(expMap.KeyType, actMap.KeyType) && typesMatch(expMap.ValueType, actMap.ValueType)
		}
	}

	return actual.Signature() == expected.Signature()

	return false
}
