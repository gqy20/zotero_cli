package zoteroapi

import (
	"net/http"
	"testing"
)

func TestAPIErrorBadRequestIncludesResponseBody(t *testing.T) {
	t.Parallel()

	err := (&APIError{
		StatusCode: http.StatusBadRequest,
		Body:       "Item version must not be included in a partial update",
	}).Error()
	want := "zotero api bad request (400): Item version must not be included in a partial update"
	if err != want {
		t.Fatalf("unexpected error: got %q, want %q", err, want)
	}
}
