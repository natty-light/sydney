package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sydney/ast"
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
}

// TODO : unfuck this
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

	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()

	if flags[dumpAst] {
		ast.Dump(program, 0)
	}

	if len(p.Errors()) != 0 {
		printParserErrors(os.Stdout, p.Errors())
		return 1
	}

	ld := loader.New(program)
	sourceDir := filepath.Dir(filename)
	stdLib := filepath.Join(sourceDir, "stdlib")
	ld.SetPaths(stdLib, sourceDir)
	packages, tt, err := ld.Load(make(map[string]bool))
	if err != nil {
		fmt.Printf("loader error: %s\n", err)
		return 1
	}

	c := typechecker.NewWithModuleTypes(typeEnv, tt)
	typeErrs := c.Check(program, packages)

	if flags[dumpTypes] {
		ast.Dump(program, 0)
	}

	if len(typeErrs) != 0 {
		printParserErrors(os.Stdout, typeErrs)
		return 1
	}

	comp := compiler.NewWithState(symbolTable, constants)
	err = comp.CompilePackages(packages)
	if err != nil {
		fmt.Printf("compiler error: %s\n", err)
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

	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()

	if flags[dumpAst] {
		ast.Dump(program, 0)
	}

	if len(p.Errors()) != 0 {
		printParserErrors(os.Stdout, p.Errors())
		return 1
	}

	ld := loader.New(program)
	sourceDir := filepath.Dir(filename)
	stdLib := filepath.Join(sourceDir, "stdlib")
	ld.SetPaths(stdLib, sourceDir)
	packages, tt, err := ld.Load(make(map[string]bool))
	if err != nil {
		fmt.Printf("loader error: %s\n", err)
		return 1
	}

	c := typechecker.NewWithModuleTypes(nil, tt)
	errs := c.Check(program, packages)

	if flags[dumpTypes] {
		ast.Dump(program, 0)
	}

	if len(errs) != 0 {
		printParserErrors(os.Stdout, errs)
		return 1
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
