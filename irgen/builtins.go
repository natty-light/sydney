package irgen

import (
	"fmt"
	"sydney/ast"
	"sydney/types"
)

var runtimeBuiltins = map[string]string{
	"io__fopen":               "sydney_file_open",
	"io__fread":               "sydney_file_read",
	"io__fwrite":              "sydney_file_write",
	"io__fclose":              "sydney_file_close",
	"conv__atof":              "sydney_atof",
	"net__tcp_conn":           "sydney_tcp_connect",
	"net__tcp_listen":         "sydney_tcp_listen",
	"net__tcp_accept":         "sydney_tcp_accept",
	"net__tcp_read":           "sydney_tcp_read",
	"net__tcp_write":          "sydney_tcp_write",
	"net__tcp_close_stream":   "sydney_tcp_close_stream",
	"net__tcp_close_listener": "sydney_tcp_close_listener",
}

const fNaN = "0x7FF8000000000000"

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
	e.emit("call void @sydney_print_newline()")
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

func (e *Emitter) emitFileOpen(expr *ast.CallExpr) (string, IrType) {
	path, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	line := fmt.Sprintf("%s = call i64 @sydney_file_open(ptr %s)", result, path)
	e.emit(line)
	return e.wrapIntoResult(result, IrInt)
}

func (e *Emitter) emitFileRead(expr *ast.CallExpr) (string, IrType) {
	fd, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	line := fmt.Sprintf("%s = call ptr @sydney_file_read(i64 %s)", result, fd)
	e.emit(line)
	return e.wrapIntoResult(result, IrPtr)
}

func (e *Emitter) emitFileWrite(expr *ast.CallExpr) (string, IrType) {
	fd, _ := e.emitExpr(expr.Arguments[0])
	data, _ := e.emitExpr(expr.Arguments[1])
	result := e.tmp()
	line := fmt.Sprintf("%s = call i64 @sydney_file_write(i64 %s, ptr %s)", result, fd, data)
	e.emit(line)
	return e.wrapIntoResult(result, IrInt)
}

func (e *Emitter) emitFileClose(expr *ast.CallExpr) (string, IrType) {
	fd, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	line := fmt.Sprintf("%s = call i64 @sydney_file_close(i64 %s)", result, fd)
	e.emit(line)
	return e.wrapIntoResult(result, IrInt)
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

func (e *Emitter) emitStrToFloatCall(expr *ast.CallExpr) (string, IrType) {
	m, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	line := fmt.Sprintf("%s = call double @sydney_atof(ptr %s)", result, m)
	e.emit(line)
	return e.wrapIntoResult(result, IrFloat)
}

func (e *Emitter) emitTcpConnectCall(expr *ast.CallExpr) (string, IrType) {
	host, _ := e.emitExpr(expr.Arguments[0])
	port, _ := e.emitExpr(expr.Arguments[1])
	handle := e.tmp()
	line := fmt.Sprintf("%s = call i64 @sydney_tcp_connect(ptr %s, i64 %s)", handle, host, port)
	e.emit(line)

	return e.wrapIntoResult(handle, IrInt)
}

func (e *Emitter) emitTcpListenCall(expr *ast.CallExpr) (string, IrType) {
	host, _ := e.emitExpr(expr.Arguments[0])
	port, _ := e.emitExpr(expr.Arguments[1])
	handle := e.tmp()
	line := fmt.Sprintf("%s = call i64 @sydney_tcp_listen(ptr %s, i64 %s)", handle, host, port)
	e.emit(line)

	return e.wrapIntoResult(handle, IrInt)
}

func (e *Emitter) emitTcpAcceptCall(expr *ast.CallExpr) (string, IrType) {
	handler, _ := e.emitExpr(expr.Arguments[0])
	handle := e.tmp()
	line := fmt.Sprintf("%s = call i64 @sydney_tcp_accept(i64 %s)", handle, handler)
	e.emit(line)

	return e.wrapIntoResult(handle, IrInt)
}

func (e *Emitter) emitTcpReadCall(expr *ast.CallExpr) (string, IrType) {
	handler, _ := e.emitExpr(expr.Arguments[0])
	maxLen, _ := e.emitExpr(expr.Arguments[1])
	data := e.tmp()
	line := fmt.Sprintf("%s = call ptr @sydney_tcp_read(i64 %s, i64 %s)", data, handler, maxLen)
	e.emit(line)

	return e.wrapIntoResult(data, IrPtr)
}

func (e *Emitter) emitTcpWriteCall(expr *ast.CallExpr) (string, IrType) {
	handler, _ := e.emitExpr(expr.Arguments[0])
	data, _ := e.emitExpr(expr.Arguments[1])
	l, _ := e.emitExpr(expr.Arguments[2])
	lwritten := e.tmp()
	line := fmt.Sprintf("%s = call i64 @sydney_tcp_write(i64 %s, ptr %s, i64 %s)", lwritten, handler, data, l)
	e.emit(line)

	return e.wrapIntoResult(lwritten, IrInt)
}

func (e *Emitter) emitTcpCloseStreamCall(expr *ast.CallExpr) (string, IrType) {
	stream, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	line := fmt.Sprintf("%s = call i64 @sydney_tcp_close_stream(i64 %s)", result, stream)
	e.emit(line)

	return e.wrapIntoResult(result, IrInt)
}

func (e *Emitter) emitTcpCloseListenerCall(expr *ast.CallExpr) (string, IrType) {
	handler, _ := e.emitExpr(expr.Arguments[0])
	result := e.tmp()
	line := fmt.Sprintf("%s = call i64 @sydney_tcp_close_listener(i64 %s)", result, handler)
	e.emit(line)

	return e.wrapIntoResult(result, IrInt)
}
