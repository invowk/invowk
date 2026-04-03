// SPDX-License-Identifier: MPL-2.0

// Package audit provides security scanning for invowkfiles and modules.
//
// The audit scanner analyzes invowkfile command definitions, module dependency
// trees, lock file integrity, script content, and filesystem structure to
// detect supply-chain vulnerabilities, script injection, path traversal,
// suspicious patterns, and configuration risks.
//
// # Architecture
//
// The scanner uses a pluggable [Checker] interface. Each checker focuses on a
// specific security category (integrity, path traversal, exfiltration, etc.)
// and operates on an immutable [ScanContext] snapshot of all discovered
// artifacts. Checkers run concurrently; their findings are then passed through
// a [Correlator] that detects compound threats spanning multiple categories.
//
// # File Organization
//
//   - severity.go      Severity enum: iota constants, parse, validate, JSON marshal
//   - types.go         Core value types: Category, Finding, Report
//   - checker.go       Checker interface definition
//   - scan_context.go  Immutable read-only view of discovered artifacts
//   - scanner.go       Scanner orchestrator: context build, checker dispatch, correlation
//   - correlator.go    Compound threat detection and severity escalation
//   - errors.go        Sentinel errors and typed error structs
//   - checks_*.go      Individual checker implementations (one per security category)
package audit
