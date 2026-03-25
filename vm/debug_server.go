package vm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
)

func (d *Debugger) WaitForClient() {
	pid := os.Getpid()
	addr := fmt.Sprintf("/tmp/sydney-debug-%d.sock", pid)
	if err := os.RemoveAll(addr); err != nil {
		log.Fatal(err)
	}

	l, err := net.Listen("unix", addr)
	if err != nil {
		log.Fatal(err)
	}

	sigCh := make(chan os.Signal, 1)
	go func(c chan os.Signal) {
		<-c
		log.Printf("caught signal, shutting down")
		l.Close()
		os.Exit(0)
	}(sigCh)

	log.Printf("listening on socket %s", addr)
	conn, err := l.Accept()
	if err != nil {
		log.Fatal("accept error:", err)
	}
	l.Close()
	log.Printf("client connected")

	// read commands from socket → push to cmdCh
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			cmd, err := parseCommand(scanner.Text())
			if err != nil {
				log.Printf("bad command: %s", err)
				continue
			}
			d.cmdCh <- cmd
		}
	}()

	// write events from eventCh → socket
	go func() {
		encoder := json.NewEncoder(conn)
		for evt := range d.eventCh {
			encoder.Encode(serializeEvent(evt))
		}
	}()
}

func parseCommand(line string) (DebugCommand, error) {
	var raw map[string]interface{}
	json.Unmarshal([]byte(line), &raw)

	switch raw["cmd"] {
	case "set_breakpoint":
		return &AddBreakpoint{
			File: raw["file"].(string),
			Line: int(raw["line"].(float64)),
		}, nil
	case "remove_breakpoint":
		return &RemoveBreakpoint{
			File: raw["file"].(string),
			Line: int(raw["line"].(float64)),
		}, nil
	case "continue":
		return &SetMode{Flag: DebugContinue}, nil
	case "step_line":
		return &SetMode{Flag: DebugStepLine}, nil
	case "step_in":
		return &SetMode{Flag: DebugStepInstruction}, nil
	case "step_out":
		return &SetMode{Flag: DebugStepOut}, nil
	case "get_locals":
		return &GetLocals{}, nil
	case "get_stack":
		return &GetStack{}, nil
	case "get_callstack":
		return &GetCallStack{}, nil
	case "get_source":
		return &GetSource{
			File: raw["file"].(string),
		}, nil
	}

	return nil, fmt.Errorf("unknown command: %s", raw["cmd"])
}

func serializeEvent(evt DebugEvent) map[string]interface{} {
	switch e := evt.(type) {
	case *StoppedEvent:
		return map[string]interface{}{
			"event":  "stopped",
			"reason": e.Reason,
			"file":   e.File,
			"line":   e.Line,
			"fiber":  e.Fiber,
		}
	case *RunningEvent:
		return map[string]interface{}{
			"event": "running",
		}
	case *TerminatedEvent:
		return map[string]interface{}{
			"event": "terminated",
			"error": e.Error,
		}
	case *OutputEvent:
		return map[string]interface{}{
			"event": "output",
			"text":  e.Text,
		}
	case *LocalsResponse:
		return map[string]interface{}{
			"event": "response",
			"type":  "locals",
			"data":  e.Locals,
		}
	case *StackResponse:
		return map[string]interface{}{
			"event": "response",
			"type":  "stack",
			"data":  e.Stack,
		}
	case *CallStackResponse:
		return map[string]interface{}{
			"event": "response",
			"type":  "callstack",
			"data":  e.Frames,
		}
	case *SourceResponse:
		return map[string]interface{}{
			"event":   "response",
			"type":    "source",
			"file":    e.File,
			"content": e.Content,
		}
	}

	return map[string]interface{}{}
}
