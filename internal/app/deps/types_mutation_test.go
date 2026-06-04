// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"slices"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestCommandDependencyAlternativeValidateMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("rejects invalid original ref before trusting parsed parts", func(t *testing.T) {
		t.Parallel()
		alt := commandDependencyAlternative{
			Ref:   "9bad",
			Parts: invowkfile.CommandDependencyRefParts{Command: "build"},
		}
		err := alt.Validate()
		if !errors.Is(err, invowkfile.ErrInvalidCommandDependencyRef) {
			t.Fatalf("Validate() = %v, want invalid command dependency ref", err)
		}
	})

	t.Run("rejects inconsistent parsed parts after valid ref", func(t *testing.T) {
		t.Parallel()
		alt := commandDependencyAlternative{
			Ref: "build",
			Parts: invowkfile.CommandDependencyRefParts{
				Command:  "build",
				SourceID: "tools",
			},
		}
		err := alt.Validate()
		if !errors.Is(err, invowkfile.ErrInvalidCommandDependencyRef) {
			t.Fatalf("Validate() = %v, want invalid command dependency ref", err)
		}
	})
}

func TestDependencyMessageMutationContracts(t *testing.T) {
	t.Parallel()

	blank := DependencyMessage(" \t\n ")
	err := blank.Validate()
	if !errors.Is(err, ErrInvalidDependencyMessage) {
		t.Fatalf("blank Validate() = %v, want ErrInvalidDependencyMessage", err)
	}
	msgErr, ok := errors.AsType[*InvalidDependencyMessageError](err)
	if !ok {
		t.Fatalf("blank Validate() error type = %T, want *InvalidDependencyMessageError", err)
	}
	if msgErr.Value != blank {
		t.Fatalf("Value = %q, want original blank message", msgErr.Value)
	}
	if err := DependencyMessage("go").Validate(); err != nil {
		t.Fatalf("non-empty Validate() = %v, want nil", err)
	}
}

func TestDependencyFailureKindStringMutationContract(t *testing.T) {
	t.Parallel()

	if got := DependencyFailureTool.String(); got != "tool" {
		t.Fatalf("String() = %q, want tool", got)
	}
}

func TestDependencyFailureMutationContracts(t *testing.T) {
	t.Parallel()

	if got := DependencyFailureCommand.String(); got != "command" {
		t.Fatalf("DependencyFailureCommand.String() = %q, want command", got)
	}

	failure, err := NewDependencyFailure(DependencyFailureTool, "go")
	if err != nil {
		t.Fatalf("NewDependencyFailure() = %v, want nil", err)
	}
	if failure.Kind() != DependencyFailureTool || failure.Detail() != "go" {
		t.Fatalf("failure payload = %q/%q, want tool/go", failure.Kind(), failure.Detail())
	}

	if _, err := NewDependencyFailure("unknown", "go"); err == nil {
		t.Fatal("NewDependencyFailure() with invalid kind = nil, want error")
	}
	if _, err := NewDependencyFailure(DependencyFailureTool, " "); err == nil {
		t.Fatal("NewDependencyFailure() with blank detail = nil, want error")
	}
	if err := (DependencyFailure{kind: "unknown", detail: "go"}).Validate(); err == nil {
		t.Fatal("DependencyFailure.Validate() with invalid kind = nil, want error")
	}
	if err := (DependencyFailure{kind: DependencyFailureTool, detail: " "}).Validate(); err == nil {
		t.Fatal("DependencyFailure.Validate() with blank detail = nil, want error")
	}
}

func TestDependencyErrorFailuresMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("structured failures are cloned", func(t *testing.T) {
		t.Parallel()
		original := mustDependencyFailureForTypesMutation(t, DependencyFailureTool, "go")
		depErr := &DependencyError{StructuredFailures: []DependencyFailure{original}}
		got := depErr.Failures()
		if len(got) != 1 || got[0].Kind() != DependencyFailureTool || got[0].Detail() != "go" {
			t.Fatalf("Failures() = %v, want one tool/go failure", got)
		}

		got[0] = mustDependencyFailureForTypesMutation(t, DependencyFailureCommand, "build")
		if depErr.StructuredFailures[0].Kind() != DependencyFailureTool ||
			depErr.StructuredFailures[0].Detail() != "go" {
			t.Fatalf("Failures() returned aliased slice; source = %v", depErr.StructuredFailures)
		}
	})

	t.Run("legacy categorized failures skip invalid details and keep order", func(t *testing.T) {
		t.Parallel()
		depErr := &DependencyError{
			MissingTools:        []DependencyMessage{" ", "go"},
			MissingCommands:     []DependencyMessage{"build"},
			MissingFilepaths:    []DependencyMessage{"config.cue"},
			MissingCapabilities: []DependencyMessage{"tty"},
			FailedCustomChecks:  []DependencyMessage{"lint"},
			MissingEnvVars:      []DependencyMessage{"TOKEN"},
			ForbiddenCommands:   []DependencyMessage{"blocked"},
		}

		got := depErr.Failures()
		requireDependencyFailuresForTypesMutation(t, got, []string{
			"tool:go",
			"command:build",
			"filepath:config.cue",
			"capability:tty",
			"custom_check:lint",
			"env_var:TOKEN",
			"forbidden_command:blocked",
		})
	})

	t.Run("direct dependency failure helper appends valid details", func(t *testing.T) {
		t.Parallel()
		got := dependencyFailures(DependencyFailureEnvVar, []DependencyMessage{"TOKEN"})
		requireDependencyFailuresForTypesMutation(
			t,
			got,
			[]string{"env_var:TOKEN"},
		)
	})
}

func TestNormalizeDependencyMessageMutationContracts(t *testing.T) {
	t.Parallel()

	if got := normalizeDependencyMessage("  \u2022 - install go  "); got != "install go" {
		t.Fatalf("normalizeDependencyMessage() = %q, want %q", got, "install go")
	}
	if got := dependencyMessageFromDetail("\n- install docker\t"); got != "install docker" {
		t.Fatalf("dependencyMessageFromDetail() = %q, want %q", got, "install docker")
	}
}

func mustDependencyFailureForTypesMutation(t *testing.T, kind DependencyFailureKind, detail DependencyMessage) DependencyFailure {
	t.Helper()

	failure, err := NewDependencyFailure(kind, detail)
	if err != nil {
		t.Fatalf("NewDependencyFailure(%q, %q) = %v", kind, detail, err)
	}
	return failure
}

func requireDependencyFailuresForTypesMutation(
	t *testing.T,
	got []DependencyFailure,
	want []string,
) {
	t.Helper()

	gotPairs := make([]string, 0, len(got))
	for _, failure := range got {
		gotPairs = append(gotPairs, failure.Kind().String()+":"+failure.Detail().String())
	}
	if !slices.Equal(gotPairs, want) {
		t.Fatalf("Failures() = %v, want %v", gotPairs, want)
	}
}
