// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestComponentType_IsValid(t *testing.T) {
	t.Parallel()

	validTypes := []ComponentType{
		ComponentTypeInput, ComponentTypeConfirm, ComponentTypeChoose, ComponentTypeFilter,
		ComponentTypeFile, ComponentTypeWrite, ComponentTypeTextArea, ComponentTypeSpin,
		ComponentTypePager, ComponentTypeTable,
	}

	for _, ct := range validTypes {
		t.Run(string(ct), func(t *testing.T) {
			t.Parallel()
			isValid, errs := ct.IsValid()
			if !isValid {
				t.Errorf("ComponentType(%q).IsValid() = false, want true", ct)
			}
			if len(errs) > 0 {
				t.Errorf("ComponentType(%q).IsValid() returned unexpected errors: %v", ct, errs)
			}
		})
	}

	invalidTypes := []ComponentType{"", "invalid", "INPUT"}
	for _, ct := range invalidTypes {
		t.Run("invalid_"+string(ct), func(t *testing.T) {
			t.Parallel()
			isValid, errs := ct.IsValid()
			if isValid {
				t.Errorf("ComponentType(%q).IsValid() = true, want false", ct)
			}
			if len(errs) == 0 {
				t.Fatalf("ComponentType(%q).IsValid() returned no errors, want error", ct)
			}
			if !errors.Is(errs[0], ErrInvalidComponentType) {
				t.Errorf("error should wrap ErrInvalidComponentType, got: %v", errs[0])
			}
		})
	}
}
