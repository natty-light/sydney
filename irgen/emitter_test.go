package irgen

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sydney/ast"
	"sydney/lexer"
	"sydney/loader"
	"sydney/parser"
	"sydney/typechecker"
)

func TestIntInfixExpr(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{source: "print(1 + 2);", expected: "3"},
	})
}

func TestVarDeclarations(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{source: `mut float pi = 3.0 + 0.14; print(pi);`, expected: "3.14"},
	})
}

func TestVarAssignment(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{source: `mut float pi = 3.0 + 0.14; pi = 2.0; print(pi);`, expected: "2"},
	})
}

func TestIfExpr(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source: `mut int x = 0;
mut int y = 0;
if (x == y) {
    print("true");
} else {
    print("false");
}
mut z = if (x == y) { 1; } else { 0; };
print(z);`,
			expected: "true1",
		},
	})
}

func TestForLoop(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source:   `mut int i = 0; for (i < 5) { print(i); i = i + 1; }`,
			expected: "01234",
		},
	})
}

func TestNestedBlocks(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source:   `mut int i = 0; for (i < 10) { if (i % 2 == 0) { print(i); } i = i + 1; }`,
			expected: "02468",
		},
	})
}

func TestFunctions(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source: `func addFive(int i) -> int { const int r = i + 5; r; }
func addSix(int i) -> int { const int r = i + 6; return r; }
const int x = 0;
const xPlusSix = addSix(x);
const xPlusFive = addFive(x);
print("x = ", x);
print("x + 5 = ", xPlusFive);
print("x + 6 = ", xPlusSix);`,
			expected: "x = 0x + 5 = 5x + 6 = 6",
		},
	})
}

func TestStructs(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source: `define struct Point { x int, y int }
const Point p = Point { x: 0, y: 0 };
print(p.x);
print(p.y);`,
			expected: "00",
		},
	})
}

func TestSelectorAssignment(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source: `define struct Point { x int, y int }
const Point p = Point { x: 0, y: 0 };
print(p.x);
print(p.y);
p.x = 1;
p.y = 2;
print(p.x);
print(p.y);`,
			expected: "0012",
		},
	})
}

func TestInterfaceDispatch(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source: `define struct Circle { radius float }
define struct Rect { w float, h float}
define interface Area { area() -> float }
func area(Circle c) -> float { const pi = 3.14; return c.radius * c.radius * pi; }
func area(Rect r) -> float { return r.w * r.h; }
const Circle c = Circle { radius: 2.0 };
const Rect r = Rect { w: 2.0, h: 2.0 };
func getArea(Area a) -> float { return a.area(); }
const rectA = getArea(r);
print("rect area: ", rectA);
const circA = getArea(c);
print("circle area: ", circA);`,
			expected: "rect area: 4circle area: 12.56",
		},
	})
}

func TestArrays(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `const array<int> a = [0, 1, 2, 3];
print(a[1]);`,
			expected: "1",
		},
	}

	runE2ETests(t, tests)
}

func TestAnonymousFunction(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source: `const anon = func() -> int { return 1; };
print(anon());
const adder = func(int a, int b) -> int { return a + b };
const sum = adder(1, 2);
print(sum);`,
			expected: "13",
		},
	})
}

func TestCaptures(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source: `const int x = 10;
const f = func(int y) -> int { return x + y };
print(f(5));`,
			expected: "15",
		},
	})
}

func TestNestedClosures(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source: `const x = 10;
const getAdder = func(int y) -> fn<(int) -> int> { return func(int z) -> int { return x + y + z }; };
const fiveAdder = getAdder(5);
const sixAdder = getAdder(6);
print(fiveAdder(5));
print(sixAdder(5));`,
			expected: "2021",
		},
	})
}

func TestLocalVariables(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source:   `func addTwo(int x) -> int { return x + 2 }; print(addTwo(3));`,
			expected: "5",
		},
	})
}

func TestMapAllocationAndAssignment(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `const map<int, int> m = { 1: 0 }
m[0] = 1;
const a = match m[0] {
	some(v) -> { v; },
	none -> { 99; },
};
print(a);
const b = match m[1] {
	some(v) -> { v; },
	none -> { 99; },
};
print(b);
const c = match m[2] {
	some(v) -> { v; },
	none -> { 99; },
};
print(c);`,
			expected: "1099",
		},
	}
	runE2ETests(t, tests)
}

