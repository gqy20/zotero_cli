package backend

import (
	"testing"

	"zotero_cli/internal/config"
)

func TestNewReaderDefaultsToWebMode(t *testing.T) {
	reader, err := NewReader(config.Config{}, nil)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if _, ok := reader.(*WebReader); !ok {
		t.Fatalf("NewReader() returned %T, want *WebReader", reader)
	}
}

func TestNewReaderWebMode(t *testing.T) {
	reader, err := NewReader(config.Config{Mode: "web"}, nil)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if _, ok := reader.(*WebReader); !ok {
		t.Fatalf("NewReader() returned %T, want *WebReader", reader)
	}
}

func TestNewReaderRejectsUnimplementedModes(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{name: "local", mode: "local"},
		{name: "hybrid", mode: "hybrid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewReader(config.Config{Mode: tt.mode}, nil)
			if err == nil {
				t.Fatalf("NewReader() error = nil, want error")
			}
			want := tt.mode + " mode is not implemented yet"
			if err.Error() != want {
				t.Fatalf("NewReader() error = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestNewReaderRejectsUnsupportedMode(t *testing.T) {
	_, err := NewReader(config.Config{Mode: "bogus"}, nil)
	if err == nil {
		t.Fatalf("NewReader() error = nil, want error")
	}
	if err.Error() != "unsupported mode \"bogus\"" {
		t.Fatalf("NewReader() error = %q, want unsupported mode error", err.Error())
	}
}
