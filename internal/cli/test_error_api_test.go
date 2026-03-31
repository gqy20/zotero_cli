package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type errorAPIServer struct {
	url     string
	cleanup func()
}

func newErrorAPI(t *testing.T, status int, retryAfter string) errorAPIServer {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		http.Error(w, http.StatusText(status), status)
	}))

	return errorAPIServer{
		url:     server.URL,
		cleanup: server.Close,
	}
}

func newErrorAPIWithBody(t *testing.T, status int, retryAfter string, body string) errorAPIServer {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		http.Error(w, body, status)
	}))

	return errorAPIServer{
		url:     server.URL,
		cleanup: server.Close,
	}
}