func TestNestedCollections(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `const map<string, array<int>> M = { "hello": [1, 2, 3] };
			const val = match M["hello"] {
				some(arr) -> { arr[0]; },
				none -> { 0; },
			};
			print(val);`,
			expected: "1",
		},
	}
	runE2ETests(t, tests)
}

func TestRecursiveClosure(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source: `const countDown = func(int x) -> int {
    if (x == 0) { return 0; }
    return countDown(x - 1);
};
print(countDown(5));`,
			expected: "0",
		},
	})
}

func TestLocalRecursiveClosure(t *testing.T) {
	runE2ETests(t, []e2eTestCase{
		{
			source: `func callCountDown() -> int {
    const countDown = func(int x) -> int {
        print(x);
        if (x == 0) { return 0; }
        return countDown(x - 1);
    };
    return countDown(5);
}
print(callCountDown());`,
			expected: "5432100",
		},
	})
}

// e2e tests: emit IR → llc → clang → run binary → check stdout

type e2eTestCase struct {
	source   string
	expected string
}

func runE2ETests(t *testing.T, tests []e2eTestCase) {
	t.Helper()

	// find project root (where sydney_rt lives)
	projectRoot, err := filepath.Abs(filepath.Join("..", "."))
	if err != nil {
		t.Fatalf("cannot find project root: %v", err)
	}
	rtLib := filepath.Join(projectRoot, "sydney_rt", "target", "release")
	if _, err := os.Stat(filepath.Join(rtLib, "libsydney_rt.a")); err != nil {
		t.Skip("libsydney_rt.a not found, skipping e2e test")
	}

	for _, tt := range tests {
		tt := tt
		t.Run("", func(t *testing.T) {
			tmpDir := t.TempDir()

			// emit IR
			l := lexer.New(tt.source)
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}
			c := typechecker.New(nil)
			c.Check(program, nil)
			if len(c.Errors()) != 0 {
				t.Fatalf("typechecker errors: %v", c.Errors())
			}
			ast.FilterGenericTemplates(program)
			e := New()
			if err := e.Emit(program, nil); err != nil {
				t.Fatalf("emitter error: %v", err)
			}

			llFile := filepath.Join(tmpDir, "test.ll")
			objFile := filepath.Join(tmpDir, "test.o")
			binFile := filepath.Join(tmpDir, "test")

			os.WriteFile(llFile, []byte(e.buf.String()), 0o644)

			// llc
			cmd := exec.Command("llc", "-filetype=obj", llFile, "-o", objFile)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("llc failed: %s\n%s", err, out)
			}

			// clang link
			cmd = exec.Command("clang", objFile, "-L"+rtLib, "-lsydney_rt", "-o", binFile)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("clang failed: %s\n%s", err, out)
			}

			// run
			cmd = exec.Command(binFile)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("binary failed: %s\n%s", err, out)
			}

			actual := string(out)
			if actual != tt.expected {
				t.Fatalf("expected:\n%s\ngot:\n%s", tt.expected, actual)
			}
		})
	}
}

