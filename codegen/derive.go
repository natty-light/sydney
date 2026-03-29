package codegen

import (
	"fmt"
	"strings"

	"sydney/ast"
	"sydney/token"
	"sydney/types"
)

func ExpandDerives(program *ast.Program) {
	var result []ast.Stmt

	for _, stmt := range program.Stmts {
		result = append(result, stmt)

		var def *ast.StructDefinitionStmt
		isPub := false

		switch s := stmt.(type) {
		case *ast.StructDefinitionStmt:
			def = s
		case *ast.PubStatement:
			if d, ok := s.Stmt.(*ast.StructDefinitionStmt); ok {
				def = d
				isPub = true
			}
		}

		if def == nil {
			continue
		}

		for _, ann := range def.GetAnnotations() {
			if ann.Name == "derive" {
				for _, arg := range ann.Args {
					if arg == "json" {
						fn := generateJsonUnmarshal(def.Name.Value, def.Type)
						marshalFn := generateJsonMarshal(def.Name.Value, def.Type)
						if isPub {
							result = append(result, &ast.PubStatement{Stmt: fn})
							result = append(result, &ast.PubStatement{Stmt: marshalFn})
						} else {
							result = append(result, fn)
							result = append(result, marshalFn)
						}
					}
				}
			}
		}
	}

	program.Stmts = result
}

func generateJsonUnmarshal(name string, st types.StructType) *ast.FunctionDeclarationStmt {
	body := &ast.BlockStmt{Stmts: make([]ast.Stmt, 0)}
	for i, field := range st.Fields {
		stmts := generateJsonUnmarshalField(field, st.Types[i])
		if stmts != nil {
			body.Stmts = append(body.Stmts, stmts...)
		}
	}

	body.Stmts = append(body.Stmts, generateStructReturn(name, st.Fields))

	return &ast.FunctionDeclarationStmt{
		Token:  token.Token{Literal: "func"},
		Name:   ident("unmarshal_json_" + name),
		Params: []*ast.Identifier{ident("raw")},
		Body:   body,
		Type: types.FunctionType{
			Params: []types.Type{types.String},
			Return: types.ResultType{T: st},
		},
	}
}

func generateJsonUnmarshalField(field string, typ types.Type) []ast.Stmt {
	switch typ {
	case types.Int:
		return primitiveField(field, "get_int")
	case types.Float:
		return primitiveField(field, "get_float")
	case types.String:
		return primitiveField(field, "get_str")
	case types.Bool:
		return primitiveField(field, "get_bool")
	default:
		if st, ok := typ.(types.StructType); ok {
			return structField(field, st)
		}
		if at, ok := typ.(types.ArrayType); ok {
			return arrayField(field, at)
		}
		return nil
	}
}

func primitiveField(field, getFn string) []ast.Stmt {
	optName := field + "_opt"
	return []ast.Stmt{
		constDecl(optName, generateJsonCall(getFn, field)),
		constDecl(field, matchOption(
			ident(optName), "val",
			block(exprStmt(ident("val"))),
			fmt.Sprintf("missing field: %s", field),
		)),
	}
}

func structField(field string, st types.StructType) []ast.Stmt {
	rawName := field + "_raw"
	return []ast.Stmt{
		constDecl(rawName, generateJsonCall("get_object", field)),
		constDecl(field, matchOption(
			ident(rawName), "val",
			block(exprStmt(matchResult(
				&ast.CallExpr{
					Function:  ident("unmarshal_json_" + st.Name),
					Arguments: []ast.Expr{ident("val")},
				},
				"v",
				block(exprStmt(ident("v"))),
			))),
			fmt.Sprintf("missing field: %s", field),
		)),
	}
}

func arrayField(field string, at types.ArrayType) []ast.Stmt {
	rawName := field + "_raw"

	var parseFn string
	switch at.ElemType {
	case types.Int:
		parseFn = "parse_int_array"
	case types.Float:
		parseFn = "parse_float_array"
	case types.String:
		parseFn = "parse_string_array"
	case types.Bool:
		parseFn = "parse_bool_array"
	}

	if parseFn != "" {
		return []ast.Stmt{
			constDecl(rawName, generateJsonCall("get_array", field)),
			constDecl(field, matchOption(
				ident(rawName), "val",
				block(exprStmt(matchOption(
					jsonCall(parseFn, ident("val")),
					"arr",
					block(exprStmt(ident("arr"))),
					fmt.Sprintf("failed to parse array field: %s", field),
				))),
				fmt.Sprintf("missing field: %s", field),
			)),
		}
	}

	if st, ok := at.ElemType.(types.StructType); ok {
		return structArrayField(field, st)
	}

	if innerAt, ok := at.ElemType.(types.ArrayType); ok {
		return nestedArrayField(field, innerAt)
	}

	return nil
}

