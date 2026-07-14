package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"zotero_cli/internal/app"
)

func TestRendererJSONContract(t *testing.T) {
	var out, errOut bytes.Buffer
	r := Renderer{Out: &out, Err: &errOut}
	err := r.Result(app.CommandPath{Resource: "config", Action: "show"}, app.Result{
		Data: map[string]any{"ok": true}, Meta: map[string]any{"source": "test"},
		Warnings: []app.Warning{{Code: "test_warning", Message: "test warning"}},
	}, app.OutputOptions{Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	var got Envelope
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.Command != "config show" {
		t.Fatalf("envelope = %#v", got)
	}
	if len(got.Warnings) != 1 || got.Warnings[0].Code != "test_warning" {
		t.Fatalf("warnings = %#v", got.Warnings)
	}
	if errOut.Len() != 0 {
		t.Fatalf("JSON warnings must not be duplicated on stderr: %q", errOut.String())
	}
}

func TestRendererErrorUsesCanonicalPath(t *testing.T) {
	var out, errOut bytes.Buffer
	r := Renderer{Out: &out, Err: &errOut}
	if err := r.Error(app.CommandPath{Resource: "config", Action: "check"}, errors.New("boom"), 3, "json"); err != nil {
		t.Fatal(err)
	}
	var got Envelope
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.OK || got.Command != "config check" || got.Code != 3 {
		t.Fatalf("envelope = %#v", got)
	}
	if got.Data != nil || got.Error == nil || got.Error.Type != "config" || got.Error.Message != "boom" {
		t.Fatalf("error envelope = %#v", got)
	}
}
