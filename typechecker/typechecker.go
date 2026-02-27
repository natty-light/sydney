package typechecker

import (
	"fmt"
	"slices"
	"sort"
	"sydney/ast"
	"sydney/object"
	"sydney/types"
)

type Checker struct {
	env               *TypeEnv
	errors            []string
	currentReturnType types.Type
	definedStructs    map[string]types.StructType
}

func New(globalEnv *TypeEnv) *Checker {
	env := globalEnv
	if env == nil {
		env = NewTypeEnv(nil)
	}
	for _, v := range object.Builtins {
		env.Set(v.Name, v.BuiltIn.T)
	}

	errors := make([]string, 0)

	return &Checker{
		env,
		errors,
		nil,
		make(map[string]types.StructType),
	}
}

func (c *Checker) Errors() []string {
	return c.errors
}

func (c *Checker) Check(node ast.Node) []string {
	c.check(node)
	return c.errors
}

func (c *Checker) check(n ast.Node) types.Type {
	switch node := n.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			c.hoistBase(stmt)
		}

		for _, stmt := range node.Stmts {
			c.hoistImplementationIntent(stmt)
		}

		for _, stmt := range node.Stmts {
			c.hoistFunctions(stmt)
		}

		for _, stmt := range node.Stmts {
			c.hoistImplementations(stmt)
		}

		for _, stmt := range node.Stmts {
			c.check(stmt)
		}
		return types.Unit
	case *ast.ExpressionStmt:
		return c.typeOf(node.Expr, nil)
	case *ast.ReturnStmt:
		if c.currentReturnType == nil {
			c.errors = append(c.errors, "return statement outside of function body")
		}

		valType := c.typeOf(node.ReturnValue, c.currentReturnType)

		if !c.typesMatch(valType, c.currentReturnType) {
			c.errors = append(c.errors, fmt.Sprintf("cannot return %s from function expecting %s", valType.Signature(), c.currentReturnType.Signature()))
		} else {
			c.boxIfNecessary(node.ReturnValue, valType, c.currentReturnType)
		}
		return valType
	case *ast.ForStmt:
		conditionType := c.typeOf(node.Condition, types.Bool)
		if conditionType != types.Bool {
			c.errors = append(c.errors, fmt.Sprintf("cannot use expression of type %s for loop condition", conditionType.Signature()))
		}
		return c.check(node.Body)
	case *ast.VarDeclarationStmt:
		name := node.Name.Value
		varType, outer, ok := c.env.Get(name)
		valType := c.typeOf(node.Value, varType)
		if node.Type == nil {
			node.Type = valType
		}

		if ok && !outer {
			if !c.typesMatch(valType, varType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot assign %s to variable %s of type %s", valType.Signature(), node.Name.String(), node.Type.Signature()))
			}
		} else {
			c.boxIfNecessary(node.Value, valType, varType)
			c.env.Set(name, valType)
			if node.Constant {
				c.env.SetConst(name)
			}
		}
		n = node
	case *ast.VarAssignmentStmt:
		name := node.Identifier.Value
		varType, _, ok := c.env.Get(name)
		valType := c.typeOf(node.Value, varType)
		isConst := c.env.IsConst(name)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("cannot assign to undefined variable %s", name))
		}
		if isConst {
			c.errors = append(c.errors, fmt.Sprintf("cannot assign to constant variable %s", name))
		}

		if !c.typesMatch(varType, valType) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot assign %s to variable %s of type %s", valType.Signature(), name, varType.Signature()))
		}
		c.boxIfNecessary(node.Value, valType, varType)

		return types.Unit
	case *ast.IndexAssignmentStmt:
		return c.checkIndexAssignment(node)
	case *ast.BlockStmt:
		oldEnv := c.env
		c.env = NewTypeEnv(oldEnv)
		var lastType types.Type = types.Unit
		for _, stmt := range node.Stmts {
			lastType = c.check(stmt)
		}

		c.env = oldEnv

		return lastType
	case *ast.FunctionDeclarationStmt:
		return c.checkFunctionDeclaration(n, node)
	case *ast.SelectorAssignmentStmt:
		str := c.typeOf(node.Left.Left, nil)

		structType, ok := toStruct(str)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("cannot assign to field of non-struct value of type %s", str.Signature()))
			return types.Unit
		}

		field := node.Left.Value.(*ast.Identifier)
		if !ok {
			return types.Unit
		}

		idx := slices.Index(structType.Fields, field.Value)
		if idx == -1 {
			c.errors = append(c.errors, fmt.Sprintf("struct %s of type %s has no field %s", structType.Name, structType.Name, field.Value))
			return types.Unit
		}

		valType := c.typeOf(node.Value, structType.Types[idx])

		if !c.typesMatch(valType, structType.Types[idx]) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot assign %s to struct %s field of type %s", valType.Signature(), structType.Name, structType.Types[idx].Signature()))
		}

		n.(*ast.SelectorAssignmentStmt).Left.ResolvedType = structType

		return types.Unit
	}

	return types.Unit
}