func structArrayField(field string, st types.StructType) []ast.Stmt {
	rawName := field + "_raw"
	arrType := types.ArrayType{ElemType: st}
	return []ast.Stmt{
		constDecl(rawName, generateJsonCall("get_array", field)),
		constDecl(field, matchOption(
			ident(rawName), "val",
			block(
				constDecl("elems", jsonCall("split_elements", ident("val"))),
				mutDecl("arr", arrType, &ast.ArrayLiteral{}),
				&ast.ForInStmt{
					Value:    ident("elem"),
					Iterable: ident("elems"),
					Body: block(
						constDecl("parsed", matchResult(
							&ast.CallExpr{
								Function:  ident("unmarshal_json_" + st.Name),
								Arguments: []ast.Expr{ident("elem")},
							},
							"v",
							block(exprStmt(ident("v"))),
						)),
						appendStmt("arr", ident("parsed")),
					),
				},
				exprStmt(ident("arr")),
			),
			fmt.Sprintf("missing field: %s", field),
		)),
	}
}

func nestedArrayField(field string, innerAt types.ArrayType) []ast.Stmt {
	var parseFn string
	switch innerAt.ElemType {
	case types.Int:
		parseFn = "parse_int_array"
	case types.Float:
		parseFn = "parse_float_array"
	case types.String:
		parseFn = "parse_string_array"
	case types.Bool:
		parseFn = "parse_bool_array"
	default:
		return nil
	}

	rawName := field + "_raw"
	arrType := types.ArrayType{ElemType: innerAt}
	return []ast.Stmt{
		constDecl(rawName, generateJsonCall("get_array", field)),
		constDecl(field, matchOption(
			ident(rawName), "val",
			block(
				constDecl("elems", jsonCall("split_elements", ident("val"))),
				mutDecl("arr", arrType, &ast.ArrayLiteral{}),
				&ast.ForInStmt{
					Value:    ident("elem"),
					Iterable: ident("elems"),
					Body: block(
						constDecl("stripped", jsonCall("strip_brackets", ident("elem"))),
						constDecl("parsed", matchOption(
							jsonCall(parseFn, ident("stripped")),
							"v",
							block(exprStmt(ident("v"))),
							fmt.Sprintf("failed to parse array element in: %s", field),
						)),
						appendStmt("arr", ident("parsed")),
					),
				},
				exprStmt(ident("arr")),
			),
			fmt.Sprintf("missing field: %s", field),
		)),
	}
}

func generateStructReturn(structName string, fields []string) ast.Stmt {
	values := make([]ast.Expr, len(fields))
	fieldNames := make([]string, len(fields))
	for i, f := range fields {
		fieldNames[i] = f
		values[i] = ident(f)
	}

	return &ast.ReturnStmt{
		ReturnValue: &ast.CallExpr{
			Function: ident("ok"),
			Arguments: []ast.Expr{
				&ast.StructLiteral{
					Name:   structName,
					Fields: fieldNames,
					Values: values,
				},
			},
		},
	}
}

func generateJsonCall(fnName, field string) *ast.CallExpr {
	return &ast.CallExpr{
		Function: &ast.ScopeAccessExpr{
			Module: ident("json"),
			Member: ident(fnName),
		},
		Arguments: []ast.Expr{ident("raw"), &ast.StringLiteral{Value: field}},
	}
}

func convCall(fnName string, arg ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{
		Function: &ast.ScopeAccessExpr{
			Module: ident("conv"),
			Member: ident(fnName),
		},
		Arguments: []ast.Expr{arg},
	}
}

func jsonCall(fnName string, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{
		Function: &ast.ScopeAccessExpr{
			Module: ident("json"),
			Member: ident(fnName),
		},
		Arguments: args,
	}
}

func ident(name string) *ast.Identifier {
	return &ast.Identifier{Value: name}
}

func constDecl(name string, value ast.Expr) *ast.VarDeclarationStmt {
	return &ast.VarDeclarationStmt{
		Constant: true,
		Name:     ident(name),
		Value:    value,
	}
}

func mutDecl(name string, typ types.Type, value ast.Expr) *ast.VarDeclarationStmt {
	return &ast.VarDeclarationStmt{
		Constant: false,
		Name:     ident(name),
		Type:     typ,
		Value:    value,
	}
}

