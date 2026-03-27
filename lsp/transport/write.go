package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"lsp/messages"
)

func WriteResponse(w io.Writer, r *messages.Response) error {
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	if err != nil {
		return err
	}
	return nil
}
