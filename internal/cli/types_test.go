package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/zoteroapi"
)

func TestExitCodeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant int
		expected int
	}{
		{"ExitOK", ExitOK, 0},
		{"ExitError", ExitError, 1},
		{"ExitUsage", ExitUsage, 2},
		{"ExitConfig", ExitConfig, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestExitCodeDistinct(t *testing.T) {
	codes := []int{ExitOK, ExitError, ExitUsage, ExitConfig}
	seen := make(map[int]bool)
	for _, c := range codes {
		if seen[c] {
			t.Errorf("duplicate exit code: %d", c)
		}
		seen[c] = true
	}
	if len(seen) != len(codes) {
		t.Errorf("expected %d unique codes, got %d", len(codes), len(seen))
	}
}

func TestJSONResponseStructure(t *testing.T) {
	resp := jsonResponse{
		OK:      true,
		Command: "test",
		Data:    "hello",
		Meta:    map[string]any{"count": 1},
	}

	if !resp.OK {
		t.Error("expected OK=true")
	}
	if resp.Command != "test" {
		t.Errorf("command = %q, want %q", resp.Command, "test")
	}
	if resp.Data != "hello" {
		t.Error("data mismatch")
	}
	if resp.Meta["count"] != 1 {
		t.Error("meta count mismatch")
	}
}

func TestBoolToInt(t *testing.T) {
	if boolToInt(true) != ExitError {
		t.Error("boolToInt(true) should be ExitError")
	}
	if boolToInt(false) != ExitOK {
		t.Error("boolToInt(false) should be ExitOK")
	}
}

func TestJSONErrorsEnabledOutputsStructuredError(t *testing.T) {
	t.Setenv("ZOT_JSON_ERRORS", "1")

	stdout, stderr := captureOutput(t)
	cli := New()
	cli.stdout = stdout
	cli.stderr = stderr

	err := fmt.Errorf("test error: item not found")
	exitCode := cli.jsonError(err, "show")

	// JSON mode always returns exit 0; error details are in the JSON payload
	if exitCode != ExitOK {
		t.Fatalf("expected exit code %d (JSON mode), got %d", ExitOK, exitCode)
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}

	if got["ok"] != false {
		t.Fatalf("expected ok=false, got: %#v", got["ok"])
	}
	if got["command"] != "show" {
		t.Fatalf("expected command=show, got: %#v", got["command"])
	}
	if _, ok := got["code"]; !ok {
		t.Fatal("expected 'code' field in JSON error response")
	}
}

func TestJSONErrorsDisabledOutputsPlainText(t *testing.T) {
	t.Setenv("ZOT_JSON_ERRORS", "0")

	stdout, stderr := captureOutput(t)
	cli := New()
	cli.stdout = stdout
	cli.stderr = stderr

	err := fmt.Errorf("test error: something failed")
	exitCode := cli.jsonError(err, "find")

	if exitCode != ExitError {
		t.Fatalf("expected exit code %d, got %d", ExitError, exitCode)
	}

	got := stderr.String()
	if !strings.Contains(got, "error:") {
		t.Fatalf("expected plain text error prefix, got: %q", got)
	}
	if !strings.Contains(got, "something failed") {
		t.Fatalf("expected error message in output, got: %q", got)
	}

	if stdout.Len() > 0 {
		t.Fatalf("expected no stdout for plain text mode, got: %q", stdout.String())
	}
}

func TestStructuredErrorDataFormat(t *testing.T) {
	t.Setenv("ZOT_JSON_ERRORS", "1")

	stdout, _ := captureOutput(t)
	cli := New()
	cli.stdout = stdout

	err := fmt.Errorf("item not found: ABCD1234: %w", backend.ErrItemNotFound)
	cli.jsonError(err, "show")

	var got map[string]any
	_ = json.Unmarshal(stdout.Bytes(), &got)

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be an object, got: %T (%#v)", got["data"], got["data"])
	}
	if data["error"] == nil {
		t.Fatal("expected 'error' field in error data")
	}
	if data["type"] != "not_found" {
		t.Errorf("expected type=not_found, got: %#v", data["type"])
	}
	if data["code"] == nil {
		t.Fatal("expected 'code' field in error data")
	}
}

func TestClassifyErrorType(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{fmt.Errorf("item X not found: k: %w", backend.ErrItemNotFound), "not_found"},
		{fmt.Errorf("local does not support find --qmode: %w", backend.ErrUnsupportedFeature), "unsupported_feature"},
		{fmt.Errorf("db locked: %w", backend.ErrLocalTemporarilyUnavailable), "temporarily_unavailable"},
		{&zoteroapi.APIError{StatusCode: 404}, "not_found"},
		{&zoteroapi.APIError{StatusCode: 403}, "forbidden"},
		{&zoteroapi.APIError{StatusCode: 429}, "rate_limited"},
		{&zoteroapi.APIError{StatusCode: 412}, "precondition_failed"},
		{&zoteroapi.APIError{StatusCode: 500}, "server_error_500"},
		{fmt.Errorf("wrapped: %w", backend.ErrItemNotFound), "not_found"},
		{fmt.Errorf("plain"), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := classifyErrorType(tt.err); got != tt.expected {
				t.Errorf("classifyErrorType(%v) = %q, want %q", tt.err, got, tt.expected)
			}
		})
	}
}
