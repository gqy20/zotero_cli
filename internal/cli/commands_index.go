package cli

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/domain"
)

const (
	defaultMaxWorkers = 10
	hardMaxWorkers    = 20
)

type indexResult struct {
	TotalItems       int      `json:"total_items_with_pdf"`
	TotalAttachments int      `json:"total_attachments"`
	Indexed          int      `json:"indexed"`
	Skipped          int      `json:"skipped"`
	Failed           int      `json:"failed"`
	Errors           []string `json:"errors,omitempty"`
	Elapsed          float64  `json:"elapsed_seconds"`
}

func (c *CLI) runIndex(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageIndex)
	}
	if len(args) == 0 || args[0] != "build" {
		fmt.Fprintln(c.stderr, usageIndex)
		return 2
	}

	if isHelpOnly(args[1:]) {
		return c.printCommandUsage(usageIndex)
	}

	opts, ok := c.parseIndexBuildArgs(args[1:])
	if !ok {
		return 2
	}

	cfg, exitCode := c.loadConfig()
	if exitCode != 0 {
		return exitCode
	}

	localReader, err := c.newLocalReader(cfg)
	if err != nil {
		return c.printErr(err)
	}

	result, err := c.indexBuild(context.Background(), localReader, opts)
	if err != nil {
		return c.printErr(err)
	}

	if opts.JSONOutput {
		return c.writeJSON(jsonResponse{
			OK:      true,
			Command: "index build",
			Data:    result,
			Meta: map[string]any{
				"elapsed": result.Elapsed,
			},
		})
	}

	c.printIndexResult(result)
	return 0
}

func (c *CLI) parseIndexBuildArgs(args []string) (indexBuildOpts, bool) {
	opts := indexBuildOpts{
		Workers: min(runtime.NumCPU(), defaultMaxWorkers),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force":
			opts.Force = true
		case "--json":
			opts.JSONOutput = true
		case "--workers":
			if i+1 >= len(args) {
				fmt.Fprintln(c.stderr, usageIndex)
				return indexBuildOpts{}, false
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				fmt.Fprintf(c.stderr, "error: --workers must be a positive integer\n")
				fmt.Fprintln(c.stderr, usageIndex)
				return indexBuildOpts{}, false
			}
			opts.Workers = min(n, hardMaxWorkers)
		default:
			fmt.Fprintf(c.stderr, "error: unknown flag: %s\n", args[i])
			fmt.Fprintln(c.stderr, usageIndex)
			return indexBuildOpts{}, false
		}
	}
	return opts, true
}

type indexBuildOpts struct {
	Force      bool
	Workers    int
	JSONOutput bool
}

