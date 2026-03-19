package codegen

import (
	"fmt"
	"sydney/ast"
	"sydney/token"
	"sydney/types"
)

func ExpandDerives(program *ast.Program) {
	var generated []ast.Stmt

	for _, stmt := range program.Stmts {
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
						if isPub {
							generated = append(generated, &ast.PubStatement{Stmt: fn})
						} else {
							generated = append(generated, fn)
						}
					}
				}
			}
		}
	}

	program.Stmts = append(program.Stmts, generated...)
}

func generateJsonUnmarshal(name string, st types.StructType) *ast.FunctionDeclarationStmt {
	fnName := "unmarshal_json_" + name
	param := &ast.Identifier{Value: "raw"}

	body := &ast.BlockStmt{Stmts: make([]ast.Stmt, 0)}
	for i, field := range st.Fields {
		stmts := generateJsonUnmarshalField(field, st.Types[i])
		if stmts != nil {
			body.Stmts = append(body.Stmts, stmts...)
		}
	}

	body.Stmts = append(body.Stmts, generateStructReturn(name, st.Fields))

	fnType := types.FunctionType{
		Params: []types.Type{types.String},
		Return: types.ResultType{T: st},
	}

	return &ast.FunctionDeclarationStmt{
		Token:  token.Token{Literal: "func"},
		Name:   &ast.Identifier{Value: fnName},
		Params: []*ast.Identifier{param},
		Body:   body,
		Type:   fnType,
	}
}

func generateJsonUnmarshalField(field string, typ types.Type) []ast.Stmt {
	stmts := make([]ast.Stmt, 2)

	optDecl := &ast.VarDeclarationStmt{Constant: true}
	optName := field + "_opt"
	optDecl.Name = &ast.Identifier{Value: optName}

	switch typ {
	case types.Int:
		optDecl.Value = generateJsonCall("get_int", field)
	case types.Float:
		optDecl.Value = generateJsonCall("get_float", field)
	case types.String:
		optDecl.Value = generateJsonCall("get_str", field)
	case types.Bool:
		optDecl.Value = generateJsonCall("get_bool", field)
	default:
		return nil
	}
	stmts[0] = optDecl

	valDecl := &ast.VarDeclarationStmt{Constant: true}
	valDecl.Name = &ast.Identifier{Value: field}
	valDecl.Value = &ast.MatchExpr{
		Subject: &ast.Identifier{Value: optName},
		SomeArm: &ast.MatchArm{
			Pattern: &ast.MatchPattern{
				IsSome:  true,
				IsOk:    false,
				Binding: &ast.Identifier{Value: "val"},
			},
			Body: &ast.BlockStmt{
				Stmts: []ast.Stmt{
					&ast.ExpressionStmt{
						Expr: &ast.Identifier{
							Value: "val",
						},
					},
				},
			},
		},
		NoneArm: &ast.MatchArm{
			Pattern: &ast.MatchPattern{
				IsOk:   false,
				IsSome: false,
			},
			Body: &ast.BlockStmt{
				Stmts: []ast.Stmt{
					&ast.ReturnStmt{
						ReturnValue: &ast.CallExpr{
							Function: &ast.Identifier{
								Value: "err",
							},
							Arguments: []ast.Expr{
								&ast.StringLiteral{
									Value: fmt.Sprintf("missing field: %s", field),
								},
							},
						},
					},
				},
			},
		},
	}
	stmts[1] = valDecl
	return stmts
}

func generateStructReturn(structName string, fields []string) ast.Stmt {
	values := make([]ast.Expr, len(fields))
	fieldNames := make([]string, len(fields))
	for i, f := range fields {
		fieldNames[i] = f
		values[i] = &ast.Identifier{Value: f}
	}

	return &ast.ReturnStmt{
		ReturnValue: &ast.CallExpr{
			Function: &ast.Identifier{Value: "ok"},
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
	fn := &ast.ScopeAccessExpr{
		Module: &ast.Identifier{
			Value: "json",
		},
		Member: &ast.Identifier{
			Value: fnName,
		},
	}
	args := make([]ast.Expr, 2)
	args[0] = &ast.Identifier{Value: "raw"}
	args[1] = &ast.StringLiteral{Value: field}

	call := &ast.CallExpr{}
	call.Function = fn
	call.Arguments = args

	return call
}
