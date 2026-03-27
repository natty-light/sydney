package handlers

import (
	"log"
	"net/url"
	"sydney/errors"
	"sydney/lsp/messages"
)

type DiagnosticSeverity int

const (
	ErrorSeverity DiagnosticSeverity = iota + 1
	WarningSeverity
	InformationSeverity
	HintSeverity
)
const (
	TypecheckerSource string = "typechecker"
	ParserSource      string = "parser"
)

type PublishDiagnosticsParams struct {
	Uri         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func (p *PublishDiagnosticsParams) Params() {}

type Diagnostic struct {
	Range    messages.Range     `json:"range"`
	Source   string             `json:"source"`
	Message  string             `json:"message"`
	Severity DiagnosticSeverity `json:"severity"`
}

func (l *LSP) SendTypecheckerDiagnostics(errors []errors.PositionError, filePath string) {
	uri := url.URL{Path: filePath, Scheme: "file"}

	params := &PublishDiagnosticsParams{
		Uri: uri.String(),
	}

	diagnostics := make([]Diagnostic, len(errors))
	for i, err := range errors {
		diagnostics[i] = Diagnostic{
			Range: messages.Range{
				Start: messages.Position{
					Line:      err.Line,
					Character: err.Col,
				},
				End: messages.Position{
					Line:      err.Line,
					Character: err.Col + 99, // im hoping this just does the whole line
				},
			},
			Source:   TypecheckerSource,
			Severity: ErrorSeverity,
			Message:  err.Message,
		}
	}

	params.Diagnostics = diagnostics

	notif := &messages.Notification{
		Method: messages.PublishDiagnostics,
		Params: params,
	}

	err := l.WriteNotification(notif)
	if err != nil {
		log.Printf("%s: Error writing notification: %v", messages.Hover, err)
	}
}
