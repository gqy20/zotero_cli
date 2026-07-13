package app

import (
	"context"
	"errors"
	"reflect"
	"regexp"
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
	stats         zoteroapi.LibraryStats
	deleted       []string
	items         []zoteroapi.Item
	tags          []zoteroapi.Tag
	findResults   map[string][]zoteroapi.Item
	itemsByKey    map[string]zoteroapi.Item
	stateful      bool
	updatePayload []map[string]any
	updateResult  zoteroapi.BatchWriteResult
}

func (f *fakeWriteClient) GetLibraryStats(context.Context) (zoteroapi.LibraryStats, error) {
	return f.stats, nil
}
func (f *fakeWriteClient) ListTags(context.Context) ([]zoteroapi.Tag, error) {
	return f.tags, nil
}
func (f *fakeWriteClient) FindItems(_ context.Context, opts zoteroapi.FindOptions) ([]zoteroapi.Item, error) {
	if f.findResults != nil {
		return f.findResults[opts.Tag], nil
	}
	return f.items, nil
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
func (f *fakeWriteClient) GetItemsByKeys(_ context.Context, keys []string) ([]zoteroapi.Item, error) {
	if f.itemsByKey == nil {
		return f.items, nil
	}
	items := make([]zoteroapi.Item, 0, len(keys))
	for _, key := range keys {
		if item, ok := f.itemsByKey[key]; ok {
			items = append(items, item)
		}
	}
	return items, nil
}
func (f *fakeWriteClient) UpdateItems(_ context.Context, payload []map[string]any, _ int) (zoteroapi.BatchWriteResult, error) {
	f.updatePayload = payload
	if f.stateful {
		result := zoteroapi.BatchWriteResult{LastModifiedVersion: 8}
		for _, row := range payload {
			key := row["key"].(string)
			item := f.itemsByKey[key]
			item.TagObjects = append([]zoteroapi.ItemTag(nil), row["tags"].([]zoteroapi.ItemTag)...)
			item.Tags = make([]string, 0, len(item.TagObjects))
			for _, tag := range item.TagObjects {
				item.Tags = append(item.Tags, tag.Tag)
			}
			item.Version = 8
			f.itemsByKey[key] = item
			result.Successful = append(result.Successful, zoteroapi.WriteResult{Key: key, LastModifiedVersion: 8})
		}
		return result, nil
	}
	return f.updateResult, nil
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

func TestWriteServiceTagsPreservesAutomaticTagTypes(t *testing.T) {
	automatic := 1
	client := &fakeWriteClient{
		stats: zoteroapi.LibraryStats{LastLibraryVersion: 7},
		items: []zoteroapi.Item{{
			Key:     "ITEMA001",
			Version: 6,
			Tags:    []string{"old", "keep"},
			TagObjects: []zoteroapi.ItemTag{
				{Tag: "old", Type: &automatic},
				{Tag: "keep", Type: &automatic},
			},
		}},
		updateResult: zoteroapi.BatchWriteResult{
			Successful:          []zoteroapi.WriteResult{{Key: "ITEMA001", LastModifiedVersion: 8}},
			LastModifiedVersion: 8,
		},
	}
	service := WriteService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowWrite: true}, "", nil },
		NewClient:  func(config.Config) (WriteClient, error) { return client, nil },
	}

	if _, err := service.Tags(context.Background(), TagWriteRequest{Keys: []string{"ITEMA001"}, Tag: "old"}); err != nil {
		t.Fatal(err)
	}
	tags, ok := client.updatePayload[0]["tags"].([]zoteroapi.ItemTag)
	if !ok {
		t.Fatalf("tag payload type = %T, want []zoteroapi.ItemTag", client.updatePayload[0]["tags"])
	}
	if len(tags) != 1 || tags[0].Tag != "keep" || tags[0].Type == nil || *tags[0].Type != 1 {
		t.Fatalf("tag payload = %#v", tags)
	}
}

