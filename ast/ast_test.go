package ast

import (
	"sydney/token"
	"testing"
)

func TestString(t *testing.T) {
	program := &Program{
		Stmts: []Stmt{
			&VarDeclarationStmt{
				Token:    token.Token{Type: token.Mut, Literal: "mut"},
				Name:     &Identifier{Token: token.Token{Type: token.Identifier, Literal: "x"}, Value: "x"},
				Value:    &Identifier{Token: token.Token{Type: token.Identifier, Literal: "y"}, Value: "y"},
				Constant: false,
			},
		},
	}

	if program.String() != "mut x = y;" {
		t.Errorf("program.String() wrong. got=%q", program.String())
	}
}
