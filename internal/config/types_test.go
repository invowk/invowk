// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"testing"
)

func TestContainerEngine_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		engine  ContainerEngine
		want    bool
		wantErr bool
	}{
		{ContainerEnginePodman, true, false},
		{ContainerEngineDocker, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"PODMAN", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.engine), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.engine.IsValid()
			if isValid != tt.want {
				t.Errorf("ContainerEngine(%q).IsValid() = %v, want %v", tt.engine, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ContainerEngine(%q).IsValid() returned no errors, want error", tt.engine)
				}
				if !errors.Is(errs[0], ErrInvalidContainerEngine) {
					t.Errorf("error should wrap ErrInvalidContainerEngine, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ContainerEngine(%q).IsValid() returned unexpected errors: %v", tt.engine, errs)
			}
		})
	}
}

func TestConfigRuntimeMode_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode    RuntimeMode
		want    bool
		wantErr bool
	}{
		{RuntimeNative, true, false},
		{RuntimeVirtual, true, false},
		{RuntimeContainer, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"NATIVE", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.mode.IsValid()
			if isValid != tt.want {
				t.Errorf("RuntimeMode(%q).IsValid() = %v, want %v", tt.mode, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("RuntimeMode(%q).IsValid() returned no errors, want error", tt.mode)
				}
				if !errors.Is(errs[0], ErrInvalidConfigRuntimeMode) {
					t.Errorf("error should wrap ErrInvalidConfigRuntimeMode, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("RuntimeMode(%q).IsValid() returned unexpected errors: %v", tt.mode, errs)
			}
		})
	}
}

func TestColorScheme_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scheme  ColorScheme
		want    bool
		wantErr bool
	}{
		{ColorSchemeAuto, true, false},
		{ColorSchemeDark, true, false},
		{ColorSchemeLight, true, false},
		{"", false, true},
		{"garbage", false, true},
		{"AUTO", false, true},
		{"Dark", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.scheme), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.scheme.IsValid()
			if isValid != tt.want {
				t.Errorf("ColorScheme(%q).IsValid() = %v, want %v", tt.scheme, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ColorScheme(%q).IsValid() returned no errors, want error", tt.scheme)
				}
				if !errors.Is(errs[0], ErrInvalidColorScheme) {
					t.Errorf("error should wrap ErrInvalidColorScheme, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ColorScheme(%q).IsValid() returned unexpected errors: %v", tt.scheme, errs)
			}
		})
	}
}
