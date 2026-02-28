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

func TestComponent_Validate(t *testing.T) {
	t.Parallel()

	validComponents := []Component{
		ComponentInput, ComponentConfirm, ComponentChoose, ComponentFilter,
		ComponentFile, ComponentWrite, ComponentTextArea, ComponentSpin,
		ComponentPager, ComponentTable,
	}

	for _, comp := range validComponents {
		t.Run(string(comp), func(t *testing.T) {
			t.Parallel()
			err := comp.Validate()
			if err != nil {
				t.Errorf("Component(%q).Validate() returned unexpected error: %v", comp, err)
			}
		})
	}

	invalidComponents := []Component{"", "invalid", "INPUT", "unknown"}
	for _, comp := range invalidComponents {
		t.Run("invalid_"+string(comp), func(t *testing.T) {
			t.Parallel()
			err := comp.Validate()
			if err == nil {
				t.Fatalf("Component(%q).Validate() returned nil, want error", comp)
			}
			if !errors.Is(err, ErrInvalidComponent) {
				t.Errorf("error should wrap ErrInvalidComponent, got: %v", err)
			}
		})
	}
}
