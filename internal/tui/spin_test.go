// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewSpinModel(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Loading...",
		Command: []string{"echo", "hello"},
		Type:    SpinnerDot,
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if model.IsDone() {
		t.Error("expected model not to be done initially")
	}
	if model.title != "Loading..." {
		t.Errorf("expected title 'Loading...', got %q", model.title)
	}
}

func TestNewSpinModel_EmptyCommand(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "No command",
		Command: []string{}, // Empty command
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Empty command should result in done model
	if !model.done {
		t.Error("expected model to be done with empty command")
	}
}

func TestSpinModel_SetSize(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Test",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)
	model.SetSize(80, 24)

	if model.width != 80 {
		t.Errorf("expected width 80, got %d", model.width)
	}
	if model.height != 24 {
		t.Errorf("expected height 24, got %d", model.height)
	}
}

func TestSpinModel_ViewWhenDone(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Test",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)
	model.done = true

	view := model.View()

	if view != "" {
		t.Errorf("expected empty view when done, got %q", view)
	}
}

func TestSpinModel_ViewWithWidth(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Processing",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)
	model.SetSize(40, 10)

	view := model.View()

	// View should be non-empty when not done
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestSpinModel_Cancelled(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Test",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)

	// Spinner doesn't have a cancel concept in the same way
	if model.Cancelled() {
		t.Error("expected Cancelled to return false")
	}
}

func TestSpinModel_Result(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Test",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)
	model.done = true
	model.result = SpinResult{
		Stdout:   "test output",
		ExitCode: 0,
	}

	result, err := model.Result()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	spinResult, ok := result.(SpinResult)
	if !ok {
		t.Fatalf("expected SpinResult, got %T", result)
	}

	if spinResult.Stdout != "test output" {
		t.Errorf("expected stdout 'test output', got %q", spinResult.Stdout)
	}
	if spinResult.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", spinResult.ExitCode)
	}
}

func TestSpinModel_UpdateTickMsg(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Test",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)
	initialSpinner := model.spinner

	// Simulate tick message
	msg := spinnerTickMsg{}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*spinModel)

	// Spinner frame should advance
	if m.spinner == initialSpinner {
		t.Error("expected spinner frame to advance after tick")
	}
}

func TestSpinModel_UpdateDoneMsg(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Test",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)

	// Simulate done message
	msg := spinnerDoneMsg{
		result: SpinResult{
			Stdout:   "output",
			ExitCode: 0,
		},
	}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*spinModel)

	if !m.IsDone() {
		t.Error("expected model to be done after done message")
	}
	if m.result.Stdout != "output" {
		t.Errorf("expected stdout 'output', got %q", m.result.Stdout)
	}
}

func TestSpinModel_UpdateCtrlC(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Test",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)

	// Simulate Ctrl+C key press
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*spinModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Ctrl+C")
	}
}

func TestSpinModel_Init(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Test",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)
	cmd := model.Init()

	// Init should return a batch command (tick + run)
	if cmd == nil {
		t.Error("expected non-nil cmd from Init")
	}
}

func TestSpinModel_Frames(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Test",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)

	// Should have multiple frames
	if len(model.frames) == 0 {
		t.Error("expected frames to be initialized")
	}
}

