package server

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	loggerKey    contextKey = "logger"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func requestIDMiddleware(baseLogger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = uuid.New().String()[:12]
			}
			w.Header().Set("X-Request-Id", requestID)

			reqLogger := baseLogger.With("request_id", requestID)
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			ctx = context.WithValue(ctx, loggerKey, reqLogger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func loggingMiddleware(baseLogger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			duration := time.Since(start)

			logger := getRequestLogger(r.Context(), baseLogger)
			logger.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				"status", rw.status,
				"duration_ms", duration.Milliseconds(),
				"user_agent", r.UserAgent(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

func recoverMiddleware(baseLogger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rv := recover(); rv != nil {
					logger := getRequestLogger(r.Context(), baseLogger)
					logger.Error("panic recovered",
						"panic", formatPanic(rv),
						"path", r.URL.Path,
						"method", r.Method,
					)
					writeError(w, http.StatusInternalServerError, nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func GetLogger(ctx context.Context) *Logger {
	if l, ok := ctx.Value(loggerKey).(*Logger); ok {
		return l
	}
	return DefaultLogger()
}

func getRequestLogger(ctx context.Context, fallback *Logger) *Logger {
	if l, ok := ctx.Value(loggerKey).(*Logger); ok {
		return l
	}
	return fallback
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func formatPanic(v any) string {
	if err, ok := v.(error); ok {
		return err.Error()
	}
	b, _ := v.([]byte)
	if b != nil {
		return string(b)
	}
	s, _ := v.(string)
	if s != "" {
		return s
	}
	return "unknown panic"
}
