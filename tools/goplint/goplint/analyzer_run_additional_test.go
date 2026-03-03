// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"strings"
	"testing"
)

func TestLoadRunInputsExplicitMissingConfigPath(t *testing.T) {
	t.Parallel()

	state := &flagState{}
	resetFlagStateDefaults(state)

	rc := runConfig{
		configPath:         "does-not-exist.toml",
		configPathExplicit: true,
	}
	_, _, err := loadRunInputs(state, rc)
	if err == nil {
		t.Fatal("expected missing explicit config path to fail")
	}
	if !strings.Contains(err.Error(), "reading config") || !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("expected config-file-not-found error, got %v", err)
	}
}

func TestLoadRunInputsExplicitMissingBaselinePath(t *testing.T) {
	t.Parallel()

	state := &flagState{}
	resetFlagStateDefaults(state)

	rc := runConfig{
		baselinePath:         "does-not-exist-baseline.toml",
		baselinePathExplicit: true,
	}
	_, _, err := loadRunInputs(state, rc)
	if err == nil {
		t.Fatal("expected missing explicit baseline path to fail")
	}
	if !strings.Contains(err.Error(), "reading baseline") || !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("expected baseline-file-not-found error, got %v", err)
	}
}

func TestValidateRunConfigAllowsNoCFAWhenMissionChecksDisabled(t *testing.T) {
	t.Parallel()

	rc := runConfig{checkValidateUsage: true, noCFA: true}
	if err := validateRunConfig(rc); err != nil {
		t.Fatalf("expected no-cfa to be allowed for non-mission check, got %v", err)
	}
}

func TestValidateRunConfigRejectsExplicitEmptyPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rc   runConfig
		want string
	}{
		{
			name: "empty explicit config path",
			rc: runConfig{
				configPathExplicit: true,
				configPath:         "",
			},
			want: "flag --config was provided with an empty path",
		},
		{
			name: "empty explicit baseline path",
			rc: runConfig{
				baselinePathExplicit: true,
				baselinePath:         "  ",
			},
			want: "flag --baseline was provided with an empty path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateRunConfig(tt.rc)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateRunConfig() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestParseIncludePackagesOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		rc            runConfig
		want          []string
		wantHas       bool
		wantErrSubstr string
	}{
		{
			name: "not explicit",
			rc:   runConfig{},
		},
		{
			name: "explicit empty clears override",
			rc: runConfig{
				includePackagesExplicit: true,
				includePackages:         "",
			},
			want:    []string{},
			wantHas: true,
		},
		{
			name: "trims entries",
			rc: runConfig{
				includePackagesExplicit: true,
				includePackages:         " pkg/a ,pkg/b ",
			},
			want:    []string{"pkg/a", "pkg/b"},
			wantHas: true,
		},
		{
			name: "rejects empty segment",
			rc: runConfig{
				includePackagesExplicit: true,
				includePackages:         "pkg/a,,pkg/b",
			},
			wantErrSubstr: "empty package prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, has, err := parseIncludePackagesOverride(tt.rc)
			if tt.wantErrSubstr != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("parseIncludePackagesOverride() error = %q, want substring %q", err.Error(), tt.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseIncludePackagesOverride() unexpected error: %v", err)
			}
			if has != tt.wantHas {
				t.Fatalf("parseIncludePackagesOverride() has = %v, want %v", has, tt.wantHas)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("parseIncludePackagesOverride() got=%v want=%v", got, tt.want)
			}
		})
	}
}
