package codegen

import (
	"sydney/lexer"
	"sydney/token"
)

func ScanDeriveImports(source string) []string {
	lx := lexer.New(source)
	var imports []string
	for {
		tok := lx.NextToken()
		if tok.Type == token.EOF {
			break
		}
		if tok.Type == token.AnnotationStart {
			tok = lx.NextToken()
			if tok.Literal == "derive" {
				tok = lx.NextToken()
				if tok.Type == token.LeftParen {
					for {
						tok = lx.NextToken()
						if tok.Type == token.RightParen || tok.Type == token.EOF {
							break
						}
						if tok.Literal == "json" {
							imports = append(imports, "json")
						}
					}
				}
			}
		}
	}
	return imports
}
