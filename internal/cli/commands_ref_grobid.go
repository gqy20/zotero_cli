package cli

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

type grobidBuildReport struct {
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
type grobidJob struct {
	item domain.Item
	pdf  string
}
type grobidOutcome struct {
	job      grobidJob
	result   references.Result
	cacheHit bool
	err      error
}

func (c *CLI) runRefGrobid(args []string) int {
	if len(args) == 0 {
		return c.refUsageError("grobid requires status or build")
	}
	switch args[0] {
	case "status":
		return c.runRefGrobidStatus(args[1:])
	case "build":
		return c.runRefGrobidBuild(args[1:])
	default:
		return c.refUsageError("unknown grobid subcommand: " + args[0])
	}
}

func (c *CLI) runRefGrobidStatus(args []string) int {
	jsonOutput, ok := parseRefJSONOnly(args)
	if !ok {
		return c.refUsageError("grobid status accepts only --json")
	}
	cfg, reader, code := c.loadReader()
	if code != 0 {
		return code
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	items, err := store.Unsupported(context.Background())
	if err != nil {
		return c.printErr(err)
	}
	withPDF := 0
	for _, x := range items {
		if item, e := reader.GetItem(context.Background(), x.ItemKey); e == nil {
			if _, ok := resolvedPDF(item); ok {
				withPDF++
			}
		}
	}
	client := newGrobidClient(cfg)
	start := time.Now()
	healthErr := client.Health(context.Background())
	data := map[string]any{"url": client.BaseURL, "support_level": "experimental", "core_route": "pmc_pubmed", "healthy": healthErr == nil, "unsupported": len(items), "with_pdf": withPDF, "missing_pdf": len(items) - withPDF, "elapsed_ms": time.Since(start).Milliseconds()}
	if healthErr != nil {
		data["error"] = healthErr.Error()
	}
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: healthErr == nil, Command: "ref-grobid-status", Data: data})
	}
	fmt.Fprintf(c.stdout, "EXPERIMENTAL GROBID fallback %s healthy=%v; %d/%d unsupported items have PDF\n", client.BaseURL, healthErr == nil, withPDF, len(items))
	return ExitOK
}

func (c *CLI) runRefGrobidBuild(args []string) int {
	workers, limit := 1, 5
	jsonOutput, refresh, all := false, false, false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--refresh":
			refresh = true
		case "--all":
			all = true
		case "--workers":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --workers")
			}
			i++
			v, e := strconv.Atoi(args[i])
			if e != nil || v < 1 || v > 8 {
				return c.refUsageError("invalid value for --workers")
			}
			workers = v
		case "--limit":
			if i+1 >= len(args) {
				return c.refUsageError("missing value for --limit")
			}
			i++
			v, e := strconv.Atoi(args[i])
			if e != nil || v < 1 {
				return c.refUsageError("invalid value for --limit")
			}
			limit = v
		default:
			return c.refUsageError("unknown argument: " + args[i])
		}
	}
	if all {
		limit = 0
	}
	cfg, reader, code := c.loadReader()
	if code != 0 {
		return code
	}
	store, err := openReferenceStore(cfg)
	if err != nil {
		return c.printErr(err)
	}
	defer store.Close()
	unsupported, err := store.Unsupported(context.Background())
	if err != nil {
		return c.printErr(err)
	}
	report := grobidBuildReport{Candidates: len(unsupported)}
	jobs := []grobidJob{}
	for _, x := range unsupported {
		item, e := reader.GetItem(context.Background(), x.ItemKey)
		if e != nil {
			continue
		}
		pdf, ok := resolvedPDF(item)
		if !ok {
			report.MissingPDF++
			continue
		}
		report.WithPDF++
		if limit == 0 || len(jobs) < limit {
			jobs = append(jobs, grobidJob{item: item, pdf: pdf})
		}
	}
	report.Selected = len(jobs)
	client := newGrobidClient(cfg)
	if err := client.Health(context.Background()); err != nil {
		return c.printErr(err)
	}
	started := time.Now()
	jobCh := make(chan grobidJob)
	outCh := make(chan grobidOutcome, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				begin := time.Now()
				data, hit, e := client.Process(context.Background(), job.pdf, job.item.Key, refresh)
				res := references.Result{ItemKey: job.item.Key, ItemTitle: job.item.Title, Identifiers: references.Identifiers{DOI: job.item.DOI}, Strategy: string(references.SourceGROBID)}
				if e == nil {
					res.References, res.Contexts, e = references.ParseGrobidTEI(data)
					res.ContextSummary = references.SummarizeContexts(res.Strategy, res.References, res.Contexts)
					references.AnnotateReferenceContexts(res.References, res.Contexts, res.ContextSummary.Status)
					res.ElapsedMS = time.Since(begin).Milliseconds()
				}
				outCh <- grobidOutcome{job: job, result: res, cacheHit: hit, err: e}
			}
		}()
	}
	go func() {
		for _, j := range jobs {
			jobCh <- j
		}
		close(jobCh)
		wg.Wait()
		close(outCh)
	}()
	for out := range outCh {
		report.Processed++
		if out.cacheHit {
			report.CacheHits++
		} else {
			report.NetworkCalls++
		}
		if out.err != nil {
			report.Failed++
			report.Failures = append(report.Failures, references.FailedItem{ItemKey: out.job.item.Key, Title: out.job.item.Title, Error: out.err.Error()})
			continue
		}
		if err := store.SaveResult(context.Background(), out.result, references.Fingerprint(out.job.item)); err != nil {
			return c.printErr(err)
		}
		report.Succeeded++
		report.References += len(out.result.References)
		report.Contexts += len(out.result.Contexts)
	}
	report.ElapsedMS = time.Since(started).Milliseconds()
	if jsonOutput {
		return c.writeJSON(jsonResponse{OK: report.Failed == 0, Command: "ref-grobid-build", Data: report, Meta: map[string]any{"url": client.BaseURL, "workers": workers, "limit": limit, "refresh": refresh, "support_level": "experimental", "core_route": "pmc_pubmed"}})
	}
	fmt.Fprintf(c.stdout, "EXPERIMENTAL GROBID build: %d succeeded, %d failed; %d references, %d contexts (%s)\n", report.Succeeded, report.Failed, report.References, report.Contexts, time.Duration(report.ElapsedMS)*time.Millisecond)
	return ExitOK
}

func resolvedPDF(item domain.Item) (string, bool) {
	for _, a := range item.Attachments {
		if (a.ContentType == "application/pdf" || strings.EqualFold(filepath.Ext(a.Filename), ".pdf")) && a.Resolved && strings.TrimSpace(a.ResolvedPath) != "" {
			if info, err := os.Stat(a.ResolvedPath); err == nil && !info.IsDir() {
				return a.ResolvedPath, true
			}
		}
	}
	return "", false
}
func newGrobidClient(cfg config.Config) *references.GrobidClient {
	base := strings.TrimSpace(os.Getenv("ZOT_GROBID_URL"))
	if base == "" {
		base = defaultGrobidURL
	}
	timeout := 300 * time.Second
	if raw := os.Getenv("ZOT_GROBID_TIMEOUT_SECONDS"); raw != "" {
		if n, e := strconv.Atoi(raw); e == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	client := references.NewGrobidClient(base, filepath.Join(referenceRoot(cfg), "grobid"), os.Getenv("HF_TOKEN"), timeout)
	if strings.Contains(strings.ToLower(base), "hf.space") {
		client.MaxAttempts = 2
	}
	return client
}
