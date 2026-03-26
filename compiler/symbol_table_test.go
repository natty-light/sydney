package compiler

import (
	"testing"
)

func TestDefine(t *testing.T) {
	expected := map[string]Symbol{
		"a": {Name: "a", Scope: "GLOBAL", Index: 0, IsConstant: true},
		"b": {Name: "b", Scope: "GLOBAL", Index: 1, IsConstant: false},
		"c": {Name: "c", Scope: "LOCAL", Index: 0, IsConstant: true},
		"d": {Name: "d", Scope: "LOCAL", Index: 1, IsConstant: false},
		"e": {Name: "e", Scope: "LOCAL", Index: 0, IsConstant: true},
		"f": {Name: "f", Scope: "LOCAL", Index: 1, IsConstant: false},
	}

	globalScope := NewSymbolTable()

	a := globalScope.DefineImmutable("a")
	if a != expected["a"] {
		t.Errorf("a = %v, expected %v", a, expected["a"])
	}

	b := globalScope.DefineMutable("b")
	if b != expected["b"] {
		t.Errorf("b = %v, expected %v", b, expected["b"])
	}

	firstLocal := NewEnclosedSymbolTable(globalScope)
	c := firstLocal.DefineImmutable("c")
	if c != expected["c"] {
		t.Errorf("c = %v, expected %v", c, expected["c"])
	}

	d := firstLocal.DefineMutable("d")
	if d != expected["d"] {
		t.Errorf("d = %v, expected %v", d, expected["d"])
	}

	secondLocal := NewEnclosedSymbolTable(firstLocal)
	e := secondLocal.DefineImmutable("e")
	if e != expected["e"] {
		t.Errorf("e = %v, expected %v", e, expected["e"])
	}

	f := secondLocal.DefineMutable("f")
	if f != expected["f"] {
		t.Errorf("f = %v, expected %v", f, expected["f"])
	}

}

func TestResolveGlobal(t *testing.T) {
	globalScope := NewSymbolTable()
	globalScope.DefineImmutable("a")
	globalScope.DefineMutable("b")

	expected := []Symbol{
		{Name: "a", Scope: "GLOBAL", Index: 0, IsConstant: true},
		{Name: "b", Scope: "GLOBAL", Index: 1, IsConstant: false},
	}

	for _, sym := range expected {
		result, _, ok := globalScope.Resolve(sym.Name)
		if !ok {
			t.Errorf("name %s not resolvable", sym.Name)
		}

		if result != sym {
			t.Errorf("expected %s to resolve to %+v, got=%+v", sym.Name, sym, result)
		}
	}
}

func TestResolveNestedLocal(t *testing.T) {
	globalScope := NewSymbolTable()
	globalScope.DefineImmutable("a")
	globalScope.DefineMutable("b")

	firstLocal := NewEnclosedSymbolTable(globalScope)
	firstLocal.DefineImmutable("c")
	firstLocal.DefineMutable("d")

	secondLocal := NewEnclosedSymbolTable(firstLocal)
	secondLocal.DefineImmutable("e")
	secondLocal.DefineMutable("f")

	tests := []struct {
		table           *SymbolTable
		expectedSymbols []Symbol
	}{
		{
			firstLocal,
			[]Symbol{
				{Name: "a", Scope: GlobalScope, Index: 0, IsConstant: true},
				{Name: "b", Scope: GlobalScope, Index: 1, IsConstant: false},
				{Name: "c", Scope: LocalScope, Index: 0, IsConstant: true},
				{Name: "d", Scope: LocalScope, Index: 1, IsConstant: false},
			},
		},
		{
			secondLocal,
			[]Symbol{
				{Name: "a", Scope: GlobalScope, Index: 0, IsConstant: true},
				{Name: "b", Scope: GlobalScope, Index: 1, IsConstant: false},
				{Name: "e", Scope: LocalScope, Index: 0, IsConstant: true},
				{Name: "f", Scope: LocalScope, Index: 1, IsConstant: false},
			},
		},
	}

	for _, tt := range tests {
		for _, sym := range tt.expectedSymbols {
			result, _, ok := tt.table.Resolve(sym.Name)
			if !ok {
				t.Errorf("name %s not resolvable", sym.Name)
			}

			if result != sym {
				t.Errorf("expected %s to resolve to %+v, got=%+v", sym.Name, sym, result)
			}
		}
	}
}

func TestResolveLocal(t *testing.T) {
	globalScope := NewSymbolTable()
	globalScope.DefineImmutable("a")
	globalScope.DefineMutable("b")

	local := NewEnclosedSymbolTable(globalScope)
	local.DefineImmutable("c")
	local.DefineMutable("d")

	expected := []Symbol{
		{Name: "a", Scope: GlobalScope, Index: 0, IsConstant: true},
		{Name: "b", Scope: GlobalScope, Index: 1, IsConstant: false},
		{Name: "c", Scope: LocalScope, Index: 0, IsConstant: true},
		{Name: "d", Scope: LocalScope, Index: 1, IsConstant: false},
	}

	for _, sym := range expected {
		result, _, ok := local.Resolve(sym.Name)
		if !ok {
			t.Errorf("name %s not resolvable", sym.Name)
		}

		if result != sym {
			t.Errorf("expected %s to resolve to %+v, got=%+v", sym.Name, sym, result)
		}
	}
}

