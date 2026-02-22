// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
	"time"
)

func TestGlobPattern_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern GlobPattern
		want    bool
		wantErr bool
	}{
		{"simple pattern", GlobPattern("*.go"), true, false},
		{"recursive pattern", GlobPattern("**/*.go"), true, false},
		{"directory pattern", GlobPattern("src/**/*.ts"), true, false},
		{"single file", GlobPattern("Makefile"), true, false},
		{"empty is invalid", GlobPattern(""), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.pattern.IsValid()
			if isValid != tt.want {
				t.Errorf("GlobPattern(%q).IsValid() = %v, want %v", tt.pattern, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("GlobPattern(%q).IsValid() returned no errors, want error", tt.pattern)
				}
				if !errors.Is(errs[0], ErrInvalidGlobPattern) {
					t.Errorf("error should wrap ErrInvalidGlobPattern, got: %v", errs[0])
				}
				var gpErr *InvalidGlobPatternError
				if !errors.As(errs[0], &gpErr) {
					t.Errorf("error should be *InvalidGlobPatternError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("GlobPattern(%q).IsValid() returned unexpected errors: %v", tt.pattern, errs)
			}
		})
	}
}

func TestGlobPattern_String(t *testing.T) {
	t.Parallel()
	g := GlobPattern("**/*.go")
	if g.String() != "**/*.go" {
		t.Errorf("GlobPattern.String() = %q, want %q", g.String(), "**/*.go")
	}
}

func TestParseDebounce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		debounce DurationString
		want     time.Duration
		wantErr  bool
	}{
		{
			name:     "empty string returns zero",
			debounce: "",
			want:     0,
			wantErr:  false,
		},
		{
			name:     "500 milliseconds",
			debounce: "500ms",
			want:     500 * time.Millisecond,
			wantErr:  false,
		},
		{
			name:     "1 second",
			debounce: "1s",
			want:     1 * time.Second,
			wantErr:  false,
		},
		{
			name:     "invalid string returns error",
			debounce: "invalid",
			want:     0,
			wantErr:  true,
		},
		{
			name:     "zero duration returns error",
			debounce: "0s",
			want:     0,
			wantErr:  true,
		},
		{
			name:     "negative duration returns error",
			debounce: "-1s",
			want:     0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := &WatchConfig{Debounce: tt.debounce}
			got, err := w.ParseDebounce()

			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseDebounce() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseDebounce() = %v, want %v", got, tt.want)
			}
		})
	}
}
