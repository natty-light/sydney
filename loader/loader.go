package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sydney/ast"
	"sydney/codegen"
	"sydney/lexer"
	"sydney/parser"
	"sydney/token"
	"sydney/types"
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

func NewFromImports(imports []string) *Loader {
	importStmts := make([]*ast.ImportStatement, 0)
	for _, stmt := range imports {
		importStmts = append(importStmts, &ast.ImportStatement{
			Name: &ast.StringLiteral{
				Value: stmt,
			},
		})
	}
	return &Loader{
		imports: importStmts,
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

	var sources []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sy") || strings.HasSuffix(entry.Name(), "_test.sy") {
			continue
		}
		source, err := l.Read(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

	allStructs := map[string]types.Type{}
	allInterfaces := map[string]types.Type{}
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

	pkg := &Package{}
	for _, source := range sources {
		p := parser.New(lexer.New(source))
		p.SetDefinedTypes(allStructs, allInterfaces)
		program := p.ParseProgram()
		if len(p.Errors()) > 0 {
			return nil, fmt.Errorf(strings.Join(p.Errors(), "\n"))
		}

		deriveImports := codegen.ScanDeriveImports(source)
		for _, di := range deriveImports {
			program.Stmts = append([]ast.Stmt{&ast.ImportStatement{
				Name: &ast.StringLiteral{Value: di},
			}}, program.Stmts...)
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

func (l *Loader) Load(visited map[string]bool) ([]*Package, map[string]map[string]types.Type, map[string]bool, error) {
	packages := make([]*Package, 0)
	moduleTypes := make(map[string]map[string]types.Type)
	genericNames := make(map[string]bool)
	for _, imp := range l.imports {
		name := imp.Name.Value
		if visited[name] {
			continue
		}
		visited[name] = true
		dir, err := l.resolveDir(name)
		if err != nil {
			return nil, nil, nil, err
		}
		pkg, err := l.LoadPackage(dir)
		if err != nil {
			return nil, nil, nil, err
		}
		pkgTypes := l.ExtractTypes(pkg)
		moduleTypes[name] = pkgTypes
		moduleGenerics := l.ExtractGenericNames(pkg)
		for gn := range moduleGenerics {
			genericNames[gn] = true
		}

		for _, program := range pkg.Programs {
			child := New(program)
			child.stdLib = l.stdLib
			child.sourceDir = l.sourceDir
			childPkgs, childPkgTypes, childGenericNames, err := child.Load(visited)
			if err != nil {
				return nil, nil, nil, err
			}
			for mod, tt := range childPkgTypes {
				moduleTypes[mod] = tt
			}
			for gn := range childGenericNames {
				genericNames[gn] = true
			}

			packages = append(packages, childPkgs...)
		}

		packages = append(packages, pkg)
	}

	return packages, moduleTypes, genericNames, nil
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

func ResolveStdlib(sourceDir string) string {
	if root := os.Getenv("SYDNEY_PATH"); root != "" {
		return filepath.Join(root, "stdlib")
	}
	return filepath.Join(sourceDir, "stdlib")
}

func (l *Loader) ExtractTypes(pkg *Package) map[string]types.Type {
	tt := map[string]types.Type{}
	for _, prog := range pkg.Programs {
		for _, stmt := range prog.Stmts {
			if pub, ok := stmt.(*ast.PubStatement); ok {
				if sd, ok := pub.Stmt.(*ast.StructDefinitionStmt); ok {
					tt[sd.Name.Value] = sd.Type
				}
				if id, ok := pub.Stmt.(*ast.InterfaceDefinitionStmt); ok {
					id.Type.MethodIndices = make(map[string]int)
					for i, mn := range id.Type.Methods {
						id.Type.MethodIndices[mn] = i
					}
					tt[id.Name.Value] = id.Type
				}
			}
		}
	}
	return tt
}

func ScanImports(source string) []string {
	lx := lexer.New(source)
	imports := make([]string, 0)
	for {
		tok := lx.NextToken()
		if tok.Type == token.Import {
			tok = lx.NextToken()
			if tok.Type == token.String {
				imports = append(imports, tok.Literal)
			}
		} else if tok.Type == token.EOF {
			break
		} else if tok.Type != token.Module && tok.Type != token.String {
			break
		}
	}

	return imports
}

func (l *Loader) ExtractGenericNames(pkg *Package) map[string]bool {
	names := make(map[string]bool)
	for _, prog := range pkg.Programs {
		for _, stmt := range prog.Stmts {
			if pub, ok := stmt.(*ast.PubStatement); ok {
				if fd, ok := pub.Stmt.(*ast.FunctionDeclarationStmt); ok {
					if len(fd.TypeParams) > 0 {
						names[fd.Name.Value] = true
					}
				}

				if sd, ok := pub.Stmt.(*ast.StructDefinitionStmt); ok {
					if len(sd.Type.TypeParams) > 0 {
						names[sd.Name.Value] = true
					}
				}
			}
		}
	}

	return names
}
