// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/tuiwire"
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

	options, err := json.Marshal(tuiwire.SpinRequest{
		Title:   "loading",
		Spinner: "dot",
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
	if model.title != "loading" {
		t.Fatalf("spin title = %q, want loading", model.title)
	}
	wantFrames := getSpinnerType(SpinnerDot).Frames
	if len(model.frames) != len(wantFrames) || model.frames[0] != wantFrames[0] {
		t.Fatalf("spin frames = %#v, want spinner dot frames", model.frames)
	}
}

func TestCreateEmbeddableComponent_FileUsesWireRequest(t *testing.T) {
	t.Parallel()

	options, err := json.Marshal(tuiwire.FileRequest{
		Title:       "pick",
		Description: "choose carefully",
		Path:        "/tmp",
		AllowedExts: []string{
			".go",
			".cue",
		},
		ShowHidden: true,
		ShowDirs:   true,
		ShowFiles:  false,
		Height:     7,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	component, err := CreateEmbeddableComponent(ComponentTypeFile, options, 80, 24)
	if err != nil {
		t.Fatalf("CreateEmbeddableComponent() error = %v", err)
	}
	model, ok := component.(*fileModel)
	if !ok {
		t.Fatalf("component type = %T, want *fileModel", component)
	}
	if model.picker.CurrentDirectory != "/tmp" {
		t.Fatalf("current directory = %q, want /tmp", model.picker.CurrentDirectory)
	}
	if model.picker.FileAllowed {
		t.Fatal("file selection should be disabled from ShowFiles=false")
	}
	if !model.picker.DirAllowed {
		t.Fatal("directory selection should be enabled from ShowDirs=true")
	}
	if len(model.picker.AllowedTypes) != 2 || model.picker.AllowedTypes[0] != ".go" {
		t.Fatalf("allowed types = %#v, want .go/.cue", model.picker.AllowedTypes)
	}
	if !model.picker.ShowHidden {
		t.Fatal("show hidden should be enabled")
	}
}

func TestCreateEmbeddableComponent_TableUsesWireRequest(t *testing.T) {
	t.Parallel()

	options, err := json.Marshal(tuiwire.TableRequest{
		Columns: []string{"name", "version"},
		Rows: [][]string{
			{"invowk", "1.0.0"},
		},
		Widths: []int{12, 8},
		Height: 5,
		Border: "none",
		Print:  true,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	component, err := CreateEmbeddableComponent(ComponentTypeTable, options, 80, 24)
	if err != nil {
		t.Fatalf("CreateEmbeddableComponent() error = %v", err)
	}
	model, ok := component.(*tableModel)
	if !ok {
		t.Fatalf("component type = %T, want *tableModel", component)
	}
	if len(model.rows) != 1 || len(model.rows[0]) != 2 || model.rows[0][0] != "invowk" {
		t.Fatalf("rows = %#v, want mapped wire rows", model.rows)
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
		wire   tuiwire.Component
	}{
		{"input", ComponentTypeInput, tuiwire.ComponentInput},
		{"confirm", ComponentTypeConfirm, tuiwire.ComponentConfirm},
		{"choose", ComponentTypeChoose, tuiwire.ComponentChoose},
		{"filter", ComponentTypeFilter, tuiwire.ComponentFilter},
		{"file", ComponentTypeFile, tuiwire.ComponentFile},
		{"write", ComponentTypeWrite, tuiwire.ComponentWrite},
		{"textarea", ComponentTypeTextArea, tuiwire.ComponentTextArea},
		{"spin", ComponentTypeSpin, tuiwire.ComponentSpin},
		{"pager", ComponentTypePager, tuiwire.ComponentPager},
		{"table", ComponentTypeTable, tuiwire.ComponentTable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if string(tt.client) != string(tt.wire) {
				t.Fatalf("client component = %q, wire component = %q", tt.client, tt.wire)
			}
		})
	}
}
