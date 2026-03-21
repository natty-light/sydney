package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sydney/ast"
	"sydney/codegen"
	"sydney/compiler"
	"sydney/irgen"
	"sydney/lexer"
	"sydney/loader"
	"sydney/object"
	"sydney/parser"
	"sydney/repl"
	"sydney/typechecker"
	"sydney/vm"
)

type Flag string

const (
	dumpAst   Flag = "dump-ast"
	dumpTypes Flag = "dump-types"
)

var allowedFlags = map[Flag]bool{
	dumpTypes: true,
	dumpAst:   true,
}

type CommandFunc func(args []string, flags map[Flag]bool) int

var commands = map[string]CommandFunc{
	"help":    Help,
	"version": Version,
	"compile": Compile,
	"run":     Run,
	"test":    Test,
}

func main() {
	args := os.Args
	status := 0
	flags := parseFlags(args)

	if len(args) == 1 {
		// If no filename was passed as a command line argument, run the repl
		repl.StartVM(os.Stdin, os.Stdout)
	} else {
		command, ok := commands[strings.ToLower(args[1])]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[1])
			os.Exit(1)
		}
		status = command(args[2:], flags)
	}
	os.Exit(status)
}

func Help(args []string, flags map[Flag]bool) int {
	fmt.Println("Usage: sydney [version|run|compile|help] [filename]")
	return 0
}

func Version(args []string, flags map[Flag]bool) int {
	fmt.Println("Sydney v0.1.0")
	return 0
}

func Run(args []string, flags map[Flag]bool) int {
	filename := args[0]
	constants := []object.Object{}
	globals := make([]object.Object, vm.GlobalsSize)
	symbolTable := compiler.NewSymbolTable()
	typeEnv := typechecker.NewTypeEnv(nil)
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	file, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Honk! Cannot read file %s\n", filename)
		return 1
	}

	src := string(file)

	imports := loader.ScanImports(src)
	deriveImports := codegen.ScanDeriveImports(src)
	imports = append(imports, deriveImports...)
	ld := loader.NewFromImports(imports)
	sourceDir := filepath.Dir(filename)
	stdLib := filepath.Join(sourceDir, "stdlib")
	ld.SetPaths(stdLib, sourceDir)
	packages, tt, gns, err := ld.Load(make(map[string]bool))
	if err != nil {
		fmt.Printf("loader error: %s\n", err)
		return 1
	}

	l := lexer.New(src)
	p := parser.NewWithGenericNames(l, gns)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		printParserErrors(os.Stdout, p.Errors())
		return 1
	}

	for _, pkg := range packages {
		for _, pr := range pkg.Programs {
			codegen.ExpandDerives(pr)
		}
	}

	codegen.ExpandDerives(program)

	if flags[dumpAst] {
		ast.Dump(program, 0)
		for _, pkg := range packages {
			fmt.Println(pkg)
			for _, pr := range pkg.Programs {
				ast.Dump(pr, 0)
			}
		}
	}

	c := typechecker.NewWithModuleTypes(typeEnv, tt)
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("typechecker panic:", r)
			for _, e := range c.Errors() {
				fmt.Println("  error:", e)
			}
		}
	}()
	typeErrs := c.Check(program, packages)

	if flags[dumpTypes] {
		ast.Dump(program, 0)
	}

	if len(typeErrs) != 0 {
		printParserErrors(os.Stdout, typeErrs)
		return 1
	}

	ast.FilterGenericTemplates(program)
	for _, pkg := range packages {
		for _, prog := range pkg.Programs {
			ast.FilterGenericTemplates(prog)
		}
	}

	comp := compiler.NewWithState(symbolTable, constants)
	err = comp.CompilePackages(packages)
	if err != nil {
		fmt.Printf("compiler error: %s\n", err)
		return 1
	}
	err = comp.Compile(program)
	if err != nil {
		fmt.Printf("compiler error: %s\n", err)
		return 1
	}

	machine := vm.NewWithGlobalStore(comp.Bytecode(), globals)
	err = machine.Run()
	if err != nil {
		fmt.Printf("Runtime error: %s\n", err)
		return 1
	}

	return 0
}

