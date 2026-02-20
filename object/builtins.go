package object

import (
	"bytes"
	"fmt"
	"strings"
	"sydney/types"
)

var Builtins = []struct {
	Name    string
	BuiltIn *BuiltIn
}{
	{
		"len",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				if len(args) != 1 {
					return newError("`len` expects one argument")
				}

				switch arg := args[0].(type) {
				case *String:
					return &Integer{Value: int64(len(arg.Value))}
				case *Array:
					return &Integer{Value: int64(len(arg.Elements))}
				case *Hash:
					return &Integer{Value: int64(len(arg.Pairs))}
				default:
					return newError("argument to `len` of wrong type. got=%s", args[0].Type())
				}
			},
			T: types.FunctionType{Params: []types.Type{types.ArrayType{ElemType: types.Any}}, Return: types.Infer},
		},
	},
	{
		"print",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				var out bytes.Buffer

				elems := make([]string, 0)
				for _, a := range args {
					elems = append(elems, a.Inspect())
				}
				out.WriteString(strings.Join(elems, " "))

				fmt.Println(out.String())
				return nil
			},
			T: types.FunctionType{Params: []types.Type{types.Any}, Return: types.Unit, Variadic: true},
		},
	},
	{
		"first",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				if len(args) != 1 {
					return newError("`first` expects a single argument")
				}
				if args[0].Type() != ArrayObj {
					return newError("argument to `first` must be array type")
				}

				arr := args[0].(*Array)
				if len(arr.Elements) > 0 {
					return arr.Elements[0]
				}
				return nil
			},
			T: types.FunctionType{Params: []types.Type{types.ArrayType{ElemType: types.Any}}, Return: types.Infer},
		},
	},
	{
		"last",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				if len(args) != 1 {
					return newError("`last` expects a single argument.")
				}
				if args[0].Type() != ArrayObj {
					return newError("argument to `last` must be array type")
				}

				arr := args[0].(*Array)
				length := len(arr.Elements)
				if length > 0 {
					return arr.Elements[length-1]
				}
				return nil
			},
			T: types.FunctionType{Params: []types.Type{types.ArrayType{ElemType: types.Any}}, Return: types.Infer},
		},
	},
	{
		"rest",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				if len(args) != 1 {
					return newError("`rest` expects one argument")
				}
				if args[0].Type() != ArrayObj {
					return newError("argument to `rest` must be array type")
				}
				arr := args[0].(*Array)
				length := len(arr.Elements)
				if length > 0 {
					newElems := make([]Object, length-1)
					copy(newElems, arr.Elements[1:length])
					return &Array{Elements: newElems}
				}
				return nil
			},
			T: types.FunctionType{Params: []types.Type{types.ArrayType{ElemType: types.Any}}, Return: types.ArrayType{ElemType: types.Infer}},
		},
	},
	{
		"append",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				if len(args) != 2 {
					return newError("`append` expects two arguments")
				}

				if args[0].Type() != ArrayObj {
					return newError("first argument to `append` must be array type")
				}

				arr := args[0].(*Array)
				length := len(arr.Elements)

				newElems := make([]Object, length+1)
				copy(newElems, arr.Elements)
				newElems[length] = args[1]

				return &Array{Elements: newElems}
			},
			T: types.FunctionType{Params: []types.Type{types.ArrayType{ElemType: types.Any}}, Return: types.ArrayType{ElemType: types.Infer}},
		},
	},
	{
		"slice",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				if len(args) != 3 {
					return newError("`slice` expects three arguments")
				}

				if args[0].Type() != ArrayObj {
					return newError("first argument to `slice` must be array type")
				}
				if args[1].Type() != IntegerObj {
					return newError("`start` argument to `slice` must be int")
				}

				if args[2].Type() != IntegerObj {
					return newError("`end` argument to `slice` must be int")
				}

				arr := args[0].(*Array)
				start := args[1].(*Integer).Value
				end := args[2].(*Integer).Value
				arrLength := int64(len(arr.Elements) - 1)

				if start < 0 {
					start = 0
				}
				if end > arrLength {
					end = arrLength - 1
				}
				slicedLength := int64(end - start)

				newElems := make([]Object, slicedLength)
				copy(newElems, arr.Elements[start:end])

				return &Array{Elements: newElems}
			},
			T: types.FunctionType{Params: []types.Type{types.ArrayType{ElemType: types.Any}}, Return: types.ArrayType{ElemType: types.Infer}},
		},
	},
	{
		"keys",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				if len(args) != 1 {
					return newError("`keys` expects one argument")
				}

				hash, ok := args[0].(*Hash)
				if !ok {
					return newError("unknown argument type for `keys`: %T", args[0])
				}

				keys := make([]Object, 0)
				for key := range hash.Pairs {
					switch val := key.ObjectValue.(type) {
					case bool:
						keys = append(keys, &Boolean{Value: val})
					case string:
						keys = append(keys, &String{Value: val})
					case int64:
						keys = append(keys, &Integer{Value: val})
					}
				}

				return &Array{Elements: keys}
			},
			T: types.FunctionType{Params: []types.Type{types.MapType{KeyType: types.Any, ValueType: types.Any}}, Return: types.ArrayType{ElemType: types.Infer}},
		},
	},
	{
		"values",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				if len(args) != 1 {
					return newError("`values` expects one argument")
				}

				hash, ok := args[0].(*Hash)
				if !ok {
					return newError("unknown argument type for `values`: %T", args[0])
				}

				values := make([]Object, 0)
				for _, pair := range hash.Pairs {
					values = append(values, pair.Value)
				}

				return &Array{Elements: values}
			},
			T: types.FunctionType{Params: []types.Type{types.MapType{KeyType: types.Any, ValueType: types.Any}}, Return: types.ArrayType{ElemType: types.Infer}},
		},
	},
}

var BuiltInMap = map[string]bool{
	"len":    true,
	"print":  true,
	"first":  true,
	"last":   true,
	"append": true,
	"rest":   true,
	"slice":  true,
	"keys":   true,
	"values": true,
}

func GetBuiltInByName(name string) *BuiltIn {
	for _, bi := range Builtins {
		if bi.Name == name {
			return bi.BuiltIn
		}
	}

	return nil
}
