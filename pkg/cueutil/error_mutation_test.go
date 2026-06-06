// SPDX-License-Identifier: MPL-2.0

package cueutil

import (
	"fmt"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/token"
)

type syntheticCUEError struct {
	path []string
	msg  string
}

func TestFormatErrorMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("single CUE error trims redundant path prefix", func(t *testing.T) {
		t.Parallel()

		err := FormatError(cueValidationError(t, `container: auto_provision: enabled: bool & "yes"`), "config.cue")
		want := `config.cue: container.auto_provision.enabled: conflicting values bool and "yes" (mismatched types bool and string)`
		if got := err.Error(); got != want {
			t.Fatalf("FormatError() = %q, want %q", got, want)
		}
	})

	t.Run("multiple CUE errors use validation block", func(t *testing.T) {
		t.Parallel()

		err := FormatError(cueValidationError(t, `name: string & 1
count: int & "bad"`), "invowkfile.cue")
		want := `invowkfile.cue: validation failed:
  name: conflicting values string and 1 (mismatched types string and int)
  count: conflicting values int and "bad" (mismatched types int and string)`
		if got := err.Error(); got != want {
			t.Fatalf("FormatError() = %q, want %q", got, want)
		}
	})

	t.Run("pathless CUE error keeps message as single line", func(t *testing.T) {
		t.Parallel()

		err := FormatError(cueValidationError(t, `1 & "x"`), "expr.cue")
		want := `expr.cue: conflicting values 1 and "x" (mismatched types int and string)`
		if got := err.Error(); got != want {
			t.Fatalf("FormatError() = %q, want %q", got, want)
		}
	})

	t.Run("pathful CUE error keeps non-prefixed leading colon message", func(t *testing.T) {
		t.Parallel()

		err := FormatError(syntheticCUEError{
			path: []string{"cmds", "0", "name"},
			msg:  ": upstream error already omitted path",
		}, "invowkfile.cue")
		want := `invowkfile.cue: cmds[0].name: : upstream error already omitted path`
		if got := err.Error(); got != want {
			t.Fatalf("FormatError() = %q, want %q", got, want)
		}
	})

	t.Run("pathless CUE error keeps leading colon message", func(t *testing.T) {
		t.Parallel()

		err := FormatError(syntheticCUEError{msg: ": root expression failed"}, "expr.cue")
		want := `expr.cue: : root expression failed`
		if got := err.Error(); got != want {
			t.Fatalf("FormatError() = %q, want %q", got, want)
		}
	})
}

func TestFormatPathMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path []string
		want string
	}{
		{
			name: "numeric first segment is a field name",
			path: []string{"0", "name"},
			want: "0.name",
		},
		{
			name: "numeric segment after field is an index",
			path: []string{"cmds", "10", "name"},
			want: "cmds[10].name",
		},
		{
			name: "nine remains a valid index digit",
			path: []string{"cmds", "9", "name"},
			want: "cmds[9].name",
		},
		{
			name: "punctuation below zero is a field segment",
			path: []string{"cmds", "/", "name"},
			want: "cmds./.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := formatPath(tt.path); got != tt.want {
				t.Fatalf("formatPath(%v) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func cueValidationError(t *testing.T, source string) error {
	t.Helper()

	ctx := cuecontext.New()
	value := ctx.CompileString(source)
	err := value.Validate(cue.Concrete(true))
	if err == nil {
		err = value.Err()
	}
	if err == nil {
		t.Fatalf("CUE source %q produced nil error", source)
	}
	return fmt.Errorf("build CUE validation error: %w", err)
}

func (e syntheticCUEError) Position() token.Pos {
	return token.NoPos
}

func (e syntheticCUEError) InputPositions() []token.Pos {
	return nil
}

func (e syntheticCUEError) Error() string {
	return e.msg
}

func (e syntheticCUEError) Path() []string {
	return e.path
}

func (e syntheticCUEError) Msg() (format string, args []any) {
	return e.msg, nil
}
