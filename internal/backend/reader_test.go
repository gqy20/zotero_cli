package backend

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

type stubReader struct {
	findItems       func(context.Context, FindOptions) ([]domain.Item, error)
	getItem         func(context.Context, string) (domain.Item, error)
	getRelated      func(context.Context, string) ([]domain.Relation, error)
	getLibraryStats func(context.Context) (LibraryStats, error)
}

func (r stubReader) FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	return r.findItems(ctx, opts)
}

func (r stubReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	return r.getItem(ctx, key)
}

func (r stubReader) GetRelated(ctx context.Context, key string) ([]domain.Relation, error) {
	return r.getRelated(ctx, key)
}

func (r stubReader) GetLibraryStats(ctx context.Context) (LibraryStats, error) {
	return r.getLibraryStats(ctx)
}

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

func TestNewReaderLocalModeBuildsLocalReader(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "zotero.sqlite"), []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "storage"), 0o755); err != nil {
		t.Fatal(err)
	}

	reader, err := NewReader(config.Config{Mode: "local", DataDir: root}, nil)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if _, ok := reader.(*LocalReader); !ok {
		t.Fatalf("NewReader() returned %T, want *LocalReader", reader)
	}
}

func TestNewReaderHybridModeBuildsHybridReader(t *testing.T) {
	reader, err := NewReader(config.Config{Mode: "hybrid"}, nil)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if _, ok := reader.(*HybridReader); !ok {
		t.Fatalf("NewReader() returned %T, want *HybridReader", reader)
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

func TestHybridReaderFindItemsPrefersLocal(t *testing.T) {
	want := []domain.Item{{Key: "LOCAL1"}}
	reader := &HybridReader{
		local: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) {
				return want, nil
			},
			getItem: func(context.Context, string) (domain.Item, error) {
				return domain.Item{}, errors.New("unexpected")
			},
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("unexpected")
			},
		},
		web: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) {
				return nil, errors.New("web should not be used")
			},
			getItem: func(context.Context, string) (domain.Item, error) {
				return domain.Item{}, errors.New("unexpected")
			},
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("unexpected")
			},
		},
	}

	got, err := reader.FindItems(context.Background(), FindOptions{Query: "test"})
	if err != nil {
		t.Fatalf("FindItems() error = %v", err)
	}
	if len(got) != 1 || got[0].Key != "LOCAL1" {
		t.Fatalf("FindItems() = %#v, want local result", got)
	}
}

func TestHybridReaderFindItemsFallsBackToWeb(t *testing.T) {
	reader := &HybridReader{
		local: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) {
				return nil, newUnsupportedFeatureError("local", "find --qmode")
			},
			getItem: func(context.Context, string) (domain.Item, error) {
				return domain.Item{}, errors.New("unexpected")
			},
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("unexpected")
			},
		},
		web: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) {
				return []domain.Item{{Key: "WEB1"}}, nil
			},
			getItem: func(context.Context, string) (domain.Item, error) {
				return domain.Item{}, errors.New("unexpected")
			},
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("unexpected")
			},
		},
	}

	got, err := reader.FindItems(context.Background(), FindOptions{Query: "test"})
	if err != nil {
		t.Fatalf("FindItems() error = %v", err)
	}
	if len(got) != 1 || got[0].Key != "WEB1" {
		t.Fatalf("FindItems() = %#v, want web fallback", got)
	}
}

func TestHybridReaderGetItemFallsBackToWeb(t *testing.T) {
	reader := &HybridReader{
		local: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) {
				return nil, errors.New("unexpected")
			},
			getItem: func(context.Context, string) (domain.Item, error) {
				return domain.Item{}, newItemNotFoundError("item", "X")
			},
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("unexpected")
			},
		},
		web: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) {
				return nil, errors.New("unexpected")
			},
			getItem: func(ctx context.Context, key string) (domain.Item, error) {
				return domain.Item{Key: key}, nil
			},
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("unexpected")
			},
		},
	}

	got, err := reader.GetItem(context.Background(), "WEBKEY")
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}
	if got.Key != "WEBKEY" {
		t.Fatalf("GetItem() = %#v, want web fallback item", got)
	}
}

func TestHybridReaderFindItemsDoesNotHideUnexpectedLocalError(t *testing.T) {
	reader := &HybridReader{
		local: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) {
				return nil, errors.New("sqlite corrupted")
			},
			getItem: func(context.Context, string) (domain.Item, error) {
				return domain.Item{}, errors.New("unexpected")
			},
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("unexpected")
			},
		},
		web: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) {
				return []domain.Item{{Key: "WEB1"}}, nil
			},
			getItem: func(context.Context, string) (domain.Item, error) {
				return domain.Item{}, errors.New("unexpected")
			},
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("unexpected")
			},
		},
	}

	_, err := reader.FindItems(context.Background(), FindOptions{Query: "test"})
	if err == nil || err.Error() != "sqlite corrupted" {
		t.Fatalf("FindItems() error = %v, want local error", err)
	}
}

