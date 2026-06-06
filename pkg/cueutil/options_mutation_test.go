// SPDX-License-Identifier: MPL-2.0

package cueutil

import (
	"errors"
	"testing"
)

func TestCUEPathMutationErrorPreservesValue(t *testing.T) {
	t.Parallel()

	const input = CUEPath(" \t ")
	err := input.Validate()
	var invalidErr *InvalidCUEPathError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("CUEPath(%q).Validate() error = %v, want InvalidCUEPathError", input, err)
	}
	if invalidErr.Value != input {
		t.Fatalf("InvalidCUEPathError.Value = %q, want %q", invalidErr.Value, input)
	}
}

func TestParseOptionsMutationContracts(t *testing.T) {
	t.Parallel()

	options := defaultOptions()
	WithConcrete(false)(&options)
	if options.concrete {
		t.Fatal("WithConcrete(false) left concrete=true")
	}

	WithConcrete(true)(&options)
	if !options.concrete {
		t.Fatal("WithConcrete(true) left concrete=false")
	}
}