func (c *Checker) hoistBase(n ast.Node) {
	switch node := n.(type) {
	case *ast.StructDefinitionStmt:
		c.definedStructs[node.Name.Value] = node.Type
	case *ast.InterfaceDefinitionStmt:
		node.Type.MethodIndices = make(map[string]int)
		for i, mn := range node.Type.Methods {
			node.Type.MethodIndices[mn] = i
		}
		c.env.Set(node.Name.Value, node.Type)
		n = node
	case *ast.VarDeclarationStmt:
		name := node.Name.Value
		if node.Type != nil {
			// Only check the current scope's store to allow shadowing
			if _, exists := c.env.store[name]; !exists {
				c.env.Set(name, node.Type)
				if node.Constant {
					c.env.SetConst(name)
				}
			} else {
				c.errors = append(c.errors, fmt.Sprintf("variable %s already declared", name))
			}
		}
	}
}

func (c *Checker) hoistFunctions(n ast.Node) {
	if node, ok := n.(*ast.FunctionDeclarationStmt); ok {
		fType := node.Type.(types.FunctionType)
		name := node.Name.Value
		if len(fType.Params) > 0 {
			receiverType := fType.Params[0]
			if sn, ok := c.isInterfaceMethod(receiverType, name); ok {
				name = fmt.Sprintf("%s.%s", sn, name)
				node.MangledName = name
				n = node
			}
		}
		_, fromOuter, exists := c.env.Get(node.Name.Value)
		if exists && !fromOuter {
			c.errors = append(c.errors, fmt.Sprintf("function %s already declared", node.Name.Value))
			return
		}

		c.env.Set(name, node.Type)
	}
}

func (c *Checker) hoistImplementationIntent(n ast.Node) {
	if node, ok := n.(*ast.InterfaceImplementationStmt); ok {
		c.registerImplementation(node)
	}
}

func (c *Checker) hoistImplementations(n ast.Node) {
	if node, ok := n.(*ast.InterfaceImplementationStmt); ok {
		c.validateImplementation(node)
	}
}

