package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
	"zotero_cli/internal/references"
)

const defaultGrobidURL = "https://grobidorg-grobid-full.hf.space"

type GrobidBuildReport struct {
	Candidates   int                     `json:"candidates"`
	WithPDF      int                     `json:"with_pdf"`
	MissingPDF   int                     `json:"missing_pdf"`
	Selected     int                     `json:"selected"`
	Processed    int                     `json:"processed"`
	Succeeded    int                     `json:"succeeded"`
	Failed       int                     `json:"failed"`
	References   int                     `json:"references"`
	Contexts     int                     `json:"contexts"`
	CacheHits    int                     `json:"cache_hits"`
	NetworkCalls int                     `json:"network_calls"`
	ElapsedMS    int64                   `json:"elapsed_ms"`
	Failures     []references.FailedItem `json:"failures,omitempty"`
}

func (s ReferenceService) GrobidStatus(ctx context.Context) (Result, error) {
	cfg, err := s.config()
	if err != nil {
		return Result{}, err
	}
	reader, err := s.reader(cfg)
	if err != nil {
		return Result{}, err
	}
	store, err := s.OpenStore(cfg)
	if err != nil {
		return Result{}, err
	}
	defer store.Close()
	items, err := store.Unsupported(ctx)
	if err != nil {
		return Result{}, err
	}
	withPDF := 0
	for _, row := range items {
		if item, getErr := reader.GetItem(ctx, row.ItemKey); getErr == nil {
			if _, ok := resolvedReferencePDF(item); ok {
				withPDF++
			}
		}
	}
	client := newGrobidReferenceClient(cfg)
	started := time.Now()
	healthErr := client.Health(ctx)
	data := map[string]any{"url": client.BaseURL, "support_level": "experimental", "core_route": "pmc_pubmed", "healthy": healthErr == nil, "unsupported": len(items), "with_pdf": withPDF, "missing_pdf": len(items) - withPDF, "elapsed_ms": time.Since(started).Milliseconds()}
	if healthErr != nil {
		data["error"] = healthErr.Error()
	}
	return Result{Data: data, Meta: map[string]any{"healthy": healthErr == nil}, Text: fmt.Sprintf("EXPERIMENTAL GROBID fallback %s healthy=%v; %d/%d unsupported items have PDF", client.BaseURL, healthErr == nil, withPDF, len(items))}, nil
}

func (s ReferenceService) GrobidBuild(ctx context.Context, req ReferenceBuildRequest) (Result, error) {
	if req.Workers == 0 {
		req.Workers = 1
	}
	if req.Workers < 1 || req.Workers > 8 {
		return Result{}, fmt.Errorf("GROBID --workers must be between 1 and 8")
	}
	if req.Limit == 0 && !req.All {
		req.Limit = 5
	}
	cfg, err := s.config()
	if err != nil {
		return Result{}, err
	}
	reader, err := s.reader(cfg)
	if err != nil {
		return Result{}, err
	}
	store, err := s.OpenStore(cfg)
	if err != nil {
		return Result{}, err
	}
	defer store.Close()
	unsupported, err := store.Unsupported(ctx)
	if err != nil {
		return Result{}, err
	}
	report := GrobidBuildReport{Candidates: len(unsupported)}
	type job struct {
		item domain.Item
		pdf  string
	}
	jobs := []job{}
	for _, row := range unsupported {
		item, getErr := reader.GetItem(ctx, row.ItemKey)
		if getErr != nil {
			continue
		}
		pdf, ok := resolvedReferencePDF(item)
		if !ok {
			report.MissingPDF++
			continue
		}
		report.WithPDF++
		if req.All || req.Limit == 0 || len(jobs) < req.Limit {
			jobs = append(jobs, job{item, pdf})
		}
	}
	report.Selected = len(jobs)
	client := newGrobidReferenceClient(cfg)
	if err := client.Health(ctx); err != nil {
		return Result{}, err
	}
	started := time.Now()
	type outcome struct {
		job    job
		result references.Result
		hit    bool
		err    error
	}
	in := make(chan job)
	out := make(chan outcome, req.Workers)
	var wg sync.WaitGroup
	for i := 0; i < req.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for current := range in {
				begin := time.Now()
				data, hit, processErr := client.Process(ctx, current.pdf, current.item.Key, req.Refresh)
				result := references.Result{ItemKey: current.item.Key, ItemTitle: current.item.Title, Identifiers: references.Identifiers{DOI: current.item.DOI}, Strategy: string(references.SourceGROBID)}
				if processErr == nil {
					result.References, result.Contexts, processErr = references.ParseGrobidTEI(data)
					result.ContextSummary = references.SummarizeContexts(result.Strategy, result.References, result.Contexts)
					references.AnnotateReferenceContexts(result.References, result.Contexts, result.ContextSummary.Status)
					result.ElapsedMS = time.Since(begin).Milliseconds()
				}
				out <- outcome{current, result, hit, processErr}
			}
		}()
	}
	go func() {
		for _, current := range jobs {
			in <- current
		}
		close(in)
		wg.Wait()
		close(out)
	}()
	for value := range out {
		report.Processed++
		if value.hit {
			report.CacheHits++
		} else {
			report.NetworkCalls++
		}
		if value.err != nil {
			report.Failed++
			report.Failures = append(report.Failures, references.FailedItem{ItemKey: value.job.item.Key, Title: value.job.item.Title, Error: value.err.Error()})
			continue
		}
		if err := store.SaveResult(ctx, value.result, references.Fingerprint(value.job.item)); err != nil {
			return Result{}, err
		}
		report.Succeeded++
		report.References += len(value.result.References)
		report.Contexts += len(value.result.Contexts)
	}
	report.ElapsedMS = time.Since(started).Milliseconds()
	meta := map[string]any{"url": client.BaseURL, "workers": req.Workers, "limit": req.Limit, "refresh": req.Refresh, "support_level": "experimental", "core_route": "pmc_pubmed"}
	return Result{Data: report, Meta: meta, Text: fmt.Sprintf("EXPERIMENTAL GROBID build: %d succeeded, %d failed; %d references, %d contexts (%s)", report.Succeeded, report.Failed, report.References, report.Contexts, time.Duration(report.ElapsedMS)*time.Millisecond)}, nil
}

func resolvedReferencePDF(item domain.Item) (string, bool) {
	for _, attachment := range item.Attachments {
		if (attachment.ContentType == "application/pdf" || strings.EqualFold(filepath.Ext(attachment.Filename), ".pdf")) && attachment.Resolved && strings.TrimSpace(attachment.ResolvedPath) != "" {
			if info, err := os.Stat(attachment.ResolvedPath); err == nil && !info.IsDir() {
				return attachment.ResolvedPath, true
			}
		}
	}
	return "", false
}
func newGrobidReferenceClient(cfg config.Config) *references.GrobidClient {
	base := strings.TrimSpace(os.Getenv("ZOT_GROBID_URL"))
	if base == "" {
		base = defaultGrobidURL
	}
	timeout := 300 * time.Second
	if raw := os.Getenv("ZOT_GROBID_TIMEOUT_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			timeout = time.Duration(value) * time.Second
		}
	}
	client := references.NewGrobidClient(base, filepath.Join(referenceRoot(cfg), "grobid"), os.Getenv("HF_TOKEN"), timeout)
	if strings.Contains(strings.ToLower(base), "hf.space") {
		client.MaxAttempts = 2
	}
	return client
}
