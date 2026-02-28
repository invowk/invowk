// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestComponentType_Validate(t *testing.T) {
	t.Parallel()

	validTypes := []ComponentType{
		ComponentTypeInput, ComponentTypeConfirm, ComponentTypeChoose, ComponentTypeFilter,
		ComponentTypeFile, ComponentTypeWrite, ComponentTypeTextArea, ComponentTypeSpin,
		ComponentTypePager, ComponentTypeTable,
	}

	for _, ct := range validTypes {
		t.Run(string(ct), func(t *testing.T) {
			t.Parallel()
			err := ct.Validate()
			if err != nil {
				t.Errorf("ComponentType(%q).Validate() returned unexpected error: %v", ct, err)
			}
		})
	}

	invalidTypes := []ComponentType{"", "invalid", "INPUT"}
	for _, ct := range invalidTypes {
		t.Run("invalid_"+string(ct), func(t *testing.T) {
			t.Parallel()
			err := ct.Validate()
			if err == nil {
				t.Fatalf("ComponentType(%q).Validate() returned nil, want error", ct)
			}
			if !errors.Is(err, ErrInvalidComponentType) {
				t.Errorf("error should wrap ErrInvalidComponentType, got: %v", err)
			}
		})
	}
}

func TestComponentType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ct   ComponentType
		want string
	}{
		{ComponentTypeInput, "input"},
		{ComponentTypeConfirm, "confirm"},
		{ComponentTypeChoose, "choose"},
		{ComponentTypeFilter, "filter"},
		{ComponentTypeFile, "file"},
		{ComponentTypeWrite, "write"},
		{ComponentTypeTextArea, "textarea"},
		{ComponentTypeSpin, "spin"},
		{ComponentTypePager, "pager"},
		{ComponentTypeTable, "table"},
		{"custom", "custom"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.ct.String()
			if got != tt.want {
				t.Errorf("ComponentType(%q).String() = %q, want %q", tt.ct, got, tt.want)
			}
		})
	}
}

func TestComponentType_String_FmtStringer(t *testing.T) {
	t.Parallel()

	// Verify ComponentType implements fmt.Stringer.
	got := ComponentTypeInput.String()
	if got != "input" {
		t.Errorf("ComponentTypeInput.String() = %q, want %q", got, "input")
	}
}
