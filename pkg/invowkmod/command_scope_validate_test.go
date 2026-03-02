// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"
)

func TestCommandScope_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		scope     CommandScope
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid scope",
			CommandScope{
				ModuleID:      "io.invowk.sample",
				GlobalModules: map[ModuleID]bool{"global.tools": true},
				DirectDeps:    map[ModuleID]bool{"dep.module": true},
			},
			true, false, 0,
		},
		{
			"valid scope with nil maps",
			CommandScope{
				ModuleID: "mymodule",
			},
			true, false, 0,
		},
		{
			"valid scope with empty maps",
			CommandScope{
				ModuleID:      "mymodule",
				GlobalModules: map[ModuleID]bool{},
				DirectDeps:    map[ModuleID]bool{},
			},
			true, false, 0,
		},
		{
			"invalid module ID (empty)",
			CommandScope{
				ModuleID: "",
			},
			false, true, 1,
		},
		{
			"invalid module ID (bad format)",
			CommandScope{
				ModuleID: "1invalid",
			},
			false, true, 1,
		},
		{
			"zero value (empty module ID)",
			CommandScope{},
			false, true, 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.scope.Validate()
			if (err == nil) != tt.want {
				t.Errorf("CommandScope.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CommandScope.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidCommandScope) {
					t.Errorf("error should wrap ErrInvalidCommandScope, got: %v", err)
				}
				var scopeErr *InvalidCommandScopeError
				if !errors.As(err, &scopeErr) {
					t.Fatalf("error should be *InvalidCommandScopeError, got: %T", err)
				}
				if len(scopeErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(scopeErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("CommandScope.Validate() returned unexpected error: %v", err)
			}
		})
	}
}
