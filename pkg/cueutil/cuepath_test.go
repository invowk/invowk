// SPDX-License-Identifier: MPL-2.0

package cueutil_test

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/cueutil"
)

func TestCUEPath_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    cueutil.CUEPath
		wantErr bool
	}{
		{name: "valid simple path", path: "cmds", wantErr: false},
		{name: "valid dotted path", path: "cmds[0].name", wantErr: false},
		{name: "valid nested path", path: "env.vars.FOO", wantErr: false},
		{name: "empty string", path: "", wantErr: true},
		{name: "whitespace only", path: "   ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.path.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("CUEPath(%q).Validate() error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, cueutil.ErrInvalidCUEPath) {
				t.Errorf("CUEPath(%q).Validate() error does not wrap ErrInvalidCUEPath", tt.path)
			}
		})
	}
}

func TestCUEPath_String(t *testing.T) {
	t.Parallel()

	path := cueutil.CUEPath("cmds[0].name")
	if got := path.String(); got != "cmds[0].name" {
		t.Errorf("CUEPath.String() = %q, want %q", got, "cmds[0].name")
	}
}
