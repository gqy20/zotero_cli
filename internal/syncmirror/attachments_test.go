package syncmirror

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOnlyAcceptsCurrentAttachmentsTree(t *testing.T) {
	dataDir := t.TempDir()
	current := filepath.Join(dataDir, AttachmentsDir, "papers", "paper.pdf")
	if err := os.MkdirAll(filepath.Dir(current), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(current, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, ok := Resolve(dataDir, AttachmentEntry{RelativePath: "attachments/papers/paper.pdf"})
	if !ok || filepath.Clean(resolved) != filepath.Clean(current) {
		t.Fatalf("Resolve() = %q, %v; want %q, true", resolved, ok, current)
	}
	if resolved, ok := Resolve(dataDir, AttachmentEntry{RelativePath: ".zotero_cli/linked/LINK1/paper.pdf"}); ok {
		t.Fatalf("obsolete key mirror unexpectedly resolved to %q", resolved)
	}
}
