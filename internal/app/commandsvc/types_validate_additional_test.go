// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestTypesValidateErrorsAndHelpers(t *testing.T) {
	t.Parallel()

	requestErr := &InvalidRequestError{FieldErrors: []error{errors.New("one"), errors.New("two")}}
	if !errors.Is(requestErr, ErrInvalidRequest) {
		t.Fatal("requestErr should wrap ErrInvalidRequest")
	}
	if !strings.Contains(requestErr.Error(), "2 field error") {
		t.Fatalf("requestErr.Error() = %q, want containing field error count", requestErr.Error())
	}

	resultErr := &InvalidResultError{FieldErrors: []error{errors.New("one")}}
	if !errors.Is(resultErr, ErrInvalidCommandsvcResult) {
		t.Fatal("resultErr should wrap ErrInvalidCommandsvcResult")
	}
	if !strings.Contains(resultErr.Error(), "1 field error") {
		t.Fatalf("resultErr.Error() = %q, want containing field error count", resultErr.Error())
	}

	dryRunErr := &InvalidDryRunDataError{FieldErrors: []error{errors.New("one")}}
	if !errors.Is(dryRunErr, ErrInvalidDryRunData) {
		t.Fatal("dryRunErr should wrap ErrInvalidDryRunData")
	}
	if !strings.Contains(dryRunErr.Error(), "1 field error") {
		t.Fatalf("dryRunErr.Error() = %q, want containing field error count", dryRunErr.Error())
	}

	req := Request{
		Runtime:         invowkfile.RuntimeMode("bogus"),
		FromSource:      discovery.SourceID(""),
		Workdir:         invowkfile.WorkDir("   "),
		ConfigPath:      types.FilesystemPath("   "),
		EnvFiles:        []invowkfile.DotenvFilePath{"   "},
		EnvInheritMode:  invowkfile.EnvInheritMode("bogus"),
		EnvInheritAllow: []invowkfile.EnvVarName{""},
		EnvInheritDeny:  []invowkfile.EnvVarName{""},
	}

	var errs []error
	req.appendLocationValidationErrors(&errs)
	req.appendEnvValidationErrors(&errs)
	if len(errs) == 0 {
		t.Fatal("expected validation helper errors")
	}

	req.ResolvedCommand = &discovery.CommandInfo{Source: discovery.Source(99)}
	errs = nil
	req.appendResolvedCommandValidationErrors(&errs)
	if len(errs) != 1 {
		t.Fatalf("len(errs) = %d, want 1", len(errs))
	}

	data := DryRunData{
		Plan: DryRunPlan{
			SourceID: discovery.SourceID("bad source"),
			Runtime:  invowkfile.RuntimeNative,
		},
	}
	if data.Validate() == nil {
		t.Fatal("expected DryRunData validation error")
	}
}
