package server

import (
	"context"
	"net/http"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

type mockReader struct{}

func (m *mockReader) FindItems(ctx context.Context, opts backend.FindOptions) ([]domain.Item, error) {
	return []domain.Item{}, nil
}

func (m *mockReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	return domain.Item{}, backend.ErrItemNotFound
}

func (m *mockReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	return nil, nil
}

func (m *mockReader) GetLibraryStats(ctx context.Context) (backend.LibraryStats, error) {
	return backend.LibraryStats{
		LibraryType:      "user",
		LibraryID:        "test",
		TotalItems:       0,
		TotalCollections: 0,
		TotalSearches:    0,
	}, nil
}

func (m *mockReader) ListNotes(ctx context.Context) ([]domain.Note, error) {
	return []domain.Note{}, nil
}

func (m *mockReader) ListTags(ctx context.Context) ([]backend.Tag, error) {
	return []backend.Tag{}, nil
}

func (m *mockReader) ListCollections(ctx context.Context) ([]backend.Collection, error) {
	return []backend.Collection{}, nil
}

func NewMockServer() http.Handler {
	mux := http.NewServeMux()
	h := NewHandler(&mockReader{})
	h.RegisterRoutes(mux)
	return corsMiddleware(recoverMiddleware(mux))
}

func NewMockServerWithReader() http.Handler {
	return NewMockServer()
}
