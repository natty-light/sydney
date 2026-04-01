package typechecker

import (
	"fmt"
	"maps"
	"runtime/debug"
	"slices"
	"sort"
	"strings"

	"sydney/ast"
	"sydney/errors"
	"sydney/loader"
	"sydney/object"
	"sydney/types"
)

type Checker struct {
	env                    *TypeEnv
	errors                 []errors.PositionError
	currentReturnType      types.Type
	currentMatchResultType types.Type
	definedStructs         map[string]types.StructType
	definedInterfaces      map[string]types.InterfaceType
	methodToInterfaces     map[string][]types.InterfaceType
	packages               map[string]*TypeEnv

	moduleTypes map[string]map[string]types.Type

	inLoop            bool
	inDiscardPosition bool
	isLastInBlock     bool

	genericFunctions map[string]*ast.FunctionDeclarationStmt // templates
	genericStructs   map[string]*ast.StructDefinitionStmt    // templates
	monomorphized    map[string]bool                         // "identity__int" → done
	structNodes      map[string]*ast.StructDefinitionStmt    // for writing back Interfaces

	program                 *ast.Program
	stmtIndex               int
	pendingInserts          map[int][]ast.Stmt
	currentModule           string
	moduleInterfacesIndexed bool
}

func New(globalEnv *TypeEnv) *Checker {
	env := globalEnv
	if env == nil {
		env = NewTypeEnv(nil)
	}
	for _, v := range object.Builtins {
		env.Set(v.Name, v.BuiltIn.T)
	}

	return &Checker{
		env:                    env,
		errors:                 make([]errors.PositionError, 0),
		currentReturnType:      nil,
		currentMatchResultType: nil,
		definedStructs:         make(map[string]types.StructType),
		definedInterfaces:      make(map[string]types.InterfaceType),
		methodToInterfaces:     make(map[string][]types.InterfaceType),
		packages:               make(map[string]*TypeEnv),
		moduleTypes:            map[string]map[string]types.Type{},
		inLoop:                 false,
		genericFunctions:       make(map[string]*ast.FunctionDeclarationStmt),
		genericStructs:         make(map[string]*ast.StructDefinitionStmt),
		monomorphized:          make(map[string]bool),
		structNodes:            make(map[string]*ast.StructDefinitionStmt),
		pendingInserts:         make(map[int][]ast.Stmt),
		program:                nil,
	}
}

func NewWithModuleTypes(globalEnv *TypeEnv, moduleTypes map[string]map[string]types.Type) *Checker {
	env := globalEnv
	if env == nil {
		env = NewTypeEnv(nil)
	}
	for _, v := range object.Builtins {
		env.Set(v.Name, v.BuiltIn.T)
	}
	c := New(globalEnv)
	c.moduleTypes = moduleTypes

	return c
}

func (c *Checker) pushScope() *TypeEnv {
	env := c.env
	c.env = NewTypeEnv(env)

	return env
}

func (c *Checker) popScope() {
	env := c.env.outer
	c.env = env
}

func (c *Checker) insertPending(program *ast.Program) {
	if len(c.pendingInserts) == 0 {
		return
	}
	indices := slices.Sorted(maps.Keys(c.pendingInserts))
	slices.Reverse(indices)
	for _, idx := range indices {
		program.Stmts = slices.Insert(program.Stmts, idx, c.pendingInserts[idx]...)
	}
	c.pendingInserts = make(map[int][]ast.Stmt)
}

func (c *Checker) Errors() []errors.PositionError {
	return c.errors
}

func (c *Checker) SetCurrentModule(name string) {
	c.currentModule = name
}

func (c *Checker) Env() *TypeEnv {
	return c.env
}

func (c *Checker) CheckAsPackage(node ast.Node, packages []*loader.Package) []errors.PositionError {
	if packages != nil {
		c.checkPackages(packages)
	}

	pkgEnv := NewTypeEnv(nil)
	for _, v := range object.Builtins {
		pkgEnv.Set(v.Name, v.BuiltIn.T)
	}
	for _, env := range c.packages {
		for name, typ := range env.store {
			if strings.Contains(name, ".") {
				pkgEnv.Set(name, typ)
			}
		}
	}

	pkgChecker := NewWithModuleTypes(pkgEnv, c.moduleTypes)
	pkgChecker.packages = c.packages
	pkgChecker.currentModule = c.currentModule

	if program, ok := node.(*ast.Program); ok {
		pkgChecker.program = program
	}
	pkgChecker.check(node)

	c.env = pkgChecker.env
	return pkgChecker.errors
}

func (c *Checker) Check(node ast.Node, packages []*loader.Package) []errors.PositionError {
	if packages != nil {
		c.checkPackages(packages)
	}
	if program, ok := node.(*ast.Program); ok {
		c.program = program
	}

	c.check(node)
	return c.errors
}

func (c *Checker) checkPackages(packages []*loader.Package) []errors.PositionError {
	registry := make(map[string]*TypeEnv)

	for _, pkg := range packages {
		pkgEnv := NewTypeEnv(nil)
		// Populate env with struct methods from already-checked dependencies
		// (e.g. "Socket.read") so cross-module method calls work.
		// Bare names are accessed via scope
		for _, env := range registry {
			for name, typ := range env.store {
				if strings.Contains(name, ".") {
					pkgEnv.Set(name, typ)
				}
			}
		}
		pkgChecker := NewWithModuleTypes(pkgEnv, c.moduleTypes)
		pkgChecker.packages = registry
		pkgChecker.currentModule = pkg.Name

		merged := &ast.Program{}
		for _, program := range pkg.Programs {
			merged.Stmts = append(merged.Stmts, program.Stmts...)
		}
		pkgChecker.Check(merged, nil)
		for k, v := range pkgChecker.genericFunctions {
			if len(v.TypeParams) == 0 {
				panic(fmt.Sprintf("invariant violation: genericFunction %q has no TypeParams after package check\n%s\n", v.Name, debug.Stack()))
			}
			c.genericFunctions[k] = v
		}
		for k, v := range pkgChecker.genericStructs {
			if len(v.Type.TypeParams) == 0 {
				panic(fmt.Sprintf("invariant violation: genericStruct %q has no TypeParams after package check\n%s\n", v.Name, debug.Stack()))
			}
			c.genericStructs[k] = v
		}
		for _, e := range pkgChecker.errors {
			if e.File == "" {
				e.File = pkg.Name
			}
			c.errors = append(c.errors, e)
		}

		exportEnv := NewTypeEnv(nil)
		// set non-functions
		functions := make([]*ast.FunctionDeclarationStmt, 0)
		for _, program := range pkg.Programs {
			for _, stmt := range program.Stmts {
				if pub, ok := stmt.(*ast.PubStatement); ok {
					if fn, isFn := pub.Stmt.(*ast.FunctionDeclarationStmt); isFn {
						functions = append(functions, fn)
					}
					name, typ := c.extractDeclNameAndType(pub.Stmt, pkg.Name)
					if containsTypeParamRef(typ) {
						panic(fmt.Sprintf("invariant violation: exported %T %s has TypeParamRefs\n%s\n", pub.Stmt, name, debug.Stack()))
					}
					exportEnv.Set(name, typ)
				}
			}
		}
		// set functions
		for _, fn := range functions {
			name := fn.Name.Value
			typ := fn.Type
			if len(fn.TypeParams) > 0 {
				if ft, ok := typ.(types.FunctionType); ok {
					subs := make(map[string]types.Type)
					for _, param := range fn.TypeParams {
						subs[param.Name] = types.Any
					}
					resolved := types.SubstituteTypeParams(ft, subs).(types.FunctionType)

					if len(resolved.Params) > 0 {
						if st, ok := toStruct(resolved.Params[0]); ok {
							if _, _, exported := exportEnv.Get(st.Name); exported {
								mn := st.Name + "." + name
								if containsTypeParamRef(st) {
									panic(fmt.Sprintf("invariant violation: exported struct %s has TypeParamRefs\n%s\n", name, debug.Stack()))
								}
								exportEnv.Set(mn, resolved)
							}
						}
					}
				}
				continue
			}
			if containsTypeParamRef(typ) {
				panic(fmt.Sprintf("invariant violation: exported struct %s has TypeParamRefs\n%s\n", name, debug.Stack()))
			}
			exportEnv.Set(name, typ)

			if ft, ok := typ.(types.FunctionType); ok && len(ft.Params) > 0 {
				if st, ok := ft.Params[0].(types.StructType); ok {
					if _, _, exported := exportEnv.Get(st.Name); exported {
						mn := st.Name + "." + name
						if containsTypeParamRef(ft) {
							panic(fmt.Sprintf("invariant violation: exported struct %s has TypeParamRefs\n%s\n", name, debug.Stack()))
						}
						exportEnv.Set(mn, typ)
					}
				}
			}
		}

		registry[pkg.Name] = exportEnv
	}
	c.packages = registry
	for _, env := range registry {
		for name, typ := range env.store {
			c.env.Set(name, typ)
		}
	}
	return c.errors
}

func (c *Checker) checkReturnStmt(node *ast.ReturnStmt) types.Type {
	if c.currentReturnType == nil {
		c.appendError("return statement outside of function body", node)
	}

	var valType types.Type = types.Unit
	if node.ReturnValue != nil {
		valType = c.typeOf(node.ReturnValue, c.currentReturnType)
	}

	if !c.typesMatch(valType, c.currentReturnType) {
		c.appendError(fmt.Sprintf("cannot return %s from function expecting %s", valType.Signature(), c.currentReturnType.Signature()), node)
	} else {
		c.boxIfNecessary(node.ReturnValue, valType, c.currentReturnType)
	}
	return types.Never
}

func (c *Checker) checkForStmt(node *ast.ForStmt) types.Type {
	oldEnv := c.env
	c.env = NewTypeEnv(oldEnv)
	if node.Init != nil {
		c.check(node.Init)
	}
	c.inLoop = true
	conditionType := c.typeOf(node.Condition, types.Bool)
	if conditionType != types.Bool {
		c.appendError(fmt.Sprintf("cannot use expression of type %s for loop condition", conditionType.Signature()), node)
	}
	if node.Post != nil {
		c.check(node.Post)
	}
	c.checkBlockStmtInCurrentScope(node.Body)
	c.env = oldEnv
	c.inLoop = false

	return types.Unit
}

