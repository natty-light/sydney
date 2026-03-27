package handlers

import (
	"sydney/ast"
	"sydney/typechecker"
)

type LSP struct {
	program *ast.Program
	env     *typechecker.TypeEnv
}

func New() *LSP {
	return &LSP{}
}
