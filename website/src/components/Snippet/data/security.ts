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
    code: `# Basic LLM audit (requires Ollama running locally)
invowk audit --llm

# Use a larger model for better analysis
invowk audit --llm --llm-model qwen2.5-coder:32b

# Use LM Studio instead of Ollama
invowk audit --llm --llm-url http://localhost:1234/v1

# Use a cloud provider
invowk audit --llm --llm-url https://api.openai.com/v1 --llm-api-key sk-...

# Combined: LLM + high severity + JSON
invowk audit --llm --severity high --format json`,
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

# LLM-powered analysis
invowk audit --llm

# LLM with custom model and server
invowk audit --llm --llm-model qwen2.5-coder:32b --llm-url http://localhost:1234/v1`,
  },
} as const;
