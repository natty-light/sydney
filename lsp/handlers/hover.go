package handlers

import (
	"encoding/json"
	"io"
	"log"
	"sydney/ast"
	"sydney/lsp/messages"
	"sydney/lsp/transport"
)

type HoverResult struct {
	Contents MarkupContents `json:"contents"`
}

func (h *HoverResult) Result() {}

type MarkupContents struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func (l *LSP) HandleHover(w io.Writer, req *messages.Request) {
	if l.program == nil || l.env == nil {
		return
	}
	var params messages.HoverParams
	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		log.Printf("%s Error unmarshalling params: %v", messages.Hover, err)
		return
	}

	log.Printf("hover: line=%d col=%d", params.Position.Line+1, params.Position.Character+1)
	ident := ast.FindAt(l.program, params.Position.Line+1, params.Position.Character+1)
	if ident == nil {
		log.Printf("hover: no ident found")
		return
	}
	log.Printf("hover: found ident %s", ident.Value)
	typ, _, ok := l.env.Get(ident.Value)
	if !ok {
		log.Printf("hover: cannot resolve type for %s", ident.Value)
	}
	result := &HoverResult{
		Contents: MarkupContents{
			Kind:  "plaintext",
			Value: ident.Value + ": " + typ.Signature(),
		},
	}
	resp := &messages.Response{
		Id:      req.Id,
		Version: messages.Version,
		Result:  result,
	}
	err = transport.WriteResponse(w, resp)
	if err != nil {
		log.Printf("%s: Error writing response: %v", messages.Hover, err)
	}
}
