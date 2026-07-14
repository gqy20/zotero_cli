package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
	"zotero_cli/internal/zoteroconnector"
)

type fakeItemImportConnector struct {
	pinged     bool
	imported   bool
	request    zoteroconnector.ImportPDFRequest
	updated    zoteroconnector.UpdateSessionRequest
	recognized bool
}

func (f *fakeItemImportConnector) WaitForRecognizedItem(context.Context, string) (zoteroconnector.RecognizedItem, bool, error) {
	return zoteroconnector.RecognizedItem{}, f.recognized, nil
}

func (f *fakeItemImportConnector) UpdateSession(_ context.Context, req zoteroconnector.UpdateSessionRequest) error {
	f.updated = req
	return nil
}

type fakeItemImportCollectionResolver struct {
	target      backend.CollectionTarget
	attachments []backend.ImportedAttachment
}

func (f fakeItemImportCollectionResolver) CollectionTarget(context.Context, string) (backend.CollectionTarget, error) {
	return f.target, nil
}

func (f fakeItemImportCollectionResolver) ImportedPDFAttachments(context.Context, string) ([]backend.ImportedAttachment, error) {
	return f.attachments, nil
}

type fakeItemImportDeleteClient struct {
	deleted []string
	version int
}

type fakeItemImportIndexBuilder struct {
	request IndexBuildRequest
}

func (f *fakeItemImportIndexBuilder) Build(_ context.Context, request IndexBuildRequest) (Result, error) {
	f.request = request
	return Result{Data: IndexBuildResult{TotalItems: 1, TotalAttachments: 1, Indexed: 1}}, nil
}

func (f *fakeItemImportDeleteClient) GetLibraryVersion(context.Context) (int, error) {
	return 17, nil
}

func (f *fakeItemImportDeleteClient) DeleteItems(_ context.Context, keys []string, version int) (zoteroapi.BatchWriteResult, error) {
	f.deleted = append([]string(nil), keys...)
	f.version = version
	return zoteroapi.BatchWriteResult{}, nil
}

func (f *fakeItemImportConnector) Ping(context.Context) error {
	f.pinged = true
	return nil
}

func (f *fakeItemImportConnector) ImportPDF(_ context.Context, req zoteroconnector.ImportPDFRequest) (zoteroconnector.ImportPDFResult, error) {
	f.imported = true
	f.request = req
	return zoteroconnector.ImportPDFResult{CanRecognize: true}, nil
}

func TestItemImportDryRunDoesNotUpload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(path, []byte("%PDF-test"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &fakeItemImportConnector{}
	service := ItemImportService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowWrite: true}, "", nil },
		NewClient:  func(config.Config) ItemImportConnector { return client },
	}
	result, err := service.Import(context.Background(), ItemImportRequest{Path: path, DryRun: true})
	if err != nil {
		t.Fatalf("Import() error=%v", err)
	}
	if !client.pinged || client.imported {
		t.Fatalf("pinged=%v imported=%v", client.pinged, client.imported)
	}
	data := result.Data.(ItemImportResult)
	if !data.DryRun || data.Accepted {
		t.Fatalf("data=%+v", data)
	}
}

func TestItemImportUploadsPDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paper.PDF")
	content := "%PDF-test"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &fakeItemImportConnector{}
	service := ItemImportService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowWrite: true}, "", nil },
		NewClient:  func(config.Config) ItemImportConnector { return client },
	}
	result, err := service.Import(context.Background(), ItemImportRequest{Path: path})
	if err != nil {
		t.Fatalf("Import() error=%v", err)
	}
	data := result.Data.(ItemImportResult)
	if !data.Accepted || !data.CanRecognize || !data.RecognitionQueued {
		t.Fatalf("data=%+v", data)
	}
	if client.request.ContentLength != int64(len(content)) || client.request.SessionID == "" || !strings.HasPrefix(client.request.SourceURL, "file:") {
		t.Fatalf("request=%+v", client.request)
	}
}

func TestItemImportAssignsCollection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(path, []byte("%PDF-test"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &fakeItemImportConnector{}
	service := ItemImportService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowWrite: true}, "", nil },
		NewClient:  func(config.Config) ItemImportConnector { return client },
		NewResolver: func(config.Config) (itemImportCollectionResolver, error) {
			return fakeItemImportCollectionResolver{target: backend.CollectionTarget{ID: 23, Key: "COLLKEY", Name: "Genetics", Path: "Research/Genetics"}}, nil
		},
	}
	result, err := service.Import(context.Background(), ItemImportRequest{Path: path, Collection: "COLLKEY"})
	if err != nil {
		t.Fatalf("Import() error=%v", err)
	}
	data := result.Data.(ItemImportResult)
	if data.CollectionKey != "COLLKEY" || data.CollectionName != "Genetics" || data.CollectionPath != "Research/Genetics" || !data.CollectionAssigned {
		t.Fatalf("data=%+v", data)
	}
	if client.updated.Target != "C23" || client.updated.SessionID == "" || client.updated.SessionID != client.request.SessionID {
		t.Fatalf("import=%+v update=%+v", client.request, client.updated)
	}
}

