package errors

import (
	"fmt"
	"io"
)

type PositionError struct {
	Line    int
	Col     int
	Message string
	File    string
}

func PrintPositionErrors(w io.Writer, errs []PositionError) {
	for _, msg := range errs {
		var str string
		if msg.File != "" {
			str = fmt.Sprintf("\t%s:%d:%d:%s\n", msg.File, msg.Line, msg.Col, msg.Message)
		} else {
			str = fmt.Sprintf("\t%d:%d:%s\n", msg.Line, msg.Col, msg.Message)
		}
		io.WriteString(w, str)
	}
}
