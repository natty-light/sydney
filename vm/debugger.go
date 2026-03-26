package vm

import (
	"sydney/code"
	"sydney/compiler"
	"sydney/object"
)

type DebugMode int

const (
	DebugContinue DebugMode = iota
	DebugStepLine
	DebugStepInstruction
	DebugStepOver
	DebugStepOut
)

type Debugger struct {
	Flag        DebugMode
	cmdCh       chan DebugCommand
	eventCh     chan DebugEvent
	breakpoints map[string]map[int]bool // file  -> line -> enabled
	sources     map[string]string       // file  -> contents
	lastLine    int
	lastFile    string
	stepFrame   int
	symbolTable *compiler.SymbolTable
}

func NewDebugger(symbolTable *compiler.SymbolTable) *Debugger {
	return &Debugger{
		Flag:        DebugStepLine,
		cmdCh:       make(chan DebugCommand),
		eventCh:     make(chan DebugEvent),
		breakpoints: make(map[string]map[int]bool),
		sources:     make(map[string]string),
		lastLine:    0,
		lastFile:    "",
		stepFrame:   0,
		symbolTable: symbolTable,
	}
}

type (
	DebugCommand interface {
		dbgCmd()
	}

	DebugEvent interface {
		dbgEvent()
	}
)

type (
	LocalVar struct {
		Name  string
		Type  string
		Value string
	}
	StackEntry struct {
		Value string
		Type  string
	}
	FrameInfo struct{}
)

// events
type (
	StoppedEvent struct {
		Reason string
		File   string
		Line   int
		Fiber  int
	}

	RunningEvent struct{}

	TerminatedEvent struct {
		Error string
	}

	OutputEvent struct {
		Text string
	}

	LocalsResponse struct {
		Locals []LocalVar
	}

	StackResponse struct {
		Stack []StackEntry
	}

	CallStackResponse struct {
		Frames []FrameInfo
	}

	SourceResponse struct {
		File    string
		Content string
	}
)

func (s *StoppedEvent) dbgEvent()      {}
func (r *RunningEvent) dbgEvent()      {}
func (t *TerminatedEvent) dbgEvent()   {}
func (o *OutputEvent) dbgEvent()       {}
func (l *LocalsResponse) dbgEvent()    {}
func (s *StackResponse) dbgEvent()     {}
func (c *CallStackResponse) dbgEvent() {}
func (s *SourceResponse) dbgEvent()    {}

// commands
type (
	SetMode struct {
		Flag DebugMode
	}

	AddBreakpoint struct {
		File string
		Line int
	}

	RemoveBreakpoint struct {
		File string
		Line int
	}

	GetLocals struct{}

	GetStack struct{}

	GetCallStack struct{}

	GetSource struct {
		File string
	}
)

func (s *SetMode) dbgCmd()          {}
func (a *AddBreakpoint) dbgCmd()    {}
func (r *RemoveBreakpoint) dbgCmd() {}
func (g *GetLocals) dbgCmd()        {}
func (g *GetStack) dbgCmd()         {}
func (g *GetCallStack) dbgCmd()     {}
func (g *GetSource) dbgCmd()        {}

func (d *Debugger) shouldStop(ip int, frameIdx int, sm *code.SourceMap) bool {
	line, _, file := sm.LineForOffset(ip)
	if line == 0 {
		return false
	}

	switch d.Flag {
	case DebugContinue:
		return d.breakpoints[file][line]
	case DebugStepLine:
		return file != d.lastFile || line != d.lastLine
	case DebugStepOver:
		return (file != d.lastFile || line != d.lastLine) && frameIdx <= d.stepFrame
	case DebugStepOut:
		return frameIdx < d.stepFrame
	case DebugStepInstruction:
		return true
	}

	return false
}

func (d *Debugger) handleCommand(cmd DebugCommand) {
	switch c := cmd.(type) {
	case *SetMode:
		d.handleSetMode(c)
	case *AddBreakpoint:
		d.handleAddBreakpoint(c)
	case *RemoveBreakpoint:
		d.handleRemoveBreakpoint(c)
	case *GetCallStack:
		d.handleGetCallStack(c)
	case *GetSource:
		d.handleGetSource(c)
	}
}

func (d *Debugger) handleSetMode(cmd *SetMode) {
	d.Flag = cmd.Flag
}

func (d *Debugger) handleAddBreakpoint(cmd *AddBreakpoint) {
	if d.breakpoints[cmd.File] == nil {
		d.breakpoints[cmd.File] = make(map[int]bool)
	}

	d.breakpoints[cmd.File][cmd.Line] = true
}

func (d *Debugger) handleRemoveBreakpoint(cmd *RemoveBreakpoint) {
	d.breakpoints[cmd.File][cmd.Line] = false
}

func (d *Debugger) handleGetLocals(dbgSyms *code.DebugSymbols, stack []object.Object, bp int) {
	locals := make([]LocalVar, len(dbgSyms.Locals))
	for i, local := range dbgSyms.Locals {
		if local == nil {
			continue
		}
		l := LocalVar{
			Name: local.Name,
			Type: local.Type,
		}
		if stack[bp+i] != nil {
			l.Value = stack[bp+i].Inspect()
		}
		locals[i] = l
	}
	d.eventCh <- &LocalsResponse{
		Locals: locals,
	}
}

func (d *Debugger) handleGetStack(stack []object.Object) {
	entries := make([]StackEntry, 0)
	for _, e := range stack {
		if e == nil {
			continue
		}
		ee := StackEntry{
			Value: e.Inspect(),
			Type:  string(e.Type()),
		}
		entries = append(entries, ee)
	}

	d.eventCh <- &StackResponse{
		Stack: entries,
	}
}

func (d *Debugger) handleGetCallStack(cmd *GetCallStack) {
}

func (d *Debugger) handleGetSource(cmd *GetSource) {
	content := d.sources[cmd.File]
	d.eventCh <- &SourceResponse{File: cmd.File, Content: content}
}

func (d *Debugger) AddSource(file string, content string) {
	d.sources[file] = content
}

func isResumeCommand(cmd DebugCommand) bool {
	_, ok := cmd.(*SetMode)
	return ok
}

func isMode(cmd DebugCommand, mode DebugMode) bool {
	sm, ok := cmd.(*SetMode)
	return ok && sm.Flag == mode
}

func (d *Debugger) SendCommand(cmd DebugCommand) {
	d.cmdCh <- cmd
}

func (d *Debugger) EventCh() chan DebugEvent {
	return d.eventCh
}