func (c *CLI) indexBuild(ctx context.Context, reader backend.Reader, opts indexBuildOpts) (indexResult, error) {
	result := indexResult{}

	extractor, ok := reader.(attachmentFullTextExtractor)
	if !ok {
		return result, fmt.Errorf("index build requires local mode with full-text extraction support")
	}

	writer, hasWriter := reader.(fullTextWriter)
	cacheChecker, hasCacheCheck := reader.(fullTextCacheChecker)
	failedMarker, hasFailedMark := reader.(failedMarker)

	startTime := time.Now()

	allItems, err := reader.FindItems(ctx, backend.FindOptions{
		HasPDF:        true,
		IncludeFields: []string{"attachments"},
	})
	if err != nil {
		return result, err
	}

	type attachmentTask struct {
		item       domain.Item
		attachment domain.Attachment
	}

	type extractResult struct {
		task    attachmentTask
		doc     backend.FullTextDocument
		success bool
		err     error
	}

	var tasks []attachmentTask
	totalAttachments := 0
	for _, item := range allItems {
		pdfs := filterPDFAttachments(item.Attachments)
		if len(pdfs) == 0 {
			continue
		}
		totalAttachments += len(pdfs)
		for _, att := range pdfs {
			if opts.Force {
				tasks = append(tasks, attachmentTask{item: item, attachment: att})
				continue
			}
			if hasCacheCheck && cacheChecker.IsFullTextCached(att) {
				result.Skipped++
				continue
			}
			if hasFailedMark && cacheChecker.IsMarkedFailed(att.Key) {
				result.Skipped++
				continue
			}
			tasks = append(tasks, attachmentTask{item: item, attachment: att})
		}
	}

	result.TotalItems = len(allItems)
	result.TotalAttachments = totalAttachments
	totalToIndex := len(tasks)
	if totalToIndex == 0 {
		result.Elapsed = time.Since(startTime).Seconds()
		return result, nil
	}

	if !opts.JSONOutput {
		fmt.Fprintf(c.stdout, "Indexing PDF full-text...\n")
		fmt.Fprintf(c.stdout, "  Items with PDF:   %d\n", result.TotalItems)
		fmt.Fprintf(c.stdout, "  Total attachments: %d\n", totalAttachments)
		fmt.Fprintf(c.stdout, "  Need indexing:     %d (%d cached/failed, skipped)\n", totalToIndex, result.Skipped)
		fmt.Fprintf(c.stdout, "  Workers:           %d\n\n", opts.Workers)
	}

	var (
		mu           sync.Mutex
		extractWg    sync.WaitGroup
		doneCount    int64
		indexedCount int64
		failedCount  int64
		errList      []string
		extractSem   = make(chan struct{}, opts.Workers)
		resultCh     = make(chan extractResult, totalToIndex)
	)

	progressCh := make(chan int64, opts.Workers*2)
	stopProgress := make(chan struct{})
	if !opts.JSONOutput {
		go c.indexProgressPrinter(progressCh, stopProgress, totalToIndex)
		defer close(stopProgress)
	}

	for _, task := range tasks {
		extractWg.Add(1)
		t := task
		go func() {
			defer extractWg.Done()
			extractSem <- struct{}{}
			defer func() { <-extractSem }()

			doc, docOk, extractErr := extractor.ExtractAttachmentFullTextOnly(ctx, t.item, t.attachment)
			success := docOk && extractErr == nil

			if !success && hasFailedMark {
				_ = failedMarker.MarkExtractFailed(t.attachment.Key)
			}

			if extractErr != nil {
				mu.Lock()
				errList = append(errList, fmt.Sprintf("%s/%s: %s", t.item.Key, t.attachment.Key, extractErr.Error()))
				mu.Unlock()
			}

			resultCh <- extractResult{task: t, doc: doc, success: success, err: extractErr}

			done := atomic.AddInt64(&doneCount, 1)
			select {
			case progressCh <- done:
			default:
			}
		}()
	}

	extractWg.Wait()
	close(resultCh)

	var successDocs []backend.FullTextDocument

	for res := range resultCh {
		if res.success {
			successDocs = append(successDocs, res.doc)
			atomic.AddInt64(&indexedCount, 1)
		} else if res.err != nil {
			atomic.AddInt64(&failedCount, 1)
		} else {
			atomic.AddInt64(&failedCount, 1)
		}
	}

	if len(successDocs) > 0 && hasWriter {
		batchWriter, hasBatch := reader.(fullTextBatchWriter)
		if !opts.JSONOutput {
			fmt.Fprintf(c.stdout, "\n  Writing index (%d docs)...\n", len(successDocs))
		}
		var writeErr error
		if hasBatch {
			writeErr = batchWriter.SaveFullTextBatch(successDocs)
		} else {
			for _, doc := range successDocs {
				if err := writer.SaveFullText(doc); err != nil {
					writeErr = err
					break
				}
			}
		}
		if writeErr != nil {
			errList = append(errList, fmt.Sprintf("batch write failed: %s", writeErr.Error()))
		}
	}

	result.Errors = errList
	result.Indexed = int(atomic.LoadInt64(&indexedCount))
	result.Failed = int(atomic.LoadInt64(&failedCount))
	result.Elapsed = time.Since(startTime).Seconds()

	return result, nil
}

func (c *CLI) indexProgressPrinter(progressCh <-chan int64, stopCh <-chan struct{}, total int) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	lastPrinted := int64(0)
	for {
		select {
		case <-stopCh:
			c.indexPrintProgress(total, total)
			return
		case done, ok := <-progressCh:
			if ok && done > lastPrinted {
				lastPrinted = done
			}
			if !ok {
				c.indexPrintProgress(total, total)
				return
			}
		case <-ticker.C:
			if lastPrinted > 0 {
				c.indexPrintProgress(int(lastPrinted), total)
			}
		}
	}
}

func (c *CLI) indexPrintProgress(done, total int) {
	pct := float64(done) / float64(total) * 100
	width := 30
	filled := int(float64(done) / float64(total) * float64(width))
	bar := strings.Repeat("#", filled) + strings.Repeat("-", width-filled)
	fmt.Fprintf(c.stdout, "\r  [%s] %d/%d (%.0f%%)", bar, done, total, pct)
	if done >= total {
		fmt.Fprintln(c.stdout)
	}
}

func (c *CLI) printIndexResult(r indexResult) {
	fmt.Fprintf(c.stdout, "\nDone.\n")
	fmt.Fprintf(c.stdout, "  Indexed:  %d\n", r.Indexed)
	fmt.Fprintf(c.stdout, "  Skipped:  %d\n", r.Skipped)
	fmt.Fprintf(c.stdout, "  Failed:   %d\n", r.Failed)
	fmt.Fprintf(c.stdout, "  Elapsed:  %.1fs\n", r.Elapsed)
	if len(r.Errors) > 0 {
		fmt.Fprintln(c.stdout, "\n  Errors:")
		for _, e := range r.Errors {
			fmt.Fprintf(c.stdout, "    - %s\n", e)
		}
	}
}
