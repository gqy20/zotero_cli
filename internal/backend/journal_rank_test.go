package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"zotero_cli/internal/domain"
)

func sampleRankJSON() map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"Nature":                          json.RawMessage(`{"rank":{"sciif":"48.5","sci":"Q1","jci":"11.12","sciUp":"综合性期刊1区","swjtu":"A++","nju":"超一流期刊","esi":"多学科"}}`),
		"Molecular Biology and Evolution": json.RawMessage(`{"rank":{"sciif":"5.3","sci":"Q1","jci":"2.23","swjtu":"A++","esi":"分子生物与遗传学"}}`),
		"The New Phytologist":             json.RawMessage(`{"rank":{"sciif":"8.1","sci":"Q1","jci":"2.06","swjtu":"A++","xju":"一区"}}`),
		"Journal of Experimental Botany":  json.RawMessage(`{"rank":{"sciif":"4.9","sci":"Q1","jci":"1.20","swjtu":"A++","xju":"一区"}}`),
		"PLoS ONE":                        json.RawMessage(`{"rank":{"sciif":"2.6","sci":"Q2","jci":"0.85","swjtu":"A++","hhu":"C类"}}`),
		"植物学报":                            json.RawMessage(`{"rank":{"swjtu":"A+","xju":"二区","cscd":"核心刊"}}`),
		"Empty Journal":                   json.RawMessage(`{}`),
	}
}

func createTempRankDB(t *testing.T) string {
	t.Helper()
	data := sampleRankJSON()
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "zoterostyle.json")
	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadJournalRank(t *testing.T) {
	path := createTempRankDB(t)

	err := LoadJournalRank(path)
	if err != nil {
		t.Fatalf("LoadJournalRank failed: %v", err)
	}
	if !JournalRankLoaded() {
		t.Error("expected JournalRankLoaded() = true")
	}
}

func TestLookupExact(t *testing.T) {
	path := createTempRankDB(t)
	if err := LoadJournalRank(path); err != nil {
		t.Fatal(err)
	}

	rank := LookupJournalRank("Nature")
	if rank == nil {
		t.Fatal("expected non-nil rank for Nature")
	}
	if rank.Ranks["sciif"] != "48.5" {
		t.Errorf("expected sciif=48.5, got %s", rank.Ranks["sciif"])
	}
	if rank.Ranks["swjtu"] != "A++" {
		t.Errorf("expected swjtu=A++, got %s", rank.Ranks["swjtu"])
	}
}

func TestLookupCaseInsensitive(t *testing.T) {
	path := createTempRankDB(t)
	if err := LoadJournalRank(path); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query  string
		wantIF string
	}{
		{"nature", "48.5"},
		{"NATURE", "48.5"},
		{"NaTuRe", "48.5"},
		{"the new phytologist", "8.1"},
		{"THE NEW PHYTOLOGIST", "8.1"},
	}
	for _, tc := range tests {
		rank := LookupJournalRank(tc.query)
		if rank == nil {
			t.Errorf("lookup(%q): expected non-nil, got nil", tc.query)
			continue
		}
		if rank.Ranks["sciif"] != tc.wantIF {
			t.Errorf("lookup(%q): sciif=%q, want %q", tc.query, rank.Ranks["sciif"], tc.wantIF)
		}
	}
}

func TestLookupAbbreviation(t *testing.T) {
	path := createTempRankDB(t)
	if err := LoadJournalRank(path); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query  string
		wantIF string
		desc   string
	}{
		{"Mol Biol Evol", "5.3", "space abbrev"},
		{"Mol. Biol. Evol.", "5.3", "dot abbrev"},
		{"PLOS ONE", "2.6", "PLOS style"},
		{"PLoS One", "2.6", "mixed case PLoS"},
		{"THE NEW PHYTOL", "8.1", "abbreviated THE NEW PHYTOL"},
		{"Journal of Experimental Botany", "4.9", "full name"},
		{"New Phytologist", "8.1", "partial name"},
	}
	for _, tc := range tests {
		rank := LookupJournalRank(tc.query)
		if rank == nil {
			t.Errorf("[%s] lookup(%q): expected non-nil, got nil", tc.desc, tc.query)
			continue
		}
		if rank.Ranks["sciif"] != tc.wantIF {
			t.Errorf("[%s] lookup(%q): sciif=%q, want %q", tc.desc, tc.query, rank.Ranks["sciif"], tc.wantIF)
		}
	}
}

