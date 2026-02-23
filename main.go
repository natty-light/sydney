package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sydney/compiler"
	"sydney/irgen"
	"sydney/lexer"
	"sydney/object"
	"sydney/parser"
	"sydney/repl"
	"sydney/typechecker"
	"sydney/vm"
)

// TODO : unfuck this
func main() {

	args := os.Args

	if len(args) == 1 {
		// If no filename was passed as a command line argument, run the repl
		repl.StartVM(os.Stdin, os.Stdout)
	} else {
		if args[1] == "run" {
			Run(args[2])
		} else if args[1] == "compile" {
			Compile(args[2])
		} else if args[1] == "help" {
			fmt.Println("Usage: quonk [run|compile|help] [filename]")
		}
	}

}

func Run(filename string) {
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
		return
	}

	src := string(file)

	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		printParserErrors(os.Stdout, p.Errors())
		return
	}

	c := typechecker.New(typeEnv)
	typeErrs := c.Check(program)

	if len(typeErrs) != 0 {
		printParserErrors(os.Stdout, typeErrs)
		return
	}

	comp := compiler.NewWithState(symbolTable, constants)
	err = comp.Compile(program)
	if err != nil {
		fmt.Printf("Compiler error: %s\n", err)
		return
	}

	machine := vm.NewWithGlobalStore(comp.Bytecode(), globals)
	err = machine.Run()
	if err != nil {
		fmt.Printf("Runtime error: %s\n", err)
		return
	}
}

func Compile(filename string) {
	file, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Honk! Cannot read file %s\n", filename)
		return
	}

	src := string(file)

	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		printParserErrors(os.Stdout, p.Errors())
		return
	}

	c := typechecker.New(nil)
	errs := c.Check(program)
	if len(errs) != 0 {
		printParserErrors(os.Stdout, errs)
	}

	i := irgen.New()
	err = i.Emit(program)

	if err != nil {
		fmt.Printf("Compiler error: %s\n", err)
		return
	}
	i.Write(strings.Replace(filename, ".sy", ".ll", -1))
}

func printParserErrors(out io.Writer, errors []string) {
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
