// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditClaudeReferences_NoFindings(t *testing.T) {
	repoRoot := t.TempDir()
	claudePath := filepath.Join(repoRoot, ".claude", "CLAUDE.md")
	writeTestFile(t, claudePath, `# Overview

## Rules for Agents (Critical)

**Index / Sync Map (must match .claude/rules/):**
- [.claude/rules/general-rules.md](.claude/rules/general-rules.md) - General rules.
`)

	writeTestFile(t, filepath.Join(repoRoot, ".claude", "rules", "general-rules.md"), "# General Rules\n")

	findings, err := AuditClaudeReferences(context.Background(), repoRoot)
	if err != nil {
		t.Fatalf("audit error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %#v", len(findings), findings)
	}
}

func TestAuditClaudeReferences_MissingIndexEntry(t *testing.T) {
	repoRoot := t.TempDir()
	claudePath := filepath.Join(repoRoot, ".claude", "CLAUDE.md")
	writeTestFile(t, claudePath, `# Overview

## Rules for Agents (Critical)

**Index / Sync Map (must match .claude/rules/):**
- [.claude/rules/general-rules.md](.claude/rules/general-rules.md) - General rules.
`)

	writeTestFile(t, filepath.Join(repoRoot, ".claude", "rules", "general-rules.md"), "# General Rules\n")
	writeTestFile(t, filepath.Join(repoRoot, ".claude", "rules", "extra.md"), "# Extra Rules\n")

	findings, err := AuditClaudeReferences(context.Background(), repoRoot)
	if err != nil {
		t.Fatalf("audit error: %v", err)
	}
	if !hasFindingSummary(findings, "Rules index missing entry") {
		t.Fatalf("expected missing index entry finding")
	}
}

func TestAuditClaudeReferences_BrokenLink(t *testing.T) {
	repoRoot := t.TempDir()
	claudePath := filepath.Join(repoRoot, ".claude", "CLAUDE.md")
	writeTestFile(t, claudePath, `# Overview

## Rules for Agents (Critical)

**Index / Sync Map (must match .claude/rules/):**
- [.claude/rules/general-rules.md](.claude/rules/general-rules.md) - General rules.
`)

	writeTestFile(t, filepath.Join(repoRoot, ".claude", "rules", "general-rules.md"), "See [Missing](internal/missing.md).\n")

	findings, err := AuditClaudeReferences(context.Background(), repoRoot)
	if err != nil {
		t.Fatalf("audit error: %v", err)
	}
	if !hasFindingSummary(findings, "Broken reference to internal/missing.md") {
		t.Fatalf("expected broken reference finding")
	}
}

func writeTestFile(t testing.TB, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func hasFindingSummary(findings []Finding, contains string) bool {
	for i := range findings {
		if strings.Contains(findings[i].Summary, contains) {
			return true
		}
	}
	return false
}