func (c *Checker) checkForInStmt(node *ast.ForInStmt) types.Type {
	oldEnv := c.env
	c.env = NewTypeEnv(oldEnv)

	iterType := c.typeOf(node.Iterable, nil)
	m, mok := iterType.(types.MapType)
	a, aok := iterType.(types.ArrayType)
	if mok {
		node.Key.SetResolvedType(m.KeyType)
		node.Value.SetResolvedType(m.ValueType)
		c.env.Set(node.Key.Value, m.KeyType)
		c.env.Set(node.Value.Value, m.ValueType)
	} else if aok {
		if node.Key != nil {
			node.Key.SetResolvedType(types.Int)
			c.env.Set(node.Key.Value, types.Int)
		}
		node.Value.SetResolvedType(a.ElemType)
		c.env.Set(node.Value.Value, a.ElemType)
	} else {
		c.appendError(fmt.Sprintf("cannot iterate over value of type %s", iterType.Signature()), node)
	}

	c.inLoop = true
	c.checkBlockStmtInCurrentScope(node.Body)
	c.env = oldEnv
	c.inLoop = false

	return types.Unit
}

func (c *Checker) checkVarDeclStmt(node *ast.VarDeclarationStmt) types.Type {
	name := node.Name.Value
	if node.Type != nil {
		if resolved := c.resolveType(node.Type); resolved != nil {
			node.Type = resolved
		}
	}
	varType, outer, ok := c.env.Get(name)
	if !ok && node.Type != nil {
		varType = node.Type
	}

	valType := c.typeOf(node.Value, varType)
	if node.Type == nil {
		node.Type = valType
	}

	if valType == types.Unit {
		c.appendError(fmt.Sprintf("cannot assign unit to variable %s", name), node)
		return types.Unit
	}

	if node.Value == nil && !node.Constant {
		valType = varType
	}

	if valType == nil && varType == nil {
		c.appendError(fmt.Sprintf("cannot resolve type for variable %s", name), node)
		return types.Unit
	}

	if ok && !outer {
		if valType == nil || varType == nil {
			c.appendError(fmt.Sprintf("cannot resolve type for variable %s", name), node)
			return types.Unit
		}
		if !c.typesMatch(valType, varType) {
			c.appendError(fmt.Sprintf("type mismatch: cannot assign %s to variable %s of type %s", valType.Signature(), node.Name.String(), node.Type.Signature()), node)
		}
	} else {
		c.boxIfNecessary(node.Value, valType, varType)
		c.env.Set(name, valType)
		if node.Constant {
			c.env.SetConst(name)
		}
	}

	return types.Unit
}

func (c *Checker) checkVarAssignmentStmt(node *ast.VarAssignmentStmt) types.Type {
	name := node.Identifier.Value
	varType, _, ok := c.env.Get(name)
	if st, isScopeType := varType.(types.ScopeType); isScopeType {
		varType = c.resolveType(st)
	}
	valType := c.typeOf(node.Value, varType)
	isConst := c.env.IsConst(name)
	if !ok {
		c.appendError(fmt.Sprintf("cannot assign to undefined variable %s", name), node)
		return types.Unit
	}
	if isConst {
		c.appendError(fmt.Sprintf("cannot assign to constant variable %s", name), node)
	}

	if !c.typesMatch(varType, valType) {
		c.appendError(fmt.Sprintf("type mismatch: cannot assign %s to variable %s of type %s", valType.Signature(), name, varType.Signature()), node)
	}
	c.boxIfNecessary(node.Value, valType, varType)

	return types.Unit
}

func (c *Checker) checkBlockStmt(node *ast.BlockStmt) types.Type {
	oldEnv := c.env
	c.env = NewTypeEnv(oldEnv)
	lastType := c.checkBlockStmtInCurrentScope(node)
	c.env = oldEnv
	return lastType
}

func (c *Checker) checkBlockStmtInCurrentScope(node *ast.BlockStmt) types.Type {
	var lastType types.Type = types.Unit
	for i, stmt := range node.Stmts {
		prev := c.isLastInBlock
		c.isLastInBlock = i == len(node.Stmts)-1
		lastType = c.check(stmt)
		c.isLastInBlock = prev
	}
	node.Scope = c.env
	return lastType
}

func (c *Checker) checkSelectorAssignmentStmt(node *ast.SelectorAssignmentStmt) types.Type {
	str := c.typeOf(node.Left.Left, nil)

	structType, ok := toStruct(str)
	if !ok {
		c.appendError(fmt.Sprintf("cannot assign to field of non-struct value of type %s", str.Signature()), node)
		return types.Unit
	}

	field, ok := node.Left.Value.(*ast.Identifier)
	if !ok {
		return types.Unit
	}

	idx := slices.Index(structType.Fields, field.Value)
	if idx == -1 {
		c.appendError(fmt.Sprintf("struct %s of type %s has no field %s", structType.Name, structType.Name, field.Value), node)
		return types.Unit
	}

	valType := c.typeOf(node.Value, structType.Types[idx])

	if !c.typesMatch(valType, structType.Types[idx]) {
		c.appendError(fmt.Sprintf("type mismatch: cannot assign %s to struct %s field of type %s", valType.Signature(), structType.Name, structType.Types[idx].Signature()), node)
	}

	node.Left.ContainerType = structType
	node.Left.ResolvedType = structType.Types[idx]

	return types.Unit
}

func (c *Checker) check(n ast.Node) types.Type {
	switch node := n.(type) {
	case *ast.Program:
		for _, stmt := range node.Stmts {
			c.hoistBase(stmt)
		}

		for _, stmt := range node.Stmts {
			c.hoistFunctions(stmt)
		}

		c.discoverImplementations()

		for i, stmt := range node.Stmts {
			c.stmtIndex = i
			c.check(stmt)
		}
		c.insertPending(node)
		return types.Unit
	case *ast.PubStatement:
		return c.check(node.Stmt)
	case *ast.ExpressionStmt:
		prev := c.inDiscardPosition
		c.inDiscardPosition = c.currentReturnType == nil || c.currentReturnType == types.Unit || !c.isLastInBlock
		t := c.typeOf(node.Expr, nil)
		c.inDiscardPosition = prev
		return t
	case *ast.ReturnStmt:
		return c.checkReturnStmt(node)
	case *ast.ForStmt:
		return c.checkForStmt(node)
	case *ast.VarDeclarationStmt:
		return c.checkVarDeclStmt(node)
	case *ast.VarAssignmentStmt:
		return c.checkVarAssignmentStmt(node)
	case *ast.IndexAssignmentStmt:
		return c.checkIndexAssignment(node)
	case *ast.BlockStmt:
		return c.checkBlockStmt(node)
	case *ast.FunctionDeclarationStmt:
		return c.checkFunctionDeclaration(node)
	case *ast.SelectorAssignmentStmt:
		return c.checkSelectorAssignmentStmt(node)
	case *ast.ContinueStmt:
		if !c.inLoop {
			c.appendError(fmt.Sprintf("continue statement cannot be outside of loop"), node)
			return nil
		}
		return types.Unit
	case *ast.BreakStmt:
		if !c.inLoop {
			c.appendError(fmt.Sprintf("break statement cannot be outside of loop"), node)
			return nil
		}
		return types.Unit
	case *ast.ForInStmt:
		c.checkForInStmt(node)
	case *ast.SpawnStmt:
		callExpr, ok := node.CallExpr.(*ast.CallExpr)
		if !ok {
			c.appendError("must spawn function call", node)
			return types.Unit
		}
		c.typeOf(callExpr, nil)
		return types.Unit
	case *ast.SendStmt:
		chTypeRaw := c.typeOf(node.Chan, nil)
		if chTypeRaw == nil {
			c.appendError(fmt.Sprintf("cannot resolve type for ident %s", node.Chan), node)
			return types.Unit
		}
		chType, ok := chTypeRaw.(types.ChannelType)
		if !ok {
			c.appendError(fmt.Sprintf("%s is not a channel", node.Chan), node)
			return types.Unit
		}

		valType := c.typeOf(node.Value, nil)

		if !c.typesMatch(chType.ElemType, valType) {
			c.appendError(fmt.Sprintf("type mismatch: cannot send value of type %s to channel expecting %s", valType, chType.ElemType.Signature()), node)
		}
		return types.Unit
	}

	return types.Unit
}

func (c *Checker) hoistBase(n ast.Node) {
	switch node := n.(type) {
	case *ast.PubStatement:
		c.hoistBase(node.Stmt)
	case *ast.StructDefinitionStmt:
		if len(node.Type.TypeParams) > 0 {
			c.genericStructs[node.Name.Value] = node
			return
		}
		c.definedStructs[node.Name.Value] = node.Type
		c.structNodes[node.Name.Value] = node

	case *ast.InterfaceDefinitionStmt:
		node.Type.MethodIndices = make(map[string]int)
		for i, mn := range node.Type.Methods {
			node.Type.MethodIndices[mn] = i
		}
		c.env.Set(node.Name.Value, node.Type)
		c.assertInterfaceConsistent(node.Type)
		c.definedInterfaces[node.Name.Value] = node.Type
		c.indexInterface(node.Type)
		n = node
	case *ast.VarDeclarationStmt:
		name := node.Name.Value
		if node.Type != nil {
			node.Type = c.resolveType(node.Type)
			// Only check the current scope's store to allow shadowing
			if _, exists := c.env.store[name]; !exists {
				c.env.Set(name, node.Type)
				if node.Constant {
					c.env.SetConst(name)
				}
			} else {
				c.appendError(fmt.Sprintf("variable %s already declared", name), node)
			}
		}
	}
}

func (c *Checker) hoistFunctions(n ast.Node) {
	if pub, ok := n.(*ast.PubStatement); ok {
		if fdecl, ok := pub.Stmt.(*ast.FunctionDeclarationStmt); ok {
			c.hoistFunctions(fdecl)
		}
	}

	if node, ok := n.(*ast.FunctionDeclarationStmt); ok {
		fType := node.Type.(types.FunctionType)
		resolved := c.resolveFunctionType(fType)
		node.Type = resolved
		fType = resolved
		name := node.Name.Value

		if len(node.TypeParams) > 0 {
			c.genericFunctions[node.Name.Value] = node
			subs := make(map[string]types.Type)
			for _, tp := range node.TypeParams {
				subs[tp.Name] = types.Any
			}
			resolved := types.SubstituteTypeParams(fType, subs).(types.FunctionType)
			c.env.Set(name, resolved)

			if len(resolved.Params) > 0 {
				if st, ok := toStruct(resolved.Params[0]); ok {
					mangled := st.Name + "." + node.Name.Value
					node.MangledName = mangled
					c.env.Set(mangled, resolved)
				}
			}
			return
		}

		if len(fType.Params) > 0 {
			receiverType := fType.Params[0]
			if st, ok := toStruct(receiverType); ok {
				name = mangleMethod(st.Name, name)
				node.MangledName = name
			}

			if sn, ok := c.isInterfaceMethod(receiverType, name); ok {
				name = mangleMethod(sn, name)
				node.MangledName = name
			}
		}
		if name != node.Name.Value {
			c.env.Set(name, node.Type)
			return
		}

		_, fromOuter, exists := c.env.Get(node.Name.Value)
		if exists && !fromOuter && !node.IsExtern {
			c.appendError(fmt.Sprintf("function %s already declared", node.Name.Value), node)
			return
		}

		c.env.Set(name, node.Type)
	}
}