func Compile(args []string, flags map[Flag]bool) int {
	filename := args[0]
	file, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("cannot read file %s\n", filename)
		return 1
	}

	src := string(file)

	imports := loader.ScanImports(src)
	deriveImports := codegen.ScanDeriveImports(src)
	imports = append(imports, deriveImports...)
	ld := loader.NewFromImports(imports)
	sourceDir := filepath.Dir(filename)
	stdLib := filepath.Join(sourceDir, "stdlib")
	ld.SetPaths(stdLib, sourceDir)
	packages, tt, gns, err := ld.Load(make(map[string]bool))
	if err != nil {
		fmt.Printf("loader error: %s\n", err)
		return 1
	}

	l := lexer.New(src)
	p := parser.NewWithGenericNames(l, gns)
	program := p.ParseProgram()

	if flags[dumpAst] {
		ast.Dump(program, 0)
	}

	if len(p.Errors()) != 0 {
		printParserErrors(os.Stdout, p.Errors())
		return 1
	}

	for _, pkg := range packages {
		for _, pr := range pkg.Programs {
			codegen.ExpandDerives(pr)
		}
	}

	codegen.ExpandDerives(program)

	c := typechecker.NewWithModuleTypes(nil, tt)
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("typechecker panic:", r)
			for _, e := range c.Errors() {
				fmt.Println("  error:", e)
			}
		}
	}()
	errs := c.Check(program, packages)

	if flags[dumpTypes] {
		ast.Dump(program, 0)
	}

	if len(errs) != 0 {
		printParserErrors(os.Stdout, errs)
		return 1
	}

	ast.FilterGenericTemplates(program)
	for _, pkg := range packages {
		for _, prog := range pkg.Programs {
			ast.FilterGenericTemplates(prog)
		}
	}

	i := irgen.New()
	err = i.Emit(program, packages)
	if err != nil {
		fmt.Printf("Compiler error: %s\n", err)
		return 1
	}
	i.Write(strings.Replace(filename, ".sy", ".ll", -1))

	return 0
}

func Test(args []string, flags map[Flag]bool) int {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("cannot read directory %s\n", dir)
		return 1
	}

	var testFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.sy") {
			testFiles = append(testFiles, filepath.Join(dir, e.Name()))
		}
	}

	if len(testFiles) == 0 {
		fmt.Println("no test files found")
		return 0
	}

	totalPassed, totalFailed := 0, 0
	for _, filename := range testFiles {
		fmt.Printf("--- %s\n", filepath.Base(filename))
		p, f := runTestFile(filename)
		totalPassed += p
		totalFailed += f
	}

	fmt.Printf("\n%d passed, %d failed, %d total\n", totalPassed, totalFailed, totalPassed+totalFailed)
	if totalFailed > 0 {
		return 1
	}
	return 0
}

