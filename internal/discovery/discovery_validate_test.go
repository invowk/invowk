// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestCommandInfo_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cmd     CommandInfo
		wantErr bool
	}{
		{
			name:    "zero value is valid",
			cmd:     CommandInfo{},
			wantErr: false,
		},
		{
			name: "valid command info",
			cmd: CommandInfo{
				Name:       invowkfile.CommandName("build"),
				Source:     SourceCurrentDir,
				SourceID:   SourceIDInvowkfile,
				SimpleName: invowkfile.CommandName("build"),
			},
			wantErr: false,
		},
		{
			name: "invalid source",
			cmd: CommandInfo{
				Source: Source(99),
			},
			wantErr: true,
		},
		{
			name: "valid with module ID",
			cmd: CommandInfo{
				Source:   SourceModule,
				SourceID: SourceID("foo"),
				ModuleID: validModuleIDPtr(),
			},
			wantErr: false,
		},
		{
			name: "valid with file path",
			cmd: CommandInfo{
				Source:   SourceCurrentDir,
				FilePath: types.FilesystemPath("/tmp/invowkfile.cue"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cmd.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("CommandInfo.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommandInfo_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	cmd := CommandInfo{Source: Source(99)}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error for invalid source")
	}

	if !errors.Is(err, ErrInvalidCommandInfo) {
		t.Errorf("errors.Is(err, ErrInvalidCommandInfo) = false, want true")
	}

	var invalidErr *InvalidCommandInfoError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidCommandInfoError) = false, want true")
	}
	if len(invalidErr.FieldErrors) == 0 {
		t.Error("expected non-empty FieldErrors")
	}
}

func TestDiscoveredFile_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    DiscoveredFile
		wantErr bool
	}{
		{
			name:    "zero value is valid",
			file:    DiscoveredFile{},
			wantErr: false,
		},
		{
			name: "valid with path and source",
			file: DiscoveredFile{
				Path:   types.FilesystemPath("/tmp/invowkfile.cue"),
				Source: SourceCurrentDir,
			},
			wantErr: false,
		},
		{
			name: "valid module source",
			file: DiscoveredFile{
				Path:   types.FilesystemPath("/tmp/foo.invowkmod/invowkfile.cue"),
				Source: SourceModule,
			},
			wantErr: false,
		},
		{
			name: "invalid source",
			file: DiscoveredFile{
				Source: Source(42),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.file.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DiscoveredFile.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDiscoveredFile_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	file := DiscoveredFile{Source: Source(42)}
	err := file.Validate()
	if err == nil {
		t.Fatal("expected error for invalid source")
	}

	if !errors.Is(err, ErrInvalidDiscoveredFile) {
		t.Errorf("errors.Is(err, ErrInvalidDiscoveredFile) = false, want true")
	}

	var invalidErr *InvalidDiscoveredFileError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidDiscoveredFileError) = false, want true")
	}
}

func TestLookupResult_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		result  LookupResult
		wantErr bool
	}{
		{
			name:    "zero value is valid (nil command)",
			result:  LookupResult{},
			wantErr: false,
		},
		{
			name: "valid with command",
			result: LookupResult{
				Command: &CommandInfo{
					Source: SourceCurrentDir,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid command propagates error",
			result: LookupResult{
				Command: &CommandInfo{
					Source: Source(99),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.result.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("LookupResult.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLookupResult_Validate_ErrorTypes(t *testing.T) {
	t.Parallel()

	result := LookupResult{
		Command: &CommandInfo{Source: Source(99)},
	}
	err := result.Validate()
	if err == nil {
		t.Fatal("expected error for invalid command")
	}

	if !errors.Is(err, ErrInvalidLookupResult) {
		t.Errorf("errors.Is(err, ErrInvalidLookupResult) = false, want true")
	}

	var invalidErr *InvalidLookupResultError
	if !errors.As(err, &invalidErr) {
		t.Errorf("errors.As(err, *InvalidLookupResultError) = false, want true")
	}
}

func validModuleIDPtr() *invowkmod.ModuleID {
	id := invowkmod.ModuleID("io.invowk.sample")
	return &id
}