func (c *Checker) typeOf(e ast.Expr, expectedType types.Type) types.Type {
	if e == nil {
		return types.Null
	}

	switch expr := e.(type) {
	case *ast.IntegerLiteral:
		if expectedType == types.Float {
			expr.ResolvedType = types.Float
			return types.Float
		}
		return types.Int
	case *ast.StringLiteral:
		return types.String
	case *ast.FloatLiteral:
		return types.Float
	case *ast.BooleanLiteral:
		return types.Bool
	case *ast.NullLiteral:
		return types.Null
	case *ast.ByteLiteral:
		return types.Byte
	case *ast.FunctionLiteral:
		expr.Type = c.resolveFunctionType(expr.Type)
		oldInLoop := c.inLoop
		c.inLoop = true
		oldReturnType := c.currentReturnType
		c.currentReturnType = expr.Type.Return
		oldEnv := c.env
		c.env = NewTypeEnv(oldEnv)

		if expr.Name != "" {
			c.env.Set(expr.Name, expr.Type)
		}

		for i, param := range expr.Parameters {
			c.env.Set(param.Value, expr.Type.Params[i])
		}

		c.check(expr.Body)

		c.env = oldEnv
		c.currentReturnType = oldReturnType
		c.inLoop = oldInLoop

		return expr.Type
	case *ast.ArrayLiteral:
		var targetedElemType types.Type
		if expected, ok := toArray(expectedType); ok {
			targetedElemType = expected.ElemType
		}

		var elemType types.Type
		for i, element := range expr.Elements {
			eType := c.typeOf(element, targetedElemType)
			if elemType == nil {
				elemType = eType
			}

			if targetedElemType != nil && !c.typesMatch(eType, targetedElemType) {
				c.appendError(fmt.Sprintf("type mismatch: array element got %s, expected %s", eType.Signature(), targetedElemType.Signature()), expr)
				continue
			}

			c.boxIfNecessary(expr.Elements[i], eType, targetedElemType)
		}
		isEmpty := len(expr.Elements) == 0

		var resolved types.ArrayType
		if elemType == nil {
			if targetedElemType != nil {
				resolved = types.ArrayType{ElemType: targetedElemType, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
			} else {
				if isEmpty {
					c.appendError("empty array literal requires a type annotation", expr)
				}
				resolved = types.ArrayType{ElemType: types.Null, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
			}
		} else if targetedElemType != nil {
			// Prefer the targeted element type (e.g., any) over inferred element type
			resolved = types.ArrayType{ElemType: targetedElemType, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
		} else {
			resolved = types.ArrayType{ElemType: elemType, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
		}

		expr.ResolvedType = resolved

		return resolved
	case *ast.HashLiteral:
		expected, _ := toMap(expectedType)
		var keyType, valType types.Type

		keys := make([]ast.Expr, 0, len(expr.Pairs))
		for k := range expr.Pairs {
			keys = append(keys, k)
		}

		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})

		for _, k := range keys {
			v := expr.Pairs[k]
			kType := c.typeOf(k, expected.KeyType)
			vType := c.typeOf(v, expected.ValueType)

			if keyType == nil {
				keyType = kType
				valType = vType
				continue
			}

			if !c.typesMatch(kType, keyType) {
				c.appendError(fmt.Sprintf("type mismatch: map key got %s, expected %s", kType.Signature(), keyType.Signature()), expr)
			}
			if !c.typesMatch(valType, vType) {
				c.appendError(fmt.Sprintf("type mismatch: map value got %s, expected %s", vType.Signature(), valType.Signature()), expr)
			}
			c.boxIfNecessary(k, kType, keyType)
			c.boxIfNecessary(v, vType, valType)
		}

		expr.ResolvedType = types.MapType{ValueType: valType, KeyType: keyType}
		e = expr

		isEmpty := len(expr.Pairs) == 0
		if keyType == nil {
			return types.MapType{ValueType: types.Null, KeyType: types.Null, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
		}

		return types.MapType{KeyType: keyType, ValueType: valType, CollectionType: types.CollectionType{IsEmpty: isEmpty}}
	case *ast.Identifier:
		t, _, ok := c.env.Get(expr.Value)
		if !ok {
			c.appendError(fmt.Sprintf("undefined identifier: %s", expr.Value), expr)
			return types.Unit
		}
		expr.ResolvedType = t
		e = expr
		return t
	case *ast.InfixExpr:
		lt := c.typeOf(expr.Left, nil)
		rt := c.typeOf(expr.Right, nil)
		resolved := c.checkInfixExpr(expr.Operator, lt, rt, expr)
		expr.ResolvedType = resolved
		e = expr
		return resolved
	case *ast.PrefixExpr:
		t := c.typeOf(expr.Right, nil)
		resolved := c.checkPrefixExpr(expr.Operator, t, expr)
		expr.ResolvedType = resolved
		e = expr
		return resolved
	case *ast.IfExpr:
		t := c.typeOf(expr.Condition, types.Bool)
		if t != types.Bool {
			c.appendError(fmt.Sprintf("cannot use expression of type %s for if condition", t), expr)
			return nil
		}
		discard := c.inDiscardPosition
		c.inDiscardPosition = false
		cType := c.check(expr.Consequence)
		var aType types.Type
		if expr.Alternative != nil {
			aType = c.check(expr.Alternative)

			if !discard && !c.typesMatch(cType, aType) {
				c.appendError(fmt.Sprintf("consequence and alternative for if expression must result in same type"), expr)
				c.inDiscardPosition = discard
				return types.Unit
			}
		}
		c.inDiscardPosition = discard

		expr.ResolvedType = cType
		e = expr
		return cType
	case *ast.CallExpr:
		t := c.typeOfCallExpr(expr, expectedType)
		e = expr // this is fucky -- side effects :(
		return t
	case *ast.IndexExpr:
		resolved := c.checkIndexExpr(e, expr)
		expr.ResolvedType = resolved
		e = expr
		return resolved
	case *ast.SelectorExpr:
		t := c.typeOf(expr.Left, nil)
		structType, ok := t.(types.StructType)
		if !ok {
			c.appendError(fmt.Sprintf("cannot access field of non-struct value %s of type %s", expr.Left.TokenLiteral(), t.Signature()), expr)
		}
		val, ok := expr.Value.(*ast.Identifier)
		if !ok {
			c.appendError(fmt.Sprintf("idk what to put here I'll figure it out later"), expr)
			return types.Unit
		}
		i := slices.Index(structType.Fields, val.Value)
		if i == -1 {
			c.appendError(fmt.Sprintf("field %s of struct type %s not found", val.Value, expr.Left.TokenLiteral()), expr)
			return types.Unit
		}

		expr.ContainerType = structType
		expr.ResolvedType = structType.Types[i]
		e = expr
		return structType.Types[i]
	case *ast.StructLiteral:
		var structType types.StructType
		if len(expr.TypeArgs) > 0 {
			// generic
			var templateType types.StructType
			var found bool
			if expr.Module != "" {
				if mt, ok := c.moduleTypes[expr.Module]; ok {
					if t, ok := mt[expr.Name]; ok {
						templateType, found = t.(types.StructType)
					}
				}
			} else if tmpl, ok := c.genericStructs[expr.Name]; ok {
				templateType = tmpl.Type
				found = true
			}

			if !found {
				c.appendError(fmt.Sprintf("unknown generic struct %s", expr.Name), expr)
				return types.Unit
			}

			var ok bool
			structType, ok = c.monomorphizeStructLiteral(expr, templateType)
			if !ok {
				return types.Unit
			}
		} else if expr.Module != "" {
			var found bool
			if mt, ok := c.moduleTypes[expr.Module]; ok {
				if t, ok := mt[expr.Name]; ok {
					structType, found = t.(types.StructType)
				}
			}

			if !found {
				c.appendError(fmt.Sprintf("unknown type %s:%s", expr.Module, expr.Name), expr)
				return types.Unit
			}
		} else {
			var ok bool
			structType, ok = c.definedStructs[expr.Name]
			if !ok {
				c.appendError(fmt.Sprintf("unknown type %s", expr.Name), expr)
				return types.Unit
			}
		}

		providedFields := make(map[string]ast.Expr)
		for i, name := range expr.Fields {
			providedFields[name] = expr.Values[i]
		}

		for _, expected := range structType.Fields {
			if _, ok := providedFields[expected]; !ok {
				c.appendError(fmt.Sprintf("missing field %s in struct literal %s", expected, expr.Name), expr)
			}
		}

		for i, fieldName := range expr.Fields {
			idx := slices.Index(structType.Fields, fieldName)
			if idx == -1 {
				c.appendError(fmt.Sprintf("field %s of struct type %s not found", fieldName, expr.Name), expr)
				continue
			}

			expectedType = structType.Types[idx]
			actualType := c.typeOf(expr.Values[i], expectedType)
			if !c.typesMatch(actualType, expectedType) {
				c.appendError(fmt.Sprintf("type mismatch for field %s in struct %s: expected %s, got %s", fieldName, expr.Name, expectedType.Signature(), actualType.Signature()), expr)
			} else {
				c.boxIfNecessary(expr.Values[i], actualType, expectedType)
			}
		}
		expr.ResolvedType = structType
		e = expr

		return structType
	case *ast.ScopeAccessExpr:
		pkgEnv, ok := c.packages[expr.Module.Value]
		if !ok {
			c.appendError(fmt.Sprintf("module %s not found", expr.Module.Value), expr)
			return types.Unit
		}
		typ, _, found := pkgEnv.Get(expr.Member.Value)
		if !found {
			c.appendError(fmt.Sprintf("%s is not exported from module %s", expr.Member.Value, expr.Module.Value), expr)
			return types.Unit
		}

		return typ
	case *ast.MatchExpr:
		resolved := c.checkMatchExpr(expr)
		expr.ResolvedType = resolved
		e = expr
		return resolved
	case *ast.MatchTypeExpr:
		resolved := c.checkTypeMatchExpr(expr)
		e.SetResolvedType(resolved)
		return resolved
	case *ast.SliceExpr:
		leftType := c.typeOf(expr.Left, nil)
		if _, ok := toArray(leftType); !ok && !isString(leftType) {
			c.appendError(fmt.Sprintf("unsupported slice type %s", leftType.Signature()), expr)
			return types.Unit
		}
		var startType types.Type = nil
		if expr.Start != nil {
			startType = c.typeOf(expr.Start, types.Int)
			if startType != types.Int {
				c.appendError(fmt.Sprintf("unsupported start type %s", startType.Signature()), expr)
				return types.Unit
			}
		}
		var endType types.Type = nil
		if expr.End != nil {
			endType = c.typeOf(expr.End, types.Int)
			if endType != types.Int {
				c.appendError(fmt.Sprintf("unsupported end type %s", endType.Signature()), expr)
				return types.Unit
			}
		}
		if endType == nil && startType == nil {
			c.appendError("must provide start or end for slice expression", expr)
			return types.Unit
		}
		expr.SetResolvedType(leftType)

		return leftType
	case *ast.ChannelConstructorExpr:
		if expr.Capacity != nil {
			capType := c.typeOf(expr.Capacity, types.Int)
			if capType != types.Int {
				c.appendError("channel capacity must be int", expr)
				return types.Unit
			}
		}
		expr.SetResolvedType(expr.Type)
		return expr.Type
	case *ast.ReceiveExpr:
		chTypeRaw := c.typeOf(expr.Chan, nil)
		if chTypeRaw == nil {
			return types.Unit
		}
		chType, ok := chTypeRaw.(types.ChannelType)
		if !ok {
			c.appendError(fmt.Sprintf("only channels can receive, got %s", chTypeRaw.Signature()), expr)
		}
		expr.SetResolvedType(chType.ElemType)
		return chType.ElemType
	}
	return nil
}

func (c *Checker) checkInfixExpr(operator string, lt types.Type, rt types.Type, expr ast.Node) types.Type {
	switch operator {
	case "==":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()), expr)
		}

		return types.Bool
	case ">":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()), expr)
		}

		if lt != types.Float && lt != types.Int && lt != types.Byte {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}

		return types.Bool
	case ">=":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()), expr)
		}

		if lt != types.Float && lt != types.Int && lt != types.Byte {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}

		return types.Bool
	case "<":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()), expr)
		}

		if lt != types.Float && lt != types.Int && lt != types.Byte {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}

		return types.Bool
	case "<=":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()), expr)
		}

		if lt != types.Float && lt != types.Int && lt != types.Byte {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}

		return types.Bool
	case "!=":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot compare types %s to %s", lt.Signature(), rt.Signature()), expr)
		}

		return types.Bool
	case "&&":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot perform boolean operation on types %s and %s", lt.Signature(), rt.Signature()), expr)
		}
		if lt != types.Bool {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}
		return types.Bool
	case "||":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot perform boolean operation on types %s and %s", lt.Signature(), rt.Signature()), expr)
		}
		if lt != types.Bool {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}
		return types.Bool
	case "+":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot add types %s and %s", lt.Signature(), rt.Signature()), expr)
		}

		if lt != types.String && lt != types.Float && lt != types.Int && lt != types.Byte {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}

		return lt

	case "-":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot subtract types %s and %s", lt.Signature(), rt.Signature()), expr)
		}

		if lt != types.Float && lt != types.Int && lt != types.Byte {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}

		return lt
	case "*":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot multiply types %s and %s", lt.Signature(), rt.Signature()), expr)
		}

		if lt != types.Float && lt != types.Int {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}

		return lt
	case "/":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot divide types %s and %s", lt.Signature(), rt.Signature()), expr)
		}

		if lt != types.Float && lt != types.Int {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}

		return lt
	case "%":
		if !c.typesMatch(lt, rt) {
			c.appendError(fmt.Sprintf("type mismatch: cannot modulo types %s and %s", lt.Signature(), rt.Signature()), expr)
		}

		if lt != types.Int {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for type %s", operator, lt.Signature()), expr)
		}

		return types.Int
	default:
		c.appendError(fmt.Sprintf("unknown operator %s", operator), expr)
		return nil
	}
}

