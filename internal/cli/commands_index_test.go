package cli

import (
	"context"
	"testing"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

type stubIndexReader struct {
	items     []domain.Item
	cached    map[string]bool
	indexed   map[string]bool
	docs      map[string]backend.FullTextDocument
	savedDocs []backend.FullTextDocument
}

func (r *stubIndexReader) FindItems(context.Context, backend.FindOptions) ([]domain.Item, error) {
	return append([]domain.Item(nil), r.items...), nil
}

func (r *stubIndexReader) GetItem(context.Context, string) (domain.Item, error) {
	return domain.Item{}, nil
}

func (r *stubIndexReader) GetRelated(context.Context, string) ([]domain.Relation, error) {
	return nil, nil
}

func (r *stubIndexReader) GetLibraryStats(context.Context) (backend.LibraryStats, error) {
	return backend.LibraryStats{}, nil
}

func (r *stubIndexReader) ListNotes(context.Context) ([]domain.Note, error) {
	return nil, nil
}

func (r *stubIndexReader) ListTags(context.Context) ([]backend.Tag, error) {
	return nil, nil
}

func (r *stubIndexReader) ListCollections(context.Context) ([]backend.Collection, error) {
	return nil, nil
}

func (r *stubIndexReader) GetAttachmentFile(context.Context, string) (string, string, error) {
	return "", "", nil
}

func (r *stubIndexReader) ExtractAttachmentFullTextOnly(_ context.Context, _ domain.Item, att domain.Attachment) (backend.FullTextDocument, bool, error) {
	doc, ok := r.docs[att.Key]
	return doc, ok, nil
}

func (r *stubIndexReader) SaveFullText(doc backend.FullTextDocument) error {
	r.savedDocs = append(r.savedDocs, doc)
	return nil
}

func (r *stubIndexReader) SaveFullTextBatch(docs []backend.FullTextDocument) error {
	r.savedDocs = append(r.savedDocs, docs...)
	return nil
}

func (r *stubIndexReader) IsFullTextCached(att domain.Attachment) bool {
	return r.cached[att.Key]
}

func (r *stubIndexReader) IsFullTextIndexed(att domain.Attachment) bool {
	return r.indexed[att.Key]
}

func (r *stubIndexReader) IsMarkedFailed(string) bool {
	return false
}

func TestIndexBuildRepairsCachedButUnindexedAttachments(t *testing.T) {
	reader := &stubIndexReader{
		items: []domain.Item{{
			Key: "ITEM123",
			Attachments: []domain.Attachment{{
				Key:         "ATT123",
				ContentType: "application/pdf",
				Resolved:    true,
			}},
		}},
		cached:  map[string]bool{"ATT123": true},
		indexed: map[string]bool{"ATT123": false},
		docs: map[string]backend.FullTextDocument{
			"ATT123": {Text: "cached text", CacheHit: true},
		},
	}

	result, err := (&CLI{}).indexBuild(context.Background(), reader, indexBuildOpts{Workers: 1, JSONOutput: true})
	if err != nil {
		t.Fatalf("indexBuild() error = %v", err)
	}
	if result.Indexed != 1 || result.Skipped != 0 || result.Failed != 0 {
		t.Fatalf("indexBuild() result = %#v, want indexed repair", result)
	}
	if len(reader.savedDocs) != 1 || reader.savedDocs[0].Text != "cached text" {
		t.Fatalf("savedDocs = %#v, want cached doc saved to index", reader.savedDocs)
	}
}

func TestIndexBuildSkipsCachedAndIndexedAttachments(t *testing.T) {
	reader := &stubIndexReader{
		items: []domain.Item{{
			Key: "ITEM123",
			Attachments: []domain.Attachment{{
				Key:         "ATT123",
				ContentType: "application/pdf",
				Resolved:    true,
			}},
		}},
		cached:  map[string]bool{"ATT123": true},
		indexed: map[string]bool{"ATT123": true},
	}

	result, err := (&CLI{}).indexBuild(context.Background(), reader, indexBuildOpts{Workers: 1, JSONOutput: true})
	if err != nil {
		t.Fatalf("indexBuild() error = %v", err)
	}
	if result.Indexed != 0 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("indexBuild() result = %#v, want skip", result)
	}
	if len(reader.savedDocs) != 0 {
		t.Fatalf("savedDocs = %#v, want none", reader.savedDocs)
	}
}
