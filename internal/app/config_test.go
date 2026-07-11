package app

import (
	"testing"

	"zotero_cli/internal/config"
)

func TestMaskConfigKeepsOnlySecretSuffix(t *testing.T) {
	masked := MaskConfig(config.Config{APIKey: "abcdefgh", ServerAuthKey: "xy", LibraryID: "123"})
	if got := masked["api_key"]; got != "****efgh" {
		t.Fatalf("api_key = %#v", got)
	}
	if got := masked["server_auth_key"]; got != "****" {
		t.Fatalf("server_auth_key = %#v", got)
	}
	if got := masked["library_id"]; got != "123" {
		t.Fatalf("library_id = %#v", got)
	}
}

func TestCommandPathString(t *testing.T) {
	if got := (CommandPath{Resource: "config", Action: "check"}).String(); got != "config check" {
		t.Fatalf("path = %q", got)
	}
	if got := (CommandPath{Resource: "version"}).String(); got != "version" {
		t.Fatalf("path = %q", got)
	}
}
