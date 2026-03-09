# Sydney - A language that's the way I want it

Sydney is a compiled, statically-typed programming language. It compiles to bytecode and runs on a custom VM, or compiles to native binaries via LLVM IR.

## Values, literals, and types

### Primitive types
Sydney supports the following primitive types: `int`, `float`, `string`, `bool`, `byte`, and `null`.

```
mut int i = 10;
mut float f = 10.5;
mut string s = "hello";
mut bool b = true;
mut byte c = 'a';
mut null n = null;
```

Strings support escape sequences: `\n`, `\t`, `\r`, `\\`, `\"`, `\'`, and `\0`. Byte literals support the same escapes:
```
const msg = "hello\tworld\n";
const newline = '\n';
```

### Type conversions
Convert between primitive types using the `int()`, `byte()`, and `char()` builtins:
```
const code = int('a');       // 97
const b = byte(65);          // byte with value 65
const ch = char(byte(72));   // "H"
```

### Variables
A variable can either be mutable or constant. This is specified with the `mut` and `const` keywords.

Variable type annotations are optional for initialized variables:
```
mut x = 5;
const y = 5;
```

Constant variables must be initialized, mutable variables do not.
```
const x = 5;
mut int y;
```

### Functions
There are two ways of creating functions. Anonymous functions are treated as values, and can be assigned to variables:
```
const add = func(int a, int b) -> int {
    a + b;
};
```

Functions can also be declared in the traditional sense:
```
func add(int a, int b) -> int {
    a + b;
}
```

The last expression in a function body is implicitly returned, or `return` can be used explicitly.

Closures are supported:
```
const f = func(int x) -> fn<() -> int> {
    return func() -> int {
        return x;
    };
};

const store5 = f(5);
store5(); // returns 5;
```

### Maps
Maps are dictionaries with strict typings. The keys of a map must all be of the same type, as with the values.
Values are accessed with square bracket index notation. Map values can be updated by key.
```
const map<string, int> m = { "hello": 0, "world": 1 };
m["hello"]; // 0;
m["hello"] = 2;
m["hello"]; // 2;
```

### Arrays
Arrays also have strict types. The values must be homogenous. Square bracket index notation is again used to access and update by index. Indices start at 0.
```
const array<int> a = [1, 2, 3];
a[0]; // 1
a[2] = 1;
a; // [1, 2, 1];

const b = append(a, 4); // [1, 2, 1, 4]
```

### Structs
Structs allow for the creation of custom data types with named fields. They must be defined before they can be used.

```
define struct Point {
    x int,
    y int
}

const Point p = Point { x: 0, y: 0 };
p.x = 10;
p.x; // 10
```

Structs can be nested and passed as arguments to functions:
```
define struct Circle {
    center Point,
    radius int
}

func isOrigin(Point p) -> bool {
    return p.x == 0 && p.y == 0;
}

const Circle c = Circle { center: Point { x: 0, y: 0 }, radius: 5 };
isOrigin(c.center); // true
```

Functions that take a struct as their first argument can be called as methods on that struct using dot syntax:
```
func sum(Point p) -> int {
    return p.x + p.y;
}

const Point p = Point { x: 3, y: 4 };
sum(p);     // these two calls
p.sum();    // are equivalent
```

## Control flow

### If-else expressions
`if-else` blocks are expressions and produce a value, allowing:
```
const int bit = if (true) { 1 } else { 0 };
```

They can also be used as statements:
```
if (x > 0) {
    print("positive");
} else {
    print("non-positive");
}
```

### For loops
`for` loops support both a condition-only form and a three-part form with init, condition, and post:
```
for (mut i = 0; i < 10; i = i + 1) {
    print(i);
}

mut x = 0;
for (x < 10) {
    x = x + 1;
}
```

Loop variables declared in the init clause are scoped to the loop.

### Break and continue
`break` exits a loop early. `continue` skips to the next iteration:
```
for (mut i = 0; i < 10; i = i + 1) {
    if (i == 5) { break; }
    if (i % 2 == 0) { continue; }
    print(i);
}
```

### Match expressions
`match` is used to deconstruct result types. It is exhaustive — both `ok` and `err` arms must be provided:
```
func divide(int a, int b) -> result<int> {
    if (b == 0) { return err("division by zero"); }
    return ok(a / b);
}

const answer = match divide(10, 2) {
    ok(val) -> { val; },
    err(msg) -> { 0; },
};
```

### Result type
The `result<T>` type represents a value that may be an error. Construct with `ok(val)` or `err(msg)`, deconstruct with `match`:
```
func safeParse(string s) -> result<int> {
    return err("not implemented");
}
```

