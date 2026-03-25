package vm

import "sydney/code"

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
	lastLine    int
	lastFile    string
}

func NewDebugger() *Debugger {
	return &Debugger{
		Flag:        DebugStepLine,
		cmdCh:       make(chan DebugCommand),
		eventCh:     make(chan DebugEvent),
		breakpoints: make(map[string]map[int]bool),
		lastLine:    0,
		lastFile:    "",
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
	LocalVar   struct{}
	StackEntry struct{}
	FrameInfo  struct{}
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

func (d *Debugger) shouldStop(ip int, sm *code.SourceMap) bool {
	line, _, file := sm.LineForOffset(ip)
	if line == 0 {
		return false
	}

	switch d.Flag {
	case DebugContinue:
		return d.breakpoints[file][line]
	case DebugStepLine:
		return file != d.lastFile || line != d.lastLine
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
	case *GetLocals:
		d.handleGetLocals(c)
	case *GetStack:
		d.handleGetStack(c)
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

func (d *Debugger) handleGetLocals(cmd *GetLocals) {
}

func (d *Debugger) handleGetStack(cmd *GetStack) {
}

func (d *Debugger) handleGetCallStack(cmd *GetCallStack) {
}

func (d *Debugger) handleGetSource(cmd *GetSource) {
}

func isResumeCommand(cmd DebugCommand) bool {
	_, ok := cmd.(*SetMode)
	return ok
}

func (d *Debugger) SendCommand(cmd DebugCommand) {
	d.cmdCh <- cmd
}

func (d *Debugger) EventCh() chan DebugEvent {
	return d.eventCh
}
