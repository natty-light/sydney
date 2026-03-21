package vm

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sydney/ast"
	"sydney/codegen"
	"sydney/compiler"
	"sydney/lexer"
	"sydney/loader"
	"sydney/object"
	"sydney/parser"
	"sydney/typechecker"
	"testing"
)

func stdlibPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "stdlib")
}

func runDeriveTest(t *testing.T, source string) *VM {
	t.Helper()

	imports := loader.ScanImports(source)
	deriveImports := codegen.ScanDeriveImports(source)
	imports = append(imports, deriveImports...)
	ld := loader.NewFromImports(imports)
	ld.SetPaths(stdlibPath(), ".")
	packages, tt, gns, err := ld.Load(make(map[string]bool))
	if err != nil {
		t.Fatalf("loader error: %s", err)
	}

	l := lexer.New(source)
	p := parser.NewWithGenericNames(l, gns)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parse errors: %s", strings.Join(p.Errors(), "\n"))
	}

	codegen.ExpandDerives(program)

	c := typechecker.NewWithModuleTypes(typechecker.NewTypeEnv(nil), tt)
	typeErrs := c.Check(program, packages)
	if len(typeErrs) != 0 {
		t.Fatalf("type errors: %s", strings.Join(typeErrs, "\n"))
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
	if err := comp.CompilePackages(packages); err != nil {
		t.Fatalf("compiler error (packages): %s", err)
	}
	if err := comp.Compile(program); err != nil {
		t.Fatalf("compiler error: %s", err)
	}

	globals := make([]object.Object, GlobalsSize)
	machine := NewWithGlobalStore(comp.Bytecode(), globals)
	if err := machine.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}
	return machine
}

func captureOutput(t *testing.T, source string) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	runDeriveTest(t, source)

	w.Close()
	os.Stdout = old
	var buf [4096]byte
	n, _ := r.Read(buf[:])
	return string(buf[:n])
}