func (c *Checker) typeOf(e ast.Expr, expectedType types.Type) types.Type {
	if e == nil {
		return types.Null
	}

	switch expr := e.(type) {
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

		if expr.Name != "" {
			c.env.Set(expr.Name, expr.Type)
		}

		for i, param := range expr.Parameters {
			c.env.Set(param.Value, expr.Type.Params[i])
		}

		c.check(expr.Body)

		c.env = oldEnv
		c.currentReturnType = oldReturnType

		return expr.Type
	case *ast.ArrayLiteral:
		var targetedElemType types.Type
		if expected, ok := toArray(expectedType); ok {
			targetedElemType = expected.ElemType
		}

		var elemType types.Type
		for i, element := range expr.Elements {
			eType := c.typeOf(element, targetedElemType)
			if elemType == nil {
				elemType = eType
			}

			if targetedElemType != nil && !c.typesMatch(eType, targetedElemType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: array element got %s, expected %s", eType.Signature(), targetedElemType.Signature()))
				continue
			}

			c.boxIfNecessary(expr.Elements[i], eType, targetedElemType)
		}
		isEmpty := len(expr.Elements) == 0

		if elemType == nil {
			return types.ArrayType{ElemType: types.Null, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
		}

		return types.ArrayType{ElemType: elemType, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
	case *ast.HashLiteral:
		expected, _ := toMap(expectedType)
		var keyType, valType types.Type

		keys := make([]ast.Expr, 0, len(expr.Pairs))
		for k := range expr.Pairs {
			keys = append(keys, k)
		}

		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})

		for _, k := range keys {
			v := expr.Pairs[k]
			kType := c.typeOf(k, expected.KeyType)
			vType := c.typeOf(v, expected.ValueType)

			if keyType == nil {
				keyType = kType
				valType = vType
				continue
			}

			if !c.typesMatch(kType, keyType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: map key got %s, expected %s", kType.Signature(), keyType.Signature()))
			}
			if !c.typesMatch(valType, vType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch: map value got %s, expected %s", vType.Signature(), valType.Signature()))
			}
			c.boxIfNecessary(k, kType, keyType)
			c.boxIfNecessary(v, vType, valType)
		}

		isEmpty := len(expr.Pairs) == 0
		if keyType == nil {
			return types.MapType{ValueType: types.Null, KeyType: types.Null, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
		}

		return types.MapType{KeyType: keyType, ValueType: valType, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
	case *ast.Identifier:
		t, _, ok := c.env.Get(expr.Value)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("undefined identifier: %s", expr.Value))
			return types.Unit
		}
		expr.ResolvedType = t
		e = expr
		return t
	case *ast.InfixExpr:
		lt := c.typeOf(expr.Left, nil)
		rt := c.typeOf(expr.Right, nil)
		return c.checkInfixExpr(expr.Operator, lt, rt)
	case *ast.PrefixExpr:
		t := c.typeOf(expr.Right, nil)
		return c.checkPrefixExpr(expr.Operator, t)
	case *ast.IfExpr:
		t := c.typeOf(expr.Condition, types.Bool)
		if t != types.Bool {
			c.errors = append(c.errors, fmt.Sprintf("cannot use expression of type %s for if condition", t))
			return nil
		}
		cType := c.check(expr.Consequence)
		var aType types.Type
		if expr.Alternative != nil {
			aType = c.check(expr.Alternative)

			if !c.typesMatch(cType, aType) {
				c.errors = append(c.errors, fmt.Sprintf("consequence and alternative for if expression must result in same type"))
				return types.Unit
			}
		}

		return cType
	case *ast.CallExpr:
		t := c.typeOfCallExpr(expr)
		e = expr
		return t
	case *ast.IndexExpr:
		return c.checkIndexExpr(e, expr)
	case *ast.SelectorExpr:
		t := c.typeOf(expr.Left, nil)
		structType, ok := t.(types.StructType)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("cannot access field of non-struct value %s of type %s", expr.Left.TokenLiteral(), t.Signature()))
		}
		val, ok := expr.Value.(*ast.Identifier)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("idk what to put here I'll figure it out later"))
			return types.Unit
		}
		i := slices.Index(structType.Fields, val.Value)
		if i == -1 {
			c.errors = append(c.errors, fmt.Sprintf("field %s of struct type %s not found", val.Value, expr.Left.TokenLiteral()))
			return types.Unit
		}

		expr.ResolvedType = structType
		e = expr
		return structType.Types[i]
	case *ast.StructLiteral:
		structType, ok := c.definedStructs[expr.Name]
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("unknown type %s", expr.Name))
			return types.Unit
		}

		providedFields := make(map[string]ast.Expr)
		for i, name := range expr.Fields {
			providedFields[name] = expr.Values[i]
		}

		for _, expected := range structType.Fields {
			if _, ok := providedFields[expected]; !ok {
				c.errors = append(c.errors, fmt.Sprintf("missing field %s in struct literal %s", expected, expr.Name))
			}
		}

		for i, fieldName := range expr.Fields {
			idx := slices.Index(structType.Fields, fieldName)
			if idx == -1 {
				c.errors = append(c.errors, fmt.Sprintf("field %s of struct type %s not found", fieldName, expr.Name))
				continue
			}

			expectedType := structType.Types[idx]
			actualType := c.typeOf(expr.Values[i], expectedType)
			if !c.typesMatch(actualType, expectedType) {
				c.errors = append(c.errors, fmt.Sprintf("type mismatch for field %s in struct %s: expected %s, got %s", fieldName, expr.Name, expectedType.Signature(), actualType.Signature()))
			} else {
				c.boxIfNecessary(expr.Values[i], actualType, expectedType)
			}
		}
		e.(*ast.StructLiteral).ResolvedType = structType

		return structType
	}
	return nil
}

