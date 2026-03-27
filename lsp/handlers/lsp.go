package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
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
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			log.Printf("%s: PANIC: %v\n%s", method, r, buf[:n])
		}
	}()

	if strings.HasPrefix(src, "module ") {
		l.parseModule(method, filePath, src)
	} else {
		l.parseProgram(method, filePath, src)
	}
}

func (l *LSP) parseModule(method messages.Method, filePath string, src string) {
	sourceDir := filepath.Dir(filePath)
	base := filepath.Base(filePath)

	allSources, allNames := l.readDirSources(sourceDir, base, src)

	allStructs := map[string]types.Type{}
	allInterfaces := map[string]types.Type{}
	for _, source := range allSources {
		scan := parser.New(lexer.New(source))
		scan.ParseDefinitions()
		for k, v := range scan.DefinedStructs() {
			allStructs[k] = v
		}
		for k, v := range scan.DefinedInterfaces() {
			allInterfaces[k] = v
		}
	}

	var allImports []string
	var programs []*ast.Program
	var currentProgram *ast.Program
	for i, source := range allSources {
		p := parser.New(lexer.New(source))
		p.SetDefinedTypes(allStructs, allInterfaces)
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			log.Printf("%s: parse errors in %s: %v", method, allNames[i], p.Errors())
			continue
		}
		codegen.ExpandDerives(prog)
		for _, imp := range loader.ScanImports(source) {
			allImports = append(allImports, imp)
		}
		for _, imp := range codegen.ScanDeriveImports(source) {
			allImports = append(allImports, imp)
		}
		programs = append(programs, prog)
		if allNames[i] == base {
			currentProgram = prog
		}
	}

	if currentProgram == nil {
		log.Printf("%s: could not find current file in parsed programs", method)
		return
	}

	ld := loader.NewFromImports(allImports)
	stdLib := loader.ResolveStdlib(sourceDir)
	ld.SetPaths(stdLib, sourceDir)
	packages, tt, _, err := ld.Load(make(map[string]bool))
	if err != nil {
		log.Printf("%s: Error loading packages: %v", method, err)
		return
	}

	for _, pkg := range packages {
		for _, pr := range pkg.Programs {
			codegen.ExpandDerives(pr)
		}
	}

	merged := &ast.Program{}
	for _, prog := range programs {
		merged.Stmts = append(merged.Stmts, prog.Stmts...)
	}

	// assumes module declaration is on the first line (TODO: enforce in parser)
	moduleName := strings.TrimPrefix(strings.Split(src, "\n")[0], "module ")
	moduleName = strings.Trim(moduleName, "\" \r")

	typeEnv := typechecker.NewTypeEnv(nil)
	c := typechecker.NewWithModuleTypes(typeEnv, tt)
	c.SetCurrentModule(moduleName)
	errs := c.CheckAsPackage(merged, packages)
	if len(errs) > 0 {
		log.Printf("%s: Errors found: %v", method, errs)
	}
	log.Printf("%s: Sending diagnostics for file %s", method, filePath)
	l.SendTypecheckerDiagnostics(errs, filePath)

	l.program = merged
	l.env = c.Env()
	log.Printf("%s: parsed %d statements", method, len(merged.Stmts))
}

func (l *LSP) parseProgram(method messages.Method, filePath string, src string) {
	sourceDir := filepath.Dir(filePath)

	typeEnv := typechecker.NewTypeEnv(nil)
	imports := loader.ScanImports(src)
	deriveImports := codegen.ScanDeriveImports(src)
	imports = append(imports, deriveImports...)

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

	c := typechecker.NewWithModuleTypes(typeEnv, tt)
	errs := c.Check(program, packages)
	if len(errs) > 0 {
		log.Printf("%s: Errors found: %v", method, errs)
	}
	log.Printf("%s: Sending diagnostics for file %s", method, filePath)
	l.SendTypecheckerDiagnostics(errs, filePath)

	l.program = program
	l.env = c.Env()
	log.Printf("%s: parsed %d statements", method, len(program.Stmts))
}

func (l *LSP) readDirSources(dir, currentBase, currentSrc string) ([]string, []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{currentSrc}, []string{currentBase}
	}

	var sources []string
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sy") || strings.HasSuffix(entry.Name(), "_test.sy") {
			continue
		}
		if entry.Name() == currentBase {
			sources = append(sources, currentSrc)
			names = append(names, entry.Name())
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		sources = append(sources, string(data))
		names = append(names, entry.Name())
	}

	if len(sources) == 0 {
		return []string{currentSrc}, []string{currentBase}
	}
	return sources, names
}
