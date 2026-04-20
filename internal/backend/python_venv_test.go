package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPythonVenvDir(t *testing.T) {
	tests := []struct {
		dataDir string
		want    string
	}{
		{`D:\zotero`, filepath.Join(`D:\zotero`, ".zotero_cli", "venv")},
		{"/opt/zotero", filepath.Join("/opt/zotero", ".zotero_cli", "venv")},
		{"C:\\Users\\test\\data", filepath.Join("C:\\Users\\test\\data", ".zotero_cli", "venv")},
	}
	for _, tt := range tests {
		t.Run(tt.dataDir, func(t *testing.T) {
			if got := pythonVenvDir(tt.dataDir); got != tt.want {
				t.Fatalf("pythonVenvDir(%q) = %q, want %q", tt.dataDir, got, tt.want)
			}
		})
	}
}

func TestPythonVenvExecutable(t *testing.T) {
	venvDir := t.TempDir()
	got := pythonVenvExecutable(venvDir)
	if got == "" {
		t.Fatal("pythonVenvExecutable returned empty string")
	}
	if !strings.Contains(got, "python") {
		t.Fatalf("pythonVenvExecutable(%q) = %q, want path containing 'python'", venvDir, got)
	}
}

func TestCheckVenvStatus_NoVenv(t *testing.T) {
	tmpDir := t.TempDir()
	status := CheckVenvStatus(tmpDir)
	if !status.SetupNeeded {
		t.Fatal("expected SetupNeeded=true when no venv exists")
	}
	if status.VenvPath == "" {
		t.Fatal("expected VenvPath to be set")
	}
}

func TestCheckVenvStatus_VenvWithoutPymupdf(t *testing.T) {
	tmpDir := t.TempDir()
	venvDir := pythonVenvDir(tmpDir)
	os.MkdirAll(filepath.Join(venvDir, "Scripts"), 0o755)
	status := CheckVenvStatus(tmpDir)
	if !status.SetupNeeded {
		t.Fatal("expected SetupNeeded when venv lacks pymupdf")
	}
}

func TestDetectPDFiumCorruption_NFD(t *testing.T) {
	base := "Wu\u0308rzburg is a city in Germany. Herna\u0301ndez et al. published results on Go\u0301tz and Nu\u0308rnberg. "
	corrupted := base
	for i := 0; i < 8; i++ {
		corrupted += corrupted
	}
	if !detectPDFiumCorruption(corrupted) {
		t.Fatal("expected NFD corruption detected")
	}
}

func TestDetectPDFiumCorruption_ASCIIOnly(t *testing.T) {
	asciiOnly := "This is a very long document about Hernandez and Wurzburg "
	for i := 0; i < 10; i++ {
		asciiOnly += asciiOnly
	}
	if !detectPDFiumCorruption(asciiOnly) {
		t.Fatal("expected ASCII-only corruption detected in long text")
	}
}

func TestDetectPDFiumCorruption_CleanText(t *testing.T) {
	clean := "Normal academic text with proper Unicode: Munchen, Zurich, Garcia."
	if detectPDFiumCorruption(clean) {
		t.Fatal("false positive on clean text")
	}
}

func TestDetectPDFiumCorruption_ShortText(t *testing.T) {
	short := "Hi"
	if detectPDFiumCorruption(short) {
		t.Fatal("short text should not trigger corruption detection")
	}
}

func TestShouldDropFullTextLine_FigureCaptions(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"Figure 1. Results overview.", true},
		{"FIG. 2. Experimental setup.", true},
		{"Table 3. Comparison of methods.", true},
		{"The figure shows that...", false},
	}
	for _, tt := range tests {
		got := shouldDropFullTextLine(tt.line)
		if got != tt.expected {
			t.Errorf("shouldDropFullTextLine(%q) = %v, want %v", tt.line, got, tt.expected)
		}
	}
}

func TestShouldDropFullTextLine_StandalonePageNumbers(t *testing.T) {
	if !shouldDropFullTextLine("42") {
		t.Error("expected standalone '42' to be dropped")
	}
	if !shouldDropFullTextLine("3") {
		t.Error("expected standalone '3' to be dropped")
	}
	if shouldDropFullTextLine("1984") {
		t.Error("expected year-like '1984' to be kept (4 digits)")
	}
}