func (c *Checker) checkInfixExpr(operator string, lt types.Type, rt types.Type) types.Type {
	switch operator {
	case "==":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		return types.Bool
	case ">":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return types.Bool
	case ">=":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return types.Bool
	case "<":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return types.Bool
	case "!=":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		return types.Bool
	case "<=":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()))
		}

		return types.Bool

	case "&&":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot perform boolean operation on types %s and %s", lt.Signature(), rt.Signature()))
		}
		if lt != types.Bool {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}
		return types.Bool
	case "||":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot perform boolean operation on types %s and %s", lt.Signature(), rt.Signature()))
		}
		if lt != types.Bool {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}
		return types.Bool
	case "+":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot add types %s and %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.String && lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return lt

	case "-":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot subtract types %s and %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return lt
	case "*":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot multiply types %s and %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return lt
	case "/":
		if !c.typesMatch(lt, rt) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot divide types %s and %s", lt.Signature(), rt.Signature()))
		}

		if lt != types.Float && lt != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()))
		}

		return lt
	case "%":
		if !c.typesMatch(lt, rt) {
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

func (c *Checker) typeOfCallExpr(expr *ast.CallExpr) types.Type {
	if ident, ok := expr.Function.(*ast.Identifier); ok {
		if len(expr.Arguments) > 0 {
			receiverType := c.typeOf(expr.Arguments[0], nil)
			mangled := fmt.Sprintf("%s.%s", receiverType.Signature(), ident.Value)

			if t, _, ok := c.env.Get(mangled); ok {
				return c.validateFunctionCall(expr, t)
			}
		}
	}

	if selector, ok := expr.Function.(*ast.SelectorExpr); ok {
		receiverType := c.typeOf(selector.Left, nil)
		methodName := selector.Value.(*ast.Identifier).Value
		mangled := fmt.Sprintf("%s.%s", receiverType.Signature(), methodName)
		if t, _, ok := c.env.Get(mangled); ok {
			expr.Arguments = append([]ast.Expr{selector.Left}, expr.Arguments...)
			expr.MangledName = mangled
			return c.validateFunctionCall(expr, t)
		}

		if it, ok := toInterface(receiverType); ok {
			for i, m := range it.Methods {
				if m == methodName {
					return c.validateFunctionCall(expr, it.Types[i])
				}
			}
		}
	}

	fnTypeRaw := c.typeOf(expr.Function, nil)
	if ident, ok := expr.Function.(*ast.Identifier); ok {
		switch ident.Value {
		case "len":
			return c.checkLenBuiltIn(expr)
		case "print":
			return c.checkPrintBuiltIn(expr)
		case "first":
			return c.checkFirstBuiltIn(expr)
		case "last":
			return c.checkLastBuiltIn(expr)
		case "append":
			return c.checkAppendBuiltIn(expr)
		case "rest":
			return c.checkRestBuiltIn(expr)
		case "slice":
			return c.checkSliceBuiltIn(expr)
		case "keys":
			return c.checkKeysBuiltIn(expr)
		case "values":
			return c.checkValuesBuiltIn(expr)

		}
	}

	return c.validateFunctionCall(expr, fnTypeRaw)
}

func (c *Checker) checkLenBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.errors = append(c.errors, fmt.Sprintf("len() expects exactly 1 argument"))
		return types.Int
	}

	argType := c.typeOf(expr.Arguments[0], nil)
	_, isArray := argType.(types.ArrayType)
	_, isMap := argType.(types.MapType)
	if argType != types.String && !isArray && !isMap && !isString(argType) {
		c.errors = append(c.errors, fmt.Sprintf("invalid argument type %s for len()", argType.Signature()))
	}

	return types.Int
}