func (c *Checker) checkPrefixExpr(operator string, t types.Type, expr ast.Node) types.Type {
	if operator == "!" {
		if t != types.Bool {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for %s", operator, t.Signature()), expr)
			return nil
		}
		return t
	} else if operator == "-" {
		if t != types.Float && t != types.Int {
			c.appendError(fmt.Sprintf("invalid operation: %s is not defined for %s", operator, t.Signature()), expr)
			return nil
		}

		return t
	}

	c.appendError(fmt.Sprintf("unknown operator %s", operator), expr)
	return nil
}

func (c *Checker) typeOfCallExpr(expr *ast.CallExpr, expected types.Type) types.Type {
	if scope, ok := expr.Function.(*ast.ScopeAccessExpr); ok {
		template, generic := c.genericFunctions[scope.Member.Value]

		if generic && len(expr.TypeArgs) == 0 {
			typeArgs := c.inferTypeArgs(expr, template)
			if typeArgs != nil {
				expr.TypeArgs = typeArgs
			}
		}

		if generic && len(expr.TypeArgs) > 0 {
			return c.monomorphizeCall(expr, template)
		}
	}

	if selector, ok := expr.Function.(*ast.SelectorExpr); ok {
		receiverType := c.typeOf(selector.Left, nil)
		if st, ok := receiverType.(types.ScopeType); ok {
			receiverType = c.resolveType(st)
		}
		methodName := selector.Value.(*ast.Identifier).Value
		mangled := mangleMethod(receiverType.Signature(), methodName)
		if t, _, ok := c.env.Get(mangled); ok {
			if len(expr.Arguments) == 0 || expr.Arguments[0] != selector.Left {
				expr.Arguments = append([]ast.Expr{selector.Left}, expr.Arguments...)
			}
			if st, ok := receiverType.(types.StructType); ok {
				if st.Module != "" {
					mangled = st.Module + "__" + mangled
				} else {
					for modName, env := range c.packages {
						if _, ok := env.store[st.Name]; ok {
							mangled = modName + "__" + mangled
							break
						}
					}
				}
			}
			expr.MangledName = mangled
			return c.validateFunctionCall(expr, t)
		}

		if it, ok := toInterface(receiverType); ok {
			for i, m := range it.Methods {
				if m == methodName {
					return c.validateFunctionCall(expr, it.Types[i])
				}
			}
		}
	}

	if ident, ok := expr.Function.(*ast.Identifier); ok {
		template, generic := c.genericFunctions[ident.Value]

		if generic && len(expr.TypeArgs) == 0 {
			typeArgs := c.inferTypeArgs(expr, template)
			if typeArgs != nil {
				expr.TypeArgs = typeArgs
			}
		}

		if generic && len(expr.TypeArgs) > 0 {
			return c.monomorphizeCall(expr, template)
		}

		if len(expr.Arguments) > 0 {
			receiverType := c.typeOf(expr.Arguments[0], nil)
			if receiverType == nil {
				c.appendError(fmt.Sprintf("cannot resolve type for %s function call", ident.Value), expr)
				return nil
			}
			mangled := mangleMethod(receiverType.Signature(), ident.Value)
			if t, _, ok := c.env.Get(mangled); ok {
				return c.validateFunctionCall(expr, t)
			}
		}

		switch ident.Value {
		case "len":
			return c.checkLenBuiltIn(expr)
		case "print":
			return c.checkPrintBuiltIn(expr)
		case "first":
			return c.checkFirstBuiltIn(expr)
		case "last":
			return c.checkLastBuiltIn(expr)
		case "append":
			return c.checkAppendBuiltIn(expr)
		case "rest":
			return c.checkRestBuiltIn(expr)
		case "slice":
			return c.checkSliceBuiltIn(expr)
		case "keys":
			return c.checkKeysBuiltIn(expr)
		case "values":
			return c.checkValuesBuiltIn(expr)
		case "ok":
			return c.checkOkBuiltIn(expr)
		case "err":
			return c.checkErrBuiltIn(expr, expected)
		case "some":
			return c.checkSomeBuiltIn(expr)
		case "none":
			return c.checkNoneBuiltIn(expr, expected)
		case "float":
			return c.checkFloatBuiltIn(expr)
		case "panic":
			return c.checkPanicCall(expr)
		case "chan":
			return c.checkChanBuiltIn(expr)
		}

		if builtin := object.GetBuiltInByName(ident.Value); builtin != nil {
			return c.validateFunctionCall(expr, builtin.T)
		}
	}

	fnTypeRaw := c.typeOf(expr.Function, nil)

	return c.validateFunctionCall(expr, fnTypeRaw)
}

func (c *Checker) checkLenBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError(fmt.Sprintf("len() expects exactly 1 argument"), expr)
		return types.Int
	}

	argType := c.typeOf(expr.Arguments[0], nil)
	_, isArray := argType.(types.ArrayType)
	_, isMap := argType.(types.MapType)
	if argType != types.String && !isArray && !isMap && !isString(argType) {
		c.appendError(fmt.Sprintf("invalid argument type %s for len()", argType.Signature()), expr)
	}

	return types.Int
}

func (c *Checker) checkPrintBuiltIn(expr *ast.CallExpr) types.Type {
	for _, arg := range expr.Arguments {
		c.typeOf(arg, nil)
	}
	return types.Unit
}

func (c *Checker) checkFirstBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError(fmt.Sprintf("first() expects exactly 1 argument"), expr)
		return types.Int
	}

	argType := c.typeOf(expr.Arguments[0], nil)
	arrType, isArray := argType.(types.ArrayType)

	if !isArray {
		c.appendError(fmt.Sprintf("invalid argument type %s for first()", argType.Signature()), expr)
		return types.Unit
	}

	return arrType.ElemType
}

func (c *Checker) checkLastBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError(fmt.Sprintf("last() expects exactly 1 argument"), expr)
		return types.Int
	}

	argType := c.typeOf(expr.Arguments[0], nil)
	arrType, isArray := argType.(types.ArrayType)

	if !isArray {
		c.appendError(fmt.Sprintf("invalid argument type %s for last()", argType.Signature()), expr)
		return types.Unit
	}

	return arrType.ElemType
}

func (c *Checker) checkAppendBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 2 {
		c.appendError(fmt.Sprintf("append() expects exactly 2 arguments"), expr)
		return types.Unit
	}
	argType := c.typeOf(expr.Arguments[0], nil)

	arrType, isArray := argType.(types.ArrayType)

	if !isArray {
		c.appendError(fmt.Sprintf("invalid argument type %s for append()", argType.Signature()), expr)
		return types.Unit
	}

	valType := c.typeOf(expr.Arguments[1], nil)
	if !c.typesMatch(valType, arrType.ElemType) {
		c.appendError(fmt.Sprintf("type mismatch: got %s for append() value", valType.Signature()), expr)
	}

	return arrType
}