func TestHybridReaderFindItemsFallsBackForUnsupportedLocalFlags(t *testing.T) {
	reader := &HybridReader{
		local: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) {
				return nil, newUnsupportedFeatureError("local", "find --qmode")
			},
			getItem: func(context.Context, string) (domain.Item, error) {
				return domain.Item{}, errors.New("unexpected")
			},
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("unexpected")
			},
		},
		web: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) {
				return []domain.Item{{Key: "WEB1"}}, nil
			},
			getItem: func(context.Context, string) (domain.Item, error) {
				return domain.Item{}, errors.New("unexpected")
			},
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("unexpected")
			},
		},
	}

	got, err := reader.FindItems(context.Background(), FindOptions{Query: "test", QMode: "everything"})
	if err != nil {
		t.Fatalf("FindItems() error = %v", err)
	}
	if len(got) != 1 || got[0].Key != "WEB1" {
		t.Fatalf("FindItems() = %#v, want web fallback", got)
	}
}

func TestHybridReaderGetRelatedPrefersLocal(t *testing.T) {
	reader := &HybridReader{
		local: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) { return nil, errors.New("unexpected") },
			getItem:   func(context.Context, string) (domain.Item, error) { return domain.Item{}, errors.New("unexpected") },
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return []domain.Relation{{Predicate: "dc:relation", Direction: "outgoing", Target: domain.ItemRef{Key: "LOCAL1"}}}, nil
			},
		},
		web: stubReader{
			findItems: func(context.Context, FindOptions) ([]domain.Item, error) { return nil, errors.New("unexpected") },
			getItem:   func(context.Context, string) (domain.Item, error) { return domain.Item{}, errors.New("unexpected") },
			getRelated: func(context.Context, string) ([]domain.Relation, error) {
				return nil, errors.New("web should not be used")
			},
		},
	}

	got, err := reader.GetRelated(context.Background(), "ITEM1")
	if err != nil {
		t.Fatalf("GetRelated() error = %v", err)
	}
	if len(got) != 1 || got[0].Target.Key != "LOCAL1" {
		t.Fatalf("GetRelated() = %#v, want local result", got)
	}
}

func TestHybridReaderGetLibraryStatsPrefersLocal(t *testing.T) {
	reader := &HybridReader{
		local: stubReader{
			findItems:  func(context.Context, FindOptions) ([]domain.Item, error) { return nil, errors.New("unexpected") },
			getItem:    func(context.Context, string) (domain.Item, error) { return domain.Item{}, errors.New("unexpected") },
			getRelated: func(context.Context, string) ([]domain.Relation, error) { return nil, errors.New("unexpected") },
			getLibraryStats: func(context.Context) (LibraryStats, error) {
				return LibraryStats{LibraryType: "user", LibraryID: "123", TotalItems: 3}, nil
			},
		},
		web: stubReader{
			findItems:  func(context.Context, FindOptions) ([]domain.Item, error) { return nil, errors.New("unexpected") },
			getItem:    func(context.Context, string) (domain.Item, error) { return domain.Item{}, errors.New("unexpected") },
			getRelated: func(context.Context, string) ([]domain.Relation, error) { return nil, errors.New("unexpected") },
			getLibraryStats: func(context.Context) (LibraryStats, error) {
				return LibraryStats{}, errors.New("web should not be used")
			},
		},
	}

	got, err := reader.GetLibraryStats(context.Background())
	if err != nil {
		t.Fatalf("GetLibraryStats() error = %v", err)
	}
	if got.TotalItems != 3 || got.LibraryID != "123" {
		t.Fatalf("GetLibraryStats() = %#v, want local result", got)
	}
}

func TestHybridReaderGetLibraryStatsFallsBackToWeb(t *testing.T) {
	reader := &HybridReader{
		local: stubReader{
			findItems:  func(context.Context, FindOptions) ([]domain.Item, error) { return nil, errors.New("unexpected") },
			getItem:    func(context.Context, string) (domain.Item, error) { return domain.Item{}, errors.New("unexpected") },
			getRelated: func(context.Context, string) ([]domain.Relation, error) { return nil, errors.New("unexpected") },
			getLibraryStats: func(context.Context) (LibraryStats, error) {
				return LibraryStats{}, newUnsupportedFeatureError("local", "stats")
			},
		},
		web: stubReader{
			findItems:  func(context.Context, FindOptions) ([]domain.Item, error) { return nil, errors.New("unexpected") },
			getItem:    func(context.Context, string) (domain.Item, error) { return domain.Item{}, errors.New("unexpected") },
			getRelated: func(context.Context, string) ([]domain.Relation, error) { return nil, errors.New("unexpected") },
			getLibraryStats: func(context.Context) (LibraryStats, error) {
				return LibraryStats{LibraryType: "user", LibraryID: "123", TotalItems: 9}, nil
			},
		},
	}

	got, err := reader.GetLibraryStats(context.Background())
	if err != nil {
		t.Fatalf("GetLibraryStats() error = %v", err)
	}
	if got.TotalItems != 9 {
		t.Fatalf("GetLibraryStats() = %#v, want web fallback result", got)
	}
}
