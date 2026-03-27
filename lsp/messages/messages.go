package messages

import "encoding/json"

type Method string

const (
	Initialize         Method = "initialize"
	Initialized               = "initialized"
	DocumentOpen              = "textDocument/didOpen"
	DocumentChange            = "textDocument/didChange"
	Hover                     = "textDocument/hover"
	DocumentClose             = "textDocument/didClose"
	Shutdown                  = "shutdown"
	PublishDiagnostics        = "textDocument/publishDiagnostics"
)

const Version = "2.0"

type Request struct {
	Version string          `json:"jsonrpc"`
	Method  Method          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Id      interface{}     `json:"id"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type DocumentOpenParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type ContentChanges struct {
	Text string `json:"text"`
}

type DocumentChangeParams struct {
	TextDocument   TextDocumentItem `json:"textDocument"`
	ContentChanges []ContentChanges `json:"contentChanges"`
}

type Response struct {
	Version string      `json:"jsonrpc"`
	Result  Result      `json:"result"`
	Error   interface{} `json:"error"`
	Id      interface{} `json:"id"`
}

type Notification struct {
	Method Method `json:"method"`
	Params Params `json:"params"`
}

type Params interface {
	Params()
}

type Result interface {
	Result()
}
