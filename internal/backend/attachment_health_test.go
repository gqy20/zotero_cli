package backend

import (
	"os"
	"path/filepath"
	"testing"

	"zotero_cli/internal/domain"
)

func TestInspectAttachmentHealthReportsMissingAndBadName(t *testing.T) {
	attachment := domain.Attachment{
		Key:         "ATT1",
		ContentType: "application/pdf",
		Filename:    "download",
		ZoteroPath:  "storage:download",
	}

	health := InspectAttachmentHealth(attachment)
	if health.OK {
		t.Fatalf("expected unhealthy attachment: %#v", health)
	}
	if health.Status != "error" {
		t.Fatalf("status = %q, want error", health.Status)
	}
	if !AttachmentHasMissingFile(attachment) {
		t.Fatalf("expected missing-file helper to match")
	}
	if !AttachmentHasBadName(attachment) {
		t.Fatalf("expected bad-name helper to match")
	}
	if !AttachmentHealthMatches(attachment, "warning") {
		t.Fatalf("expected warning-or-worse health match")
	}
}

func TestInspectAttachmentHealthOKForResolvedPDF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(path, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}

	health := InspectAttachmentHealth(domain.Attachment{
		Key:          "ATT1",
		ContentType:  "application/pdf",
		Filename:     "paper.pdf",
		ResolvedPath: path,
		Resolved:     true,
	})
	if !health.OK || health.Status != "ok" || len(health.Issues) != 0 {
		t.Fatalf("unexpected health: %#v", health)
	}
}
