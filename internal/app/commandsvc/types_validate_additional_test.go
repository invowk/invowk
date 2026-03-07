// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestTypesValidateErrorsAndHelpers(t *testing.T) {
	t.Parallel()

	requestErr := &InvalidRequestError{FieldErrors: []error{errors.New("one"), errors.New("two")}}
	if requestErr.Error() != "invalid request: 2 field error(s)" || !errors.Is(requestErr, ErrInvalidRequest) {
		t.Fatalf("requestErr = %v", requestErr)
	}

	resultErr := &InvalidResultError{FieldErrors: []error{errors.New("one")}}
	if resultErr.Error() != "invalid commandsvc result: 1 field error(s)" || !errors.Is(resultErr, ErrInvalidCommandsvcResult) {
		t.Fatalf("resultErr = %v", resultErr)
	}

	dryRunErr := &InvalidDryRunDataError{FieldErrors: []error{errors.New("one")}}
	if dryRunErr.Error() != "invalid dry run data: 1 field error(s)" || !errors.Is(dryRunErr, ErrInvalidDryRunData) {
		t.Fatalf("dryRunErr = %v", dryRunErr)
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
		SourceID: discovery.SourceID("bad source"),
	}
	if data.Validate() == nil {
		t.Fatal("expected DryRunData validation error")
	}
}
