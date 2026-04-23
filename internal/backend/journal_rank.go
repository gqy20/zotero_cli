package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"zotero_cli/internal/domain"
)

type journalRankDB struct {
	mu     sync.RWMutex
	byName map[string]map[string]string
	normal map[string]string
	loaded bool
	path   string
}

var globalJournalRank = &journalRankDB{
	byName: make(map[string]map[string]string),
	normal: make(map[string]string),
}

func LoadJournalRank(path string) error {
	return globalJournalRank.load(path)
}

func LookupJournalRank(name string) *domain.JournalRank {
	return globalJournalRank.lookup(name)
}

func JournalRankLoaded() bool {
	globalJournalRank.mu.RLock()
	defer globalJournalRank.mu.RUnlock()
	return globalJournalRank.loaded
}

func (db *journalRankDB) load(path string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.loaded && db.path == path {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read journal rank json: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse journal rank json: %w", err)
	}

	byName := make(map[string]map[string]string, len(raw))
	normal := make(map[string]string, len(raw))

	for name, entryRaw := range raw {
		var entry struct {
			Rank map[string]string `json:"rank"`
		}
		if err := json.Unmarshal(entryRaw, &entry); err != nil {
			continue
		}
		if entry.Rank == nil || len(entry.Rank) == 0 {
			continue
		}
		byName[name] = entry.Rank
		norm := normalizeJournalName(name)
		if norm != "" {
			normal[norm] = name
		}
	}

	db.byName = byName
	db.normal = normal
	db.path = path
	db.loaded = true
	return nil
}

func (db *journalRankDB) lookup(name string) *domain.JournalRank {
	db.mu.RLock()
	if !db.loaded {
		db.mu.RUnlock()
		return nil
	}

	ranks := db.lookupUnlocked(name)
	db.mu.RUnlock()

	if ranks == nil {
		return nil
	}
	return &domain.JournalRank{
		MatchedName: name,
		Ranks:       ranks,
	}
}

func (db *journalRankDB) lookupUnlocked(name string) map[string]string {
	if ranks, ok := db.byName[name]; ok {
		return ranks
	}

	norm := normalizeJournalName(name)
	if norm == "" {
		return nil
	}

	if original, ok := db.normal[norm]; ok {
		return db.byName[original]
	}

	for n, r := range db.normal {
		if strings.EqualFold(norm, n) {
			return db.byName[r]
		}
	}

	for key, ranks := range db.byName {
		keyNorm := normalizeJournalName(key)
		if keyNorm == "" {
			continue
		}
		if strings.Contains(keyNorm, norm) || normContainsWord(norm, keyNorm) {
			return ranks
		}
		if abbrevMatch(name, key) {
			return ranks
		}
	}

	return nil
}

func normalizeJournalName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			prevSpace = false
		} else if !prevSpace {
			b.WriteRune(' ')
			prevSpace = true
		}
	}
	result := strings.TrimSpace(b.String())
	return result
}

func normContainsWord(norm, containerNorm string) bool {
	normWords := strings.Fields(norm)
	if len(normWords) == 0 {
		return false
	}
	contWords := strings.Fields(containerNorm)
	if len(contWords) == 0 {
		return false
	}
	matchCount := 0
	for _, nw := range normWords {
		if len(nw) <= 3 {
			continue
		}
		for _, cw := range contWords {
			if strings.EqualFold(nw, cw) {
				matchCount++
				break
			}
		}
	}
	return matchCount >= 2
}

func abbrevMatch(query, candidate string) bool {
	qWords := splitAlphaWords(query)
	cWords := splitAlphaWords(candidate)
	if len(qWords) == 0 || len(cWords) == 0 {
		return false
	}

	qInitials := extractInitials(qWords)
	cInitials := extractInitials(cWords)
	if len(qInitials) >= 2 && len(cInitials) >= 2 && strings.EqualFold(qInitials, cInitials) {
		return true
	}

	qAbbr := buildDotAbbrev(qWords)
	cAbbr := buildDotAbbrev(cWords)
	if len(qAbbr) >= 4 && (qAbbr == cAbbr || strings.HasPrefix(cAbbr, qAbbr)) {
		return true
	}

	matchCount := 0
	for _, qw := range qWords {
		if len(qw) <= 2 {
			continue
		}
		for _, cw := range cWords {
			if strings.EqualFold(qw, cw) || (len(qw) >= 4 && strings.HasPrefix(strings.ToLower(cw), strings.ToLower(qw))) {
				matchCount++
				break
			}
		}
	}
	return matchCount >= 2 && matchCount >= len(qWords)/2
}

func splitAlphaWords(s string) []string {
	var words []string
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) {
			b.WriteRune(r)
		} else {
			if b.Len() > 0 {
				words = append(words, b.String())
				b.Reset()
			}
		}
	}
	if b.Len() > 0 {
		words = append(words, b.String())
	}
	return words
}

func extractInitials(words []string) string {
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	for _, w := range words {
		runes := []rune(w)
		if len(runes) > 0 {
			b.WriteRune(unicode.ToUpper(runes[0]))
		}
	}
	return b.String()
}

func buildDotAbbrev(words []string) string {
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	for i, w := range words {
		runes := []rune(w)
		if len(runes) == 0 {
			continue
		}
		b.WriteRune(unicode.ToUpper(runes[0]))
		if i < len(words)-1 {
			b.WriteRune('.')
		}
	}
	return b.String()
}

func DetectJournalRankPath(dataDir string) string {
	path := filepath.Join(dataDir, "zoterostyle.json")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func ResolveJournalRankPath(basePath string, dataDir string) string {
	if filepath.IsAbs(basePath) {
		return basePath
	}
	return filepath.Join(dataDir, basePath)
}

func AutoLoadJournalRank(dataDir string) error {
	path := os.Getenv("ZOT_JOURNAL_RANK_PATH")
	if path == "" {
		path = DetectJournalRankPath(dataDir)
	} else {
		path = ResolveJournalRankPath(path, dataDir)
	}
	if path == "" {
		return nil
	}
	return LoadJournalRank(path)
}
