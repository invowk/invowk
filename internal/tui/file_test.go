// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewFileModel(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:             "Select a file",
		Description:       "Choose a configuration file",
		CurrentDirectory:  "/tmp",
		AllowedExtensions: []string{".json", ".yaml"},
		ShowHidden:        true,
		ShowSize:          true,
		ShowPermissions:   true,
		Height:            15,
		FileAllowed:       true,
		DirAllowed:        false,
		Config:            DefaultConfig(),
	}

	model := NewFileModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if model.IsDone() {
		t.Error("expected model not to be done initially")
	}
	if model.Cancelled() {
		t.Error("expected model not to be cancelled initially")
	}
}

func TestNewFileModel_Defaults(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:  "Select",
		Config: DefaultConfig(),
	}

	model := NewFileModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Model should be created even with minimal options
}

func TestFileModel_CancelWithEsc(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:       "Test",
		FileAllowed: true,
		Config:      DefaultConfig(),
	}

	model := NewFileModel(opts)

	// Simulate Esc key press
	keyMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*fileModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Esc")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Esc")
	}

	// Result should return ErrCancelled
	_, err := m.Result()
	if !errors.Is(err, ErrCancelled) {
		t.Errorf("expected ErrCancelled, got %v", err)
	}
}

func TestFileModel_CancelWithCtrlC(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:       "Test",
		FileAllowed: true,
		Config:      DefaultConfig(),
	}

	model := NewFileModel(opts)

	// Simulate Ctrl+C key press
	keyMsg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*fileModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Ctrl+C")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Ctrl+C")
	}
}

func TestFileModel_SetSize(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:       "Test",
		FileAllowed: true,
		Config:      DefaultConfig(),
	}

	model := NewFileModel(opts)
	model.SetSize(100, 30)

	if model.width != 100 {
		t.Errorf("expected width 100, got %d", model.width)
	}
	if model.height != 30 {
		t.Errorf("expected height 30, got %d", model.height)
	}
}

func TestFileModel_ViewWhenDone(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:       "Test",
		FileAllowed: true,
		Config:      DefaultConfig(),
	}

	model := NewFileModel(opts)
	model.done = true

	view := model.View().Content

	if view != "" {
		t.Errorf("expected empty view when done, got %q", view)
	}
}

func TestFileModel_ViewWithWidth(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:       "Test",
		FileAllowed: true,
		Config:      DefaultConfig(),
	}

	model := NewFileModel(opts)
	model.SetSize(60, 15)

	view := model.View().Content

	// View should be non-empty when not done
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestFileModel_Init(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:       "Test",
		FileAllowed: true,
		Config:      DefaultConfig(),
	}

	model := NewFileModel(opts)
	cmd := model.Init()

	// Init should return a command from the underlying form
	_ = cmd
}

