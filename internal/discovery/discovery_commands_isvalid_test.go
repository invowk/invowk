// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"testing"
)

func TestSourceID_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value SourceID
		want  string
	}{
		{SourceIDInvowkfile, "invowkfile"},
		{SourceID("foo"), "foo"},
		{SourceID("io.invowk.sample"), "io.invowk.sample"},
		{SourceID(""), ""},
	}

	for _, tt := range tests {
		if got := tt.value.String(); got != tt.want {
			t.Errorf("SourceID(%q).String() = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestSourceID_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   SourceID
		wantOK  bool
		wantErr bool
	}{
		{"invowkfile constant", SourceIDInvowkfile, true, false},
		{"simple name", SourceID("foo"), true, false},
		{"name with dot", SourceID("io.invowk.sample"), true, false},
		{"name with hyphen", SourceID("my-module"), true, false},
		{"name with underscore", SourceID("my_module"), true, false},
		{"mixed characters", SourceID("foo-bar.baz_42"), true, false},
		{"single letter", SourceID("x"), true, false},
		{"uppercase", SourceID("MyModule"), true, false},
		{"empty string", SourceID(""), false, true},
		{"starts with digit", SourceID("123abc"), false, true},
		{"contains space", SourceID("foo bar"), false, true},
		{"starts with dot", SourceID(".hidden"), false, true},
		{"starts with hyphen", SourceID("-invalid"), false, true},
		{"contains at sign", SourceID("@invalid"), false, true},
		{"contains slash", SourceID("path/to"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("SourceID(%q).Validate() error = %v, wantOK %v", tt.value, err, tt.wantOK)
			}

			if tt.wantErr {
				if err == nil {
					t.Error("expected validation error, got nil")
				} else if !errors.Is(err, ErrInvalidSourceID) {
					t.Errorf("expected errors.Is(err, ErrInvalidSourceID), got %v", err)
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}
