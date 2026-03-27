package errors

import (
	"fmt"
	"io"
)

type PositionError struct {
	Line    int
	Col     int
	Message string
}

func PrintPositionErrors(w io.Writer, errs []PositionError) {
	for _, msg := range errs {
		str := fmt.Sprintf("\t%d:%d:%s\n", msg.Line, msg.Col, msg.Message)
		io.WriteString(w, str)
	}
}
