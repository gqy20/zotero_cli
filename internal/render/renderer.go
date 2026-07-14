package render

import (
	"encoding/json"
	"fmt"
	"io"

	"zotero_cli/internal/app"
)

type Envelope struct {
	OK       bool           `json:"ok"`
	Command  string         `json:"command"`
	Data     any            `json:"data,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
	Warnings []app.Warning  `json:"warnings,omitempty"`
	Error    *ErrorData     `json:"error,omitempty"`
	Code     int            `json:"code,omitempty"`
}

type ErrorData struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type Renderer struct {
	Out io.Writer
	Err io.Writer
}

func (r Renderer) Result(path app.CommandPath, result app.Result, opts app.OutputOptions) error {
	if opts.Format == "json" {
		enc := json.NewEncoder(r.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(Envelope{OK: true, Command: path.String(), Data: result.Data, Meta: result.Meta, Warnings: result.Warnings})
	}
	for _, warning := range result.Warnings {
		if !opts.Quiet {
			fmt.Fprintln(r.Err, "warning:", warning.Message)
		}
	}
	if result.Text != "" && !opts.Quiet {
		_, err := fmt.Fprintln(r.Out, result.Text)
		return err
	}
	return nil
}

func (r Renderer) Error(path app.CommandPath, err error, code int, format string) error {
	if format == "json" {
		errorType := app.ClassifyError(err)
		if errorType == "unknown" {
			switch code {
			case 2:
				errorType = "usage"
			case 3:
				errorType = "config"
			case 130:
				errorType = "cancelled"
			}
		}
		return json.NewEncoder(r.Out).Encode(Envelope{OK: false, Command: path.String(), Error: &ErrorData{Type: errorType, Message: err.Error()}, Code: code})
	}
	_, writeErr := fmt.Fprintln(r.Err, "error:", err)
	return writeErr
}
