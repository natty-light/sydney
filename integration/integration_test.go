package integration

import (
	"os"
	"path/filepath"
	"sydney/ast"
	"sydney/codegen"
	"sydney/compiler"
	"sydney/loader"
	"sydney/object"
	"sydney/typechecker"
	"sydney/vm"
	"testing"
)

func runIntegration(t *testing.T, dir string, mainSource string) object.Object {
	t.Helper()

	imports := loader.ScanImports(mainSource)
	deriveImports := codegen.ScanDeriveImports(mainSource)
	imports = append(imports, deriveImports...)
	ld := loader.NewFromImports(imports)

	stdLib := filepath.Join(dir, "stdlib")
	ld.SetPaths(stdLib, dir)

	program, errs := ld.Parse(mainSource)
	if errs != nil {
		t.Fatalf("parse errors: %v", errs)
	}

	packages, tt, _, err := ld.Load(make(map[string]bool))
	if err != nil {
		t.Fatalf("loader error: %s", err)
	}

	for _, pkg := range packages {
		for _, pr := range pkg.Programs {
			codegen.ExpandDerives(pr)
		}
	}
	codegen.ExpandDerives(program)

	typeEnv := typechecker.NewTypeEnv(nil)
	c := typechecker.NewWithModuleTypes(typeEnv, tt)
	typeErrs := c.Check(program, packages)
	if len(typeErrs) != 0 {
		t.Fatalf("type errors: %v", typeErrs)
	}

	ast.FilterGenericTemplates(program)
	for _, pkg := range packages {
		for _, prog := range pkg.Programs {
			ast.FilterGenericTemplates(prog)
		}
	}

	symbolTable := compiler.NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}
	comp := compiler.NewWithState(symbolTable, []object.Object{})
	err = comp.CompilePackages(packages)
	if err != nil {
		t.Fatalf("compile packages error: %s", err)
	}
	err = comp.Compile(program)
	if err != nil {
		t.Fatalf("compile error: %s", err)
	}

	machine := vm.New(comp.Bytecode())
	err = machine.Run()
	if err != nil {
		t.Fatalf("vm error: %s", err)
	}

	return machine.LastPoppedStackElem()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %s", path, err)
	}
}

func TestMultiFilePackage(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "stdlib", "math", "math.sy"), `
module "math"

pub define struct Vec { x int, y int }

pub func add(Vec a, Vec b) -> Vec {
	Vec { x: a.x + b.x, y: a.y + b.y };
}
`)
	writeFile(t, filepath.Join(dir, "stdlib", "math", "ops.sy"), `
module "math"

pub func dot(Vec a, Vec b) -> int {
	a.x * b.x + a.y * b.y;
}
`)

	result := runIntegration(t, dir, `
import "math"

const a = math:Vec { x: 1, y: 2 };
const b = math:Vec { x: 3, y: 4 };
const int d = a.dot(b);
d;
`)

	assertInteger(t, result, 11)
}

func TestCrossFileStructAccess(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "stdlib", "geo", "point.sy"), `
module "geo"

pub define struct Point { x int, y int }
`)
	writeFile(t, filepath.Join(dir, "stdlib", "geo", "util.sy"), `
module "geo"

pub func origin() -> Point {
	Point { x: 0, y: 0 };
}

pub func move(Point p, int dx, int dy) -> Point {
	Point { x: p.x + dx, y: p.y + dy };
}
`)

	result := runIntegration(t, dir, `
import "geo"

const p = geo:origin();
const moved = p.move(5, 10);
moved.x + moved.y;
`)

	assertInteger(t, result, 15)
}

func TestPackageWithStructMethod(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "stdlib", "shapes", "shapes.sy"), `
module "shapes"

pub define struct Rect { w int, h int }

pub func area(Rect r) -> int {
	r.w * r.h;
}
`)

	result := runIntegration(t, dir, `
import "shapes"

const r = shapes:Rect { w: 3, h: 4 };
shapes:area(r);
`)

	assertInteger(t, result, 12)
}

