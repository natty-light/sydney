package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"lsp/handlers"
	"lsp/messages"
	"os"
	"strings"
)

func main() {
	w := io.Writer(os.Stdout)
	r := io.Reader(os.Stdin)

	reader := bufio.NewReader(r)
	for {
		contentLength := 0
		for {
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			if len(line) == 0 {
				break
			}
			if strings.HasPrefix(line, "Content-Length: ") {
				_, err := fmt.Sscanf(line, "Content-Length: %d", &contentLength)
				if err != nil {
					log.Fatal(err)
				}
			}
		}

		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			log.Fatal(err)
		}

		var req messages.Request
		err := json.Unmarshal(body, &req)
		if err != nil {
			log.Fatal(err)
		}

		switch req.Method {
		case messages.Initialize:
			handlers.HandleInitialize(w, &req)
		}

	}

	os.Exit(0)
}
