package handlers

import (
	"encoding/json"
	"log"
	"sydney/ast"
	"sydney/lsp/messages"
)

type HoverResult struct {
	Contents MarkupContents `json:"contents"`
}

func (h *HoverResult) Result() {}

type MarkupContents struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func (l *LSP) HandleHover(req *messages.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Hover panicked: %v", r)
		}
	}()
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
	err = l.WriteResponse(resp)
	if err != nil {
		log.Printf("%s: Error writing response: %v", messages.Hover, err)
	}
}