func TestNewFileModelForModal(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:       "Modal File Picker",
		FileAllowed: true,
		Config:      DefaultConfig(),
	}

	model := NewFileModelForModal(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestFileBuilder_FluentAPI(t *testing.T) {
	t.Parallel()

	builder := NewFile().
		Title("Choose File").
		Description("Select a configuration file").
		CurrentDirectory("/home/user").
		AllowedExtensions(".json", ".yaml", ".toml").
		ShowHidden(true).
		ShowSize(true).
		ShowPermissions(true).
		Height(20).
		FileAllowed(true).
		DirAllowed(false).
		Theme(ThemeCharm).
		Accessible(true)

	if builder.opts.Title != "Choose File" {
		t.Errorf("expected title 'Choose File', got %q", builder.opts.Title)
	}
	if builder.opts.Description != "Select a configuration file" {
		t.Errorf("expected description, got %q", builder.opts.Description)
	}
	if builder.opts.CurrentDirectory != "/home/user" {
		t.Errorf("expected directory '/home/user', got %q", builder.opts.CurrentDirectory)
	}
	if len(builder.opts.AllowedExtensions) != 3 {
		t.Errorf("expected 3 extensions, got %d", len(builder.opts.AllowedExtensions))
	}
	if !builder.opts.ShowHidden {
		t.Error("expected show hidden to be true")
	}
	if !builder.opts.ShowSize {
		t.Error("expected show size to be true")
	}
	if !builder.opts.ShowPermissions {
		t.Error("expected show permissions to be true")
	}
	if builder.opts.Height != 20 {
		t.Errorf("expected height 20, got %d", builder.opts.Height)
	}
	if !builder.opts.FileAllowed {
		t.Error("expected file allowed to be true")
	}
	if builder.opts.DirAllowed {
		t.Error("expected dir allowed to be false")
	}
	if builder.opts.Config.Theme != ThemeCharm {
		t.Errorf("expected theme ThemeCharm, got %v", builder.opts.Config.Theme)
	}
	if !builder.opts.Config.Accessible {
		t.Error("expected accessible mode to be enabled")
	}
}

func TestFileBuilder_DefaultValues(t *testing.T) {
	t.Parallel()

	builder := NewFile()

	// Default should allow files
	if !builder.opts.FileAllowed {
		t.Error("expected file allowed to default to true")
	}
	// Default should not allow directories
	if builder.opts.DirAllowed {
		t.Error("expected dir allowed to default to false")
	}
}

func TestFileBuilder_Model(t *testing.T) {
	t.Parallel()

	builder := NewFile().
		Title("Test")

	model := builder.Model()

	if model == nil {
		t.Fatal("expected non-nil model from builder")
	}
	if model.IsDone() {
		t.Error("expected model not to be done")
	}
}

func TestFileOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:             "File Picker",
		Description:       "Pick a file",
		CurrentDirectory:  "/var/log",
		AllowedExtensions: []string{".log", ".txt"},
		ShowHidden:        true,
		ShowSize:          true,
		ShowPermissions:   false,
		Height:            18,
		FileAllowed:       true,
		DirAllowed:        true,
		Config: Config{
			Theme:      ThemeDracula,
			Accessible: true,
		},
	}

	if opts.Title != "File Picker" {
		t.Errorf("expected title 'File Picker', got %q", opts.Title)
	}
	if opts.Description != "Pick a file" {
		t.Errorf("expected description 'Pick a file', got %q", opts.Description)
	}
	if opts.CurrentDirectory != "/var/log" {
		t.Errorf("expected directory '/var/log', got %q", opts.CurrentDirectory)
	}
	if len(opts.AllowedExtensions) != 2 {
		t.Errorf("expected 2 extensions, got %d", len(opts.AllowedExtensions))
	}
	if !opts.ShowHidden {
		t.Error("expected show hidden to be true")
	}
	if !opts.ShowSize {
		t.Error("expected show size to be true")
	}
	if opts.ShowPermissions {
		t.Error("expected show permissions to be false")
	}
	if opts.Height != 18 {
		t.Errorf("expected height 18, got %d", opts.Height)
	}
	if !opts.FileAllowed {
		t.Error("expected file allowed to be true")
	}
	if !opts.DirAllowed {
		t.Error("expected dir allowed to be true")
	}
	if opts.Config.Theme != ThemeDracula {
		t.Errorf("expected theme ThemeDracula, got %v", opts.Config.Theme)
	}
	if !opts.Config.Accessible {
		t.Error("expected accessible to be true")
	}
}

func TestNewFileModel_DirOnly(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:       "Select Directory",
		FileAllowed: false,
		DirAllowed:  true,
		Config:      DefaultConfig(),
	}

	model := NewFileModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestNewFileModel_BothFileAndDir(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:       "Select File or Directory",
		FileAllowed: true,
		DirAllowed:  true,
		Config:      DefaultConfig(),
	}

	model := NewFileModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestNewFileModel_NeitherFileNorDir(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:       "Neither",
		FileAllowed: false,
		DirAllowed:  false,
		Config:      DefaultConfig(),
	}

	model := NewFileModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// When neither is specified, file should default to allowed
}

func TestNewFileModel_WithExtensions(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:             "Select Config",
		AllowedExtensions: []string{".json", ".yaml", ".yml", ".toml"},
		FileAllowed:       true,
		Config:            DefaultConfig(),
	}

	model := NewFileModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestNewFileModel_WithAllOptions(t *testing.T) {
	t.Parallel()

	opts := FileOptions{
		Title:             "Full Options",
		Description:       "All options enabled",
		CurrentDirectory:  "/",
		AllowedExtensions: []string{".go"},
		ShowHidden:        true,
		ShowSize:          true,
		ShowPermissions:   true,
		Height:            25,
		FileAllowed:       true,
		DirAllowed:        true,
		Config:            DefaultConfig(),
	}

	model := NewFileModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestFileBuilder_DirAllowed(t *testing.T) {
	t.Parallel()

	builder := NewFile().
		Title("Directory Picker").
		FileAllowed(false).
		DirAllowed(true)

	if builder.opts.FileAllowed {
		t.Error("expected file allowed to be false")
	}
	if !builder.opts.DirAllowed {
		t.Error("expected dir allowed to be true")
	}
}

func TestFileBuilder_ShowOptions(t *testing.T) {
	t.Parallel()

	builder := NewFile().
		ShowHidden(true).
		ShowSize(true).
		ShowPermissions(true)

	if !builder.opts.ShowHidden {
		t.Error("expected show hidden to be true")
	}
	if !builder.opts.ShowSize {
		t.Error("expected show size to be true")
	}
	if !builder.opts.ShowPermissions {
		t.Error("expected show permissions to be true")
	}
}

func TestFileBuilder_Height(t *testing.T) {
	t.Parallel()

	builder := NewFile().
		Height(30)

	if builder.opts.Height != 30 {
		t.Errorf("expected height 30, got %d", builder.opts.Height)
	}
}
