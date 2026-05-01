// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"encoding/json"
	"errors"
	"strings"
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

func TestCreateEmbeddableComponent_InputUsesWireRequest(t *testing.T) {
	t.Parallel()

	options, err := json.Marshal(tuiwire.InputRequest{
		Title:       "name",
		Description: "enter a name",
		Placeholder: "Ada",
		Value:       "Lovelace",
		CharLimit:   12,
		Password:    true,
		Prompt:      "$ ",
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	component, err := CreateEmbeddableComponent(ComponentTypeInput, options, 80, 24)
	if err != nil {
		t.Fatalf("CreateEmbeddableComponent() error = %v", err)
	}
	model, ok := component.(*inputModel)
	if !ok {
		t.Fatalf("component type = %T, want *inputModel", component)
	}
	if model.input.CharLimit != 12 {
		t.Fatalf("char limit = %d, want 12", model.input.CharLimit)
	}
	if model.input.Placeholder != "Ada" {
		t.Fatalf("placeholder = %q, want Ada", model.input.Placeholder)
	}
	if model.input.Value() != "Lovelace" {
		t.Fatalf("value = %q, want Lovelace", model.input.Value())
	}
	if model.input.Prompt != "$ " {
		t.Fatalf("prompt = %q, want '$ '", model.input.Prompt)
	}
}

func TestCreateEmbeddableComponent_FilterUsesWireRequest(t *testing.T) {
	t.Parallel()

	options, err := json.Marshal(tuiwire.FilterRequest{
		Title:       "pick",
		Options:     []string{"one", "two"},
		Limit:       3,
		NoLimit:     true,
		Placeholder: "search",
		Height:      6,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	component, err := CreateEmbeddableComponent(ComponentTypeFilter, options, 80, 24)
	if err != nil {
		t.Fatalf("CreateEmbeddableComponent() error = %v", err)
	}
	model, ok := component.(*filterModel)
	if !ok {
		t.Fatalf("component type = %T, want *filterModel", component)
	}
	if !model.noLimit {
		t.Fatal("no_limit should be preserved")
	}
	if model.limit != 3 {
		t.Fatalf("limit = %d, want 3", model.limit)
	}
	if model.list.FilterInput.Placeholder != "search" {
		t.Fatalf("placeholder = %q, want search", model.list.FilterInput.Placeholder)
	}
}

func TestCreateEmbeddableComponent_TextAreaUsesWireRequest(t *testing.T) {
	t.Parallel()

	options, err := json.Marshal(tuiwire.TextAreaRequest{
		Title:           "body",
		Placeholder:     "type",
		Value:           "initial",
		CharLimit:       20,
		ShowLineNumbers: true,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	component, err := CreateEmbeddableComponent(ComponentTypeTextArea, options, 80, 24)
	if err != nil {
		t.Fatalf("CreateEmbeddableComponent() error = %v", err)
	}
	model, ok := component.(*writeModel)
	if !ok {
		t.Fatalf("component type = %T, want *writeModel", component)
	}
	if model.textarea.CharLimit != 20 {
		t.Fatalf("char limit = %d, want 20", model.textarea.CharLimit)
	}
	if !model.textarea.ShowLineNumbers {
		t.Fatal("show_line_numbers should be preserved")
	}
	if model.textarea.Value() != "initial" {
		t.Fatalf("value = %q, want initial", model.textarea.Value())
	}
}

func TestCreateEmbeddableComponent_PagerUsesWireRequest(t *testing.T) {
	t.Parallel()

	options, err := json.Marshal(tuiwire.PagerRequest{
		Title:       "manual",
		Content:     "body",
		ShowLineNum: true,
		SoftWrap:    true,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	component, err := CreateEmbeddableComponent(ComponentTypePager, options, 80, 24)
	if err != nil {
		t.Fatalf("CreateEmbeddableComponent() error = %v", err)
	}
	model, ok := component.(*pagerModel)
	if !ok {
		t.Fatalf("component type = %T, want *pagerModel", component)
	}
	if model.title != "manual" {
		t.Fatalf("title = %q, want manual", model.title)
	}
	if !strings.Contains(model.viewport.View(), "body") {
		t.Fatalf("content = %q, want body", model.viewport.View())
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
