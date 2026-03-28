package irgen

import (
	"fmt"
	"strings"

	"sydney/ast"
	"sydney/types"
)

type RuntimeBuiltin struct {
	RuntimeName string
	ParamTypes  []IrType
	ReturnType  IrType
	WrapResult  bool
}

var runtimeBuiltins = map[string]RuntimeBuiltin{
	"io__fopen":               {RuntimeName: "sydney_file_open", ParamTypes: []IrType{IrPtr}, ReturnType: IrInt, WrapResult: true},
	"io__fread":               {RuntimeName: "sydney_file_read", ParamTypes: []IrType{IrInt}, ReturnType: IrPtr, WrapResult: true},
	"io__fwrite":              {RuntimeName: "sydney_file_write", ParamTypes: []IrType{IrInt, IrPtr}, ReturnType: IrInt, WrapResult: true},
	"io__fclose":              {RuntimeName: "sydney_file_close", ParamTypes: []IrType{IrInt}, ReturnType: IrInt, WrapResult: true},
	"io__fcreate":             {RuntimeName: "sydney_file_create", ParamTypes: []IrType{IrPtr}, ReturnType: IrInt, WrapResult: true},
	"io__freadn":              {RuntimeName: "sydney_file_readn", ParamTypes: []IrType{IrInt, IrInt}, ReturnType: IrPtr, WrapResult: true},
	"conv__atof":              {RuntimeName: "sydney_atof", ParamTypes: []IrType{IrPtr}, ReturnType: IrFloat, WrapResult: true},
	"conv__ftoa":              {RuntimeName: "sydney_ftoa", ParamTypes: []IrType{IrFloat}, ReturnType: IrPtr, WrapResult: false},
	"net__tcp_conn":           {RuntimeName: "sydney_tcp_connect", ParamTypes: []IrType{IrPtr, IrInt}, ReturnType: IrInt, WrapResult: true},
	"net__tcp_listen":         {RuntimeName: "sydney_tcp_listen", ParamTypes: []IrType{IrPtr, IrInt}, ReturnType: IrInt, WrapResult: true},
	"net__tcp_accept":         {RuntimeName: "sydney_tcp_accept", ParamTypes: []IrType{IrInt}, ReturnType: IrInt, WrapResult: true},
	"net__tcp_read":           {RuntimeName: "sydney_tcp_read", ParamTypes: []IrType{IrInt, IrInt}, ReturnType: IrPtr, WrapResult: true},
	"net__tcp_write":          {RuntimeName: "sydney_tcp_write", ParamTypes: []IrType{IrInt, IrPtr, IrInt}, ReturnType: IrInt, WrapResult: true},
	"net__tcp_close_stream":   {RuntimeName: "sydney_tcp_close_stream", ParamTypes: []IrType{IrInt}, ReturnType: IrInt, WrapResult: true},
	"net__tcp_close_listener": {RuntimeName: "sydney_tcp_close_listener", ParamTypes: []IrType{IrInt}, ReturnType: IrInt, WrapResult: true},
	"net__tls_conn":           {RuntimeName: "sydney_tls_connect", ParamTypes: []IrType{IrPtr, IrInt}, ReturnType: IrInt, WrapResult: true},
	"net__tls_read":           {RuntimeName: "sydney_tls_read", ParamTypes: []IrType{IrInt, IrInt}, ReturnType: IrPtr, WrapResult: true},
	"net__tls_write":          {RuntimeName: "sydney_tls_write", ParamTypes: []IrType{IrInt, IrPtr, IrInt}, ReturnType: IrInt, WrapResult: true},
	"net__tls_close_stream":   {RuntimeName: "sydney_tls_close", ParamTypes: []IrType{IrInt}, ReturnType: IrInt, WrapResult: true},
	"term__term_set_raw":      {RuntimeName: "sydney_term_enable_raw", ParamTypes: []IrType{IrInt}, ReturnType: IrInt, WrapResult: true},
	"term__term_reset":        {RuntimeName: "sydney_restore_state", ParamTypes: []IrType{IrInt}, ReturnType: IrInt, WrapResult: true},
}

func (e *Emitter) emitRuntimeBuiltinCall(builtin RuntimeBuiltin, expr *ast.CallExpr) (string, IrType) {
	args := make([]string, len(expr.Arguments))
	for i, arg := range expr.Arguments {
		val, _ := e.emitExpr(arg)
		args[i] = fmt.Sprintf("%s %s", builtin.ParamTypes[i], val)
	}
	argsStr := strings.Join(args, ", ")

	result := e.tmp()
	line := fmt.Sprintf("%s = call %s @%s(%s)", result, builtin.ReturnType, builtin.RuntimeName, argsStr)
	e.emit(line)

	if builtin.WrapResult {
		return e.wrapIntoResult(result, builtin.ReturnType)
	}
	return result, builtin.ReturnType
}

