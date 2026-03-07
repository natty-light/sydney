package irgen

import (
	"fmt"
	"sydney/ast"
)

var runtimeBuiltins = map[string]string{
	"io__open":  "sydney_file_open",
	"io__read":  "sydney_file_read",
	"io__write": "sydney_file_write",
	"io__close": "sydney_file_close",
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
		case IrBool:
			e.emit(fmt.Sprintf("call void @sydney_print_bool(i8 %s)", arg))
		}
	}
	e.emit("call void @sydney_print_newline()")
	return "", IrUnit
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

	// alloca and store
	rt := GetResultTaggedUnion(typ)
	resultAddr := e.tmp()
	e.emitAlloca(resultAddr, rt)

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