func block(stmts ...ast.Stmt) *ast.BlockStmt {
	return &ast.BlockStmt{Stmts: stmts}
}

func exprStmt(expr ast.Expr) *ast.ExpressionStmt {
	return &ast.ExpressionStmt{Expr: expr}
}

func returnErr(msg string) *ast.ReturnStmt {
	return &ast.ReturnStmt{
		ReturnValue: &ast.CallExpr{
			Function:  ident("err"),
			Arguments: []ast.Expr{&ast.StringLiteral{Value: msg}},
		},
	}
}

func returnErrIdent(name string) *ast.ReturnStmt {
	return &ast.ReturnStmt{
		ReturnValue: &ast.CallExpr{
			Function:  ident("err"),
			Arguments: []ast.Expr{ident(name)},
		},
	}
}

func appendStmt(arrName string, value ast.Expr) *ast.VarAssignmentStmt {
	return &ast.VarAssignmentStmt{
		Identifier: ident(arrName),
		Value: &ast.CallExpr{
			Function:  ident("append"),
			Arguments: []ast.Expr{ident(arrName), value},
		},
	}
}

func matchOption(subject ast.Expr, binding string, someBody *ast.BlockStmt, errMsg string) *ast.MatchExpr {
	return &ast.MatchExpr{
		Subject: subject,
		SomeArm: &ast.MatchArm{
			Pattern: &ast.MatchPattern{IsSome: true, Binding: ident(binding)},
			Body:    someBody,
		},
		NoneArm: &ast.MatchArm{
			Pattern: &ast.MatchPattern{},
			Body:    block(returnErr(errMsg)),
		},
	}
}

func matchResult(subject ast.Expr, binding string, okBody *ast.BlockStmt) *ast.MatchExpr {
	return &ast.MatchExpr{
		Subject: subject,
		OkArm: &ast.MatchArm{
			Pattern: &ast.MatchPattern{IsOk: true, Binding: ident(binding)},
			Body:    okBody,
		},
		ErrArm: &ast.MatchArm{
			Pattern: &ast.MatchPattern{Binding: ident("msg")},
			Body:    block(returnErrIdent("msg")),
		},
	}
}

func strConcat(parts ...ast.Expr) ast.Expr {
	result := parts[0]
	for _, p := range parts[1:] {
		result = &ast.InfixExpr{
			Operator: "+",
			Left:     result,
			Right:    p,
		}
	}
	return result
}

func strLit(s string) *ast.StringLiteral {
	return &ast.StringLiteral{Value: s}
}

func selector(obj, field string) *ast.SelectorExpr {
	return &ast.SelectorExpr{
		Left:  ident(obj),
		Value: ident(field),
	}
}

func generateJsonMarshal(name string, st types.StructType) *ast.FunctionDeclarationStmt {
	body := &ast.BlockStmt{Stmts: make([]ast.Stmt, 0)}
	paramName := strings.ToLower(name) + "_struct"
	for i, field := range st.Fields {
		stmt := generateJsonMarshalField(paramName, field, st.Types[i])
		if stmt != nil {
			body.Stmts = append(body.Stmts, stmt...)
		}
	}

	body.Stmts = append(body.Stmts, generateStringReturn(st.Fields))

	return &ast.FunctionDeclarationStmt{
		Name:   ident("marshal_json_" + name),
		Type:   types.FunctionType{Params: []types.Type{st}, Return: types.String},
		Params: []*ast.Identifier{ident(strings.ToLower(name) + "_struct")},
		Body:   body,
	}
}

func generateJsonMarshalField(paramName, field string, typ types.Type) []ast.Stmt {
	access := selector(paramName, field)
	strVar := field + "_str"
	switch typ {
	case types.Int:
		return []ast.Stmt{constDecl(strVar, convCall("itoa", access))}
	case types.Float:
		return []ast.Stmt{constDecl(strVar, convCall("ftoa", access))}
	case types.Bool:
		return []ast.Stmt{constDecl(strVar, convCall("bool_to_str", access))}
	case types.String:
		return []ast.Stmt{constDecl(strVar, strConcat(strLit("\""), access, strLit("\"")))}
	default:
		if st, ok := typ.(types.StructType); ok {
			return []ast.Stmt{constDecl(strVar, &ast.CallExpr{
				Function:  ident("marshal_json_" + st.Name),
				Arguments: []ast.Expr{access},
			})}
		}
		if at, ok := typ.(types.ArrayType); ok {
			return marshalArrayField(paramName, field, at)
		}
		return nil
	}
}

