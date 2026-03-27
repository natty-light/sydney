package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"sydney/ast"
	"sydney/codegen"
	"sydney/lexer"
	"sydney/loader"
	"sydney/lsp/messages"
	"sydney/parser"
	"sydney/typechecker"
	"sydney/types"
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

	siblingPrograms, siblingImports := l.loadSiblings(filePath, src)
	imports = append(imports, siblingImports...)

	ld := loader.NewFromImports(imports)
	stdLib := loader.ResolveStdlib(sourceDir)
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

	for _, sp := range siblingPrograms {
		codegen.ExpandDerives(sp)
		program.Stmts = append(program.Stmts, sp.Stmts...)
	}

	c := typechecker.NewWithModuleTypes(typeEnv, tt)
	errs := c.Check(program, packages)
	if len(errs) > 0 {
		log.Printf("%s: Errors found: %v", method, errs)
	}
	log.Printf("%s: Sending diagnostics for file %s", method, filePath)
	l.SendTypecheckerDiagnostics(errs, filePath)

	l.program = program
	l.env = typeEnv
	log.Printf("%s: parsed %d statements", method, len(program.Stmts))
}

func (l *LSP) loadSiblings(filePath string, currentSrc string) ([]*ast.Program, []string) {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var sources []string
	var filenames []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sy") || strings.HasSuffix(entry.Name(), "_test.sy") {
			continue
		}
		if entry.Name() == base {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		sources = append(sources, string(data))
		filenames = append(filenames, entry.Name())
	}

	if len(sources) == 0 {
		return nil, nil
	}

	allStructs := map[string]types.Type{}
	allInterfaces := map[string]types.Type{}

	scan := parser.New(lexer.New(currentSrc))
	scan.ParseDefinitions()
	for k, v := range scan.DefinedStructs() {
		allStructs[k] = v
	}
	for k, v := range scan.DefinedInterfaces() {
		allInterfaces[k] = v
	}
	for _, source := range sources {
		scan := parser.New(lexer.New(source))
		scan.ParseDefinitions()
		for k, v := range scan.DefinedStructs() {
			allStructs[k] = v
		}
		for k, v := range scan.DefinedInterfaces() {
			allInterfaces[k] = v
		}
	}

	var programs []*ast.Program
	var extraImports []string
	for _, source := range sources {
		p := parser.New(lexer.New(source))
		p.SetDefinedTypes(allStructs, allInterfaces)
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			log.Printf("loadSiblings: parse errors in sibling: %v", p.Errors())
			continue
		}

		for _, imp := range loader.ScanImports(source) {
			extraImports = append(extraImports, imp)
		}
		for _, imp := range codegen.ScanDeriveImports(source) {
			extraImports = append(extraImports, imp)
		}

		programs = append(programs, prog)
	}

	return programs, extraImports
}
