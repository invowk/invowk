// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"errors"
	"testing"
)

func TestComponent_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		comp Component
		want string
	}{
		{ComponentInput, "input"},
		{ComponentConfirm, "confirm"},
		{ComponentChoose, "choose"},
		{ComponentFilter, "filter"},
		{ComponentFile, "file"},
		{ComponentWrite, "write"},
		{ComponentTextArea, "textarea"},
		{ComponentSpin, "spin"},
		{ComponentPager, "pager"},
		{ComponentTable, "table"},
		{Component("custom"), "custom"},
		{Component(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.comp.String()
			if got != tt.want {
				t.Errorf("Component(%q).String() = %q, want %q", tt.comp, got, tt.want)
			}
		})
	}
}

func TestComponent_IsValid(t *testing.T) {
	t.Parallel()

	validComponents := []Component{
		ComponentInput, ComponentConfirm, ComponentChoose, ComponentFilter,
		ComponentFile, ComponentWrite, ComponentTextArea, ComponentSpin,
		ComponentPager, ComponentTable,
	}

	for _, comp := range validComponents {
		t.Run(string(comp), func(t *testing.T) {
			t.Parallel()
			isValid, errs := comp.IsValid()
			if !isValid {
				t.Errorf("Component(%q).IsValid() = false, want true", comp)
			}
			if len(errs) > 0 {
				t.Errorf("Component(%q).IsValid() returned unexpected errors: %v", comp, errs)
			}
		})
	}

	invalidComponents := []Component{"", "invalid", "INPUT", "unknown"}
	for _, comp := range invalidComponents {
		t.Run("invalid_"+string(comp), func(t *testing.T) {
			t.Parallel()
			isValid, errs := comp.IsValid()
			if isValid {
				t.Errorf("Component(%q).IsValid() = true, want false", comp)
			}
			if len(errs) == 0 {
				t.Fatalf("Component(%q).IsValid() returned no errors, want error", comp)
			}
			if !errors.Is(errs[0], ErrInvalidComponent) {
				t.Errorf("error should wrap ErrInvalidComponent, got: %v", errs[0])
			}
		})
	}
}