func marshalArrayField(paramName, field string, at types.ArrayType) []ast.Stmt {
	access := selector(paramName, field)
	strVar := field + "_str"
	var marshalFn string
	switch at.ElemType {
	case types.Int:
		marshalFn = "marshal_int_array"
	case types.Float:
		marshalFn = "marshal_float_array"
	case types.String:
		marshalFn = "marshal_string_array"
	case types.Bool:
		marshalFn = "marshal_bool_array"
	}
	if marshalFn != "" {
		return []ast.Stmt{constDecl(strVar, jsonCall(marshalFn, access))}
	}
	if st, ok := at.ElemType.(types.StructType); ok {
		return marshalStructArrayField(paramName, field, st)
	}
	if innerAt, ok := at.ElemType.(types.ArrayType); ok {
		return marshalNestedArrayField(paramName, field, innerAt)
	}
	return nil
}

func marshalStructArrayField(paramName, field string, st types.StructType) []ast.Stmt {
	access := selector(paramName, field)
	strVar := field + "_str"
	return []ast.Stmt{
		mutDecl(strVar, types.String, strLit("[")),
		forInWithIndex("idx", "elem", access, block(
			constDecl("s", &ast.CallExpr{
				Function:  ident("marshal_json_" + st.Name),
				Arguments: []ast.Expr{ident("elem")},
			}),
			ifLessThan("idx", lenCall(access), block(
				assignStmt(strVar, strConcat(ident(strVar), ident("s"), strLit(","))),
			), block(
				assignStmt(strVar, strConcat(ident(strVar), ident("s"))),
			)),
		)),
		assignStmt(strVar, strConcat(ident(strVar), strLit("]"))),
	}
}

func marshalNestedArrayField(paramName, field string, innerAt types.ArrayType) []ast.Stmt {
	var marshalFn string
	switch innerAt.ElemType {
	case types.Int:
		marshalFn = "marshal_int_array"
	case types.Float:
		marshalFn = "marshal_float_array"
	case types.String:
		marshalFn = "marshal_string_array"
	case types.Bool:
		marshalFn = "marshal_bool_array"
	default:
		return nil
	}
	access := selector(paramName, field)
	strVar := field + "_str"
	return []ast.Stmt{
		mutDecl(strVar, types.String, strLit("[")),
		forInWithIndex("idx", "elem", access, block(
			constDecl("s", jsonCall(marshalFn, ident("elem"))),
			ifLessThan("idx", lenCall(access), block(
				assignStmt(strVar, strConcat(ident(strVar), ident("s"), strLit(","))),
			), block(
				assignStmt(strVar, strConcat(ident(strVar), ident("s"))),
			)),
		)),
		assignStmt(strVar, strConcat(ident(strVar), strLit("]"))),
	}
}

func forInWithIndex(key, value string, iterable ast.Expr, body *ast.BlockStmt) *ast.ForInStmt {
	return &ast.ForInStmt{
		Key:      ident(key),
		Value:    ident(value),
		Iterable: iterable,
		Body:     body,
	}
}

func lenCall(arg ast.Expr) ast.Expr {
	return &ast.CallExpr{
		Function:  ident("len"),
		Arguments: []ast.Expr{arg},
	}
}

func intLit(val int64) *ast.IntegerLiteral {
	return &ast.IntegerLiteral{Value: val}
}

func infix(op string, left, right ast.Expr) *ast.InfixExpr {
	return &ast.InfixExpr{Operator: op, Left: left, Right: right}
}

func ifLessThan(idxName string, limit ast.Expr, consequence, alternative *ast.BlockStmt) *ast.ExpressionStmt {
	return exprStmt(&ast.IfExpr{
		Condition:   infix("<", ident(idxName), infix("-", limit, intLit(1))),
		Consequence: consequence,
		Alternative: alternative,
	})
}

func assignStmt(name string, value ast.Expr) *ast.VarAssignmentStmt {
	return &ast.VarAssignmentStmt{
		Identifier: ident(name),
		Value:      value,
	}
}

func generateStringReturn(fields []string) *ast.ReturnStmt {
	parts := make([]ast.Expr, 0)
	parts = append(parts, strLit("{"))
	for i, field := range fields {
		parts = append(parts, strLit("\""+field+"\":"))
		parts = append(parts, ident(field+"_str"))
		if i < len(fields)-1 {
			parts = append(parts, strLit(","))
		}
	}
	parts = append(parts, strLit("}"))
	return &ast.ReturnStmt{
		ReturnValue: strConcat(parts...),
	}
}
