package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "debug")

	logger.Debug("debug msg", "key", "val")
	logger.Info("info msg", "count", 42)
	logger.Warn("warn msg", "reason", "slow")
	logger.Error("error msg", "err", "not found")

	output := buf.String()

	if !strings.Contains(output, `"level":"DEBUG"`) {
		t.Errorf("expected DEBUG level in output, got: %s", output)
	}
	if !strings.Contains(output, `"level":"INFO"`) {
		t.Errorf("expected INFO level in output, got: %s", output)
	}
	if !strings.Contains(output, `"level":"WARN"`) {
		t.Errorf("expected WARN level in output, got: %s", output)
	}
	if !strings.Contains(output, `"level":"ERROR"`) {
		t.Errorf("expected ERROR level in output, got: %s", output)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "info")

	logger.Debug("should not appear")
	logger.Info("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Error("debug message should be filtered at info level")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("info message should be present at info level")
	}
}

func TestLoggerStructuredOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "debug")

	logger.Info("item fetched", "key", "ABC123", "type", "journalArticle")

	var entry map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(buf.Bytes()))), &entry); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
	if entry["msg"] != "item fetched" {
		t.Errorf("unexpected msg: %v", entry["msg"])
	}
	if entry["key"] != "ABC123" {
		t.Errorf("unexpected key: %v", entry["key"])
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		got := parseLogLevel(tt.input)
		if got != tt.expected {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	srv := NewMockServerWithReader()

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	requestID := rec.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Fatal("expected X-Request-Id header to be set")
	}
	if len(requestID) < 8 {
		t.Errorf("request ID too short: %q", requestID)
	}
}

func TestEnhancedAccessLog(t *testing.T) {
	var logBuf bytes.Buffer
	srv := NewMockServerWithCustomLog(&logBuf)

	req := httptest.NewRequest("GET", "/api/v1/stats?limit=10", nil)
	req.Header.Set("User-Agent", "test-agent/1.0")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, `"method":"GET"`) {
		t.Errorf("log should contain method, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"path":"/api/v1/stats"`) {
		t.Errorf("log should contain path, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"status":200`) {
		t.Errorf("log should contain status, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"duration_ms"`) {
		t.Errorf("log should contain duration_ms, got: %s", logOutput)
	}
}
