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

	tests := []struct {
		name     string
		filename string
		buildErr func(*testing.T) error
		want     string
	}{
		{
			name:     "single CUE error trims redundant path prefix",
			filename: "config.cue",
			buildErr: func(t *testing.T) error {
				t.Helper()

				return cueValidationError(t, `container: auto_provision: enabled: bool & "yes"`)
			},
			want: `config.cue: container.auto_provision.enabled: conflicting values bool and "yes" (mismatched types bool and string)`,
		},
		{
			name:     "multiple CUE errors use validation block",
			filename: "invowkfile.cue",
			buildErr: func(t *testing.T) error {
				t.Helper()
				return cueValidationError(t, "name: string & 1\ncount: int & \"bad\"")
			},
			want: "invowkfile.cue: validation failed:\n  name: conflicting values string and 1 (mismatched types string and int)\n  count: conflicting values int and \"bad\" (mismatched types int and string)",
		},
		{
			name:     "pathless CUE error keeps message as single line",
			filename: "expr.cue",
			buildErr: func(t *testing.T) error {
				t.Helper()
				return cueValidationError(t, `1 & "x"`)
			},
			want: `expr.cue: conflicting values 1 and "x" (mismatched types int and string)`,
		},
		{
			name:     "pathful CUE error keeps non-prefixed leading colon message",
			filename: "invowkfile.cue",
			buildErr: func(*testing.T) error {
				return syntheticCUEError{path: []string{"cmds", "0", "name"}, msg: ": upstream error already omitted path"}
			},
			want: `invowkfile.cue: cmds[0].name: : upstream error already omitted path`,
		},
		{
			name:     "pathless CUE error keeps leading colon message",
			filename: "expr.cue",
			buildErr: func(*testing.T) error { return syntheticCUEError{msg: ": root expression failed"} },
			want:     `expr.cue: : root expression failed`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := FormatError(tt.buildErr(t), tt.filename).Error(); got != tt.want {
				t.Fatalf("FormatError() = %q, want %q", got, tt.want)
			}
		})
	}
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
