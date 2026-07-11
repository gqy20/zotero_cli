package references

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"zotero_cli/internal/domain"
)

type BuildOptions struct {
	Workers int
	Force   bool
	Refresh bool
	Source  string
	Limit   int
}

type BuildReport struct {
	Total            int          `json:"total"`
	Eligible         int          `json:"eligible"`
	Processed        int          `json:"processed"`
	Succeeded        int          `json:"succeeded"`
	Failed           int          `json:"failed"`
	Unsupported      int          `json:"unsupported"`
	Skipped          int          `json:"skipped"`
	References       int          `json:"references"`
	CacheHits        int          `json:"cache_hits"`
	NetworkCalls     int          `json:"network_calls"`
	ElapsedMS        int64        `json:"elapsed_ms"`
	Failures         []FailedItem `json:"failures,omitempty"`
	UnsupportedItems []FailedItem `json:"unsupported_items,omitempty"`
}

type Builder struct {
	service *Service
	store   *Store
}

func NewBuilder(service *Service, store *Store) *Builder {
	return &Builder{service: service, store: store}
}

type buildJob struct {
	item        domain.Item
	fingerprint string
}
type buildOutcome struct {
	job    buildJob
	result Result
	err    error
}

func (b *Builder) Build(ctx context.Context, items []domain.Item, opts BuildOptions) (BuildReport, error) {
	started := time.Now()
	report := BuildReport{Total: len(items)}
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	if opts.Workers > 16 {
		opts.Workers = 16
	}
	jobs := make([]buildJob, 0, len(items))
	for _, item := range items {
		ids := identifiersFromItem(item)
		if ids.DOI == "" && ids.PMID == "" && ids.PMCID == "" {
			continue
		}
		report.Eligible++
		fingerprint := Fingerprint(item)
		if !opts.Force {
			fresh, err := b.store.IsFresh(ctx, item.Key, fingerprint)
			if err != nil {
				return report, err
			}
			if fresh {
				report.Skipped++
				continue
			}
		}
		jobs = append(jobs, buildJob{item: item, fingerprint: fingerprint})
		if opts.Limit > 0 && len(jobs) >= opts.Limit {
			break
		}
	}

	jobCh := make(chan buildJob)
	outCh := make(chan buildOutcome, opts.Workers)
	var wg sync.WaitGroup
	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				result, err := b.service.References(ctx, job.item, Options{Source: opts.Source, Refresh: opts.Refresh})
				outCh <- buildOutcome{job: job, result: result, err: err}
			}
		}()
	}
	go func() {
		for _, job := range jobs {
			jobCh <- job
		}
		close(jobCh)
		wg.Wait()
		close(outCh)
	}()

	for outcome := range outCh {
		report.Processed++
		if outcome.err != nil {
			var unsupported *UnsupportedError
			if errors.As(outcome.err, &unsupported) {
				report.Unsupported++
				if err := b.store.SaveUnsupported(ctx, outcome.job.item.Key, outcome.job.item.Title, outcome.job.fingerprint, outcome.err); err != nil {
					return report, err
				}
				report.UnsupportedItems = append(report.UnsupportedItems, FailedItem{ItemKey: outcome.job.item.Key, Title: outcome.job.item.Title, Error: outcome.err.Error()})
				continue
			}
			report.Failed++
			if err := b.store.SaveFailure(ctx, outcome.job.item.Key, outcome.job.item.Title, outcome.job.fingerprint, outcome.err); err != nil {
				return report, err
			}
			report.Failures = append(report.Failures, FailedItem{ItemKey: outcome.job.item.Key, Title: outcome.job.item.Title, Error: outcome.err.Error()})
			continue
		}
		if err := b.store.SaveResult(ctx, outcome.result, outcome.job.fingerprint); err != nil {
			return report, err
		}
		report.Succeeded++
		report.References += len(outcome.result.References)
	}
	report.CacheHits, report.NetworkCalls = b.service.client.Stats()
	report.ElapsedMS = time.Since(started).Milliseconds()
	return report, nil
}

func Fingerprint(item domain.Item) string {
	value := fmt.Sprintf("v1\x00%s\x00%d\x00%s\x00%s\x00%s", item.Key, item.Version, normalizeDOI(item.DOI), item.URL, item.Title)
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}