func (c *Checker) checkRestBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError(fmt.Sprintf("rest() expects exactly 1 argument"), expr)
	}

	argType := c.typeOf(expr.Arguments[0], nil)
	arrType, isArray := argType.(types.ArrayType)
	if !isArray {
		c.appendError(fmt.Sprintf("invalid argument type %s for rest()", argType.Signature()), expr)
		return types.Unit
	}

	return arrType
}

func (c *Checker) checkSliceBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 3 {
		c.appendError(fmt.Sprintf("slice() expects exactly 3 arguments"), expr)
	}
	arrayArgType := c.typeOf(expr.Arguments[0], nil)
	startType := c.typeOf(expr.Arguments[1], types.Int)
	endType := c.typeOf(expr.Arguments[2], types.Int)
	arrType, isArray := arrayArgType.(types.ArrayType)
	if !isArray {
		c.appendError(fmt.Sprintf("invalid argument type %s for slice()", arrType.Signature()), expr)
		return types.Unit
	}

	if startType != types.Int {
		c.appendError(fmt.Sprintf("invalid argument type %s for slice()", arrType.Signature()), expr)
	}

	if endType != types.Int {
		c.appendError(fmt.Sprintf("invalid argument type %s for slice()", arrType.Signature()), expr)
	}

	return arrType
}

func (c *Checker) checkKeysBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError(fmt.Sprintf("keys() expects exactly 1 argument"), expr)
	}
	t := c.typeOf(expr.Arguments[0], nil)
	mapType, ok := t.(types.MapType)
	if !ok {
		c.appendError(fmt.Sprintf("invalid argument type %s for keys()", t.Signature()), expr)
	}

	return types.ArrayType{ElemType: mapType.KeyType}
}

func (c *Checker) checkValuesBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError(fmt.Sprintf("values() expects exactly 1 argument"), expr)
	}
	t := c.typeOf(expr.Arguments[0], nil)
	mapType, ok := t.(types.MapType)
	if !ok {
		c.appendError(fmt.Sprintf("invalid argument type %s for keys()", t.Signature()), expr)
	}

	return types.ArrayType{ElemType: mapType.ValueType}
}

func (c *Checker) checkOkBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError(fmt.Sprintf("ok() expects exactly 1 argument"), expr)
	}

	t := c.typeOf(expr.Arguments[0], nil)

	resolved := types.ResultType{T: t}
	expr.ResolvedType = &resolved
	return resolved
}

func (c *Checker) checkSomeBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError(fmt.Sprintf("some() expects exactly 1 argument"), expr)
	}

	t := c.typeOf(expr.Arguments[0], nil)

	resolved := types.OptionType{T: t}
	expr.ResolvedType = &resolved
	return resolved
}

func (c *Checker) checkErrBuiltIn(expr *ast.CallExpr, contextType types.Type) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError(fmt.Sprintf("err() expects exactly 1 argument"), expr)
	}

	t := c.typeOf(expr.Arguments[0], nil)
	if t != types.String {
		c.appendError(fmt.Sprintf("invalid argument type %s for err()", t.Signature()), expr)
	}

	if contextType == nil && c.currentMatchResultType != nil {
		contextType = c.currentMatchResultType
	} else if contextType == nil {
		c.appendError("cannot infer result type for err()", expr)
		return types.ResultType{T: types.Unit}
	}
	var resolved types.ResultType
	if rt, ok := contextType.(types.ResultType); ok {
		resolved = types.ResultType{T: rt.T}
	} else {
		resolved = types.ResultType{T: contextType}
	}
	expr.ResolvedType = &resolved
	return resolved
}

func (c *Checker) checkNoneBuiltIn(expr *ast.CallExpr, contextType types.Type) types.Type {
	if len(expr.Arguments) != 0 {
		c.appendError(fmt.Sprintf("none() expects no arguments"), expr)
	}

	if contextType == nil && c.currentMatchResultType != nil {
		contextType = c.currentMatchResultType
	} else if contextType == nil {
		c.appendError("cannot infer option type for none()", expr)
		return types.OptionType{T: types.Unit}
	}
	var resolved types.OptionType
	if ot, ok := contextType.(types.OptionType); ok {
		resolved = types.OptionType{T: ot.T}
	} else {
		resolved = types.OptionType{T: contextType}
	}
	expr.ResolvedType = &resolved
	return resolved
}

func (c *Checker) checkChanBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 0 {
		c.appendError(fmt.Sprintf("chan() expects no arguments"), expr)
		return nil
	}
	if len(expr.TypeArgs) != 1 {
		c.appendError(fmt.Sprintf("chan() expects exactly 1 type argument argument"), expr)
		return nil
	}

	return types.ChannelType{ElemType: expr.TypeArgs[0]}
}

func (c *Checker) resolveInterfaceName(expr ast.Expr) (types.InterfaceType, string, bool) {
	switch e := expr.(type) {
	case *ast.Identifier:
		t, _, ok := c.env.Get(e.Value)
		if !ok {
			return types.InterfaceType{}, e.Value, false
		}
		it, ok := t.(types.InterfaceType)
		return it, e.Value, ok
	case *ast.ScopeAccessExpr:
		mod := e.Module.Value
		name := e.Member.Value
		if mt, ok := c.moduleTypes[mod]; ok {
			if t, ok := mt[name]; ok {
				it, ok := t.(types.InterfaceType)
				return it, name, ok
			}
		}
		return types.InterfaceType{}, name, false
	}
	return types.InterfaceType{}, "", false
}

func (c *Checker) isInterfaceMethod(t types.Type, name string) (string, bool) {
	structType, ok := toStruct(t)
	if !ok {
		return "", false
	}

	sn := structType.Name
	withInterfaces, ok := c.definedStructs[sn]
	if !ok {
		return "", false
	}

	for _, interfaceRaw := range withInterfaces.Interfaces {
		interfaceType, _ := toInterface(interfaceRaw)
		for _, mn := range interfaceType.Methods {
			if mn == name {
				return sn, true
			}
		}
	}

	return "", false
}

func (c *Checker) validateFunctionCall(expr *ast.CallExpr, fnTypeRaw types.Type) types.Type {
	if fnTypeRaw == nil || fnTypeRaw == types.Unit {
		c.appendError(fmt.Sprintf("unresolved symbol: %s", expr.Function.String()), expr)
		return nil
	}

	fnType, ok := fnTypeRaw.(types.FunctionType)
	if !ok {
		c.appendError(fmt.Sprintf("cannot call non-function %s %s", fnTypeRaw.Signature(), expr.Function.String()), expr)
		return nil
	}
	if len(expr.Arguments) != len(fnType.Params) || fnType.Variadic {
		c.appendError(fmt.Sprintf("wrong number of arguments for function %s, wanted %d, got %d", expr.Function.String(), len(expr.Arguments), len(fnType.Params)), expr)
		return nil
	}

	for i, arg := range expr.Arguments {
		aType := c.typeOf(arg, fnType.Params[i])
		if st, isScopeType := aType.(types.ScopeType); isScopeType {
			aType = c.resolveType(st)
		}
		pType := fnType.Params[i]
		if !c.typesMatch(aType, pType) {
			c.appendError(fmt.Sprintf("type mismatch: got %s for arg %d in function %s call, expected %s", aType.Signature(), i+1, expr.Function.String(), fnType.Params[i].Signature()), expr)
			return nil
		}
		c.boxIfNecessary(expr.Arguments[i], aType, pType)
	}
	expr.ResolvedType = fnType.Return
	return fnType.Return
}

func (c *Checker) structSatisfiesInterface(s types.StructType, i types.InterfaceType, node ast.Node, appendErr bool) bool {
	satisfies := true
	for idx, method := range i.Methods {
		emt := i.Types[idx].(types.FunctionType)

		mangledName := mangleMethod(s.Name, method)
		mtr, _, ok := c.env.Get(mangledName)
		if !ok {
			if appendErr {
				c.appendError(fmt.Sprintf("struct %s does not satisfy interface %s, missing method %s", s.Name, i.Name, method), node)
			}
			satisfies = false
			continue
		}

		mt := mtr.(types.FunctionType)
		if len(mt.Params) > 0 {
			if _, ok := mt.Params[0].(types.StructType); !ok {
				panic(fmt.Sprintf("invariant violation: method %q first param is %s, expected struct type\n%s",
					mangledName, mt.Params[0].Signature(), debug.Stack()))
			}
		}
		if !c.compareMethodSignature(mt, emt) {
			if appendErr {
				c.appendError(fmt.Sprintf("struct %s does not satisfy interface %s, wrong signature for method %s. got %s, want %s", s.Name, i.Name, method, mt.Signature(), emt.Signature()), node)
			}
			satisfies = false
			continue
		}
	}

	return satisfies
}

func (c *Checker) interfaceSatisfiesInterface(i1 types.InterfaceType, i2 types.InterfaceType, node ast.Node) bool {
	satisfies := true
	for idx, method := range i2.Methods {
		i1Idx := slices.Index(i1.Methods, method)
		if i1Idx == -1 {
			if node != nil {
				c.appendError(fmt.Sprintf("interface %s does not satisfy interface %s, missing method %s", i1.Name, i2.Name, method), node)
			}
			satisfies = false
			continue
		}
		i1m := i1.Types[i1Idx]
		i2m := i2.Types[idx]
		if !c.typesMatch(i1m, i2m) {
			if node != nil {
				c.appendError(fmt.Sprintf("interface %s does not satisfy interface %s, method %s has wrong signature. wanted %s, got %s", i1.Name, i2.Name, method, i2m.Signature(), i1m.Signature()), node)
			}
			satisfies = false
			continue
		}
	}

	return satisfies
}

func (c *Checker) compareMethodSignature(mt types.FunctionType, et types.FunctionType) bool {
	withoutReceiver := mt.Params[1:]
	if len(withoutReceiver) != len(et.Params) {
		return false
	}

	for i, param := range withoutReceiver {
		if !c.typesMatch(param, et.Params[i]) {
			return false
		}
	}

	return c.typesMatch(mt.Return, et.Return)
}