func (c *Checker) checkPrintBuiltIn(_ *ast.CallExpr) types.Type {
	return types.Unit
}

func (c *Checker) checkFirstBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.errors = append(c.errors, fmt.Sprintf("first() expects exactly 1 argument"))
		return types.Int
	}

	argType := c.typeOf(expr.Arguments[0], nil)
	arrType, isArray := argType.(types.ArrayType)

	if !isArray {
		c.errors = append(c.errors, fmt.Sprintf("invalid argument type %s for first()", argType.Signature()))
		return types.Unit
	}

	return arrType.ElemType
}

func (c *Checker) checkLastBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.errors = append(c.errors, fmt.Sprintf("last() expects exactly 1 argument"))
		return types.Int
	}

	argType := c.typeOf(expr.Arguments[0], nil)
	arrType, isArray := argType.(types.ArrayType)

	if !isArray {
		c.errors = append(c.errors, fmt.Sprintf("invalid argument type %s for last()", argType.Signature()))
		return types.Unit
	}

	return arrType.ElemType
}

func (c *Checker) checkAppendBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 2 {
		c.errors = append(c.errors, fmt.Sprintf("append() expects exactly 2 arguments"))
		return types.Unit
	}
	argType := c.typeOf(expr.Arguments[0], nil)

	arrType, isArray := argType.(types.ArrayType)

	if !isArray {
		c.errors = append(c.errors, fmt.Sprintf("invalid argument type %s for append()", argType.Signature()))
		return types.Unit
	}

	valType := c.typeOf(expr.Arguments[1], nil)
	if !c.typesMatch(valType, arrType.ElemType) && !arrType.IsEmpty {
		c.errors = append(c.errors, fmt.Sprintf("type mismatch: got %s for append() value", valType.Signature()))
	}

	return arrType
}

func (c *Checker) checkRestBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.errors = append(c.errors, fmt.Sprintf("rest() expects exactly 1 argument"))
	}

	argType := c.typeOf(expr.Arguments[0], nil)
	arrType, isArray := argType.(types.ArrayType)
	if !isArray {
		c.errors = append(c.errors, fmt.Sprintf("invalid argument type %s for rest()", argType.Signature()))
		return types.Unit
	}

	return arrType
}

func (c *Checker) checkSliceBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 3 {
		c.errors = append(c.errors, fmt.Sprintf("slice() expects exactly 3 arguments"))
	}
	arrayArgType := c.typeOf(expr.Arguments[0], nil)
	startType := c.typeOf(expr.Arguments[1], types.Int)
	endType := c.typeOf(expr.Arguments[2], types.Int)
	arrType, isArray := arrayArgType.(types.ArrayType)
	if !isArray {
		c.errors = append(c.errors, fmt.Sprintf("invalid argument type %s for slice()", arrType.Signature()))
		return types.Unit
	}

	if startType != types.Int {
		c.errors = append(c.errors, fmt.Sprintf("invalid argument type %s for slice()", arrType.Signature()))
	}

	if endType != types.Int {
		c.errors = append(c.errors, fmt.Sprintf("invalid argument type %s for slice()", arrType.Signature()))
	}

	return arrType
}

func (c *Checker) checkKeysBuiltIn(expr ast.Expr) types.Type {
	t := c.typeOf(expr, nil)
	mapType, ok := t.(types.MapType)
	if !ok {
		c.errors = append(c.errors, fmt.Sprintf("invalid argument type %s for keys()", t.Signature()))
	}

	return &types.ArrayType{ElemType: mapType.KeyType}
}

func (c *Checker) checkValuesBuiltIn(expr ast.Expr) types.Type {
	t := c.typeOf(expr, nil)
	mapType, ok := t.(types.MapType)
	if !ok {
		c.errors = append(c.errors, fmt.Sprintf("invalid argument type %s for keys()", t.Signature()))
	}

	return &types.ArrayType{ElemType: mapType.ValueType}
}

