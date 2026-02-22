package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sydney/compiler"
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
			// TODO: implement writing intermediate bytecode file
			Compile(args[2])
			fmt.Println("Honk! Compile not implemented yet")
		} else if args[1] == "exec" {
			// TODO: implement reading intermediate bytecode file
			fmt.Println("Honk! Exec not implemented yet")
		} else if args[1] == "help" {
			fmt.Println("Usage: quonk [run|compile|exec|help] [filename]")
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

	comp := compiler.New()
	err = comp.Compile(program)
	if err != nil {
		fmt.Printf("Honk! Compiler error: %s\n", err)
		return
	}

	bytecode := comp.Bytecode()
	var out bytes.Buffer
	out.Write(bytecode.Instructions)
	out.Write([]byte("\n"))
	// TODO: come up with way of encoding constants as bytes

	fi, err := os.Create("instructions.txt")
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := fi.Close(); err != nil {
			panic(err)
		}
	}()

	if _, err = fi.WriteString(bytecode.Instructions.String()); err != nil {
		panic(err)
	}

	fi.WriteString("\n")

	for _, c := range bytecode.Constants {
		switch c := c.(type) {
		case *object.CompiledFunction:
			fi.WriteString(c.Instructions.String())
		default:
			fi.WriteString(c.Inspect())
		}
		fi.WriteString("\n")
	}
}

func Exec(filename string) {
	file, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Honk! Cannot read file %s\n", filename)
		return
	}

	bytecode := parseFileIntoBytecode(file)
	machine := vm.New(bytecode)
	err = machine.Run()
	if err != nil {
		fmt.Printf("Honk! Runtime error: %s\n", err)
		return
	}

	stackTop := machine.StackTop()
	fmt.Println(stackTop.Inspect())
}

func parseFileIntoBytecode(file []byte) *compiler.Bytecode {
	return &compiler.Bytecode{}
}

func printParserErrors(out io.Writer, errors []string) {
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
