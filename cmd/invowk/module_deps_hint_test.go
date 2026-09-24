// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestLockIntegrityHint(t *testing.T) {
	t.Parallel()

	const key = invowkmod.ModuleRefKey("https://github.com/org/tools.git")
	commitErr := fmt.Errorf("resolving: %w", &invowkmod.LockedCommitMismatchError{ModuleKey: key, Version: "1.2.3"})
	hashErr := fmt.Errorf("failed to cache module: module cache /c: %w", &invowkmod.ContentHashMismatchError{ModuleKey: key, Version: "1.2.3"})

	tests := []struct {
		name string
		err  error
		want []string
	}{
		{name: "re-pointed tag", err: commitErr, want: []string{"different commit", "invowk module remove " + string(key)}},
		{name: "content mismatch", err: hashErr, want: []string{"module cache directory", "invowk module remove " + string(key)}},
		{name: "unrelated error", err: errors.New("network down")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hint := lockIntegrityHint(tt.err)
			if len(tt.want) == 0 {
				if hint != "" {
					t.Fatalf("lockIntegrityHint() = %q, want empty", hint)
				}
				return
			}
			for _, want := range tt.want {
				if !strings.Contains(hint, want) {
					t.Errorf("lockIntegrityHint() = %q, missing %q", hint, want)
				}
			}
		})
	}
}