func (c *Checker) registerImplementation(node *ast.InterfaceImplementationStmt) {
	structName := node.StructName.Value
	structType, ok := c.definedStructs[structName]
	if !ok {
		c.errors = append(c.errors, fmt.Sprintf("unknown struct %s", structName))
	}

	for _, name := range node.InterfaceNames {
		interfaceTypeRaw, _, ok := c.env.Get(name.Value)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("unknown interface %s", name.Value))
			continue
		}
		interfaceType, ok := interfaceTypeRaw.(types.InterfaceType)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("non-interface value %s of type %s", name.Value, interfaceTypeRaw.Signature()))
			continue
		}

		structType.Interfaces = append(structType.Interfaces, interfaceType)
	}

	c.definedStructs[structName] = structType
}

func (c *Checker) validateImplementation(node *ast.InterfaceImplementationStmt) {
	structName := node.StructName.Value
	structType, _ := c.definedStructs[structName]

	for _, name := range node.InterfaceNames {
		interfaceTypeRaw, _, _ := c.env.Get(name.Value)
		interfaceType, _ := interfaceTypeRaw.(types.InterfaceType)

		if !c.structSatisfiesInterface(structType, interfaceType) {
			c.errors = append(c.errors, fmt.Sprintf("struct %s does not satisfy interface %s", structName, name.Value))
			continue
		}
	}

}

func (c *Checker) isInterfaceMethod(t types.Type, name string) (string, bool) {
	structType, ok := toStruct(t)
	if !ok {
		return "", false
	}

	sn := structType.Name
	withInterfaces, ok := c.definedStructs[sn]
	if !ok {
		c.errors = append(c.errors, fmt.Sprintf("unknown struct %s", name))
	}

	for _, interfaceRaw := range withInterfaces.Interfaces {
		interfaceType, _ := toInterface(interfaceRaw)
		for _, mn := range interfaceType.Methods {
			if mn == name {
				return sn, true
			}
		}
	}

	return "", false
}

func (c *Checker) validateFunctionCall(expr *ast.CallExpr, fnTypeRaw types.Type) types.Type {
	if fnTypeRaw == nil || fnTypeRaw == types.Unit {
		c.errors = append(c.errors, fmt.Sprintf("unresolved symbol: %s", expr.Function.String()))
		return nil
	}

	fnType, ok := fnTypeRaw.(types.FunctionType)
	if !ok {
		c.errors = append(c.errors, fmt.Sprintf("cannot call non-function %s %s", fnTypeRaw.Signature(), expr.Function.String()))
		return nil
	}
	if len(expr.Arguments) != len(fnType.Params) || fnType.Variadic {
		c.errors = append(c.errors, fmt.Sprintf("wrong number of arguments for function %s, wanted %d, got %d", expr.Function.String(), len(expr.Arguments), len(fnType.Params)))
	}

	for i, arg := range expr.Arguments {
		aType := c.typeOf(arg, nil)
		pType := fnType.Params[i]
		if !c.typesMatch(aType, pType) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: got %s for arg %d in function %s call, expected %s", aType.Signature(), i+1, expr.Function.String(), fnType.Params[i].Signature()))
			return nil
		}
		c.boxIfNecessary(expr.Arguments[i], aType, pType)
	}
	expr.ResolvedType = fnType.Return
	return fnType.Return
}

func (c *Checker) structSatisfiesInterface(s types.StructType, i types.InterfaceType) bool {
	satisfies := true
	for idx, method := range i.Methods {
		emt := i.Types[idx].(types.FunctionType)

		mangledName := fmt.Sprintf("%s.%s", s.Name, method)
		mtr, _, ok := c.env.Get(mangledName)
		if !ok {
			c.errors = append(c.errors, fmt.Sprintf("struct %s does not satisfy interface %s, missing method %s", s.Name, i.Name, method))
			satisfies = false
			continue
		}

		mt := mtr.(types.FunctionType)
		if !c.compareMethodSignature(mt, emt) {
			c.errors = append(c.errors, fmt.Sprintf("struct %s does not satisfy interface %s, wrong signature for method %s. got %s, want %s", s.Name, i.Name, method, mt.Signature(), emt.Signature()))
			satisfies = false
			continue
		}
	}

	return satisfies
}

