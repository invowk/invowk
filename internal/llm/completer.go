// SPDX-License-Identifier: MPL-2.0

package llm

import (
	"context"
	"errors"
)

const structuredOutputUnsupportedErrMsg = "structured output unsupported"

// ErrStructuredOutputUnsupported is returned by completers that do not
// support schema-constrained output for the selected backend.
var ErrStructuredOutputUnsupported = errors.New(structuredOutputUnsupportedErrMsg)

type (
	// Completer abstracts one prompt-completion conversation with an LLM backend.
	//nolint:iface // Shared port package; consumers own the useful boundary.
	Completer interface {
		Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	}

	// StructuredCompleter is an optional completer capability for providers that
	// can ask the model for schema-constrained JSON output.
	//nolint:iface // Optional outside-device capability consumed by use cases.
	StructuredCompleter interface {
		CompleteJSONSchema(ctx context.Context, systemPrompt, userPrompt string, format JSONSchemaFormat) (string, error)
	}

	// JSONSchemaFormat describes the response schema requested from a structured
	// completion backend.
	JSONSchemaFormat struct {
		Name        string
		Description string
		Schema      any
		Strict      bool
	}
)