func TestDefineResolveBuiltins(t *testing.T) {
	global := NewSymbolTable()
	firstLocal := NewEnclosedSymbolTable(global)
	secondLocal := NewEnclosedSymbolTable(firstLocal)

	expected := []Symbol{
		{Name: "a", Scope: BuiltinScope, Index: 0, IsConstant: true},
		{Name: "b", Scope: BuiltinScope, Index: 1, IsConstant: true},
		{Name: "c", Scope: BuiltinScope, Index: 2, IsConstant: true},
		{Name: "d", Scope: BuiltinScope, Index: 3, IsConstant: true},
	}

	for i, sym := range expected {
		global.DefineBuiltin(i, sym.Name)
	}

	for _, table := range []*SymbolTable{global, firstLocal, secondLocal} {
		for _, sym := range expected {
			result, _, ok := table.Resolve(sym.Name)
			if !ok {
				t.Errorf("name %s not resolvable", sym.Name)
			}

			if result != sym {
				t.Errorf("expected %s to resolve to %+v, got=%+v", sym.Name, sym, result)
			}
		}
	}
}

func TestResolveFree(t *testing.T) {
	global := NewSymbolTable()
	global.DefineImmutable("a")
	global.DefineImmutable("b")

	firstLocal := NewEnclosedSymbolTable(global)
	firstLocal.DefineImmutable("c")
	firstLocal.DefineImmutable("d")

	secondLocal := NewEnclosedSymbolTable(firstLocal)
	secondLocal.DefineImmutable("e")
	secondLocal.DefineImmutable("f")

	tests := []struct {
		table               *SymbolTable
		expectedSymbols     []Symbol
		expectedFreeSymbols []Symbol
	}{
		{
			firstLocal,
			[]Symbol{
				{Name: "a", Scope: GlobalScope, Index: 0, IsConstant: true},
				{Name: "b", Scope: GlobalScope, Index: 1, IsConstant: true},
				{Name: "c", Scope: LocalScope, Index: 0, IsConstant: true},
				{Name: "d", Scope: LocalScope, Index: 1, IsConstant: true},
			},
			[]Symbol{},
		},
		{
			secondLocal,
			[]Symbol{
				{Name: "a", Scope: GlobalScope, Index: 0, IsConstant: true},
				{Name: "b", Scope: GlobalScope, Index: 1, IsConstant: true},
				{Name: "c", Scope: FreeScope, Index: 0, IsConstant: true},
				{Name: "d", Scope: FreeScope, Index: 1, IsConstant: true},
				{Name: "e", Scope: LocalScope, Index: 0, IsConstant: true},
				{Name: "f", Scope: LocalScope, Index: 1, IsConstant: true},
			},
			[]Symbol{
				{Name: "c", Scope: LocalScope, Index: 0, IsConstant: true},
				{Name: "d", Scope: LocalScope, Index: 1, IsConstant: true},
			},
		},
	}

	for _, tt := range tests {
		for _, sym := range tt.expectedSymbols {
			result, _, ok := tt.table.Resolve(sym.Name)

			if !ok {
				t.Errorf("name %s not resolvable", sym.Name)
				continue
			}

			if result != sym {
				t.Errorf("expected %s to resolve to %+v, got=%+v", sym.Name, sym, result)
			}
		}

		if len(tt.table.FreeSymbols) != len(tt.expectedFreeSymbols) {
			t.Errorf("wrong number of free symbols. got=%d, want=%d", len(tt.table.FreeSymbols), len(tt.expectedFreeSymbols))
			continue
		}

		for i, sym := range tt.expectedFreeSymbols {
			result := tt.table.FreeSymbols[i]
			if result != sym {
				t.Errorf("wrong free symbol. got=%+v, want=%+v", result, sym)
			}
		}
	}
}

func TestResolveUnresolvableFree(t *testing.T) {
	global := NewSymbolTable()
	global.DefineImmutable("a")

	firstLocal := NewEnclosedSymbolTable(global)
	firstLocal.DefineImmutable("c")

	secondLocal := NewEnclosedSymbolTable(firstLocal)
	secondLocal.DefineImmutable("e")
	secondLocal.DefineImmutable("f")

	expected := []Symbol{
		{"a", GlobalScope, 0, true, nil},
		{"c", FreeScope, 0, true, nil},
		{"e", LocalScope, 0, true, nil},
		{"f", LocalScope, 1, true, nil},
	}

	for _, sym := range expected {
		result, _, ok := secondLocal.Resolve(sym.Name)

		if !ok {
			t.Errorf("name %s not resolvable", sym.Name)
			continue
		}

		if result != sym {
			t.Errorf("expected %s to resolve to %+v, got=%+v", sym.Name, sym, result)
		}
	}

	expectedUnresolvable := []string{
		"b",
		"d",
	}

	for _, name := range expectedUnresolvable {
		_, _, ok := secondLocal.Resolve(name)
		if ok {
			t.Errorf("name %s resolved, but was expected not to", name)
		}
	}
}

func TestDefineAndResolveFunctionName(t *testing.T) {
	g := NewSymbolTable()
	g.DefineFunctionName("a")

	expected := Symbol{Name: "a", Scope: FunctionScope, Index: 0, IsConstant: false}

	result, _, ok := g.Resolve(expected.Name)
	if !ok {
		t.Fatalf("function name %s not resolvable", expected.Name)
	}

	if result != expected {
		t.Errorf("expected %s to resolve to %+v, got=%+v", expected.Name, expected, result)
	}
}

func TestShadowingFunctionName(t *testing.T) {
	g := NewSymbolTable()
	g.DefineFunctionName("a")
	g.DefineImmutable("a")

	expected := Symbol{Name: "a", Scope: GlobalScope, Index: 0, IsConstant: true}

	result, _, ok := g.Resolve(expected.Name)
	if !ok {
		t.Fatalf("function name %s not resolvable", expected.Name)
	}

	if result != expected {
		t.Errorf("expected %s to resolve to %+v, got=%+v", expected.Name, expected, result)
	}
}