func TestWriteServiceTagsRejectsPartialBatchFailure(t *testing.T) {
	client := &fakeWriteClient{
		stats: zoteroapi.LibraryStats{LastLibraryVersion: 7},
		items: []zoteroapi.Item{{Key: "ITEMA001", Version: 6}},
		updateResult: zoteroapi.BatchWriteResult{
			Failed: map[string]any{"0": map[string]any{"key": "ITEMA001", "message": "rejected"}},
		},
	}
	service := WriteService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowWrite: true}, "", nil },
		NewClient:  func(config.Config) (WriteClient, error) { return client, nil },
	}

	_, err := service.Tags(context.Background(), TagWriteRequest{Keys: []string{"ITEMA001"}, Tag: "new", Add: true})
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestReplaceItemTagsTransformsAndDeduplicates(t *testing.T) {
	automatic := 1
	item := zoteroapi.Item{
		Tags: []string{"SV", "keep", "结构变异"},
		TagObjects: []zoteroapi.ItemTag{
			{Tag: "SV", Type: &automatic},
			{Tag: "keep", Type: &automatic},
			{Tag: "结构变异"},
		},
	}

	tags, changed, err := replaceItemTags(item, regexp.MustCompile(`^SV$`), "结构变异")
	if err != nil {
		t.Fatal(err)
	}
	if !changed || len(tags) != 2 || tags[0].Tag != "结构变异" || tags[0].Type != nil {
		t.Fatalf("tags = %#v, changed=%t", tags, changed)
	}
	if tags[1].Tag != "keep" || tags[1].Type == nil || *tags[1].Type != 1 {
		t.Fatalf("untouched automatic tag was not preserved: %#v", tags[1])
	}
}

func TestReplaceItemTagsRejectsEmptyResult(t *testing.T) {
	_, _, err := replaceItemTags(zoteroapi.Item{Tags: []string{"remove"}}, regexp.MustCompile(`^remove$`), "")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteServiceReplaceTagsDefaultsToPreview(t *testing.T) {
	client := &fakeWriteClient{tags: []zoteroapi.Tag{
		{Name: "SV", NumItems: 7},
		{Name: "SV检测", NumItems: 6},
		{Name: "keep", NumItems: 2},
	}}
	service := WriteService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{}, "", nil },
		NewClient:  func(config.Config) (WriteClient, error) { return client, nil },
	}

	result, err := service.ReplaceTags(context.Background(), TagReplaceRequest{Match: `^SV(检测)?$`, Replace: `结构变异$1`})
	if err != nil {
		t.Fatal(err)
	}
	report := result.Data.(TagReplaceReport)
	if report.Applied || report.MatchedTags != 2 || report.MatchedAssignments != 13 || client.updatePayload != nil {
		t.Fatalf("report = %#v payload=%#v", report, client.updatePayload)
	}
	if result.Meta["preview"] != true {
		t.Fatalf("meta = %#v", result.Meta)
	}
}

func TestWriteServiceReplaceTagsAppliesAndVerifies(t *testing.T) {
	automatic := 1
	item := zoteroapi.Item{
		Key: "ITEMA001", Version: 6, Tags: []string{"SV", "keep"},
		TagObjects: []zoteroapi.ItemTag{{Tag: "SV", Type: &automatic}, {Tag: "keep", Type: &automatic}},
	}
	client := &fakeWriteClient{
		stats:       zoteroapi.LibraryStats{LastLibraryVersion: 7},
		tags:        []zoteroapi.Tag{{Name: "SV", NumItems: 1}},
		findResults: map[string][]zoteroapi.Item{"SV": {item}},
		itemsByKey:  map[string]zoteroapi.Item{"ITEMA001": item},
		stateful:    true,
	}
	service := WriteService{
		LoadConfig: func() (config.Config, string, error) { return config.Config{AllowWrite: true}, "", nil },
		NewClient:  func(config.Config) (WriteClient, error) { return client, nil },
	}

	result, err := service.ReplaceTags(context.Background(), TagReplaceRequest{
		Match: `^SV$`, Replace: "结构变异", Safety: SafetyOptions{Yes: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	report := result.Data.(TagReplaceReport)
	if !report.Applied || report.UpdatedItems != 1 || report.VerifiedItems != 1 {
		t.Fatalf("report = %#v", report)
	}
	got := client.itemsByKey["ITEMA001"].TagObjects
	if len(got) != 2 || got[0].Tag != "结构变异" || got[0].Type != nil || got[1].Type == nil || *got[1].Type != 1 {
		t.Fatalf("tags = %#v", got)
	}
}
