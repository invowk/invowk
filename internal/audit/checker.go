// SPDX-License-Identifier: MPL-2.0

package audit

import "context"

// Checker analyzes a ScanContext and produces zero or more security findings.
// Implementations must be safe for concurrent use — checkers run in parallel
// and share the same read-only ScanContext.
//
// Each checker focuses on a single security category (integrity, exfiltration,
// etc.) and reports findings with appropriate severity levels. Checkers should
// check ctx.Done() before expensive operations to support cancellation.
type Checker interface {
	// Name returns a short identifier for this checker (e.g., "lockfile", "script").
	Name() string
	// Category returns the primary security category this checker covers.
	Category() Category
	// Check analyzes the scan context and returns findings.
	// Returns nil findings and nil error when nothing is detected.
	Check(ctx context.Context, sc *ScanContext) ([]Finding, error)
}