## Operators
Sydney supports standard arithmetic, comparison, and logical operators.

### Arithmetic
- `+`: Addition (and string concatenation)
- `-`: Subtraction
- `*`: Multiplication
- `/`: Division
- `%`: Modulo

### Comparison
- `==`: Equal to
- `!=`: Not equal to
- `<`: Less than
- `>`: Greater than
- `<=`: Less than or equal to
- `>=`: Greater than or equal to

### Logical
- `&&`: Logical AND
- `||`: Logical OR
- `!`: Logical NOT

## Built-in Functions
Sydney provides several built-in functions:
- `len(iterable)`: Returns the length of an array, string, or map.
- `print(args...)`: Prints the provided arguments to the console.
- `append(array, element)`: Returns a new array with the element appended.
- `int(byte)`: Converts a byte to an integer.
- `byte(int)`: Converts an integer to a byte.
- `char(byte)`: Converts a byte to a single-character string.

### File I/O
- `fopen(path)`: Opens a file and returns a file descriptor.
- `fread(fd)`: Reads the contents of a file.
- `fwrite(fd, data)`: Writes data to a file.
- `fclose(fd)`: Closes a file descriptor.

## Modules
Sydney supports a module system for organizing code across files. Modules are defined with the `module` keyword and imported with `import`. Public functions are exported with `pub`:

```
// strings.sy
module "strings"

pub func repeat(string str, int count) -> string {
    mut string r = "";
    for (mut i = 0; i < count; i = i + 1) {
        r = r + str;
    }
    return r;
}
```

```
// main.sy
import "strings"

print(strings:repeat("ha", 3)); // "hahaha"
```

Module functions are accessed with the `:` scope operator.

## Interfaces and Implementations
Sydney supports interfaces, which allow for polymorphism and dynamic dispatch. An interface defines a set of method signatures that a struct can implement.

### Defining an Interface
An interface defines a set of method signatures. It is defined using the `define interface` keywords.

```sydney
define interface Area {
    area() -> float
}
```

### Implementing an Interface
A struct can be declared to implement an interface using the `define implementation` statement. To satisfy an interface, a struct must have functions defined where the first argument is the struct itself, matching the interface's method names and signatures.

```sydney
define struct Circle {
    radius float
}

define struct Rect {
    w float,
    h float
}

func area(Circle c) -> float {
    const pi = 3.14;
    return c.radius * c.radius * pi;
}

func area(Rect r) -> float {
    return r.w * r.h;
}

define implementation Circle -> Area;
define implementation Rect -> Area;
```

### Polymorphism and Dynamic Dispatch
Interfaces can be used as parameter types in functions. This allows for polymorphism, where the same function can operate on different types that implement the same interface.

```sydney
func printArea(Area a) {
    print(a.area());
}

const c = Circle { radius: 5.0 };
const r = Rect { w: 10.0, h: 2.0 };

printArea(c); // Works with Circle
printArea(r); // Works with Rect
```

When a concrete struct is passed to a function expecting an interface, Sydney "boxes" the struct into an interface object. This object contains the original struct value and a method table (itab) that allows the VM to perform dynamic dispatch—finding and calling the correct method at runtime even when the concrete type is hidden behind the interface.

The type checker verifies that all required methods are implemented with matching signatures before allowing an implementation to be defined.

## Macros
Sydney supports a macro system that allows for code generation and transformation. Macros are defined using the `macro` keyword and can take arguments.

```
const ifelse = macro(condition, consequence, alternative) {
    quote(if (unquote(condition)) {
        unquote(consequence);
    } else {
        unquote(alternative);
    });
};

ifelse(10 > 5, print("true"), print("false"));
```

The `quote` and `unquote` functions are used within macros to manipulate AST nodes. `quote` returns the AST of its argument, and `unquote` evaluates an expression and inserts the resulting AST into a quoted block.

## Compilation

Sydney has two compilation targets:

### VM (bytecode)
```
./sydney run file.sy
```
Compiles to bytecode and executes on a stack-based virtual machine.

### Native (LLVM IR)
```
./sydney compile file.sy    # emits file.ll
llc -filetype=obj file.ll -o file.o
clang file.o -Lsydney_rt/target/release -lsydney_rt -o file
./file
```
Compiles to LLVM IR, then assembles and links against a Rust runtime library that provides garbage collection, string operations, and print functions.

### Building
```bash
go build -o sydney                              # build the compiler
cd sydney_rt && cargo build --release            # build the runtime
```

### Testing
```bash
go test ./...
```
Emitter end-to-end tests require `llc` and `clang` (LLVM toolchain) to be available on the path.
