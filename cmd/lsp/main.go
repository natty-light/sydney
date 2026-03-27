package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sydney/lsp/handlers"
	"sydney/lsp/messages"
)

func main() {
	f, _ := os.Create("/tmp/sydney-lsp.log")
	log.SetOutput(f)

	w := io.Writer(os.Stdout)
	r := io.Reader(os.Stdin)

	lsp := handlers.New(w)
	log.Printf("sydney-lsp started")

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

		log.Printf("recv: %s", req.Method)

		switch req.Method {
		case messages.Initialize:
			lsp.HandleInitialize(&req)
		case messages.Initialized:
		case messages.DocumentOpen:
			lsp.HandleDocumentOpen(&req)
		case messages.DocumentChange:
			lsp.HandleDocumentChange(&req)
		case messages.DocumentClose:
		case messages.Hover:
			lsp.HandleHover(&req)
		case messages.Shutdown:
			resp := &messages.Response{
				Id:      req.Id,
				Version: messages.Version,
				Result:  nil,
			}
			lsp.WriteResponse(resp)
			os.Exit(0)
		}
	}
}
