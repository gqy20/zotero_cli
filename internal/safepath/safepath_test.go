package safepath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJoinRelative(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "cache", "KEY", "content.txt")
	got, err := JoinRelative(root, "cache/KEY/content.txt")
	if err != nil || got != want {
		t.Fatalf("JoinRelative() = %q, %v; want %q, nil", got, err, want)
	}
	for _, value := range []string{"", ".", "..", "../outside", `..\outside`, "/absolute", `C:\absolute`} {
		if _, err := JoinRelative(root, value); err == nil {
			t.Errorf("JoinRelative(%q) unexpectedly succeeded", value)
		}
	}
}

func TestJoinComponentsRejectsSeparators(t *testing.T) {
	root := t.TempDir()
	if _, err := JoinComponents(root, "KEY", "paper.pdf"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"../paper.pdf", `..\paper.pdf`, "folder/paper.pdf", ""} {
		if _, err := JoinComponents(root, "KEY", value); err == nil {
			t.Errorf("JoinComponents(%q) unexpectedly succeeded", value)
		}
	}
}

func TestExistingRegularFileWithinRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside.txt")
	if err := os.WriteFile(inside, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !ExistingRegularFileWithin(root, inside) {
		t.Fatal("regular file inside root was rejected")
	}

	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if ExistingRegularFileWithin(root, link) {
		t.Fatal("symlink escaping root was accepted")
	}
}
