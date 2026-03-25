package code

type SourceMap struct {
	Mappings map[int]*SourceMapping
}

type SourceMapping struct {
	InstructionOffset int
	Line              int
	Col               int
	File              string
}

func New() *SourceMap {
	return &SourceMap{
		Mappings: make(map[int]*SourceMapping),
	}
}

func (sm *SourceMap) LineForOffset(offset int) (int, int, string) {
	if mapping, ok := sm.Mappings[offset]; ok {
		return mapping.Line, mapping.Col, mapping.File
	}

	return 0, 0, ""
}
