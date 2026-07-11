package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

func TestResolvePayloadSupportsSetAndStdin(t *testing.T) {
	setPayload, err := ResolvePayload(PayloadInput{Set: []string{"title=Paper", "year=2026", "open=true"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if setPayload["title"] != "Paper" || setPayload["year"] != int64(2026) || setPayload["open"] != true {
		t.Fatalf("set payload = %#v", setPayload)
	}
	stdinPayload, err := ResolvePayload(PayloadInput{From: "-"}, strings.NewReader(`{"name":"Inbox"}`))
	if err != nil {
		t.Fatal(err)
	}
	if stdinPayload["name"] != "Inbox" {
		t.Fatalf("stdin payload = %#v", stdinPayload)
	}
}

func TestResolvePayloadRejectsAmbiguousSources(t *testing.T) {
	_, err := ResolvePayload(PayloadInput{Data: `{}`, Set: []string{"name=x"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteServiceEnforcesGateBeforeClientCreation(t *testing.T) {
	created := false
	service := WriteService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowWrite: false}, "", nil },
		NewClient: func(config.Config) (WriteClient, error) {
			created = true
			return &fakeWriteClient{}, nil
		},
	}
	_, err := service.Create(context.Background(), "item", ObjectWriteRequest{Payload: map[string]any{"itemType": "book"}, Safety: SafetyOptions{IfVersion: 1}})
	if err == nil || !strings.Contains(err.Error(), "writes are disabled") {
		t.Fatalf("error = %v", err)
	}
	if created {
		t.Fatal("write client was created before the write gate")
	}
}

func TestWriteServiceDryRunDoesNotCreateClient(t *testing.T) {
	created := false
	service := WriteService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowWrite: true}, "", nil },
		NewClient: func(config.Config) (WriteClient, error) {
			created = true
			return &fakeWriteClient{}, nil
		},
	}
	result, err := service.Create(context.Background(), "item", ObjectWriteRequest{Payload: map[string]any{"itemType": "book"}, Safety: SafetyOptions{DryRun: true}})
	if err != nil {
		t.Fatal(err)
	}
	if created || result.Meta["dry_run"] != true {
		t.Fatalf("created=%t result=%#v", created, result)
	}
}

func TestWriteServiceDeleteRequiresConfirmation(t *testing.T) {
	client := &fakeWriteClient{stats: zoteroapi.LibraryStats{LastLibraryVersion: 7}}
	service := WriteService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowDelete: true}, "", nil },
		NewClient:  func(config.Config) (WriteClient, error) { return client, nil },
	}
	_, err := service.Delete(context.Background(), "item", ObjectWriteRequest{Keys: []string{"A"}, Safety: SafetyOptions{Confirm: func(string) bool { return false }}})
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("error = %v", err)
	}
	if client.deleted != nil {
		t.Fatalf("delete called with %v", client.deleted)
	}
}

func TestUpdateStringSet(t *testing.T) {
	if got := updateStringSet([]string{"a"}, "b", true); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("add = %v", got)
	}
	if got := updateStringSet([]string{"a", "b"}, "a", false); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("remove = %v", got)
	}
}

type fakeWriteClient struct {
	stats   zoteroapi.LibraryStats
	deleted []string
}

func (f *fakeWriteClient) GetLibraryStats(context.Context) (zoteroapi.LibraryStats, error) {
	return f.stats, nil
}
func (f *fakeWriteClient) CreateItem(context.Context, map[string]any, int) (zoteroapi.WriteResult, error) {
	return zoteroapi.WriteResult{Key: "I"}, nil
}
func (f *fakeWriteClient) UpdateItem(context.Context, string, map[string]any, int) (zoteroapi.WriteResult, error) {
	return zoteroapi.WriteResult{}, nil
}
func (f *fakeWriteClient) DeleteItems(_ context.Context, keys []string, _ int) (zoteroapi.BatchWriteResult, error) {
	f.deleted = keys
	return zoteroapi.BatchWriteResult{}, nil
}
func (f *fakeWriteClient) GetItemsByKeys(context.Context, []string) ([]zoteroapi.Item, error) {
	return nil, nil
}
func (f *fakeWriteClient) UpdateItems(context.Context, []map[string]any, int) (zoteroapi.BatchWriteResult, error) {
	return zoteroapi.BatchWriteResult{}, nil
}
func (f *fakeWriteClient) CreateCollection(context.Context, map[string]any, int) (zoteroapi.WriteResult, error) {
	return zoteroapi.WriteResult{}, nil
}
func (f *fakeWriteClient) UpdateCollection(context.Context, string, map[string]any, int) (zoteroapi.WriteResult, error) {
	return zoteroapi.WriteResult{}, nil
}
func (f *fakeWriteClient) DeleteCollection(context.Context, string, int) (zoteroapi.WriteResult, error) {
	return zoteroapi.WriteResult{}, nil
}
func (f *fakeWriteClient) CreateSearch(context.Context, map[string]any, int) (zoteroapi.WriteResult, error) {
	return zoteroapi.WriteResult{}, nil
}
func (f *fakeWriteClient) UpdateSearch(context.Context, string, map[string]any, int) (zoteroapi.WriteResult, error) {
	return zoteroapi.WriteResult{}, nil
}
func (f *fakeWriteClient) DeleteSearch(context.Context, string, int) (zoteroapi.WriteResult, error) {
	return zoteroapi.WriteResult{}, nil
}
