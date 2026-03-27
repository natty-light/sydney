package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sydney/ast"
	"sydney/codegen"
	"sydney/lexer"
	"sydney/loader"
	"sydney/lsp/messages"
	"sydney/parser"
	"sydney/typechecker"
)

type LSP struct {
	program *ast.Program
	env     *typechecker.TypeEnv

	w io.Writer
}

func New(w io.Writer) *LSP {
	return &LSP{
		w: w,
	}
}

func (l *LSP) WriteResponse(r *messages.Response) error {
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(l.w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	if err != nil {
		return err
	}
	return nil
}

func (l *LSP) WriteNotification(r *messages.Notification) error {
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(l.w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	if err != nil {
		return err
	}
	return nil
}

func (l *LSP) parse(method messages.Method, filePath string, src string) {
	sourceDir := filepath.Dir(filePath)

	typeEnv := typechecker.NewTypeEnv(nil)
	imports := loader.ScanImports(src)
	deriveImports := codegen.ScanDeriveImports(src)
	imports = append(imports, deriveImports...)
	ld := loader.NewFromImports(imports)
	stdLib := filepath.Join(sourceDir, "stdlib")
	ld.SetPaths(stdLib, sourceDir)
	packages, tt, gns, err := ld.Load(make(map[string]bool))
	if err != nil {
		log.Printf("%s: Error loading packages: %v", method, err)
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
	errs := c.Check(program, packages)
	if len(errs) > 0 {
		log.Printf("%s: Errors found: %v", method, errs)
	}
	log.Printf("%s: Sending diagnostics for file %s", method, filePath)
	l.SendTypecheckerDiagnostics(errs, filePath)

	ast.FilterGenericTemplates(program)
	for _, pkg := range packages {
		for _, prog := range pkg.Programs {
			ast.FilterGenericTemplates(prog)
		}
	}

	l.program = program
	l.env = typeEnv
	log.Printf("%s: parsed %d statements", method, len(program.Stmts))
}