func (c *Checker) typesMatch(actual, expected types.Type) bool {
	if actual == nil || expected == nil {
		return actual == expected
	}
	if actual == types.Never || expected == types.Never {
		return true
	}
	if isBasicType(actual) && isBasicType(expected) {
		return actual == expected || actual == types.Any || expected == types.Any
	}

	// Handle empty collections (e.g., [] matching array<int>)
	if aa, ok := toArray(actual); ok {
		if ea, ok := toArray(expected); ok {
			if aa.IsEmpty {
				return true
			}

			return c.typesMatch(aa.ElemType, ea.ElemType)
		}
	}

	if em, ok := toMap(expected); ok {
		if am, ok := toMap(actual); ok {
			if am.IsEmpty || em.IsEmpty { // this is a shitty fix to a logic bug in where this is called for maps
				return true
			}

			return c.typesMatch(em.KeyType, am.KeyType) && c.typesMatch(em.ValueType, am.ValueType)
		}
	}

	if it, ok := toInterface(expected); ok {
		if st, ok := toStruct(actual); ok {
			return c.structSatisfiesInterface(st, it, nil, true)
		}
	}

	if es, ok := toStruct(expected); ok {
		if as, ok := toStruct(actual); ok {
			et := c.definedStructs[es.Name].Interfaces
			at := c.definedStructs[as.Name].Interfaces
			if len(et) != len(at) {
				return false
			}

			for i, aiRaw := range at {
				ai, ok := aiRaw.(types.InterfaceType)
				if !ok {
					return false
				}

				eiRaw := et[i]
				ei, ok := eiRaw.(types.InterfaceType)
				if !ok {
					return false
				}

				return ei.Name == ai.Name
			}
		}
	}

	if ei, ok := toInterface(expected); ok {
		if ai, ok := toInterface(actual); ok {
			if ei.Name == ai.Name {
				return true
			}
			return c.interfaceSatisfiesInterface(ai, ei, nil)
		}
	}

	return actual.Signature() == expected.Signature()
}

func (c *Checker) boxIfNecessary(expr ast.Expr, actual types.Type, expected types.Type) {
	if expr == nil || actual == nil || expected == nil {
		return
	}

	_, isStruct := toStruct(actual)
	it, isInterface := toInterface(expected)
	if isStruct && isInterface {
		expr.SetCastTo(&it)
	}
}

func isString(t types.Type) bool {
	return t.Signature() == types.String.Signature()
}

func toStruct(t types.Type) (types.StructType, bool) {
	typ, ok := t.(types.StructType)

	return typ, ok
}

func toInterface(t types.Type) (types.InterfaceType, bool) {
	typ, ok := t.(types.InterfaceType)
	return typ, ok
}

func isBasicType(t types.Type) bool {
	_, ok := t.(types.BasicType)
	return ok
}

func toArray(t types.Type) (types.ArrayType, bool) {
	typ, ok := t.(types.ArrayType)
	return typ, ok
}

func toMap(t types.Type) (types.MapType, bool) {
	typ, ok := t.(types.MapType)
	return typ, ok
}

func (c *Checker) checkIndexAssignment(node *ast.IndexAssignmentStmt) types.Type {
	c.typeOf(node.Left, nil)
	t := c.typeOf(node.Left.Left, nil)

	indexOrKeyType := c.typeOf(node.Left.Index, nil)

	valType := c.typeOf(node.Value, t)
	if valType == nil {
		return types.Unit
	}

	switch colType := t.(type) {
	case types.ArrayType:
		if indexOrKeyType != types.Int {
			c.appendError(fmt.Sprintf("index must be type int, got %s", indexOrKeyType.Signature()), node)
		}

		if !c.typesMatch(valType, colType.ElemType) {
			c.appendError(fmt.Sprintf("type mismatch: cannot assign %s to element of array of type %s", valType.Signature(), colType.Signature()), node)
		}
	case types.MapType:
		if !c.typesMatch(indexOrKeyType, colType.KeyType) {
			c.appendError(fmt.Sprintf("type mismatch: key for map of type %s must be %s, got %s", colType.Signature(), colType.KeyType.Signature(), indexOrKeyType.Signature()), node)
		}

		if !c.typesMatch(valType, colType.ValueType) {
			c.appendError(fmt.Sprintf("type mismatch: cannot assign %s to entry of map of type %s", valType.Signature(), colType.Signature()), node)
		}
	}

	return types.Unit
}

func (c *Checker) checkGenericFunctionBody(node *ast.FunctionDeclarationStmt) {
	subs := make(map[string]types.Type)
	for _, tp := range node.TypeParams {
		subs[tp.Name] = types.Any
	}

	cloned := ast.Clone(&ast.Program{Stmts: []ast.Stmt{node}})
	clonedFn := cloned.Stmts[0].(*ast.FunctionDeclarationStmt)
	fType := types.SubstituteTypeParams(clonedFn.Type.(types.FunctionType), subs).(types.FunctionType)
	ast.SubstituteTypeParams(clonedFn.Body, subs)

	oldErrors := c.errors
	c.errors = nil
	oldInLoop := c.inLoop
	c.inLoop = false
	oldReturnType := c.currentReturnType
	c.currentReturnType = fType.Return
	oldEnv := c.env
	c.env = NewTypeEnv(oldEnv)

	for i, param := range clonedFn.Params {
		c.env.Set(param.Value, fType.Params[i])
	}

	c.check(clonedFn.Body)

	c.env = oldEnv
	c.currentReturnType = oldReturnType
	c.inLoop = oldInLoop
	c.errors = oldErrors
}

func (c *Checker) checkFunctionDeclaration(node *ast.FunctionDeclarationStmt) types.Type {
	if len(node.TypeParams) > 0 {
		c.checkGenericFunctionBody(node)
		return types.Unit
	}
	name := node.Name.Value
	fTypeRaw := node.Type.(types.FunctionType)
	resolved := c.resolveFunctionType(fTypeRaw)
	node.Type = resolved
	fTypeRaw = resolved
	if len(fTypeRaw.Params) > 0 {
		receiverType := fTypeRaw.Params[0]
		if sn, ok := c.isInterfaceMethod(receiverType, name); ok {
			name = mangleMethod(sn, name)
			node.MangledName = name
		}
	}

	fTypeRetrieved, _, ok := c.env.Get(node.Name.Value)
	if !ok {
		c.env.Set(name, node.Type)
		fTypeRetrieved = node.Type
	}

	fType, ok := fTypeRetrieved.(types.FunctionType)
	if !ok {
		c.appendError(fmt.Sprintf("cannot use function declaration of %s", node.Name.Value), node)
	}

	if !node.IsExtern {
		oldInLoop := c.inLoop
		c.inLoop = false
		oldReturnType := c.currentReturnType
		c.currentReturnType = fType.Return
		oldEnv := c.env
		c.env = NewTypeEnv(oldEnv)

		for i, param := range node.Params {
			c.env.Set(param.Value, fType.Params[i])
		}

		c.check(node.Body)

		if fType.Return != types.Unit && !allPathsReturn(node.Body) {
			c.appendError(fmt.Sprintf("function %s missing return on all paths", node.Name.Value), node)
		}
		c.env = oldEnv
		c.currentReturnType = oldReturnType
		c.inLoop = oldInLoop
	}

	return types.Unit
}

func allPathsReturn(block *ast.BlockStmt) bool {
	if len(block.Stmts) == 0 {
		return false
	}
	last := block.Stmts[len(block.Stmts)-1]
	switch s := last.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ExpressionStmt:
		if ifExpr, ok := s.Expr.(*ast.IfExpr); ok {
			if ifExpr.Alternative == nil {
				return false
			}
			return allPathsReturn(ifExpr.Consequence) &&
				allPathsReturn(ifExpr.Alternative)
		}
		if matchExpr, ok := s.Expr.(*ast.MatchExpr); ok {
			return allPathsReturn(matchExpr.OkArm.Body) &&
				allPathsReturn(matchExpr.ErrArm.Body)
		}
		return true // implicit return via last expression
	}
	return false
}

func (c *Checker) checkIndexExpr(e ast.Node, expr *ast.IndexExpr) types.Type {
	lt := c.typeOf(expr.Left, nil)
	idxT := c.typeOf(expr.Index, nil)
	mt, mok := lt.(types.MapType)
	at, aok := lt.(types.ArrayType)

	if lt == types.String {
		if idxT != types.Int {
			c.appendError(fmt.Sprintf("index must be type int, got %s", idxT.Signature()), expr)
		}
		return types.Byte
	}

	if aok {
		if idxT != types.Int {
			c.appendError(fmt.Sprintf("index type for array must be int, got %s", idxT.Signature()), expr)
			return nil
		}
		expr.ResolvedType = at.ElemType
		expr.ContainerType = at
		e = expr
		return at.ElemType
	} else if mok {
		if idxT != mt.KeyType {
			c.appendError(fmt.Sprintf("index type for map %s must be %s, got %s", mt.Signature(), mt.KeyType.Signature(), idxT.Signature()), expr)
			return nil
		}

		optType := types.OptionType{T: mt.ValueType}
		expr.ResolvedType = optType
		expr.ContainerType = mt
		expr.Index.SetResolvedType(idxT)
		e = expr
		return optType
	}
	c.appendError(fmt.Sprintf("index operation undefined for type: %s", lt.Signature()), expr)

	return nil
}

func (c *Checker) extractDeclNameAndType(stmt ast.Stmt, pkgName string) (string, types.Type) {
	switch stmt := stmt.(type) {
	case *ast.VarDeclarationStmt:
		return stmt.Name.Value, stmt.Type
	case *ast.StructDefinitionStmt:
		st := stmt.Type
		st.Module = pkgName
		return stmt.Name.Value, st
	case *ast.FunctionDeclarationStmt:
		return stmt.Name.Value, stmt.Type
	case *ast.InterfaceDefinitionStmt:
		it := stmt.Type
		it.Module = pkgName
		return stmt.Name.Value, it
	}

	return "", nil
}

func (c *Checker) checkMatchExpr(expr *ast.MatchExpr) types.Type {
	subType := c.typeOf(expr.Subject, nil)

	if option, ok := subType.(types.OptionType); ok {
		return c.checkOptionMatch(expr, option)
	}

	result, ok := subType.(types.ResultType)
	if !ok {
		c.appendError(fmt.Sprintf("can only match on result or option type"), expr)
		return nil
	}
	expr.SubjectType = result.T

	discard := c.inDiscardPosition
	c.inDiscardPosition = false

	okEnv := NewTypeEnv(c.env)
	okEnv.Set(expr.OkArm.Pattern.Binding.Value, c.resolveType(result.T))
	oldEnv := c.env
	c.env = okEnv
	okBranch := c.check(expr.OkArm.Body)
	if okBranch == nil {
		c.appendError(fmt.Sprintf("cannot resolve type for ok branch"), expr)
		c.inDiscardPosition = discard
		return nil
	}
	c.env = oldEnv

	errEnv := NewTypeEnv(c.env)
	errEnv.Set(expr.ErrArm.Pattern.Binding.Value, types.String)
	oldEnv = c.env
	c.env = errEnv
	oldMatchResultType := c.currentMatchResultType
	if rt, ok := okBranch.(types.ResultType); ok {
		c.currentMatchResultType = rt.T
	} else {
		c.currentMatchResultType = okBranch
	}
	errBranch := c.check(expr.ErrArm.Body)
	if errBranch == nil {
		c.appendError(fmt.Sprintf("cannot resolve type for err branch"), expr)
		c.inDiscardPosition = discard
		return nil
	}
	if !discard && !c.typesMatch(errBranch, okBranch) {
		c.appendError(fmt.Sprintf("type mismatch: match arms must result in same type, got %s and %s", okBranch.Signature(), errBranch.Signature()), expr)
	}
	c.inDiscardPosition = discard
	c.env = oldEnv
	c.currentMatchResultType = oldMatchResultType

	// If one arm is Never (contains a return), use the other arm's type
	if okBranch == types.Never {
		return errBranch
	}
	return okBranch
}

