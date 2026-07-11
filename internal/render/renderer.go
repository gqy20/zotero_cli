package render

import (
	"encoding/json"
	"fmt"
	"io"

	"zotero_cli/internal/app"
)

type Envelope struct {
	OK      bool           `json:"ok"`
	Command string         `json:"command"`
	Data    any            `json:"data"`
	Meta    map[string]any `json:"meta,omitempty"`
	Code    int            `json:"code,omitempty"`
}

type ErrorData struct {
	Error string `json:"error"`
	Type  string `json:"type,omitempty"`
	Code  int    `json:"code"`
}

type Renderer struct {
	Out io.Writer
	Err io.Writer
}

func (r Renderer) Result(path app.CommandPath, result app.Result, opts app.OutputOptions) error {
	for _, warning := range result.Warnings {
		if !opts.Quiet {
			fmt.Fprintln(r.Err, "warning:", warning.Message)
		}
	}
	if opts.Format == "json" {
		enc := json.NewEncoder(r.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(Envelope{OK: true, Command: path.String(), Data: result.Data, Meta: result.Meta})
	}
	if result.Text != "" && !opts.Quiet {
		_, err := fmt.Fprintln(r.Out, result.Text)
		return err
	}
	return nil
}

func (r Renderer) Error(path app.CommandPath, err error, code int, format string) error {
	if format == "json" {
		return json.NewEncoder(r.Out).Encode(Envelope{OK: false, Command: path.String(), Data: ErrorData{Error: err.Error(), Type: app.ClassifyError(err), Code: code}, Code: code})
	}
	_, writeErr := fmt.Fprintln(r.Err, "error:", err)
	return writeErr
}
