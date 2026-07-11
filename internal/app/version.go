package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	Latest    string `json:"latest,omitempty"`
	Date      string `json:"date,omitempty"`
	Update    bool   `json:"update,omitempty"`
}

type VersionService struct {
	Current   string
	Commit    string
	BuildDate string
	Client    *http.Client
}

func (s VersionService) Show(ctx context.Context, check bool) (Result, error) {
	info := VersionInfo{Version: s.Current, Commit: s.Commit, BuildDate: s.BuildDate}
	if check {
		client := s.Client
		if client == nil {
			client = http.DefaultClient
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/gqy20/zotero_cli/releases/latest", nil)
		if err != nil {
			return Result{}, err
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		resp, err := client.Do(req)
		if err != nil {
			return Result{}, fmt.Errorf("check latest version: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return Result{}, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
		}
		var payload struct {
			TagName     string `json:"tag_name"`
			PublishedAt string `json:"published_at"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return Result{}, err
		}
		info.Latest = payload.TagName
		if len(payload.PublishedAt) >= 10 {
			info.Date = payload.PublishedAt[:10]
		}
		info.Update = payload.TagName != "" && payload.TagName != s.Current
	}
	lines := []string{fmt.Sprintf("zot %s", s.Current), "commit: " + s.Commit, "built: " + s.BuildDate}
	if check {
		lines = append(lines, "latest: "+info.Latest+" ("+info.Date+")")
	}
	return Result{Data: info, Text: strings.Join(lines, "\n")}, nil
}
