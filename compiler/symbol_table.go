package compiler

type SymbolScopes string

const (
	GlobalScope   SymbolScopes = "GLOBAL"
	LocalScope    SymbolScopes = "LOCAL"
	BuiltinScope  SymbolScopes = "BUILTIN"
	FreeScope     SymbolScopes = "FREE"
	FunctionScope SymbolScopes = "FUNCTION"
)

type Symbol struct {
	Name       string
	Scope      SymbolScopes
	Index      int
	IsConstant bool
}

type SymbolTable struct {
	Outer *SymbolTable

	store          map[string]Symbol
	numDefinitions int
	FreeSymbols    []Symbol
}

func NewSymbolTable() *SymbolTable {
	s := make(map[string]Symbol)
	free := []Symbol{}
	return &SymbolTable{store: s, FreeSymbols: free}
}

func (s *SymbolTable) DefineMutable(name string) Symbol {
	symbol := Symbol{Name: name, Index: s.numDefinitions, IsConstant: false}
	if s.Outer == nil {
		symbol.Scope = GlobalScope
	} else {
		symbol.Scope = LocalScope
	}

	s.store[name] = symbol
	s.numDefinitions++
	return symbol
}

func (s *SymbolTable) DefineImmutable(name string) Symbol {
	symbol := Symbol{Name: name, Index: s.numDefinitions, IsConstant: true}
	if s.Outer == nil {
		symbol.Scope = GlobalScope
	} else {
		symbol.Scope = LocalScope
	}

	s.store[name] = symbol
	s.numDefinitions++
	return symbol
}

func (s *SymbolTable) DefineBuiltin(index int, name string) Symbol {
	symbol := Symbol{Name: name, Index: index, Scope: BuiltinScope, IsConstant: true}
	s.store[name] = symbol
	return symbol
}

func (s *SymbolTable) defineFree(original Symbol) Symbol {
	s.FreeSymbols = append(s.FreeSymbols, original)

	symbol := Symbol{Name: original.Name, Index: len(s.FreeSymbols) - 1, IsConstant: original.IsConstant}
	symbol.Scope = FreeScope
	s.store[original.Name] = symbol

	return symbol
}

// symbol, fromOuter, ok
func (s *SymbolTable) Resolve(name string) (Symbol, bool, bool) {
	symbol, ok := s.store[name]
	if !ok && s.Outer != nil {
		symbol, _, ok = s.Outer.Resolve(name)
		if !ok {
			return symbol, true, ok
		}

		// if we are here, we resolved variable from outer scope
		if symbol.Scope == GlobalScope || symbol.Scope == BuiltinScope {
			return symbol, true, ok
		}

		free := s.defineFree(symbol)
		return free, true, true
	}

	return symbol, false, ok
}

func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	s := NewSymbolTable()
	s.Outer = outer
	return s
}

func (s *SymbolTable) DefineFunctionName(name string) Symbol {
	symbol := Symbol{Name: name, Index: 0, Scope: FunctionScope, IsConstant: false}
	s.store[name] = symbol
	return symbol
}

func (s *SymbolTable) DefineAlias(name string, original Symbol) {
	s.store[name] = original
}