func (c *Checker) checkTypeMatchExpr(expr *ast.MatchTypeExpr) types.Type {
	subjType := c.typeOf(expr.Subject, nil)

	returnedTypes := make([]types.Type, len(expr.Arms))

	isAny := subjType == types.Any
	if !isAny {
		if _, ok := subjType.(types.InterfaceType); !ok {
			c.appendError("match typeof requires any or interface type", expr)
			return types.Unit
		}
	}

	discard := c.inDiscardPosition
	c.inDiscardPosition = false

	for i, arm := range expr.Arms {
		resolved := c.resolveType(arm.Type)
		if it, ok := subjType.(types.InterfaceType); ok {
			if st, ok := resolved.(types.StructType); ok {
				if !c.structSatisfiesInterface(st, it, expr, true) {
					c.appendError(fmt.Sprintf("struct %s does not satisfy interface %s", st.Name, it.Name), expr)
					c.inDiscardPosition = discard
					return types.Unit
				}
			}
		}
		c.pushScope()
		c.env.Set(arm.Binding.Value, resolved)
		retType := c.check(arm.Body)
		c.popScope()
		returnedTypes[i] = retType
	}

	var resultType types.Type
	if expr.Default != nil {
		c.pushScope()
		resultType = c.check(expr.Default)
		c.popScope()
	}

	c.inDiscardPosition = discard

	for _, t := range returnedTypes {
		if resultType == nil {
			resultType = t
		}

		if !discard && resultType != t {
			c.appendError("all arms of type match must result in same type", expr)
			return types.Unit
		}
	}

	return resultType
}

func (c *Checker) checkOptionMatch(expr *ast.MatchExpr, option types.OptionType) types.Type {
	expr.SubjectType = option.T

	discard := c.inDiscardPosition
	c.inDiscardPosition = false

	someEnv := NewTypeEnv(c.env)
	someEnv.Set(expr.SomeArm.Pattern.Binding.Value, option.T)
	oldEnv := c.env
	c.env = someEnv
	someBranch := c.check(expr.SomeArm.Body)
	if someBranch == nil {
		c.appendError("cannot resolve type for some branch", expr)
		c.inDiscardPosition = discard
		return nil
	}
	c.env = oldEnv

	noneEnv := NewTypeEnv(c.env)
	oldEnv = c.env
	c.env = noneEnv
	noneBranch := c.check(expr.NoneArm.Body)
	if noneBranch == nil {
		c.appendError("cannot resolve type for none branch", expr)
		c.inDiscardPosition = discard
		return nil
	}
	if !discard && !c.typesMatch(noneBranch, someBranch) {
		c.appendError(fmt.Sprintf("type mismatch: match arms must result in same type, got %s and %s", someBranch.Signature(), noneBranch.Signature()), expr)
	}
	c.inDiscardPosition = discard
	c.env = oldEnv

	if someBranch == types.Never {
		return noneBranch
	}
	return someBranch
}

func (c *Checker) resolveType(t types.Type) types.Type {
	result := c.doResolveType(t)
	if _, ok := result.(types.ScopeType); ok {
		panic(fmt.Sprintf("invariant violation: resolveType returned unresolved ScopeType %s\n%s",
			result.Signature(), debug.Stack()))
	}
	return result
}

func (c *Checker) doResolveType(t types.Type) types.Type {
	switch t := t.(type) {
	case types.ScopeType:
		tt, ok := c.moduleTypes[t.Module][t.Name]
		if !ok {
			c.appendError(fmt.Sprintf("module %s type %s is not declared", t.Module, t.Name), nil)
			return nil
		}
		return tt
	case types.MapType:
		kt := c.resolveType(t.KeyType)
		vt := c.resolveType(t.ValueType)

		return types.MapType{KeyType: kt, ValueType: vt}
	case types.ArrayType:
		et := c.resolveType(t.ElemType)

		return types.ArrayType{ElemType: et}
	case types.StructType:
		if t.TypeArgs != nil {
			return c.resolveGenericStructType(t)
		}
		for i, typ := range t.Types {
			t.Types[i] = c.resolveType(typ)
		}
		return t
	case types.InterfaceType:
		for i, typ := range t.Types {
			t.Types[i] = c.resolveType(typ)
		}
		if t.Module == "" && c.currentModule != "" {
			t.Module = c.currentModule
		}
		return t
	case types.ResultType:
		rt := c.resolveType(t.T)

		return types.ResultType{T: rt}
	case types.FunctionType:
		return c.resolveFunctionType(t)
	}

	return t
}

func (c *Checker) resolveFunctionType(ft types.FunctionType) types.FunctionType {
	for i, p := range ft.Params {
		ft.Params[i] = c.resolveType(p)
	}
	ft.Return = c.resolveType(ft.Return)
	return ft
}

func (c *Checker) checkFloatBuiltIn(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError("float() expects exactly 1 argument", expr)
		return types.Float
	}
	t := c.typeOf(expr.Arguments[0], nil)
	if t != types.Int && t != types.Byte && t != types.Float {
		c.appendError(fmt.Sprintf("invalid argument type %s for float(), expected int or byte", t.Signature()), expr)
	}

	return types.Float
}

func (c *Checker) checkPanicCall(expr *ast.CallExpr) types.Type {
	if len(expr.Arguments) != 1 {
		c.appendError("panic() expects exactly 1 argument", expr)
		return types.Never
	}

	t := c.typeOf(expr.Arguments[0], types.String)
	if t != types.String {
		c.appendError(fmt.Sprintf("invalid argument type %s for panic(), expected string", t.Signature()), expr)
	}
	return types.Never
}

func (c *Checker) monomorphizeCall(expr *ast.CallExpr, template *ast.FunctionDeclarationStmt) types.Type {
	c.assertGenericTemplate(template.Name.Value, template.TypeParams, expr.TypeArgs)
	resolved := false
	defer func() {
		if resolved {
			c.assertMonomorphized(expr.MangledName, nil)
		}
	}()
	var funcName string
	switch fn := expr.Function.(type) {
	case *ast.Identifier:
		funcName = fn.Value
	case *ast.ScopeAccessExpr:
		funcName = fn.Member.Value
	}
	templateType := template.Type.(types.FunctionType)

	if len(expr.TypeArgs) != len(template.TypeParams) {
		c.appendError(fmt.Sprintf("%s expects exactly %d type arguments", funcName, len(template.TypeParams)), expr)
		return nil
	}

	subs := make(map[string]types.Type)
	for i, tp := range template.TypeParams {
		subs[tp.Name] = expr.TypeArgs[i]
	}

	mangledName := monomorphizeMethodName(funcName, expr.TypeArgs)

	if !c.monomorphized[mangledName] {

		cloned := ast.Clone(&ast.Program{Stmts: []ast.Stmt{template}})
		clonedFn := cloned.Stmts[0].(*ast.FunctionDeclarationStmt)

		clonedFn.Type = types.SubstituteTypeParams(clonedFn.Type, subs)
		// resolve parameterized struct types
		if ft, ok := clonedFn.Type.(types.FunctionType); ok {
			for i, p := range ft.Params {
				ft.Params[i] = c.resolveGenericStructType(p)
			}
			ft.Return = c.resolveGenericStructType(ft.Return)
			clonedFn.Type = ft
		}

		clonedFn.Name.Value = mangledName
		clonedFn.TypeParams = nil // no longer generic
		ast.SubstituteTypeParams(clonedFn.Body, subs)

		c.checkFunctionDeclaration(clonedFn)

		if c.program != nil {
			c.pendingInserts[c.stmtIndex] = append(c.pendingInserts[c.stmtIndex], clonedFn)
		}

		c.monomorphized[mangledName] = true
	}

	expr.MangledName = mangledName
	resolved = true

	concreteType := types.SubstituteTypeParams(templateType, subs)
	// resolve parameterized struct types
	if ft, ok := concreteType.(types.FunctionType); ok {
		for i, param := range ft.Params {
			ft.Params[i] = c.resolveGenericStructType(param)
		}
		ft.Return = c.resolveGenericStructType(ft.Return)
		concreteType = ft
	}
	return c.validateFunctionCall(expr, concreteType)
}

func (c *Checker) monomorphizeStructLiteral(expr *ast.StructLiteral, template types.StructType) (types.StructType, bool) {
	c.assertGenericTemplate(template.Name, template.TypeParams, expr.TypeArgs)
	resolved := false
	defer func() {
		if resolved {
			c.assertMonomorphized(expr.Name, nil)
		}
	}()
	if len(expr.TypeArgs) != len(template.TypeParams) {
		c.appendError(fmt.Sprintf("%s expects exactly %d type arguments", expr.Name, len(template.TypeParams)), expr)
		return types.StructType{}, false
	}

	for _, ta := range expr.TypeArgs {
		if _, isParam := ta.(*types.TypeParamRef); isParam {
			return template, true
		}
	}

	subs := make(map[string]types.Type)
	for i, tp := range template.TypeParams {
		subs[tp.Name] = expr.TypeArgs[i]
	}

	mangled := monomorphizeMethodName(expr.Name, expr.TypeArgs)

	if !c.monomorphized[mangled] {
		// monomorphize type
		concreteType := types.SubstituteTypeParams(template, subs).(types.StructType)
		concreteType.Name = mangled
		concreteType.TypeParams = nil
		c.assertConcreteStruct(concreteType)

		// register non-generic struct
		c.definedStructs[mangled] = concreteType

		// clone definition
		synthetic := &ast.StructDefinitionStmt{
			Name: &ast.Identifier{Value: mangled},
			Type: concreteType,
		}
		c.structNodes[mangled] = synthetic

		if c.program != nil {
			c.pendingInserts[c.stmtIndex] = append(c.pendingInserts[c.stmtIndex], synthetic)
		}

		c.monomorphized[mangled] = true

		c.monomorphizeStructMethods(template.Name, expr.TypeArgs, subs)
	}
	expr.Name = mangled
	resolved = true

	return c.definedStructs[mangled], true
}

