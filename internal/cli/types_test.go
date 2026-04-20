package cli

import "testing"

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
