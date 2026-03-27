package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"sydney/ast"
	"sydney/lsp/messages"
	"sydney/types"
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
	ident, foundScope := ast.FindAt(l.program, params.Position.Line+1, params.Position.Character+1)
	if ident == nil {
		log.Printf("hover: no ident found")
		return
	}
	log.Printf("hover: found ident %s", ident.Value)
	log.Printf("hover: found scope %t", foundScope != nil)
	var typ types.Type
	var ok bool
	if foundScope != nil {
		typ, _, ok = foundScope.Get(ident.Value)
		log.Printf("hover: found block scoped type %s", typ)
	} else {
		typ, _, ok = l.env.Get(ident.Value)
		log.Printf("hover: found globally scoped type %s", typ)
	}
	if !ok {
		sel := ast.FindSelectorAt(l.program, params.Position.Line+1, params.Position.Character+1)
		if sel != nil {
			typ, ok = l.resolveMethodType(sel, ident, foundScope)
		}
	}
	if !ok {
		log.Printf("hover: cannot resolve type for %s", ident.Value)
		return
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

func (l *LSP) resolveMethodType(sel *ast.SelectorExpr, method *ast.Identifier, scope ast.Scope) (types.Type, bool) {
	if receiver, ok := sel.Left.(*ast.Identifier); ok {
		var receiverType types.Type
		var found bool
		if scope != nil {
			receiverType, _, found = scope.Get(receiver.Value)
		}
		if !found {
			receiverType, _, found = l.env.Get(receiver.Value)
		}
		if !found || receiverType == nil {
			return nil, false
		}

		var structName string
		switch rt := receiverType.(type) {
		case types.StructType:
			structName = rt.Name
		case types.ScopeType:
			structName = rt.Name
		}
		if structName == "" {
			return nil, false
		}

		mangled := fmt.Sprintf("%s.%s", structName, method.Value)
		log.Printf("hover: trying mangled method %s", mangled)
		if scope != nil {
			if typ, _, ok := scope.Get(mangled); ok {
				return typ, true
			}
		}
		if typ, _, ok := l.env.Get(mangled); ok {
			return typ, true
		}
	}
	return nil, false
}