func (c *Checker) monomorphizeStructMethods(structName string, typeArgs []types.Type, subs map[string]types.Type) {
	mangledStruct := monomorphizeMethodName(structName, typeArgs)
	defer func() {
		if st, ok := c.definedStructs[mangledStruct]; ok {
			c.assertMonomorphized(st.Name, st.TypeParams)
		}
	}()
	for fname, fn := range c.genericFunctions {
		ft, ok := fn.Type.(types.FunctionType)
		if !ok || len(ft.Params) == 0 {
			continue
		}
		firstParam, ok := ft.Params[0].(types.StructType)
		if !ok || firstParam.Name != structName {
			continue
		}
		mangledFn := monomorphizeMethodName(fname, typeArgs)
		if c.monomorphized[mangledFn] {
			continue
		}

		c.assertGenericTemplate(fn.Name.Value, fn.TypeParams, typeArgs)
		cloned := ast.Clone(&ast.Program{Stmts: []ast.Stmt{fn}})
		clonedFn := cloned.Stmts[0].(*ast.FunctionDeclarationStmt)

		clonedFn.Type = types.SubstituteTypeParams(clonedFn.Type, subs)
		if cft, ok := clonedFn.Type.(types.FunctionType); ok {
			if st, ok := cft.Params[0].(types.StructType); ok && st.TypeParams != nil {
				st.TypeArgs = typeArgs
				st.TypeParams = nil
				cft.Params[0] = st
			}
			for i, p := range cft.Params {
				cft.Params[i] = c.resolveGenericStructType(p)
			}
			cft.Return = c.resolveGenericStructType(cft.Return)
			clonedFn.Type = cft
		}

		clonedFn.Name.Value = mangledFn
		clonedFn.TypeParams = nil
		ast.SubstituteTypeParams(clonedFn.Body, subs)

		methodKey := mangleMethod(mangledStruct, fname)
		c.env.Set(methodKey, clonedFn.Type)
		clonedFn.MangledName = methodKey

		c.checkFunctionDeclaration(clonedFn)

		if c.program != nil {
			c.pendingInserts[c.stmtIndex] = append(c.pendingInserts[c.stmtIndex], clonedFn)
		}

		c.monomorphized[mangledFn] = true
	}

	mangled := monomorphizeMethodName(structName, typeArgs)
	if st, ok := c.definedStructs[mangled]; ok {
		c.indexModuleInterfaces()
		for _, it := range c.candidateInterfaces(mangled) {
			if c.structSatisfiesInterface(st, it, nil, false) {
				st.Interfaces = append(st.Interfaces, it)
				c.definedStructs[mangled] = st
				if node, ok := c.structNodes[mangled]; ok {
					node.Type = st
				}
			}
		}
	}
}

func (c *Checker) resolveGenericStructType(t types.Type) types.Type {
	var result types.Type = t
	resolved := false
	defer func() {
		if resolved {
			if st, ok := result.(types.StructType); ok {
				c.assertMonomorphized(st.Name, st.TypeParams)
			}
		}
	}()

	st, ok := t.(types.StructType)
	if !ok || st.TypeArgs == nil {
		return result
	}

	template, exists := c.genericStructs[st.Name]
	if !exists {
		return result
	}

	for _, ta := range st.TypeArgs {
		if _, isParam := ta.(*types.TypeParamRef); isParam {
			return result
		}
	}

	mangled := monomorphizeMethodName(st.Name, st.TypeArgs)
	if !c.monomorphized[mangled] {
		subs := make(map[string]types.Type)
		for i, tp := range template.Type.TypeParams {
			subs[tp.Name] = st.TypeArgs[i]
		}

		concreteType := types.SubstituteTypeParams(template.Type, subs).(types.StructType)
		concreteType.Name = mangled
		concreteType.TypeParams = nil
		c.assertConcreteStruct(concreteType)
		c.definedStructs[mangled] = concreteType

		synthetic := &ast.StructDefinitionStmt{
			Name: &ast.Identifier{Value: mangled},
			Type: concreteType,
		}

		if c.program != nil {
			c.pendingInserts[c.stmtIndex] = append(c.pendingInserts[c.stmtIndex], synthetic)
		}

		c.monomorphized[mangled] = true
	}

	result = c.definedStructs[mangled]
	resolved = true
	return result
}

func monomorphizeMethodName(base string, tt []types.Type) string {
	parts := []string{base}
	for _, t := range tt {
		parts = append(parts, t.Signature())
	}

	return strings.Join(parts, "__")
}

func mangleMethod(module, name string) string {
	return module + "." + name
}

func (c *Checker) appendError(msg string, node ast.Node) {
	e := errors.PositionError{Message: msg, File: c.currentModule}
	if node != nil {
		e.Line, e.Col = node.Pos()
	}
	c.errors = append(c.errors, e)
}

func (c *Checker) inferTypeArgs(expr *ast.CallExpr, template *ast.FunctionDeclarationStmt) []types.Type {
	templateType := template.Type.(types.FunctionType)
	if len(expr.Arguments) != len(templateType.Params) {
		c.appendError(fmt.Sprintf("%s expects %d arguments", template.Name.Value, len(templateType.Params)), expr)
		return nil
	}

	subs := make(map[string]types.Type)
	for i, p := range templateType.Params {
		argType := c.typeOf(expr.Arguments[i], nil)
		c.unifyType(p, argType, subs)
	}

	result := make([]types.Type, len(templateType.Params))
	for i, p := range template.TypeParams {
		resolved, ok := subs[p.Name]
		if !ok {
			c.appendError(fmt.Sprintf("cannot infer type parameter %s for %s", p.Name, template.Name.Value), expr)
			return nil
		}

		result[i] = resolved
	}

	return result
}

func (c *Checker) unifyType(p types.Type, arg types.Type, subs map[string]types.Type) {
	switch t := p.(type) {
	case *types.TypeParamRef:
		if _, ok := subs[t.Name]; !ok {
			subs[t.Name] = arg
		}
	case types.ArrayType:
		if a, ok := arg.(types.ArrayType); ok {
			c.unifyType(t.ElemType, a.ElemType, subs)
		}
	case types.MapType:
		if a, ok := arg.(types.MapType); ok {
			c.unifyType(t.KeyType, a.KeyType, subs)
			c.unifyType(t.ValueType, a.ValueType, subs)
		}
	case types.ResultType:
		if a, ok := arg.(types.ArrayType); ok {
			c.unifyType(t.T, a.ElemType, subs)
		}
	}
}

func (c *Checker) indexInterface(it types.InterfaceType) {
	for _, mn := range it.Methods {
		c.methodToInterfaces[mn] = append(c.methodToInterfaces[mn], it)
	}
}

func (c *Checker) indexModuleInterfaces() {
	if c.moduleInterfacesIndexed {
		return
	}
	c.moduleInterfacesIndexed = true
	for modName, mt := range c.moduleTypes {
		for _, t := range mt {
			if it, ok := t.(types.InterfaceType); ok {
				if it.Module == "" {
					it.Module = modName
				}
				if _, indexed := c.definedInterfaces[it.Name]; !indexed {
					c.assertInterfaceConsistent(it)
					c.definedInterfaces[it.Name] = it
					c.indexInterface(it)
				}
			}
		}
	}
}

func (c *Checker) candidateInterfaces(structName string) []types.InterfaceType {
	seen := make(map[string]bool)
	var candidates []types.InterfaceType
	for _, name := range c.env.MethodsOf(structName) {
		for _, iface := range c.methodToInterfaces[name] {
			if !seen[iface.Name] {
				seen[iface.Name] = true
				candidates = append(candidates, iface)
			}
		}
	}
	return candidates
}

func (c *Checker) discoverImplementations() {
	c.indexModuleInterfaces()
	for sn, st := range c.definedStructs {
		for _, it := range c.candidateInterfaces(sn) {
			if c.structSatisfiesInterface(st, it, nil, false) {
				st.Interfaces = append(st.Interfaces, it)
				c.definedStructs[sn] = st
				if node, ok := c.structNodes[sn]; ok {
					node.Type = st
				}
			}
		}
	}
}

func (c *Checker) assertInterfaceConsistent(it types.InterfaceType) {
	if len(it.Methods) != len(it.Types) {
		panic(fmt.Sprintf("invariant violation: interface %s has %d methods and %d types\n%s",
			it.Name, len(it.Methods), len(it.Types), debug.Stack()))
	}
}

func (c *Checker) assertMonomorphized(name string, typeParams []*types.TypeParam) {
	if name != "" && !strings.Contains(name, "__") {
		panic(fmt.Sprintf("invariant violation: monomorphized name %q is not mangled\n%s", name, debug.Stack()))
	}
	if typeParams != nil {
		panic(fmt.Sprintf("invariant violation: monomorphized %q still has type params", name))
	}
}

func containsTypeParamRef(t types.Type) bool {
	switch tt := t.(type) {
	case *types.TypeParamRef:
		return true
	case types.ArrayType:
		return containsTypeParamRef(tt.ElemType)
	case types.MapType:
		return containsTypeParamRef(tt.KeyType) || containsTypeParamRef(tt.ValueType)
	case types.FunctionType:
		for _, p := range tt.Params {
			if containsTypeParamRef(p) {
				return true
			}
		}
		return containsTypeParamRef(tt.Return)
	case types.StructType:
		for _, ft := range tt.Types {
			if containsTypeParamRef(ft) {
				return true
			}
		}
	case types.ResultType:
		return containsTypeParamRef(tt.T)
	case types.OptionType:
		return containsTypeParamRef(tt.T)
	}
	return false
}

func (c *Checker) assertConcreteStruct(st types.StructType) {
	for i, t := range st.Types {
		if containsTypeParamRef(t) {
			panic(fmt.Sprintf("invariant violation: concrete struct %q has TypeParamRef in field %q (type %s)\n%s",
				st.Name, st.Fields[i], t.Signature(), debug.Stack()))
		}
	}
}

func (c *Checker) assertGenericTemplate(name string, typeParams []*types.TypeParam, typeArgs []types.Type) {
	if len(typeParams) == 0 {
		panic(fmt.Sprintf("invariant violation: generic template %q does not have type parameters", name))
	}

	if len(typeArgs) == 0 {
		panic(fmt.Sprintf("invariant violation: call for template %q does not have type arguments", name))
	}
}
