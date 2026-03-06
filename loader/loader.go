package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sydney/ast"
	"sydney/lexer"
	"sydney/parser"
)

type Loader struct {
	stdLib    string
	sourceDir string
	loaded    *Package
	imports   []*ast.ImportStatement
	loading   map[string]bool
}

type Package struct {
	Name     string
	Programs []*ast.Program
}

func New(program *ast.Program) *Loader {
	imports := make([]*ast.ImportStatement, 0)
	for _, stmt := range program.Stmts {
		if imp, ok := stmt.(*ast.ImportStatement); ok {
			imports = append(imports, imp)
		} else if _, ok := stmt.(*ast.ModuleDeclarationStmt); ok {
			continue
		} else {
			break // done with imports
		}
	}

	return &Loader{
		imports: imports,
	}
}

func (l *Loader) Read(filename string) (string, error) {

	file, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("cannot read file %s", filename)
	}

	return string(file), nil
}

func (l *Loader) Parse(source string) (*ast.Program, []string) {
	lx := lexer.New(source)
	p := parser.New(lx)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		return nil, p.Errors()
	}

	return program, nil
}

func (l *Loader) LoadPackage(dir string) (*Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pkg := &Package{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sy") {
			continue
		}

		source, err := l.Read(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}

		program, errors := l.Parse(source)
		if len(errors) > 0 {
			return nil, fmt.Errorf(strings.Join(errors, "\n"))
		}

		pkg.Programs = append(pkg.Programs, program)

	}

	if len(pkg.Programs) == 0 {
		return nil, fmt.Errorf("no .sy files in package: %s", dir)
	}

	var module string
	for _, prog := range pkg.Programs {
		for _, stmt := range prog.Stmts {
			if mod, ok := stmt.(*ast.ModuleDeclarationStmt); ok {
				if module == "" {
					module = mod.Name.Value
					continue
				}

				if module != mod.Name.Value {
					return nil, fmt.Errorf("module name %s does not match package name %s", mod.Name.Value, module)
				}
			}
		}
	}
	pkg.Name = module

	return pkg, nil
}

func (l *Loader) Load(visited map[string]bool) ([]*Package, error) {
	packages := make([]*Package, 0)
	for _, imp := range l.imports {
		name := imp.Name.Value
		if visited[name] {
			return nil, fmt.Errorf("circular import: %s", name)
		}
		visited[name] = true
		pkg, err := l.LoadPackage(name)
		if err != nil {
			return nil, err
		}

		for _, program := range pkg.Programs {
			child := New(program)
			child.stdLib = l.stdLib
			child.sourceDir = l.sourceDir
			childPkgs, err := child.Load(visited)
			if err != nil {
				return nil, err
			}

			packages = append(packages, childPkgs...)
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

func (l *Loader) resolveDir(name string) (string, error) {
	if strings.HasPrefix(name, "./") {
		return filepath.Join(l.sourceDir, name), nil
	}
	// stdlib lookup
	return filepath.Join(l.stdLib, name), nil
}

func (l *Loader) SetPaths(stdlib, sourceDir string) {
	l.stdLib = stdlib
	l.sourceDir = sourceDir
}
