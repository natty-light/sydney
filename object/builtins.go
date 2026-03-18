package object

import (
	"bytes"
	"fmt"
	"io"
	"os"
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
	{
		"ok",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				return &Result{Value: args[0], Error: nil, IsOk: true}
			},
			T: types.FunctionType{Params: []types.Type{types.Infer}, Return: types.ResultType{T: types.Infer}},
		},
	},
	{
		"err",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				return &Result{Value: nil, Error: args[0].(*String), IsOk: false}
			},
			T: types.FunctionType{Params: []types.Type{types.Infer}, Return: types.ResultType{T: types.Infer}},
		},
	},
	{
		"fopen",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				path := args[0].(*String).Value
				f, err := os.Open(path)
				if err != nil {
					return &Result{Value: nil, Error: &String{Value: err.Error()}, IsOk: false}
				}
				fd := f.Fd()
				return &Result{Value: &Integer{Value: int64(fd)}, Error: nil, IsOk: true}
			},
			T: types.FunctionType{Params: []types.Type{types.String}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"fread",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				fd := args[0].(*Integer).Value
				f := os.NewFile(uintptr(fd), "")
				data, err := io.ReadAll(f)
				if err != nil {
					return &Result{Value: nil, Error: &String{Value: err.Error()}, IsOk: false}
				}

				return &Result{Value: &String{Value: string(data)}, Error: nil, IsOk: true}
			},
			T: types.FunctionType{Params: []types.Type{types.Int}, Return: types.ResultType{T: types.String}},
		},
	},
	{
		"fwrite",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				fd := args[0].(*Integer).Value
				data := []byte(args[1].(*String).Value)
				f := os.NewFile(uintptr(fd), "")
				_, err := f.Write(data)
				if err != nil {
					return &Result{Value: nil, Error: &String{Value: err.Error()}, IsOk: false}
				}

				return &Result{Value: &Integer{Value: fd}, Error: nil, IsOk: true}
			},
			T: types.FunctionType{Params: []types.Type{types.Int, types.String}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"fclose",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				fd := args[0].(*Integer).Value
				f := os.NewFile(uintptr(fd), "")
				err := f.Close()
				if err != nil {
					return &Result{Value: nil, Error: &String{Value: err.Error()}, IsOk: false}
				}
				return &Result{Value: &Integer{Value: fd}, Error: nil, IsOk: true}
			},
			T: types.FunctionType{Params: []types.Type{types.Int}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"int",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				b := args[0].(*Byte).Value
				return &Integer{Value: int64(b)}
			},
			T: types.FunctionType{Params: []types.Type{types.Byte}, Return: types.Int},
		},
	},
	{
		"byte",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				i := args[0].(*Integer).Value
				return &Byte{Value: byte(i)}
			},
			T: types.FunctionType{Params: []types.Type{types.Int}, Return: types.Byte},
		},
	},
	{
		"char",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				b := args[0].(*Byte).Value
				return &String{Value: string(b)}
			},
			T: types.FunctionType{Params: []types.Type{types.Byte}, Return: types.String},
		},
	},
	{
		"float",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				switch arg := args[0].(type) {
				case *Integer:
					return &Float{Value: float64(arg.Value)}
				case *Float:
					return arg
				}

				return newError("Argument to `float` not supported")
			},
			T: types.FunctionType{Params: []types.Type{types.Byte}, Return: types.String},
		},
	},
	{
		"panic",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				if len(args) != 1 {
					return newError("`panic` expects one argument")
				}
				msg := args[0].(*String).Value
				return &Error{Message: "panic: " + msg}
			},
			T: types.FunctionType{Params: []types.Type{types.String}, Return: types.Unit},
		},
	},
	{
		"some",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				return &Option{IsSome: true, Value: args[0]}
			},
			T: types.FunctionType{Params: []types.Type{types.Infer}, Return: types.OptionType{T: types.Infer}},
		},
	},
	{
		"none",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				return &Option{IsSome: false, Value: nil}
			},
			T: types.FunctionType{Params: []types.Type{}, Return: types.OptionType{T: types.Infer}},
		},
	},
}

func GetBuiltInByName(name string) *BuiltIn {
	for _, bi := range Builtins {
		if bi.Name == name {
			return bi.BuiltIn
		}
	}

	return nil
}
