import type { Snippet } from '../snippets';

export const securitySnippets = {
  // =============================================================================
  // SECURITY AUDITING
  // =============================================================================

  'security/audit-basic-examples': {
    language: 'bash',
    code: `# Scan current directory
invowk audit

# Scan a specific module
invowk audit ./tools.invowkmod

# Only show high and critical findings
invowk audit --severity high

# JSON output for CI
invowk audit --format json

# Include global modules
invowk audit --include-global`,
  },

  'security/audit-llm-examples': {
    language: 'bash',
    code: `# Auto-detect best available provider (local Ollama first, then cloud)
invowk audit --llm-provider auto

# Use a specific provider (works with OAuth — no API key needed)
invowk audit --llm-provider claude
invowk audit --llm-provider codex
invowk audit --llm-provider gemini

# Override model within a provider
invowk audit --llm-provider claude --llm-model claude-opus-4-6

# Manual configuration (Ollama, LM Studio, or any OpenAI-compatible server)
invowk audit --llm
invowk audit --llm --llm-url http://localhost:1234/v1

# Combined: provider + high severity + JSON
invowk audit --llm-provider auto --severity high --format json`,
  },

  'security/audit-json-example': {
    language: 'bash',
    code: `# Full JSON output
invowk audit --format json

# Parse findings count
invowk audit --format json | jq '.summary.total'

# List finding titles
invowk audit --format json | jq '.findings[] | "[\(.severity)] \(.title)"'

# Check for compound threats
invowk audit --format json | jq '.compound_threats'`,
  },

  // =============================================================================
  // CLI REFERENCE SNIPPETS
  // =============================================================================

  'reference/cli/audit-syntax': {
    language: 'bash',
    code: `invowk audit [path] [flags]`,
  },

  'reference/cli/audit-examples': {
    language: 'bash',
    code: `# Scan current workspace
invowk audit

# Scan a single module
invowk audit ./tools.invowkmod

# Scan with high severity threshold
invowk audit --severity high

# JSON output for CI pipelines
invowk audit --format json --severity high

# Include global modules in scan
invowk audit --include-global

# Auto-detect LLM provider
invowk audit --llm-provider auto

# Use specific provider (Claude Code, Codex CLI, Gemini CLI)
invowk audit --llm-provider claude

# Manual LLM with custom server
invowk audit --llm --llm-url http://localhost:1234/v1`,
  },
} satisfies Record<string, Snippet>;
