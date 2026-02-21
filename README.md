# Sydney - A language that's the way I want it

## Values, literals, and types

### Primitive types
Sydney supports the following primitive types: `int`, `float`, `string`, `bool`, and `null`.

```
mut int i = 10;
mut float f = 10.5;
mut string s = "hello";
mut bool b = true;
mut null n = null;
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

## Control flow

The flow of the program can be controlled using `if-else` expressions and `for` loops. Note that expression is correct here, `if-else` expressions will result in a value, allowing for the following:
```
const int bit = if (true) { 1 } else { 0 };
```

`for` loops can be used to iterate over a range of values or elements:
```
for (mut i = 0; i < 10; i = i + 1) {
    print(i);
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
- `first(array)`: Returns the first element of an array.
- `last(array)`: Returns the last element of an array.
- `rest(array)`: Returns a new array containing all but the first element.
- `append(array, element)`: Returns a new array with the element appended.
- `slice(array, start, end)`: Returns a new array containing the elements from `start` to `end`.
- `keys(map)`: Returns an array of keys from the map.
- `values(map)`: Returns an array of values from the map.

## Macros
Sydney supports a powerful macro system that allows for code generation and transformation. Macros are defined using the `macro` keyword and can take arguments.

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