func (c *Checker) compareMethodSignature(mt types.FunctionType, et types.FunctionType) bool {
	withoutReceiver := mt.Params[1:]
	if len(withoutReceiver) != len(et.Params) {
		return false
	}

	for i, param := range withoutReceiver {
		if !c.typesMatch(param, et.Params[i]) {
			return false
		}
	}

	return c.typesMatch(mt.Return, et.Return)
}

func (c *Checker) typesMatch(actual, expected types.Type) bool {
	if actual == nil || expected == nil {
		return actual == expected
	}
	if isBasicType(actual) && isBasicType(expected) {
		return actual == expected || actual == types.Any || expected == types.Any
	}

	// Handle empty collections (e.g., [] matching array<int>)
	if aa, ok := toArray(actual); ok {
		if ea, ok := toArray(expected); ok {
			if aa.IsEmpty {
				return true
			}

			return c.typesMatch(aa.ElemType, ea.ElemType)
		}
	}

	if em, ok := toMap(expected); ok {
		if am, ok := toMap(actual); ok {
			if am.IsEmpty || em.IsEmpty { // this is a shitty fix to a logic bug in where this is called for maps
				return true
			}

			return c.typesMatch(em.KeyType, am.KeyType) && c.typesMatch(em.ValueType, am.ValueType)
		}
	}

	if it, ok := toInterface(expected); ok {
		if st, ok := toStruct(actual); ok {
			return c.structSatisfiesInterface(st, it)
		}
	}

	if es, ok := toStruct(expected); ok {
		if as, ok := toStruct(actual); ok {
			et := c.definedStructs[es.Name].Interfaces
			at := c.definedStructs[as.Name].Interfaces
			if len(et) != len(at) {
				return false
			}

			for i, aiRaw := range at {
				ai, ok := aiRaw.(types.InterfaceType)
				if !ok {
					return false
				}

				eiRaw := et[i]
				ei, ok := eiRaw.(types.InterfaceType)
				if !ok {
					return false
				}

				return ei.Name == ai.Name
			}
		}
	}

	if ei, ok := toInterface(expected); ok {
		if ai, ok := toInterface(actual); ok {
			return ei.Name == ai.Name
		}
	}

	return actual.Signature() == expected.Signature()
}

func (c *Checker) boxIfNecessary(expr ast.Expr, actual types.Type, expected types.Type) {
	if expr == nil || actual == nil || expected == nil {
		return
	}

	_, isStruct := toStruct(actual)
	it, isInterface := toInterface(expected)
	if isStruct && isInterface {
		expr.SetCastTo(&it)
	}
}

func isString(t types.Type) bool {
	return t.Signature() == types.String.Signature()
}

func toStruct(t types.Type) (types.StructType, bool) {
	typ, ok := t.(types.StructType)

	return typ, ok
}

func toInterface(t types.Type) (types.InterfaceType, bool) {
	typ, ok := t.(types.InterfaceType)
	return typ, ok
}

func isBasicType(t types.Type) bool {
	_, ok := t.(types.BasicType)
	return ok
}

func toArray(t types.Type) (types.ArrayType, bool) {
	typ, ok := t.(types.ArrayType)
	return typ, ok
}

func toMap(t types.Type) (types.MapType, bool) {
	typ, ok := t.(types.MapType)
	return typ, ok
}