func (e *Emitter) emitPrintCall(expr *ast.CallExpr) (string, IrType) {
	for _, a := range expr.Arguments {
		arg, argType := e.emitExpr(a)
		switch argType {
		case IrInt:
			e.emit(fmt.Sprintf("call void @sydney_print_int(i64 %s)", arg))
		case IrFloat:
			e.emit(fmt.Sprintf("call void @sydney_print_float(double %s)", arg))
		case IrPtr:
			e.emit(fmt.Sprintf("call void @sydney_print_string(ptr %s)", arg))
		case IrInt8:
			e.emit(fmt.Sprintf("call void @sydney_print_byte(i8 %s)", arg))
		case IrBool:
			zextd := e.tmp()
			line := fmt.Sprintf("%s = zext i1 %s to i8", zextd, arg)
			e.emit(line)
			e.emit(fmt.Sprintf("call void @sydney_print_bool(i8 %s)", zextd))
		}
	}
	return "", IrUnit
}

func (e *Emitter) emitLenCall(expr *ast.CallExpr) (string, IrType) {
	arg, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	sydneyType := expr.Arguments[0].GetResolvedType()

	var line string
	switch sydneyType.(type) {
	case types.ArrayType:
		// length is first field of { i64, ptr }
		lenPtr := e.tmp()
		line = fmt.Sprintf("%s = getelementptr { i64, ptr }, ptr %s, i32 0, i32 0", lenPtr, arg)
		e.emit(line)
		line = fmt.Sprintf("%s = load i64, ptr %s", result, lenPtr)
	case types.MapType:
		// no op
	default:
		line = fmt.Sprintf("%s = call i64 @sydney_strlen(ptr %s)", result, arg)
	}
	e.emit(line)

	return result, IrInt
}

func (e *Emitter) wrapIntoResult(val string, typ IrType) (string, IrType) {
	cmp := e.tmp()
	var line string
	if typ == IrPtr {
		line = fmt.Sprintf("%s = icmp eq ptr %s, null", cmp, val)
	} else if typ == IrFloat {
		line = fmt.Sprintf("%s = fcmp uno double %s, 0.0", cmp, val)
	} else {
		line = fmt.Sprintf("%s = icmp eq i64 %s, -1", cmp, val)
	}
	e.emit(line)

	errLbl := e.label("result.err")
	okLbl := e.label("result.ok")
	endLbl := e.label("result.end")

	e.emitBranch(cmp, errLbl, okLbl)

	e.emitLabel(errLbl)
	errMsg := e.tmp()
	line = fmt.Sprintf("%s = call ptr @sydney_get_last_error()", errMsg)
	e.emit(line)
	e.emitJmp(endLbl)

	// --- ok branch ---
	e.emitLabel(okLbl)
	e.emitJmp(endLbl)

	// --- merge: build the tagged union ---
	e.emitLabel(endLbl)

	// phi for tag: false from err, true from ok
	tag := e.tmp()
	line = fmt.Sprintf("%s = phi i1 [ false, %%%s ], [ true, %%%s ]", tag, errLbl, okLbl)
	e.emit(line)

	// phi for error msg: errMsg from err, null from ok
	errPhi := e.tmp()
	line = fmt.Sprintf("%s = phi ptr [ %s, %%%s ], [ null, %%%s ]", errPhi, errMsg, errLbl, okLbl)
	e.emit(line)

	// heap-allocate so the result survives function returns
	rt := GetResultTaggedUnion(typ)
	resultAddr := e.tmp()
	line = fmt.Sprintf("%s = call ptr @sydney_gc_alloc(i64 24)", resultAddr)
	e.emit(line)

	tagGep := e.tmp()
	line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 0", tagGep, rt, resultAddr)
	e.emit(line)
	line = fmt.Sprintf("store i1 %s, ptr %s", tag, tagGep)
	e.emit(line)

	valGep := e.tmp()
	line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 1", valGep, rt, resultAddr)
	e.emit(line)
	if typ == IrPtr {
		line = fmt.Sprintf("store ptr %s, ptr %s", val, valGep)
	} else if typ == IrFloat {
		line = fmt.Sprintf("store double %s, ptr %s", val, valGep)
	} else {
		line = fmt.Sprintf("store i64 %s, ptr %s", val, valGep)
	}
	e.emit(line)

	errGep := e.tmp()
	line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i32 0, i32 2", errGep, rt, resultAddr)
	e.emit(line)
	line = fmt.Sprintf("store ptr %s, ptr %s", errPhi, errGep)
	e.emit(line)

	return resultAddr, IrPtr
}

func (e *Emitter) emitIntConvCall(expr *ast.CallExpr) (string, IrType) {
	result := e.tmp()
	arg, _ := e.emitExpr(expr.Arguments[0])
	line := fmt.Sprintf("%s = zext i8 %s to i64", result, arg)
	e.emit(line)
	return result, IrInt
}

