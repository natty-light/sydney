# Sydney - A language that's the way I want it

## Values, literals, and types

### Variables
A variable can either be mutable or constant. this is specified with the `mut` and `const` keywords.

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

There are two ways of creating functions. Anonymous functions are treated as values, and can be assigned to variables
```
const add = func(int a, int b) -> int { 
    a + b; 
}
```

Functions can also be declared in the traditional sense
```
func add(int a, int b) -> int {
    a + b;
}
```

Closures are supported
```
const f = func(int x) -> fn<() -> int> {
    return func() -> int {
        return x;
    };
}

const store5 = f(5);

store5(); // returns 5;
```

### Maps
Maps are dictionaries with strict typings. The keys of a map must all be of the same type, as with the values.
Values are accessed with square bracket index notation. Map values can be updated by key.
```
const map<string, int> m = { "hello": 0, "world: 1 };
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

## Control flow

The flow of the program can be controlled using `if-else` expressions and `for` loops. Note that expression is correct here, `if-else` expressions will result in a value, allowing for the following:
```
const int bit = if (true) { 1 }; else { 0; };
```