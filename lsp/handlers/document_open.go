package handlers

import (
	"encoding/json"
	"log"
	"net/url"
	"path/filepath"
	"sydney/ast"
	"sydney/codegen"
	"sydney/lexer"
	"sydney/loader"
	"sydney/lsp/messages"
	"sydney/parser"
	"sydney/typechecker"
)

func (l *LSP) HandleDocumentOpen(req *messages.Request) {
	var params messages.DocumentOpenParams
	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		log.Printf("%s: Error unmarshalling params: %v", messages.DocumentOpen, err)
		return
	}

	u, err := url.Parse(params.TextDocument.URI)
	if err != nil {
		log.Printf("%s: Error parsing URI: %v", messages.DocumentOpen, err)
		return
	}
	filePath := u.Path
	sourceDir := filepath.Dir(filePath)

	src := params.TextDocument.Text
	typeEnv := typechecker.NewTypeEnv(nil)
	imports := loader.ScanImports(src)
	deriveImports := codegen.ScanDeriveImports(src)
	imports = append(imports, deriveImports...)
	ld := loader.NewFromImports(imports)
	stdLib := filepath.Join(sourceDir, "stdlib")
	ld.SetPaths(stdLib, sourceDir)
	packages, tt, gns, err := ld.Load(make(map[string]bool))
	if err != nil {
		log.Printf("%s: Error loading packages: %v", messages.DocumentOpen, err)
		return
	}

	lx := lexer.New(src)
	p := parser.NewWithGenericNames(lx, gns)
	program := p.ParseProgram()

	for _, pkg := range packages {
		for _, pr := range pkg.Programs {
			codegen.ExpandDerives(pr)
		}
	}

	codegen.ExpandDerives(program)

	c := typechecker.NewWithModuleTypes(typeEnv, tt)
	c.Check(program, packages)

	ast.FilterGenericTemplates(program)
	for _, pkg := range packages {
		for _, prog := range pkg.Programs {
			ast.FilterGenericTemplates(prog)
		}
	}

	l.program = program
	l.env = typeEnv
	log.Printf("didOpen: parsed %d statements", len(program.Stmts))
}
