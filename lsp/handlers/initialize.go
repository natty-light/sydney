package handlers

import (
	"io"
	"log"
	"sydney/lsp/messages"
	"sydney/lsp/transport"
)

type ServerCapabilities struct {
	TextDocumentSync int  `json:"textDocumentSync"`
	HoverProvider    bool `json:"hoverProvider"`
}

type InitializeResult struct {
	ServerInfo   ServerInfo         `json:"serverInfo"`
	Capabilities ServerCapabilities `json:"capabilities"`
}

func (i *InitializeResult) Result() {}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (l *LSP) HandleInitialize(w io.Writer, r *messages.Request) {
	res := &InitializeResult{
		ServerInfo: ServerInfo{
			Name:    "sydney-lsp",
			Version: "0.1.0",
		},
		Capabilities: ServerCapabilities{
			HoverProvider:    true,
			TextDocumentSync: 1,
		},
	}

	resp := &messages.Response{
		Version: messages.Version,
		Result:  res,
		Id:      r.Id,
	}

	if err := transport.WriteResponse(w, resp); err != nil {
		log.Fatal(err)
	}
}