func TestItemImportIndexesRecognizedAttachment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(path, []byte("%PDF-test"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &fakeItemImportConnector{recognized: true}
	indexer := &fakeItemImportIndexBuilder{}
	service := ItemImportService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowWrite: true}, "", nil },
		NewClient:  func(config.Config) ItemImportConnector { return client },
		NewResolver: func(config.Config) (itemImportCollectionResolver, error) {
			return fakeItemImportCollectionResolver{attachments: []backend.ImportedAttachment{{Key: "ATT123", ParentKey: "ITEM123", LinkMode: 2, Path: `D:\\papers\\paper.pdf`}}}, nil
		},
		NewIndexBuilder: func() itemImportIndexBuilder { return indexer },
		PollInterval:    time.Millisecond,
	}
	result, err := service.Import(context.Background(), ItemImportRequest{Path: path})
	if err != nil {
		t.Fatalf("Import() error=%v", err)
	}
	data := result.Data.(ItemImportResult)
	if data.ItemKey != "ITEM123" || data.AttachmentKey != "ATT123" || !data.FullTextIndexed {
		t.Fatalf("data=%+v warnings=%+v", data, result.Warnings)
	}
	if len(indexer.request.ItemKeys) != 1 || indexer.request.ItemKeys[0] != "ITEM123" || len(indexer.request.AttachmentKeys) != 1 || indexer.request.AttachmentKeys[0] != "ATT123" {
		t.Fatalf("index request=%+v", indexer.request)
	}
}

func TestItemImportRejectsNonPDFAndDisabledWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paper.txt")
	if err := os.WriteFile(path, []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := ItemImportService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowWrite: true}, "", nil },
		NewClient:  func(config.Config) ItemImportConnector { return &fakeItemImportConnector{} },
	}
	if _, err := service.Import(context.Background(), ItemImportRequest{Path: path}); err == nil || !strings.Contains(err.Error(), "only PDF") {
		t.Fatalf("non-PDF error=%v", err)
	}
	service.LoadConfig = func() (config.Config, string, error) { return config.Config{AllowWrite: false}, "", nil }
	if _, err := service.Import(context.Background(), ItemImportRequest{Path: path}); err == nil || !strings.Contains(err.Error(), "writes are disabled") {
		t.Fatalf("disabled error=%v", err)
	}
}

func TestItemImportDuplicateCleanupDeletesWithoutGlobalDeleteFlag(t *testing.T) {
	deleteClient := &fakeItemImportDeleteClient{}
	service := ItemImportService{NewDeleteClient: func(config.Config) (itemImportDeleteClient, error) { return deleteClient, nil }}
	resolver := fakeItemImportCollectionResolver{attachments: []backend.ImportedAttachment{
		{Key: "ATT2", ParentKey: "PARENT", LinkMode: 2, Path: `D:\\papers\\paper.pdf`},
		{Key: "ATT1", ParentKey: "PARENT", LinkMode: 2, Path: `D:\\papers\\paper.pdf`},
	}}
	cleanup, err := service.cleanupDuplicateAttachments(context.Background(), config.Config{}, resolver, "file:///paper.pdf")
	if err != nil {
		t.Fatalf("cleanupDuplicateAttachments() error=%v", err)
	}
	if cleanup == nil || cleanup.Kept != "ATT1" || len(cleanup.Detected) != 2 || len(cleanup.Deleted) != 1 || cleanup.Deleted[0] != "ATT2" {
		t.Fatalf("cleanup=%+v", cleanup)
	}
	if len(deleteClient.deleted) != 1 || deleteClient.deleted[0] != "ATT2" {
		t.Fatalf("delete client=%+v", deleteClient)
	}
}

func TestItemImportDuplicateCleanupDeletesOnlyExactExtra(t *testing.T) {
	deleteClient := &fakeItemImportDeleteClient{}
	service := ItemImportService{NewDeleteClient: func(config.Config) (itemImportDeleteClient, error) { return deleteClient, nil }}
	resolver := fakeItemImportCollectionResolver{attachments: []backend.ImportedAttachment{
		{Key: "ATT2", ParentKey: "PARENT", LinkMode: 2, Path: `D:\\papers\\paper.pdf`},
		{Key: "ATT1", ParentKey: "PARENT", LinkMode: 2, Path: `D:\\papers\\paper.pdf`},
		{Key: "OTHER", ParentKey: "PARENT", LinkMode: 2, Path: `D:\\papers\\other.pdf`},
		{Key: "COPIED", ParentKey: "PARENT", LinkMode: 0, Path: `D:\\papers\\paper.pdf`},
	}}
	cleanup, err := service.cleanupDuplicateAttachments(context.Background(), config.Config{}, resolver, "file:///paper.pdf")
	if err != nil {
		t.Fatalf("cleanupDuplicateAttachments() error=%v", err)
	}
	if cleanup == nil || cleanup.Kept != "ATT1" || len(cleanup.Deleted) != 1 || cleanup.Deleted[0] != "ATT2" {
		t.Fatalf("cleanup=%+v", cleanup)
	}
	if len(deleteClient.deleted) != 1 || deleteClient.deleted[0] != "ATT2" || deleteClient.version != 17 {
		t.Fatalf("delete client=%+v", deleteClient)
	}
}

func TestImportedAttachmentTargetPrefersFinalLinkedFile(t *testing.T) {
	itemKey, attachmentKey := importedAttachmentTarget([]backend.ImportedAttachment{
		{Key: "COPIED", ParentKey: "ITEM", LinkMode: 0, Path: `storage:COPIED/paper.pdf`},
		{Key: "LINKED", ParentKey: "ITEM", LinkMode: 2, Path: `D:\\papers\\paper.pdf`},
	}, nil)
	if itemKey != "ITEM" || attachmentKey != "LINKED" {
		t.Fatalf("itemKey=%q attachmentKey=%q", itemKey, attachmentKey)
	}
}
