package app

import (
	"strings"
	"testing"
)

func TestResolveExportKeysAcceptsFindEnvelopeFromStdin(t *testing.T) {
	keys, err := ResolveExportKeys("-", strings.NewReader(`{"ok":true,"data":[{"key":"ITEM0001"},{"key":"ITEM0002"},{"key":"ITEM0001"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "ITEM0001" || keys[1] != "ITEM0002" {
		t.Fatalf("keys=%v", keys)
	}
}

func TestResolveExportKeysRejectsObjectsWithoutKeys(t *testing.T) {
	if _, err := ResolveExportKeys("-", strings.NewReader(`{"data":[{"title":"missing key"}]}`)); err == nil {
		t.Fatal("expected missing-key error")
	}
}
