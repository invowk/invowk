// SPDX-License-Identifier: MPL-2.0

package llm

import "context"

type (
	// Completer abstracts one prompt-completion conversation with an LLM backend.
	//nolint:iface // Shared port package; consumers own the useful boundary.
	Completer interface {
		Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	}
)