func (c *Checker) satisfiesSameInterface(actual, expected types.Type) bool {
	if as, ok := toStruct(actual); ok {
		if es, ok := toStruct(expected); ok {
			authas, ok := c.definedStructs[as.Name]
			if ok {
				as = authas
			}
			authes, ok := c.definedStructs[es.Name]
			if ok {
				es = authes
			}
			satisfiesSameIface := true
			for idx, iface := range es.Interfaces {
				eIfaceType, ok := iface.(types.InterfaceType)
				if !ok {
					c.errors = append(c.errors, fmt.Sprintf("%s is not an interface", eIfaceType.Signature()))
					satisfiesSameIface = false
					continue
				}
				aIfaceType, ok := as.Interfaces[idx].(types.InterfaceType)
				if !ok {
					c.errors = append(c.errors, fmt.Sprintf("%s is not an interface", aIfaceType.Signature()))
					satisfiesSameIface = false
					continue
				}

				if aIfaceType.Name != eIfaceType.Name {
					satisfiesSameIface = false
				}
			}

			return satisfiesSameIface
		}
	}

	return false
}

func (c *Checker) checkIndexAssignment(node *ast.IndexAssignmentStmt) types.Type {
	c.typeOf(node.Left, nil)
	t := c.typeOf(node.Left.Left, nil)

	indexOrKeyType := c.typeOf(node.Left.Index, nil)

	valType := c.typeOf(node.Value, t)

	switch colType := t.(type) {
	case types.ArrayType:
		if indexOrKeyType != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("index must be type int, got %s", indexOrKeyType.Signature()))
		}

		if !c.typesMatch(valType, colType.ElemType) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot assign %s to element of array of type %s", valType.Signature(), colType.Signature()))
		}
	case types.MapType:
		if !c.typesMatch(indexOrKeyType, colType.KeyType) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: key for map of type %s must be %s, got %s", colType.Signature(), colType.KeyType.Signature(), indexOrKeyType.Signature()))
		}

		if !c.typesMatch(valType, colType.ValueType) {
			c.errors = append(c.errors, fmt.Sprintf("type mismatch: cannot assign %s to entry of map of type %s", valType.Signature(), colType.Signature()))
		}
	}

	return types.Unit
}

func (c *Checker) checkFunctionDeclaration(n ast.Node, node *ast.FunctionDeclarationStmt) types.Type {
	name := node.Name.Value
	fTypeRaw := node.Type.(types.FunctionType)
	if len(fTypeRaw.Params) > 0 {
		receiverType := fTypeRaw.Params[0]
		if sn, ok := c.isInterfaceMethod(receiverType, name); ok {
			name = fmt.Sprintf("%s.%s", sn, name)
			node.MangledName = name
			n = node
		}
	}

	fTypeRetrieved, _, ok := c.env.Get(node.Name.Value)
	if !ok {
		c.env.Set(name, node.Type)
		fTypeRetrieved = node.Type
	}

	fType, ok := fTypeRetrieved.(types.FunctionType)
	if !ok {
		c.errors = append(c.errors, fmt.Sprintf("cannot use function declaration of %s", node.Name.Value))
	}

	oldReturnType := c.currentReturnType
	c.currentReturnType = fType.Return
	oldEnv := c.env
	c.env = NewTypeEnv(oldEnv)

	for i, param := range node.Params {
		c.env.Set(param.Value, fType.Params[i])
	}

	c.check(node.Body)

	c.env = oldEnv
	c.currentReturnType = oldReturnType

	return types.Unit
}

func (c *Checker) checkIndexExpr(e ast.Node, expr *ast.IndexExpr) types.Type {
	lt := c.typeOf(expr.Left, nil)
	idxT := c.typeOf(expr.Index, nil)
	mt, mok := lt.(types.MapType)
	at, aok := lt.(types.ArrayType)

	if aok {
		if idxT != types.Int {
			c.errors = append(c.errors, fmt.Sprintf("index type for array must be int, got %s", idxT.Signature()))
			return nil
		}
		expr.ResolvedType = at.ElemType
		e = expr
		return at.ElemType
	} else if mok {
		if idxT != mt.KeyType {
			c.errors = append(c.errors, fmt.Sprintf("index type for map %s must be %s, got %s", mt.Signature(), mt.KeyType.Signature(), idxT.Signature()))
			return nil
		}

		expr.ResolvedType = mt.ValueType
		e = expr
		return mt.ValueType
	}
	c.errors = append(c.errors, fmt.Sprintf("index operation undefined for type: %s", lt.Signature()))

	return nil
}
