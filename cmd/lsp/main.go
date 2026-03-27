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
	"sydney/lsp/transport"
)

func main() {
	f, _ := os.Create("/tmp/sydney-lsp.log")
	log.SetOutput(f)

	w := io.Writer(os.Stdout)
	r := io.Reader(os.Stdin)

	lsp := handlers.New()
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
			lsp.HandleInitialize(w, &req)
		case messages.Initialized:
		case messages.DocumentOpen:
			lsp.HandleDocumentOpen(&req)
		case messages.DocumentChange:
		case messages.DocumentClose:
		case messages.Hover:
			lsp.HandleHover(w, &req)
		case messages.Shutdown:
			resp := &messages.Response{
				Id:      req.Id,
				Version: messages.Version,
				Result:  nil,
			}
			transport.WriteResponse(w, resp)
			os.Exit(0)
		}
	}
}
