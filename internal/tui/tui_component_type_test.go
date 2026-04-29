// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/tuiserver"
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

func TestCreateEmbeddableComponent_SpinIsRenderOnly(t *testing.T) {
	t.Parallel()

	options, err := json.Marshal(map[string]any{
		"title":   "loading",
		"command": []string{"should-not-run"},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	component, err := CreateEmbeddableComponent(ComponentTypeSpin, options, 80, 24)
	if err != nil {
		t.Fatalf("CreateEmbeddableComponent() error = %v", err)
	}
	model, ok := component.(*spinModel)
	if !ok {
		t.Fatalf("component type = %T, want *spinModel", component)
	}
	if !model.done || model.run != nil {
		t.Fatal("delegated spin component should be render-only and must not carry a command")
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

func TestComponentTypeMatchesServerProtocol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		client ComponentType
		server tuiserver.Component
	}{
		{"input", ComponentTypeInput, tuiserver.ComponentInput},
		{"confirm", ComponentTypeConfirm, tuiserver.ComponentConfirm},
		{"choose", ComponentTypeChoose, tuiserver.ComponentChoose},
		{"filter", ComponentTypeFilter, tuiserver.ComponentFilter},
		{"file", ComponentTypeFile, tuiserver.ComponentFile},
		{"write", ComponentTypeWrite, tuiserver.ComponentWrite},
		{"textarea", ComponentTypeTextArea, tuiserver.ComponentTextArea},
		{"spin", ComponentTypeSpin, tuiserver.ComponentSpin},
		{"pager", ComponentTypePager, tuiserver.ComponentPager},
		{"table", ComponentTypeTable, tuiserver.ComponentTable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if string(tt.client) != string(tt.server) {
				t.Fatalf("client component = %q, server component = %q", tt.client, tt.server)
			}
		})
	}
}
