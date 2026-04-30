// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

const (
	llmCheckerName = "llm"

	// maxBatchChars is the target character budget per batch, leaving room
	// for the system prompt and response. Conservative at 6000 chars (~2000
	// tokens) to fit within 4096-token context windows with margin.
	maxBatchChars = 6000

	// maxScriptsPerBatch caps the number of scripts per request regardless
	// of character count, to keep the LLM focused.
	maxScriptsPerBatch = 5

	// maxScriptChars is the truncation limit for a single script. Scripts
	// exceeding this are truncated with a marker.
	maxScriptChars = 4000
)

// LLMChecker performs security analysis using a local or remote LLM via the
// OpenAI-compatible chat completions API. It batches scripts to balance
// context window utilization against parallelism, and uses a semaphore to
// limit concurrent requests.
//
// This checker is opt-in: it is NOT included in DefaultCheckers() and must
// be explicitly added via WithChecker(NewLLMChecker(...)).
type (
	LLMChecker struct {
		completer   LLMCompleter
		concurrency int
	}

	// LLMCompleter abstracts the LLM chat completion call for testability.
	// Production adapters live outside the audit domain package.
	LLMCompleter interface {
		Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	}
)

// NewLLMChecker creates an LLMChecker with the given completer and concurrency.
// The concurrency parameter controls the maximum number of parallel LLM requests.
func NewLLMChecker(completer LLMCompleter, concurrency int) *LLMChecker {
	if concurrency <= 0 {
		concurrency = DefaultLLMConcurrency
	}
	return &LLMChecker{
		completer:   completer,
		concurrency: concurrency,
	}
}

// Name returns the checker identifier.
func (c *LLMChecker) Name() string { return llmCheckerName }

// Category returns the primary category. LLM findings span multiple categories
// but we report the checker's primary as execution (the broadest security concern).
func (c *LLMChecker) Category() Category { return CategoryExecution }

// Check analyzes all scripts in the scan context using the LLM.
// Scripts are batched by character count and script count limits, then
// dispatched concurrently up to the configured concurrency level.
func (c *LLMChecker) Check(ctx context.Context, sc *ScanContext) ([]Finding, error) {
	prepared := prepareScripts(sc.AllScripts(), maxScriptChars)
	if len(prepared) == 0 {
		return nil, nil
	}

	batches := batchScripts(prepared)

	type batchResult struct {
		findings []Finding
		err      error
	}

	results := make([]batchResult, len(batches))
	sem := make(chan struct{}, c.concurrency)
	var wg sync.WaitGroup

	for i, batch := range batches {
		wg.Add(1)
		go func(idx int, batchRefs []ScriptRef) {
			defer wg.Done()

			// Check context before acquiring semaphore.
			select {
			case <-ctx.Done():
				results[idx] = batchResult{err: fmt.Errorf("LLM analysis cancelled: %w", ctx.Err())}
				return
			default:
			}

			// Acquire semaphore slot.
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = batchResult{err: fmt.Errorf("LLM analysis cancelled: %w", ctx.Err())}
				return
			}

			findings, err := c.analyzeBatch(ctx, batchRefs)
			results[idx] = batchResult{findings: findings, err: err}
		}(i, batch)
	}

	wg.Wait()

	var allFindings []Finding
	var errs []error

	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
		}
		allFindings = append(allFindings, r.findings...)
	}

	if len(errs) > 0 && len(allFindings) == 0 {
		return nil, fmt.Errorf("all LLM analysis batches failed: %w", errors.Join(errs...))
	}

	return allFindings, nil
}

// analyzeBatch sends a single batch of scripts to the LLM and parses findings.
func (c *LLMChecker) analyzeBatch(ctx context.Context, batch []ScriptRef) ([]Finding, error) {
	userPrompt := buildUserPrompt(batch)

	raw, err := c.completer.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	parsed, err := parseFindings(raw)
	if err != nil {
		return nil, err
	}

	return convertBatchFindings(parsed, batch), nil
}

// batchScripts groups prepared scripts into batches respecting character and
// count limits. Each batch targets maxBatchChars total script content and at
// most maxScriptsPerBatch scripts.
func batchScripts(prepared []ScriptRef) [][]ScriptRef {
	if len(prepared) == 0 {
		return nil
	}

	maxBatches := (len(prepared) + maxScriptsPerBatch - 1) / maxScriptsPerBatch
	batches := make([][]ScriptRef, 0, maxBatches)
	current := make([]ScriptRef, 0, maxScriptsPerBatch)
	currentChars := 0

	for i := range prepared {
		contentLen := len(prepared[i].Content())

		// Start a new batch if adding this script would exceed limits.
		if len(current) > 0 && (currentChars+contentLen > maxBatchChars || len(current) >= maxScriptsPerBatch) {
			batches = append(batches, current)
			current = make([]ScriptRef, 0, maxScriptsPerBatch)
			currentChars = 0
		}

		current = append(current, prepared[i])
		currentChars += contentLen
	}

	// Append final batch.
	if len(current) > 0 {
		batches = append(batches, current)
	}

	return batches
}