func TestPackageWithInterface(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "stdlib", "shapes", "shapes.sy"), `
module "shapes"

pub define interface Shape {
	area() -> int
}

pub define struct Rect { w int, h int }

define implementation Rect -> Shape

pub func area(Rect r) -> int {
	r.w * r.h;
}

pub func getArea(Shape s) -> int {
	s.area();
}
`)

	result := runIntegration(t, dir, `
import "shapes"

const r = shapes:Rect { w: 3, h: 4 };
shapes:getArea(r);
`)

	assertInteger(t, result, 12)
}

func TestTwoPassTypeResolution(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "stdlib", "order", "b_second.sy"), `
module "order"

pub func make() -> First {
	First { val: 42 };
}
`)
	writeFile(t, filepath.Join(dir, "stdlib", "order", "a_first.sy"), `
module "order"

pub define struct First { val int }
`)

	result := runIntegration(t, dir, `
import "order"

const f = order:make();
f.val;
`)

	assertInteger(t, result, 42)
}

func TestMultiplePackageImports(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "stdlib", "alpha", "alpha.sy"), `
module "alpha"

pub func val() -> int { 10; }
`)
	writeFile(t, filepath.Join(dir, "stdlib", "beta", "beta.sy"), `
module "beta"

pub func val() -> int { 20; }
`)

	result := runIntegration(t, dir, `
import "alpha"
import "beta"

alpha:val() + beta:val();
`)

	assertInteger(t, result, 30)
}

func TestModuleNameMismatchError(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "stdlib", "bad", "a.sy"), `
module "bad"

pub func x() -> int { 1; }
`)
	writeFile(t, filepath.Join(dir, "stdlib", "bad", "b.sy"), `
module "wrong"

pub func y() -> int { 2; }
`)

	mainSource := `import "bad"
bad:x();`

	ld := loader.NewFromImports(loader.ScanImports(mainSource))
	ld.SetPaths(filepath.Join(dir, "stdlib"), dir)

	_, _, _, err := ld.Load(make(map[string]bool))
	if err == nil {
		t.Fatal("expected error for mismatched module names, got none")
	}
}

func TestScanImports(t *testing.T) {
	tests := []struct {
		source   string
		expected []string
	}{
		{`import "net"`, []string{"net"}},
		{`import "net"
import "http"`, []string{"net", "http"}},
		{`module "foo"
import "bar"`, []string{"bar"}},
		{`const x = 5;`, []string{}},
	}

	for _, tt := range tests {
		result := loader.ScanImports(tt.source)
		if len(result) != len(tt.expected) {
			t.Fatalf("input %q: expected %d imports, got %d", tt.source, len(tt.expected), len(result))
		}
		for i, exp := range tt.expected {
			if result[i] != exp {
				t.Errorf("input %q: import %d wrong, want %s, got %s", tt.source, i, exp, result[i])
			}
		}
	}
}

func TestStructMethodAcrossFiles(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "stdlib", "counter", "types.sy"), `
module "counter"

pub define struct Counter { val int }

pub func create(int start) -> Counter {
	Counter { val: start };
}
`)
	writeFile(t, filepath.Join(dir, "stdlib", "counter", "methods.sy"), `
module "counter"

pub func increment(Counter c, int n) -> Counter {
	Counter { val: c.val + n };
}

pub func value(Counter c) -> int {
	c.val;
}
`)

	result := runIntegration(t, dir, `
import "counter"

const c = counter:create(10);
const updated = c.increment(5);
updated.value();
`)

	assertInteger(t, result, 15)
}

func TestTransitiveDependencies(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "stdlib", "base", "base.sy"), `
module "base"

pub func double(int x) -> int { x * 2; }
`)
	writeFile(t, filepath.Join(dir, "stdlib", "mid", "mid.sy"), `
module "mid"

import "base"

pub func quadruple(int x) -> int { base:double(base:double(x)); }
`)

	result := runIntegration(t, dir, `
import "mid"

mid:quadruple(3);
`)

	assertInteger(t, result, 12)
}

func assertInteger(t *testing.T, obj object.Object, expected int64) {
	t.Helper()
	result, ok := obj.(*object.Integer)
	if !ok {
		t.Fatalf("object is not Integer. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("integer value wrong. want=%d, got=%d", expected, result.Value)
	}
}

func assertString(t *testing.T, obj object.Object, expected string) {
	t.Helper()
	result, ok := obj.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Errorf("string value wrong. want=%q, got=%q", expected, result.Value)
	}
}