func TestDeriveJsonUnmarshalPrimitives(t *testing.T) {
	source := `
#[derive(json)]
define struct User { name string, age int, height float, active bool }
const res = unmarshal_json_User("{\"name\":\"alice\",\"age\":30,\"height\":5.5,\"active\":true}")
match res {
	ok(u) -> { print(u.name); print(u.age); print(u.height); print(u.active); },
	err(msg) -> { print("error: " + msg); },
}
`
	got := captureOutput(t, source)
	expected := "alice\n30\n5.5\ntrue\n"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDeriveJsonUnmarshalNestedStruct(t *testing.T) {
	source := `
#[derive(json)]
define struct Inner { x int, y int }
#[derive(json)]
define struct Outer { name string, inner Inner }
const res = unmarshal_json_Outer("{\"name\":\"test\",\"inner\":{\"x\":1,\"y\":2}}")
match res {
	ok(o) -> { print(o.name); print(o.inner.x); print(o.inner.y); },
	err(msg) -> { print("error: " + msg); },
}
`
	got := captureOutput(t, source)
	expected := "test\n1\n2\n"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDeriveJsonUnmarshalDeeplyNested(t *testing.T) {
	source := `
#[derive(json)]
define struct A { val int }
#[derive(json)]
define struct B { a A, label string }
#[derive(json)]
define struct C { b B, id int }
const res = unmarshal_json_C("{\"b\":{\"a\":{\"val\":42},\"label\":\"deep\"},\"id\":1}")
match res {
	ok(c) -> { print(c.b.a.val); print(c.b.label); print(c.id); },
	err(msg) -> { print("error: " + msg); },
}
`
	got := captureOutput(t, source)
	expected := "42\ndeep\n1\n"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDeriveJsonUnmarshalPrimitiveArrays(t *testing.T) {
	source := `
#[derive(json)]
define struct Data { nums array<int>, vals array<float>, tags array<string>, flags array<bool> }
const res = unmarshal_json_Data("{\"nums\":[1,2,3],\"vals\":[1.5,2.5],\"tags\":[\"a\",\"b\"],\"flags\":[true,false]}")
match res {
	ok(d) -> { print(d.nums); print(d.vals); print(d.tags); print(d.flags); },
	err(msg) -> { print("error: " + msg); },
}
`
	got := captureOutput(t, source)
	expected := "[1, 2, 3]\n[1.5, 2.5]\n[a, b]\n[true, false]\n"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDeriveJsonUnmarshalStructArray(t *testing.T) {
	source := `
#[derive(json)]
define struct Item { name string, val int }
#[derive(json)]
define struct Collection { items array<Item> }
const res = unmarshal_json_Collection("{\"items\":[{\"name\":\"a\",\"val\":1},{\"name\":\"b\",\"val\":2}]}")
match res {
	ok(c) -> {
		for (item in c.items) {
			print(item.name);
			print(item.val);
		}
	},
	err(msg) -> { print("error: " + msg); },
}
`
	got := captureOutput(t, source)
	expected := "a\n1\nb\n2\n"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDeriveJsonUnmarshalNestedArray(t *testing.T) {
	source := `
#[derive(json)]
define struct Matrix { grid array<array<int>> }
const res = unmarshal_json_Matrix("{\"grid\":[[1,2],[3,4]]}")
match res {
	ok(m) -> { print(m.grid); },
	err(msg) -> { print("error: " + msg); },
}
`
	got := captureOutput(t, source)
	expected := "[[1, 2], [3, 4]]\n"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDeriveJsonUnmarshalMissingField(t *testing.T) {
	source := `
#[derive(json)]
define struct Pt { x int, y int }
const res = unmarshal_json_Pt("{\"x\":1}")
match res {
	ok(p) -> { print("unexpected ok"); },
	err(msg) -> { print(msg); },
}
`
	got := captureOutput(t, source)
	if !strings.Contains(got, "missing field: y") {
		t.Errorf("expected missing field error, got %q", got)
	}
}

func TestDeriveJsonMarshalPrimitives(t *testing.T) {
	source := `
#[derive(json)]
define struct User { name string, age int }
const User u = User { name: "alice", age: 30 }
print(marshal_json_User(u))
`
	got := captureOutput(t, source)
	if !strings.Contains(got, `"name":"alice"`) || !strings.Contains(got, `"age":30`) {
		t.Errorf("unexpected marshal output: %q", got)
	}
}

func TestDeriveJsonMarshalNestedStruct(t *testing.T) {
	source := `
#[derive(json)]
define struct Inner { x int }
#[derive(json)]
define struct Outer { inner Inner, label string }
const Inner i = Inner { x: 42 }
const Outer o = Outer { inner: i, label: "test" }
print(marshal_json_Outer(o))
`
	got := captureOutput(t, source)
	if !strings.Contains(got, `"x":42`) || !strings.Contains(got, `"label":"test"`) {
		t.Errorf("unexpected marshal output: %q", got)
	}
}

func TestDeriveJsonMarshalPrimitiveArray(t *testing.T) {
	source := `
#[derive(json)]
define struct Data { nums array<int> }
const Data d = Data { nums: [1, 2, 3] }
print(marshal_json_Data(d))
`
	got := captureOutput(t, source)
	if !strings.Contains(got, `[1,2,3]`) {
		t.Errorf("unexpected marshal output: %q", got)
	}
}

func TestDeriveJsonMarshalStructArray(t *testing.T) {
	source := `
#[derive(json)]
define struct Item { name string, val int }
#[derive(json)]
define struct Bag { items array<Item> }
const Item a = Item { name: "a", val: 1 }
const Item b = Item { name: "b", val: 2 }
const Bag bag = Bag { items: [a, b] }
print(marshal_json_Bag(bag))
`
	got := captureOutput(t, source)
	if !strings.Contains(got, `"name":"a"`) || !strings.Contains(got, `"name":"b"`) {
		t.Errorf("unexpected marshal output: %q", got)
	}
}

func TestDeriveJsonRoundTrip(t *testing.T) {
	source := `
#[derive(json)]
define struct Point { x int, y int }
const Point pt = Point { x: 10, y: 20 }
const json_str = marshal_json_Point(pt)
const res = unmarshal_json_Point(json_str)
match res {
	ok(parsed) -> { print(parsed.x); print(parsed.y); },
	err(msg) -> { print("error: " + msg); },
}
`
	got := captureOutput(t, source)
	expected := "10\n20\n"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