func (e *Emitter) emitByteConvCall(expr *ast.CallExpr) (string, IrType) {
	result := e.tmp()
	arg, _ := e.emitExpr(expr.Arguments[0])
	line := fmt.Sprintf("%s = trunc i64 %s to i8", result, arg)
	e.emit(line)
	return result, IrInt8
}

func (e *Emitter) emitFloatConvCall(expr *ast.CallExpr) (string, IrType) {
	arg, argTyp := e.emitExpr(expr.Arguments[0])
	if argTyp == IrFloat {
		return arg, argTyp
	}
	result := e.tmp()
	line := fmt.Sprintf("%s = sitofp %s %s to double", result, argTyp, arg)
	e.emit(line)
	return result, IrFloat
}

func (e *Emitter) emitCharConvCall(expr *ast.CallExpr) (string, IrType) {
	result := e.tmp()
	arg, _ := e.emitExpr(expr.Arguments[0])
	line := fmt.Sprintf("%s = call ptr @sydney_byte_to_string(i8 %s)", result, arg)
	e.emit(line)
	return result, IrPtr
}

func (e *Emitter) emitAppendCall(expr *ast.CallExpr) (string, IrType) {
	arr, _ := e.emitExpr(expr.Arguments[0])
	val, valType := e.emitExpr(expr.Arguments[1])

	lenPtr := e.tmp()
	line := fmt.Sprintf("%s = getelementptr { i64, ptr }, ptr %s, i32 0, i32 0", lenPtr, arr)
	e.emit(line)
	l := e.tmp()
	line = fmt.Sprintf("%s = load i64, ptr %s", l, lenPtr)
	e.emit(line)

	newLen := e.tmp()
	line = fmt.Sprintf("%s = add i64 %s, 1", newLen, l)
	e.emit(line)

	dataPtr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr { i64, ptr }, ptr %s, i32 0, i32 1", dataPtr, arr)
	e.emit(line)
	data := e.tmp()
	line = fmt.Sprintf("%s = load ptr, ptr %s", data, dataPtr)
	e.emit(line)

	newBytes := e.tmp()
	line = fmt.Sprintf("%s = mul i64 %s, 8", newBytes, newLen)
	e.emit(line)
	newData := e.tmp()
	line = fmt.Sprintf("%s = call ptr @sydney_gc_alloc(i64 %s)", newData, newBytes)
	e.emit(line)

	// copy memory
	oldBytes := e.tmp()
	line = fmt.Sprintf("%s = mul i64 %s, 8", oldBytes, l)
	e.emit(line)
	line = fmt.Sprintf("call void @llvm.memcpy.p0.p0.i64(ptr %s, ptr %s, i64 %s, i1 false)", newData, data, oldBytes)
	e.emit(line)

	newElemPtr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr %s, ptr %s, i64 %s", newElemPtr, valType, newData, l)
	e.emit(line)
	line = fmt.Sprintf("store %s %s, ptr %s", valType, val, newElemPtr)
	e.emit(line)

	header := e.tmp()
	line = fmt.Sprintf("%s = call ptr @sydney_gc_alloc(i64 16)", header)
	e.emit(line)
	newLenPtr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr { i64, ptr }, ptr %s, i32 0, i32 0", newLenPtr, header)
	e.emit(line)
	line = fmt.Sprintf("store i64 %s, ptr %s", newLen, newLenPtr)
	e.emit(line)
	newDataPtr := e.tmp()
	line = fmt.Sprintf("%s = getelementptr { i64, ptr }, ptr %s, i32 0, i32 1", newDataPtr, header)
	e.emit(line)
	line = fmt.Sprintf("store ptr %s, ptr %s", newData, newDataPtr)
	e.emit(line)

	return header, IrPtr
}

func (e *Emitter) emitIntKeysCall(expr *ast.CallExpr) (string, IrType) {
	m, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	line := fmt.Sprintf("%s = call ptr @sydney_map_keys_int(ptr %s)", result, m)
	e.emit(line)

	return result, IrPtr
}

func (e *Emitter) emitIntValuesCall(expr *ast.CallExpr) (string, IrType) {
	m, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	line := fmt.Sprintf("%s = call ptr @sydney_map_values_int(ptr %s)", result, m)
	e.emit(line)

	return result, IrPtr
}

func (e *Emitter) emitStrKeysCall(expr *ast.CallExpr) (string, IrType) {
	m, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	line := fmt.Sprintf("%s = call ptr @sydney_map_keys_str(ptr %s)", result, m)
	e.emit(line)

	return result, IrPtr
}

func (e *Emitter) emitStrValuesCall(expr *ast.CallExpr) (string, IrType) {
	m, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	line := fmt.Sprintf("%s = call ptr @sydney_map_values_str(ptr %s)", result, m)
	e.emit(line)

	return result, IrPtr
}
