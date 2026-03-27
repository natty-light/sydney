package handlers

import (
	"encoding/json"
	"log"
	"net/url"
	"sydney/lsp/messages"
)

func (l *LSP) HandleDocumentChange(req *messages.Request) {
	var params messages.DocumentChangeParams
	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		log.Printf("%s: Error unmarshalling params: %v", messages.DocumentOpen, err)
		return
	}

	u, err := url.Parse(params.TextDocument.URI)
	if err != nil {
		log.Printf("%s: Error parsing URI: %v", messages.DocumentOpen, err)
		return
	}
	log.Printf("parsed %s uri", u.String())

	filePath := u.Path
	src := params.ContentChanges[0].Text

	l.parse(messages.DocumentChange, filePath, src)
}
