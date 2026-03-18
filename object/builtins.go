package object

import (
	"bytes"
	gotls "crypto/tls"
	"fmt"
	"io"
	gonet "net"
	"os"
	"strings"
	"sydney/types"
	"sync"
)

// Slab storage for TCP connections and listeners.
// Index into slice = handle. nil slot = closed/reusable.
var tcpStreams []gonet.Conn
var tcpListeners []gonet.Listener
var tcpMu sync.Mutex

func slabInsertConn(c gonet.Conn) int64 {
	tcpMu.Lock()
	defer tcpMu.Unlock()
	for i, s := range tcpStreams {
		if s == nil {
			tcpStreams[i] = c
			return int64(i)
		}
	}
	tcpStreams = append(tcpStreams, c)
	return int64(len(tcpStreams) - 1)
}

func slabInsertListener(l gonet.Listener) int64 {
	tcpMu.Lock()
	defer tcpMu.Unlock()
	for i, s := range tcpListeners {
		if s == nil {
			tcpListeners[i] = l
			return int64(i)
		}
	}
	tcpListeners = append(tcpListeners, l)
	return int64(len(tcpListeners) - 1)
}

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
	{
		"tcp_conn",
		&BuiltIn{
			AsyncFn: func(args []Object, done func(Object)) {
				host := args[0].(*String).Value
				port := args[1].(*Integer).Value
				addr := fmt.Sprintf("%s:%d", host, port)
				conn, err := gonet.Dial("tcp", addr)
				if err != nil {
					done(&Result{IsOk: false, Error: &String{Value: err.Error()}})
					return
				}
				done(&Result{IsOk: true, Value: &Integer{Value: slabInsertConn(conn)}})
			},
			T: types.FunctionType{Params: []types.Type{types.String, types.Int}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"tcp_listen",
		&BuiltIn{
			AsyncFn: func(args []Object, done func(Object)) {
				host := args[0].(*String).Value
				port := args[1].(*Integer).Value
				addr := fmt.Sprintf("%s:%d", host, port)
				ln, err := gonet.Listen("tcp", addr)
				if err != nil {
					done(&Result{IsOk: false, Error: &String{Value: err.Error()}})
					return
				}
				done(&Result{IsOk: true, Value: &Integer{Value: slabInsertListener(ln)}})
			},
			T: types.FunctionType{Params: []types.Type{types.String, types.Int}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"tcp_accept",
		&BuiltIn{
			AsyncFn: func(args []Object, done func(Object)) {
				idx := args[0].(*Integer).Value
				tcpMu.Lock()
				if idx < 0 || int(idx) >= len(tcpListeners) || tcpListeners[idx] == nil {
					tcpMu.Unlock()
					done(&Result{IsOk: false, Error: &String{Value: "tcp_accept: invalid listener handle"}})
					return
				}
				listener := tcpListeners[idx]
				tcpMu.Unlock()
				conn, err := listener.Accept()
				if err != nil {
					done(&Result{IsOk: false, Error: &String{Value: err.Error()}})
					return
				}
				done(&Result{IsOk: true, Value: &Integer{Value: slabInsertConn(conn)}})
			},
			T: types.FunctionType{Params: []types.Type{types.Int}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"tcp_read",
		&BuiltIn{
			AsyncFn: func(args []Object, done func(Object)) {
				idx := args[0].(*Integer).Value
				maxLen := args[1].(*Integer).Value
				tcpMu.Lock()
				if idx < 0 || int(idx) >= len(tcpStreams) || tcpStreams[idx] == nil {
					tcpMu.Unlock()
					done(&Result{IsOk: false, Error: &String{Value: "tcp_read: invalid stream handle"}})
					return
				}
				buf := make([]byte, maxLen)
				stream := tcpStreams[idx]
				tcpMu.Unlock()
				n, err := stream.Read(buf)
				if err != nil {
					done(&Result{IsOk: false, Error: &String{Value: err.Error()}})
					return
				}
				done(&Result{IsOk: true, Value: &String{Value: string(buf[:n])}})
			},
			T: types.FunctionType{Params: []types.Type{types.Int, types.Int}, Return: types.ResultType{T: types.String}},
		},
	},
	{
		"tcp_write",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				idx := args[0].(*Integer).Value
				data := args[1].(*String).Value
				tcpMu.Lock()
				if idx < 0 || int(idx) >= len(tcpStreams) || tcpStreams[idx] == nil {
					tcpMu.Unlock()
					return &Result{IsOk: false, Error: &String{Value: "tcp_write: invalid stream handle"}}
				}
				stream := tcpStreams[idx]
				tcpMu.Unlock()
				n, err := stream.Write([]byte(data))
				if err != nil {
					return &Result{IsOk: false, Error: &String{Value: err.Error()}}
				}
				return &Result{IsOk: true, Value: &Integer{Value: int64(n)}}
			},
			T: types.FunctionType{Params: []types.Type{types.Int, types.String, types.Int}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"tcp_close_stream",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				idx := args[0].(*Integer).Value
				tcpMu.Lock()
				if idx < 0 || int(idx) >= len(tcpStreams) || tcpStreams[idx] == nil {
					tcpMu.Unlock()
					return &Result{IsOk: false, Error: &String{Value: "tcp_close_stream: invalid handle"}}
				}
				stream := tcpStreams[idx]
				tcpMu.Unlock()
				err := stream.Close()
				tcpMu.Lock()
				tcpStreams[idx] = nil
				tcpMu.Unlock()
				if err != nil {
					return &Result{IsOk: false, Error: &String{Value: err.Error()}}
				}
				return &Result{IsOk: true, Value: &Integer{Value: 0}}
			},
			T: types.FunctionType{Params: []types.Type{types.Int}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"tcp_close_listener",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				idx := args[0].(*Integer).Value
				tcpMu.Lock()
				if idx < 0 || int(idx) >= len(tcpListeners) || tcpListeners[idx] == nil {
					tcpMu.Unlock()
					return &Result{IsOk: false, Error: &String{Value: "tcp_close_listener: invalid handle"}}
				}
				listener := tcpListeners[idx]
				tcpMu.Unlock()
				err := listener.Close()
				tcpMu.Lock()
				tcpListeners[idx] = nil
				tcpMu.Unlock()
				if err != nil {
					return &Result{IsOk: false, Error: &String{Value: err.Error()}}
				}
				return &Result{IsOk: true, Value: &Integer{Value: 0}}
			},
			T: types.FunctionType{Params: []types.Type{types.Int}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"tls_conn",
		&BuiltIn{
			AsyncFn: func(args []Object, done func(Object)) {
				host := args[0].(*String).Value
				port := args[1].(*Integer).Value
				addr := fmt.Sprintf("%s:%d", host, port)
				conn, err := gotls.Dial("tcp", addr, nil)
				if err != nil {
					done(&Result{IsOk: false, Error: &String{Value: err.Error()}})
					return
				}
				done(&Result{IsOk: true, Value: &Integer{Value: slabInsertConn(conn)}})
			},
			T: types.FunctionType{Params: []types.Type{types.String, types.Int}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"tls_read",
		&BuiltIn{
			AsyncFn: func(args []Object, done func(Object)) {
				idx := args[0].(*Integer).Value
				maxLen := args[1].(*Integer).Value
				tcpMu.Lock()
				if idx < 0 || int(idx) >= len(tcpStreams) || tcpStreams[idx] == nil {
					tcpMu.Unlock()
					done(&Result{IsOk: false, Error: &String{Value: "tls_read: invalid stream handle"}})
					return
				}
				buf := make([]byte, maxLen)
				stream := tcpStreams[idx]
				tcpMu.Unlock()
				n, err := stream.Read(buf)
				if err != nil {
					done(&Result{IsOk: false, Error: &String{Value: err.Error()}})
					return
				}
				done(&Result{IsOk: true, Value: &String{Value: string(buf[:n])}})
			},
			T: types.FunctionType{Params: []types.Type{types.Int, types.Int}, Return: types.ResultType{T: types.String}},
		},
	},
	{
		"tls_write",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				idx := args[0].(*Integer).Value
				data := args[1].(*String).Value
				tcpMu.Lock()
				if idx < 0 || int(idx) >= len(tcpStreams) || tcpStreams[idx] == nil {
					tcpMu.Unlock()
					return &Result{IsOk: false, Error: &String{Value: "tls_write: invalid stream handle"}}
				}
				stream := tcpStreams[idx]
				tcpMu.Unlock()
				n, err := stream.Write([]byte(data))
				if err != nil {
					return &Result{IsOk: false, Error: &String{Value: err.Error()}}
				}
				return &Result{IsOk: true, Value: &Integer{Value: int64(n)}}
			},
			T: types.FunctionType{Params: []types.Type{types.Int, types.String, types.Int}, Return: types.ResultType{T: types.Int}},
		},
	},
	{
		"tls_close_stream",
		&BuiltIn{
			Fn: func(args ...Object) Object {
				idx := args[0].(*Integer).Value
				tcpMu.Lock()
				if idx < 0 || int(idx) >= len(tcpStreams) || tcpStreams[idx] == nil {
					tcpMu.Unlock()
					return &Result{IsOk: false, Error: &String{Value: "tls_close_stream: invalid handle"}}
				}
				stream := tcpStreams[idx]
				tcpMu.Unlock()
				err := stream.Close()
				tcpMu.Lock()
				tcpStreams[idx] = nil
				tcpMu.Unlock()
				if err != nil {
					return &Result{IsOk: false, Error: &String{Value: err.Error()}}
				}
				return &Result{IsOk: true, Value: &Integer{Value: 0}}
			},
			T: types.FunctionType{Params: []types.Type{types.Int}, Return: types.ResultType{T: types.Int}},
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