func TestParseSpinnerType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected SpinnerType
		wantErr  bool
	}{
		{"line", SpinnerLine, false},
		{"dot", SpinnerDot, false},
		{"minidot", SpinnerMiniDot, false},
		{"jump", SpinnerJump, false},
		{"pulse", SpinnerPulse, false},
		{"points", SpinnerPoints, false},
		{"globe", SpinnerGlobe, false},
		{"moon", SpinnerMoon, false},
		{"monkey", SpinnerMonkey, false},
		{"meter", SpinnerMeter, false},
		{"hamburger", SpinnerHamburger, false},
		{"ellipsis", SpinnerEllipsis, false},
		{"unknown", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result, err := ParseSpinnerType(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseSpinnerType(%q) should return error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSpinnerType(%q) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("ParseSpinnerType(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSpinnerType_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		st      SpinnerType
		want    bool
		wantErr bool
	}{
		{SpinnerLine, true, false},
		{SpinnerDot, true, false},
		{SpinnerMiniDot, true, false},
		{SpinnerJump, true, false},
		{SpinnerPulse, true, false},
		{SpinnerPoints, true, false},
		{SpinnerGlobe, true, false},
		{SpinnerMoon, true, false},
		{SpinnerMonkey, true, false},
		{SpinnerMeter, true, false},
		{SpinnerHamburger, true, false},
		{SpinnerEllipsis, true, false},
		{SpinnerType(99), false, true},
		{SpinnerType(-1), false, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("SpinnerType_%d", tt.st), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.st.IsValid()
			if isValid != tt.want {
				t.Errorf("SpinnerType(%d).IsValid() = %v, want %v", tt.st, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("SpinnerType(%d).IsValid() returned no errors, want error", tt.st)
				}
				if !errors.Is(errs[0], ErrInvalidSpinnerType) {
					t.Errorf("error should wrap ErrInvalidSpinnerType, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("SpinnerType(%d).IsValid() returned unexpected errors: %v", tt.st, errs)
			}
		})
	}
}

func TestSpinnerTypeNames(t *testing.T) {
	t.Parallel()

	names := SpinnerTypeNames()

	expectedNames := []string{
		"line", "dot", "minidot", "jump", "pulse", "points",
		"globe", "moon", "monkey", "meter", "hamburger", "ellipsis",
	}

	if len(names) != len(expectedNames) {
		t.Errorf("expected %d spinner type names, got %d", len(expectedNames), len(names))
	}

	for i, name := range expectedNames {
		if names[i] != name {
			t.Errorf("expected names[%d] = %q, got %q", i, name, names[i])
		}
	}
}

func TestSpinBuilder_FluentAPI(t *testing.T) {
	t.Parallel()

	builder := NewSpin().
		Title("Processing...").
		Type(SpinnerDot).
		Theme(ThemeCharm).
		Accessible(true)

	if builder.opts.Title != "Processing..." {
		t.Errorf("expected title 'Processing...', got %q", builder.opts.Title)
	}
	if builder.opts.Type != SpinnerDot {
		t.Errorf("expected type SpinnerDot, got %d", builder.opts.Type)
	}
	if builder.opts.Config.Theme != ThemeCharm {
		t.Errorf("expected theme ThemeCharm, got %v", builder.opts.Config.Theme)
	}
	if !builder.opts.Config.Accessible {
		t.Error("expected accessible mode to be enabled")
	}
}

func TestSpinBuilder_TypeString(t *testing.T) {
	t.Parallel()

	builder := NewSpin().
		TypeString("globe")

	if builder.opts.Type != SpinnerGlobe {
		t.Errorf("expected type SpinnerGlobe, got %d", builder.opts.Type)
	}
}

func TestSpinBuilder_Action(t *testing.T) {
	t.Parallel()

	called := false
	builder := NewSpin().
		Title("Action test").
		Action(func() {
			called = true
		})

	if builder.action == nil {
		t.Error("expected action to be set")
	}

	// We can't easily test Run without terminal, but action should be set
	_ = called
}

func TestSpinBuilder_DefaultValues(t *testing.T) {
	t.Parallel()

	builder := NewSpin()

	if builder.opts.Type != SpinnerLine {
		t.Errorf("expected default type SpinnerLine, got %d", builder.opts.Type)
	}
}

func TestSpinBuilder_RunNoActionOrContext(t *testing.T) {
	t.Parallel()

	builder := NewSpin().
		Title("Empty")

	// Run with no action or context should return nil
	err := builder.Run()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSpinResult_Fields(t *testing.T) {
	t.Parallel()

	result := SpinResult{
		Stdout:   "standard output",
		Stderr:   "error output",
		ExitCode: 1,
	}

	if result.Stdout != "standard output" {
		t.Errorf("expected stdout 'standard output', got %q", result.Stdout)
	}
	if result.Stderr != "error output" {
		t.Errorf("expected stderr 'error output', got %q", result.Stderr)
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
}

func TestSpinOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := SpinOptions{
		Title: "Loading data",
		Type:  SpinnerPulse,
		Config: Config{
			Theme:      ThemeDracula,
			Accessible: true,
		},
	}

	if opts.Title != "Loading data" {
		t.Errorf("expected title 'Loading data', got %q", opts.Title)
	}
	if opts.Type != SpinnerPulse {
		t.Errorf("expected type SpinnerPulse, got %d", opts.Type)
	}
	if opts.Config.Theme != ThemeDracula {
		t.Errorf("expected theme ThemeDracula, got %v", opts.Config.Theme)
	}
	if !opts.Config.Accessible {
		t.Error("expected accessible to be true")
	}
}

func TestSpinCommandOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Running command",
		Command: []string{"ls", "-la"},
		Type:    SpinnerMoon,
		Config: Config{
			Theme:      ThemeBase16,
			Accessible: false,
		},
	}

	if opts.Title != "Running command" {
		t.Errorf("expected title 'Running command', got %q", opts.Title)
	}
	if len(opts.Command) != 2 {
		t.Errorf("expected 2 command args, got %d", len(opts.Command))
	}
	if opts.Command[0] != "ls" {
		t.Errorf("expected command 'ls', got %q", opts.Command[0])
	}
	if opts.Type != SpinnerMoon {
		t.Errorf("expected type SpinnerMoon, got %d", opts.Type)
	}
	if opts.Config.Theme != ThemeBase16 {
		t.Errorf("expected theme ThemeBase16, got %v", opts.Config.Theme)
	}
}

func TestSpinnerType_Constants(t *testing.T) {
	t.Parallel()

	// Verify spinner type constants are in expected order
	if SpinnerLine != 0 {
		t.Errorf("expected SpinnerLine to be 0, got %d", SpinnerLine)
	}
	if SpinnerDot != 1 {
		t.Errorf("expected SpinnerDot to be 1, got %d", SpinnerDot)
	}
	if SpinnerMiniDot != 2 {
		t.Errorf("expected SpinnerMiniDot to be 2, got %d", SpinnerMiniDot)
	}
	if SpinnerJump != 3 {
		t.Errorf("expected SpinnerJump to be 3, got %d", SpinnerJump)
	}
	if SpinnerPulse != 4 {
		t.Errorf("expected SpinnerPulse to be 4, got %d", SpinnerPulse)
	}
	if SpinnerPoints != 5 {
		t.Errorf("expected SpinnerPoints to be 5, got %d", SpinnerPoints)
	}
	if SpinnerGlobe != 6 {
		t.Errorf("expected SpinnerGlobe to be 6, got %d", SpinnerGlobe)
	}
	if SpinnerMoon != 7 {
		t.Errorf("expected SpinnerMoon to be 7, got %d", SpinnerMoon)
	}
	if SpinnerMonkey != 8 {
		t.Errorf("expected SpinnerMonkey to be 8, got %d", SpinnerMonkey)
	}
	if SpinnerMeter != 9 {
		t.Errorf("expected SpinnerMeter to be 9, got %d", SpinnerMeter)
	}
	if SpinnerHamburger != 10 {
		t.Errorf("expected SpinnerHamburger to be 10, got %d", SpinnerHamburger)
	}
	if SpinnerEllipsis != 11 {
		t.Errorf("expected SpinnerEllipsis to be 11, got %d", SpinnerEllipsis)
	}
}

func TestGetSpinnerType(t *testing.T) {
	t.Parallel()

	// Test that all spinner types map correctly
	types := []SpinnerType{
		SpinnerLine, SpinnerDot, SpinnerMiniDot, SpinnerJump,
		SpinnerPulse, SpinnerPoints, SpinnerGlobe, SpinnerMoon,
		SpinnerMonkey, SpinnerMeter, SpinnerHamburger, SpinnerEllipsis,
	}

	for _, st := range types {
		// Just verify the function doesn't panic
		result := getSpinnerType(st)
		_ = result
	}
}

func TestSpinModel_TickWhenDone(t *testing.T) {
	t.Parallel()

	opts := SpinCommandOptions{
		Title:   "Test",
		Command: []string{"echo", "test"},
		Config:  DefaultConfig(),
	}

	model := NewSpinModel(opts)
	model.done = true

	// Tick when done should not advance spinner
	initialSpinner := model.spinner
	msg := spinnerTickMsg{}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*spinModel)

	if m.spinner != initialSpinner {
		t.Error("spinner should not advance when model is done")
	}
}
