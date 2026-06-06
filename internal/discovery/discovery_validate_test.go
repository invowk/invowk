// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestCommandInfo_Validate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

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
				FilePath: types.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
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

func TestCommandInfoValidateMutationContracts(t *testing.T) {
	t.Parallel()

	moduleID := invowkmod.ModuleID("1bad")
	cmd := CommandInfo{
		Name:        "1bad",
		Description: " \t ",
		Source:      Source(99),
		FilePath:    " \t ",
		SimpleName:  "1bad",
		SourceID:    "1bad",
		ModuleID:    &moduleID,
	}

	err := cmd.Validate()
	invalidErr := requireInvalidCommandInfoError(t, err)
	if got, want := invalidErr.Error(), "invalid command info: 7 field error(s)"; got != want {
		t.Fatalf("InvalidCommandInfoError.Error() = %q, want %q", got, want)
	}
	if got, want := len(invalidErr.FieldErrors), 7; got != want {
		t.Fatalf("FieldErrors length = %d, want %d", got, want)
	}
	for _, want := range []error{
		invowkfile.ErrInvalidCommandName,
		invowkfile.ErrInvalidDescriptionText,
		ErrInvalidSource,
		types.ErrInvalidFilesystemPath,
		ErrInvalidSourceID,
		invowkmod.ErrInvalidModuleID,
	} {
		if !discoveryFieldErrorsContain(invalidErr.FieldErrors, want) {
			t.Fatalf("FieldErrors should contain %v, got %#v", want, invalidErr.FieldErrors)
		}
	}
}

func TestDiscoveredFile_Validate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

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
				Path:   types.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
				Source: SourceCurrentDir,
			},
			wantErr: false,
		},
		{
			name: "valid module source",
			file: DiscoveredFile{
				Path:   types.FilesystemPath(filepath.Join(tmpDir, "foo.invowkmod", "invowkfile.cue")),
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

func TestDiscoveredFileValidateMutationContracts(t *testing.T) {
	t.Parallel()

	file := DiscoveredFile{
		Path:   " \t ",
		Source: Source(42),
	}

	err := file.Validate()
	invalidErr := requireInvalidDiscoveredFileError(t, err)
	if got, want := invalidErr.Error(), "invalid discovered file: 2 field error(s)"; got != want {
		t.Fatalf("InvalidDiscoveredFileError.Error() = %q, want %q", got, want)
	}
	if got, want := len(invalidErr.FieldErrors), 2; got != want {
		t.Fatalf("FieldErrors length = %d, want %d", got, want)
	}
	for _, want := range []error{
		types.ErrInvalidFilesystemPath,
		ErrInvalidSource,
	} {
		if !discoveryFieldErrorsContain(invalidErr.FieldErrors, want) {
			t.Fatalf("FieldErrors should contain %v, got %#v", want, invalidErr.FieldErrors)
		}
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

func TestLookupResultValidateMutationContracts(t *testing.T) {
	t.Parallel()

	result := LookupResult{
		Command: &CommandInfo{
			Source: Source(99),
		},
	}

	err := result.Validate()
	invalidErr := requireInvalidLookupResultError(t, err)
	if got, want := invalidErr.Error(), "invalid lookup result: 1 field error(s)"; got != want {
		t.Fatalf("InvalidLookupResultError.Error() = %q, want %q", got, want)
	}
	if got, want := len(invalidErr.FieldErrors), 1; got != want {
		t.Fatalf("FieldErrors length = %d, want %d", got, want)
	}
	if !errors.Is(invalidErr.FieldErrors[0], ErrInvalidCommandInfo) {
		t.Fatalf("FieldErrors[0] = %v, want ErrInvalidCommandInfo", invalidErr.FieldErrors[0])
	}
}

func validModuleIDPtr() *invowkmod.ModuleID {
	id := invowkmod.ModuleID("io.invowk.sample")
	return &id
}

func requireInvalidCommandInfoError(t *testing.T, err error) *InvalidCommandInfoError {
	t.Helper()

	if !errors.Is(err, ErrInvalidCommandInfo) {
		t.Fatalf("error = %v, want ErrInvalidCommandInfo", err)
	}
	var invalidErr *InvalidCommandInfoError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("error type = %T, want *InvalidCommandInfoError", err)
	}
	return invalidErr
}

func requireInvalidDiscoveredFileError(t *testing.T, err error) *InvalidDiscoveredFileError {
	t.Helper()

	if !errors.Is(err, ErrInvalidDiscoveredFile) {
		t.Fatalf("error = %v, want ErrInvalidDiscoveredFile", err)
	}
	var invalidErr *InvalidDiscoveredFileError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("error type = %T, want *InvalidDiscoveredFileError", err)
	}
	return invalidErr
}

func requireInvalidLookupResultError(t *testing.T, err error) *InvalidLookupResultError {
	t.Helper()

	if !errors.Is(err, ErrInvalidLookupResult) {
		t.Fatalf("error = %v, want ErrInvalidLookupResult", err)
	}
	var invalidErr *InvalidLookupResultError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("error type = %T, want *InvalidLookupResultError", err)
	}
	return invalidErr
}

func discoveryFieldErrorsContain(fieldErrors []error, target error) bool {
	for _, err := range fieldErrors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
