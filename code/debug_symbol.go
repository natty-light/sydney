package code

type DebugSymbol struct {
	Scope string
	Name  string
}

type DebugSymbols struct {
	Locals []*DebugSymbol
}