func TestE2EForLoops(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `mut sum = 0; for (mut i = 0; i < 5; i = i + 1) { sum = sum + i; } print(sum);`,
			expected: "10",
		},
		{ // three-part for with break
			source:   `mut sum = 0; for (mut i = 0; i < 10; i = i + 1) { if (i == 3) { break; } sum = sum + i; } print(sum);`,
			expected: "3",
		},
		{ // continue skips even numbers
			source:   `mut sum = 0; for (mut i = 0; i < 6; i = i + 1) { if (i % 2 == 0) { continue; } sum = sum + i; } print(sum);`,
			expected: "9",
		},
		{ // reuse loop var name
			source:   `for (mut i = 0; i < 2; i = i + 1) { print(i); } for (mut i = 10; i < 12; i = i + 1) { print(i); }`,
			expected: "011011",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EStringEquality(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `print("hello" == "hello"); print("hello" == "world"); print("a" != "b");`,
			expected: "truefalsetrue",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EConversionBuiltins(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `print(int('a')); print(char(byte(72)));`,
			expected: "97H",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EModulo(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `print(10 % 3); print(15 % 5);`,
			expected: "10",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EAppend(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `const array<int> a = [1, 2, 3]; const b = append(a, 4); print(len(b)); print(b[3]);`,
			expected: "44",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EEscapeSequences(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `print("hello\tworld"); print("line1\nline2");`,
			expected: "hello\tworldline1\nline2",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EIfAsStatement(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `mut r = 0; for (mut i = 0; i < 4; i = i + 1) { if (i % 2 == 0) { r = r + 1; } else { r = r + 10; } } print(r);`,
			expected: "22",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EArraySlice(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `const a = [1, 2, 3, 4, 5]; const b = a[1:4]; print(len(b)); print(b[0]); print(b[1]); print(b[2]);`,
			expected: "3234",
		},
		{
			source:   `const a = [10, 20, 30]; const b = a[0:2]; print(len(b)); print(b[0]); print(b[1]);`,
			expected: "21020",
		},
		{
			source:   `const a = [1, 2, 3, 4, 5]; const b = a[3:]; print(len(b)); print(b[0]); print(b[1]);`,
			expected: "245",
		},
		{
			source:   `const a = [1, 2, 3, 4, 5]; const b = a[:2]; print(len(b)); print(b[0]); print(b[1]);`,
			expected: "212",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EStringSlice(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `const s = "Hello, World!"; print(s[0:5]);`,
			expected: "Hello",
		},
		{
			source:   `const s = "Hello, World!"; print(s[7:12]);`,
			expected: "World",
		},
		{
			source:   `const s = "abcdef"; print(s[2:]);`,
			expected: "cdef",
		},
		{
			source:   `const s = "abcdef"; print(s[:3]);`,
			expected: "abc",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EGenericFunctions(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `func identity<T>(T x) -> T { x; }
			         print(identity<int>(42));`,
			expected: "42",
		},
		{
			source: `func identity<T>(T x) -> T { x; }
			         print(identity<string>("hello"));`,
			expected: "hello",
		},
		{
			source: `func first<T>(array<T> a) -> T { a[0]; }
			         print(first<int>([10, 20, 30]));`,
			expected: "10",
		},
		{
			source: `func sum<T>(array<T> vals) -> T {
				mut T acc = 0;
				for (mut i = 0; i < len(vals); i = i + 1) {
					acc = acc + vals[i];
				}
				acc;
			}
			print(sum<int>([1, 2, 3, 4, 5]));`,
			expected: "15",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EGenericStructs(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `define struct Box<T> { value T }
			         const b = Box<int> { value: 42 };
			         print(b.value);`,
			expected: "42",
		},
		{
			source: `define struct Box<T> { value T }
			         const b = Box<string> { value: "hello" };
			         print(b.value);`,
			expected: "hello",
		},
		{
			source: `define struct Box<T> { value T }
			         const a = Box<int> { value: 5 };
			         const b = Box<int> { value: 10 };
			         print(a.value + b.value);`,
			expected: "15",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EForInArray(t *testing.T) {
	tests := []e2eTestCase{
		// Basic value iteration
		{
			source:   `const a = [1, 2, 3]; for (v in a) { print(v); }`,
			expected: "123",
		},
		// Index and value iteration
		{
			source:   `const a = [10, 20, 30]; for (i, v in a) { print(i); print(v); }`,
			expected: "010120230",
		},
		// Sum with accumulator
		{
			source:   `mut s = 0; const a = [1, 2, 3, 4]; for (v in a) { s = s + v; } print(s);`,
			expected: "10",
		},
		// Nested for-in
		{
			source: `const a = [1, 2];
			         const b = [10, 20];
			         for (x in a) { for (y in b) { print(x * y); } }`,
			expected: "10202040",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EForInMap(t *testing.T) {
	tests := []e2eTestCase{
		// Map iteration - print values
		{
			source:   `mut s = 0; const m = {"a": 1, "b": 2, "c": 3}; for (k, v in m) { s = s + v; } print(s);`,
			expected: "6",
		},
		// Map iteration with int keys
		{
			source:   `mut s = 0; const m = {1: 10, 2: 20}; for (k, v in m) { s = s + k + v; } print(s);`,
			expected: "33",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EBufferedChannel(t *testing.T) {
	tests := []e2eTestCase{
		{
			source:   `const ch = chan<int>(1); ch <- 42; const val = <- ch; print(val);`,
			expected: "42",
		},
	}
	runE2ETests(t, tests)
}

func TestE2ESpawnWithChannel(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `const ch = chan<int>();
			spawn func() {
				ch <- 99;
			}();
			const val = <- ch;
			print(val);`,
			expected: "99",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EOptionMatch(t *testing.T) {
	tests := []e2eTestCase{
		{ // some arm
			source: `const option<int> x = some(42);
			const y = match x {
				some(val) -> { val + 1; },
				none -> { 0; },
			};
			print(y);`,
			expected: "43",
		},
		{ // none arm
			source: `const option<int> x = none();
			const y = match x {
				some(val) -> { val + 1; },
				none -> { 0; },
			};
			print(y);`,
			expected: "0",
		},
		{ // none first ordering
			source: `const option<int> x = some(10);
			const y = match x {
				none -> { 0; },
				some(val) -> { val * 2; },
			};
			print(y);`,
			expected: "20",
		},
	}
	runE2ETests(t, tests)
}

func TestE2ETypeMatch(t *testing.T) {
	tests := []e2eTestCase{
		{ // match first arm
			source: `define struct Circle { radius int }
			define struct Rect { w int, h int }
			define interface Shape { area() -> int }
			func area(Circle c) -> int { 3 * c.radius * c.radius; }
			func area(Rect r) -> int { r.w * r.h; }
			func describe(Shape s) -> int {
				match typeof s {
					Circle(c) -> { c.radius; },
					Rect(r) -> { r.w + r.h; },
					_ -> { 0; },
				};
			}
			const c = Circle { radius: 5 };
			print(describe(c));`,
			expected: "5",
		},
		{ // match second arm
			source: `define struct Circle { radius int }
			define struct Rect { w int, h int }
			define interface Shape { area() -> int }
			func area(Circle c) -> int { 3 * c.radius * c.radius; }
			func area(Rect r) -> int { r.w * r.h; }
			func describe(Shape s) -> int {
				match typeof s {
					Circle(c) -> { c.radius; },
					Rect(r) -> { r.w + r.h; },
					_ -> { 0; },
				};
			}
			const r = Rect { w: 3, h: 4 };
			print(describe(r));`,
			expected: "7",
		},
		{ // match default arm
			source: `define struct Circle { radius int }
			define struct Rect { w int, h int }
			define struct Tri { b int }
			define interface Shape { area() -> int }
			func area(Circle c) -> int { 3 * c.radius * c.radius; }
			func area(Rect r) -> int { r.w * r.h; }
			func area(Tri t) -> int { t.b; }
			func describe(Shape s) -> int {
				match typeof s {
					Circle(c) -> { c.radius; },
					Rect(r) -> { r.w + r.h; },
					_ -> { 99; },
				};
			}
			const t = Tri { b: 10 };
			print(describe(t));`,
			expected: "99",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EArrayBoundsCheck(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", "."))
	if err != nil {
		t.Fatalf("cannot find project root: %v", err)
	}
	rtLib := filepath.Join(projectRoot, "sydney_rt", "target", "release")
	if _, err := os.Stat(filepath.Join(rtLib, "libsydney_rt.a")); err != nil {
		t.Skip("libsydney_rt.a not found, skipping e2e test")
	}

	source := `const array<int> a = [1, 2, 3]; const x = a[10];`
	tmpDir := t.TempDir()

	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	c := typechecker.New(nil)
	c.Check(program, nil)
	if len(c.Errors()) != 0 {
		t.Fatalf("typechecker errors: %v", c.Errors())
	}
	ast.FilterGenericTemplates(program)
	e := New()
	if err := e.Emit(program, nil); err != nil {
		t.Fatalf("emitter error: %v", err)
	}

	llFile := filepath.Join(tmpDir, "test.ll")
	objFile := filepath.Join(tmpDir, "test.o")
	binFile := filepath.Join(tmpDir, "test")

	os.WriteFile(llFile, []byte(e.buf.String()), 0o644)

	cmd := exec.Command("llc", "-filetype=obj", llFile, "-o", objFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("llc failed: %s\n%s", err, out)
	}

	cmd = exec.Command("clang", objFile, "-L"+rtLib, "-lsydney_rt", "-o", binFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clang failed: %s\n%s", err, out)
	}

	cmd = exec.Command(binFile)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit from OOB access, but got success")
	}

	output := string(out)
	if !strings.Contains(output, "panic: array index out of bounds: index 10 but length is 3") {
		t.Fatalf("expected OOB panic message in stderr, got: %s", output)
	}
}

func TestE2ESockets(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", "."))
	if err != nil {
		t.Fatalf("cannot find project root: %v", err)
	}
	rtLib := filepath.Join(projectRoot, "sydney_rt", "target", "release")
	if _, err := os.Stat(filepath.Join(rtLib, "libsydney_rt.a")); err != nil {
		t.Skip("libsydney_rt.a not found, skipping e2e test")
	}

	// Start a Go TCP server on a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not start test server: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	// Server: accept one connection, read what the client sends, echo it back.
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
	}()

	// Sydney client: connect, write, read, print.
	source := fmt.Sprintf(`
import "net"

const cr = net:connect("127.0.0.1", %d);
match cr {
    ok(sock) -> {
        const wr = net:write(sock, "hello", 5);
        const rr = net:read(sock, 1024);
        match rr {
            ok(data) -> { print(data); },
            err(msg) -> { print(msg); },
        }
        const cl = net:close(sock);
    },
    err(msg) -> { print(msg); },
}
`, port)

	// compile → run (same pipeline as other e2e tests)
	tmpDir := t.TempDir()

	ld := loader.NewFromImports([]string{"net"})
	ld.SetPaths(filepath.Join(projectRoot, "stdlib"), "")
	pkgs, tt, gns, err := ld.Load(map[string]bool{})
	if err != nil {
		t.Fatalf("failed to load net module: %v", err)
	}

	l := lexer.New(source)
	p := parser.NewWithGenericNames(l, gns)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	c := typechecker.NewWithModuleTypes(nil, tt)
	c.Check(program, pkgs)
	if len(c.Errors()) != 0 {
		t.Fatalf("typechecker errors: %v", c.Errors())
	}
	ast.FilterGenericTemplates(program)
	for _, pkg := range pkgs {
		for _, prog := range pkg.Programs {
			ast.FilterGenericTemplates(prog)
		}
	}
	e := New()
	if err := e.Emit(program, pkgs); err != nil {
		t.Fatalf("emitter error: %v", err)
	}

	llFile := filepath.Join(tmpDir, "test.ll")
	objFile := filepath.Join(tmpDir, "test.o")
	binFile := filepath.Join(tmpDir, "test")

	os.WriteFile(llFile, []byte(e.buf.String()), 0o644)

	cmd := exec.Command("llc", "-filetype=obj", llFile, "-o", objFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("llc failed: %s\n%s", err, out)
	}

	cmd = exec.Command("clang", objFile, "-L"+rtLib, "-lsydney_rt", "-L/opt/homebrew/opt/openssl/lib", "-lssl", "-lcrypto", "-o", binFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clang failed: %s\n%s", err, out)
	}

	cmd = exec.Command(binFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %s\n%s", err, out)
	}

	<-serverDone

	if string(out) != "hello" {
		t.Fatalf("expected hello, got %q", string(out))
	}
}

func TestE2EAnyTypeMatch(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `func check(any val) -> int {
				match typeof val {
					int(i) -> { i; },
					_ -> { 0; },
				};
			}
			print(check(42));`,
			expected: "42",
		},
		{
			source: `func check(any val) -> string {
				match typeof val {
					int(i) -> { "int"; },
					float(f) -> { "float"; },
					string(s) -> { s; },
					bool(b) -> { "bool"; },
					byte(b) -> { "byte"; },
					_ -> { "other"; },
				};
			}
			print(check("hello"));
			print(check(3.14));
			print(check(true));`,
			expected: "hellofloatbool",
		},
	}
	runE2ETests(t, tests)
}

func TestE2EAnyArrayBoxing(t *testing.T) {
	tests := []e2eTestCase{
		{
			source: `func sum_ints(array<any> args) -> int {
				mut total = 0;
				mut i = 0;
				for (i < len(args)) {
					const any a = args[i];
					match typeof a {
						int(n) -> { total = total + n; },
						_ -> {},
					};
					i = i + 1;
				}
				total;
			}
			print(sum_ints([1, 2, 3]));`,
			expected: "6",
		},
		{
			source: `func describe(array<any> items) -> string {
				mut res = "";
				mut i = 0;
				for (i < len(items)) {
					const any item = items[i];
					match typeof item {
						int(n) -> { res = res + "i"; },
						string(s) -> { res = res + "s"; },
						float(f) -> { res = res + "f"; },
						bool(b) -> { res = res + "b"; },
						_ -> { res = res + "?"; },
					};
					i = i + 1;
				}
				res;
			}
			print(describe(["hi", 1, 3.14, false]));`,
			expected: "sifb",
		},
	}
	runE2ETests(t, tests)
}
