// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"
)

func TestFormatFieldErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
		errs    []error
		want    string
	}{
		{name: "zero errors", subject: "request", want: "invalid request: 0 field error(s)"},
		{name: "single error", subject: "result", errs: []error{errors.New("bad field")}, want: "invalid result: 1 field error(s)"},
		{name: "multiple errors", subject: "execution context", errs: []error{errors.New("a"), errors.New("b"), errors.New("c")}, want: "invalid execution context: 3 field error(s)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := FormatFieldErrors(tt.subject, tt.errs); got != tt.want {
				t.Errorf("FormatFieldErrors() = %q, want %q", got, tt.want)
			}
		})
	}
}