func runTestFile(filename string) (passed, failed int) {
	file, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("  cannot read file %s\n", filename)
		return 0, 1
	}

	src := string(file)

	// Collect imports from the test file and all sibling module files
	allImports := loader.ScanImports(src)
	deriveImports := codegen.ScanDeriveImports(src)
	allImports = append(allImports, deriveImports...)
	sourceDir := filepath.Dir(filename)
	siblingStmts, siblingImports := loadSiblingModuleFiles(sourceDir, filepath.Base(filename))
	for _, imp := range siblingImports {
		allImports = appendUnique(allImports, imp)
	}

	ld := loader.NewFromImports(allImports)
	cwd, _ := os.Getwd()
	stdLib := filepath.Join(cwd, "stdlib")
	ld.SetPaths(stdLib, sourceDir)
	packages, tt, gns, err := ld.Load(make(map[string]bool))
	if err != nil {
		fmt.Printf("  loader error: %s\n", err)
		return 0, 1
	}

	l := lexer.New(src)
	p := parser.NewWithGenericNames(l, gns)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		printParserErrors(os.Stdout, p.Errors())
		return 0, 1
	}

	codegen.ExpandDerives(program)

	// Prepend sibling module declarations into the test program
	program.Stmts = append(siblingStmts, program.Stmts...)

	c := typechecker.NewWithModuleTypes(nil, tt)
	typeErrs := c.Check(program, packages)
	if len(typeErrs) != 0 {
		printParserErrors(os.Stdout, typeErrs)
		return 0, 1
	}

	ast.FilterGenericTemplates(program)
	for _, pkg := range packages {
		for _, prog := range pkg.Programs {
			ast.FilterGenericTemplates(prog)
		}
	}

	// Collect test function names
	var testFns []string
	for _, stmt := range program.Stmts {
		if fn, ok := stmt.(*ast.FunctionDeclarationStmt); ok {
			if strings.HasPrefix(fn.Name.Value, "test_") {
				testFns = append(testFns, fn.Name.Value)
			}
		}
	}

	if len(testFns) == 0 {
		fmt.Println("  no test functions found")
		return 0, 0
	}

	// Run each test in isolation
	for _, name := range testFns {
		cloned := ast.Clone(program)
		callStmt := &ast.ExpressionStmt{
			Expr: &ast.CallExpr{
				Function:  &ast.Identifier{Value: name},
				Arguments: []ast.Expr{},
			},
		}
		cloned.Stmts = append(cloned.Stmts, callStmt)

		symbolTable := compiler.NewSymbolTable()
		for i, v := range object.Builtins {
			symbolTable.DefineBuiltin(i, v.Name)
		}
		comp := compiler.NewWithState(symbolTable, []object.Object{})
		err := comp.CompilePackages(packages)
		if err != nil {
			fmt.Printf("  FAIL  %s: compile error: %s\n", name, err)
			failed++
			continue
		}
		err = comp.Compile(cloned)
		if err != nil {
			fmt.Printf("  FAIL  %s: compile error: %s\n", name, err)
			failed++
			continue
		}

		globals := make([]object.Object, vm.GlobalsSize)
		machine := vm.NewWithGlobalStore(comp.Bytecode(), globals)
		err = machine.Run()

		if err != nil {
			fmt.Printf("  FAIL  %s: %s\n", name, err)
			failed++
		} else {
			fmt.Printf("  PASS  %s\n", name)
			passed++
		}
	}

	return passed, failed
}

// loadSiblingModuleFiles reads all non-test .sy files from the same directory,
// parses them, and returns their statements (excluding module/import declarations)
// plus any imports they declare.
func loadSiblingModuleFiles(dir string, testFilename string) ([]ast.Stmt, []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var stmts []ast.Stmt
	var imports []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == testFilename || !strings.HasSuffix(e.Name(), ".sy") || strings.HasSuffix(e.Name(), "_test.sy") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		src := string(data)
		imports = append(imports, loader.ScanImports(src)...)

		l := lexer.New(src)
		p := parser.New(l)
		prog := p.ParseProgram()
		if len(p.Errors()) != 0 {
			continue
		}

		// Include all statements except module and import declarations
		// Unwrap pub statements so functions are accessible without prefix
		for _, stmt := range prog.Stmts {
			switch s := stmt.(type) {
			case *ast.ModuleDeclarationStmt, *ast.ImportStatement:
				continue
			case *ast.PubStatement:
				stmts = append(stmts, s.Stmt)
			default:
				stmts = append(stmts, stmt)
			}
		}
	}

	return stmts, imports
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func printParserErrors(out io.Writer, errors []string) {
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}

func parseFlags(args []string) map[Flag]bool {
	flags := make(map[Flag]bool)
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			flag := Flag(arg[2:])
			if _, ok := allowedFlags[flag]; ok {
				flags[flag] = true
			} else {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
			}
		}
	}

	return flags
}