func TestLookupChinese(t *testing.T) {
	path := createTempRankDB(t)
	if err := LoadJournalRank(path); err != nil {
		t.Fatal(err)
	}

	rank := LookupJournalRank("植物学报")
	if rank == nil {
		t.Fatal("expected non-nil for Chinese journal")
	}
	if rank.Ranks["swjtu"] != "A+" {
		t.Errorf("expected swjtu=A+, got %s", rank.Ranks["swjtu"])
	}
}

func TestLookupNotFound(t *testing.T) {
	path := createTempRankDB(t)
	if err := LoadJournalRank(path); err != nil {
		t.Fatal(err)
	}

	notFound := []string{
		"Nonexistent Journal That Does Not Exist",
		"",
		"XYZ123",
	}
	for _, q := range notFound {
		rank := LookupJournalRank(q)
		if rank != nil {
			t.Errorf("lookup(%q): expected nil, got %+v", q, rank.Ranks)
		}
	}
}

func TestLookupBeforeLoad(t *testing.T) {
	saved := globalJournalRank.loaded
	globalJournalRank.loaded = false
	defer func() { globalJournalRank.loaded = saved }()

	rank := LookupJournalRank("Nature")
	if rank != nil {
		t.Errorf("expected nil before load, got %+v", rank)
	}
}

func TestLoadInvalidPath(t *testing.T) {
	err := LoadJournalRank("/nonexistent/path/zoterostyle.json")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestNormalizeJournalName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Nature Communications", "nature communications"},
		{"  Nature   Communications  ", "nature communications"},
		{"MOL-BIOL-EVOL", "mol biol evol"},
		{"J. Exp. Bot.", "j exp bot"},
		{"THE NEW PHYTOLOGIST (LOND)", "the new phytologist lond"},
		{"", ""},
		{"   ", ""},
		{"植物学报", "植物学报"},
	}
	for _, tc := range tests {
		got := normalizeJournalName(tc.input)
		if got != tc.want {
			t.Errorf("normalizeJournalName(%q)=%q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestAbbrevMatch(t *testing.T) {
	tests := []struct {
		query     string
		candidate string
		want      bool
	}{
		{"Mol Biol Evol", "Molecular Biology and Evolution", true},
		{"THE NEW PHYTOL", "The New Phytologist", true},
		{"Nature", "Science", false},
		{"PNAS", "Proceedings of the National Academy of Sciences", false},
		{"Cell", "Cell Reports", false},
		{"MBE", "Molecular Biology and Evolution", false},
	}
	for _, tc := range tests {
		got := abbrevMatch(tc.query, tc.candidate)
		if got != tc.want {
			t.Errorf("abbrevMatch(%q, %q)=%v, want %v", tc.query, tc.candidate, got, tc.want)
		}
	}
}

func TestDetectJournalRankPath(t *testing.T) {
	tmpDir := t.TempDir()
	data := sampleRankJSON()
	jsonData, _ := json.Marshal(data)
	zoterostylePath := filepath.Join(tmpDir, "zoterostyle.json")
	os.WriteFile(zoterostylePath, jsonData, 0644)

	got := DetectJournalRankPath(tmpDir)
	if got != zoterostylePath {
		t.Errorf("DetectJournalRankPath(%s)=%s, want %s", tmpDir, got, zoterostylePath)
	}
}

func TestDetectJournalRankPath_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	got := DetectJournalRankPath(tmpDir)
	if got != "" {
		t.Errorf("DetectJournalRankPath(%s)=%s, want empty", tmpDir, got)
	}
}

func TestJournalRankJSONRoundtrip(t *testing.T) {
	path := createTempRankDB(t)
	if err := LoadJournalRank(path); err != nil {
		t.Fatal(err)
	}

	rank := LookupJournalRank("Nature")
	if rank == nil {
		t.Fatal("expected non-nil")
	}

	jsonData, err := json.Marshal(rank)
	if err != nil {
		t.Fatal(err)
	}

	var parsed domain.JournalRank
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Ranks["sciif"] != "48.5" {
		t.Errorf("roundtrip sciif=%q, want 48.5", parsed.Ranks["sciif"])
	}
}
