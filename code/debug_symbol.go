package code

type DebugSymbol struct {
	Scope string
	Name  string
	Type  string
}

type DebugSymbols struct {
	Locals []*DebugSymbol
}
