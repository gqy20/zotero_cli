package server

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

type Logger struct {
	*slog.Logger
}

func NewLogger(w io.Writer, level string) *Logger {
	lvl := parseLogLevel(level)
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				level := a.Value.String()
				a.Value = slog.StringValue(strings.ToUpper(level))
			}
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02T15:04:05.000"))
			}
			return a
		},
	})
	return &Logger{slog.New(handler)}
}

func DefaultLogger() *Logger {
	return NewLogger(os.Stdout, "info")
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
