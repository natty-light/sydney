package codegen

import (
	"sydney/lexer"
	"sydney/token"
)

func ScanDeriveImports(source string) []string {
	lx := lexer.New(source)
	var imports []string
	existingImports := make(map[string]bool)
	for {
		tok := lx.NextToken()
		if tok.Type == token.EOF {
			break
		}
		if tok.Type == token.Import {
			tok = lx.NextToken()
			if tok.Type == token.String {
				existingImports[tok.Literal] = true
			}
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
						if tok.Literal == "json" && !existingImports[tok.Literal] {
							imports = append(imports, "json")
							existingImports["json"] = true
						}
					}
				}
			}
		}
	}
	return imports
}
