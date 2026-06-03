// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"strings"
	"testing"
)

func TestCommandTargetValidateMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("allows local target without discovery identity", func(t *testing.T) {
		t.Parallel()

		target := CommandTarget{Reference: "build"}
		if err := target.Validate(); err != nil {
			t.Fatalf("CommandTarget.Validate() error = %v, want nil", err)
		}
	})

	t.Run("aggregates invalid reference source and module IDs", func(t *testing.T) {
		t.Parallel()

		target := CommandTarget{
			Reference: " ",
			SourceID:  "1bad-source",
			ModuleID:  "1bad.module",
		}

		err := target.Validate()
		if err == nil {
			t.Fatal("CommandTarget.Validate() error = nil, want joined validation errors")
		}
		if !errors.Is(err, ErrInvalidCommandReference) {
			t.Fatalf("error = %v, want ErrInvalidCommandReference", err)
		}
		if !errors.Is(err, ErrInvalidModuleID) {
			t.Fatalf("error = %v, want ErrInvalidModuleID", err)
		}
		if !strings.Contains(err.Error(), "module source ID") {
			t.Fatalf("error = %q, want module source ID validation detail", err)
		}
	})
}

func TestCommandScopeMutationDecisionPayloadsAndSameModuleFallbacks(t *testing.T) {
	t.Parallel()

	scope := NewCommandScope("io.example.caller")

	requireCommandScopeDecision(t, scope.CanCallTarget(CommandTarget{
		Reference: "build",
	}), CommandScopeDecision{
		Allowed:       true,
		TargetCommand: "build",
	})

	requireCommandScopeDecision(t, scope.CanCallTarget(CommandTarget{
		Reference: "tools lint",
		ModuleID:  "io.example.tools",
	}), CommandScopeDecision{
		TargetCommand:  "tools lint",
		TargetSource:   "io.example.tools",
		TargetModuleID: "io.example.tools",
		Reason:         CommandScopeDenyInaccessible,
	})

	requireCommandScopeDecision(t, scope.CanCallTarget(CommandTarget{
		Reference: "caller build",
		ModuleID:  "io.example.caller",
	}), CommandScopeDecision{
		Allowed:        true,
		TargetCommand:  "caller build",
		TargetSource:   "io.example.caller",
		TargetModuleID: "io.example.caller",
	})

	sourceUnsetScope := &CommandScope{
		ModuleID:                "io.example.caller",
		GlobalSources:           make(map[ModuleSourceID]bool),
		DirectDependencySources: make(map[ModuleID]map[ModuleSourceID]bool),
	}
	requireCommandScopeDecision(t, sourceUnsetScope.CanCallTarget(CommandTarget{
		Reference: "caller build",
		SourceID:  "caller-alias",
		ModuleID:  "io.example.caller",
	}), CommandScopeDecision{
		Allowed:        true,
		TargetCommand:  "caller build",
		TargetSource:   "caller-alias",
		TargetModuleID: "io.example.caller",
	})
}

func TestCommandScopeMutationRequiresCompleteDiscoveryIdentity(t *testing.T) {
	t.Parallel()

	globalScope := NewCommandScope("io.example.caller")
	globalScope.GlobalSources[""] = true
	requireCommandScopeDecision(t, globalScope.CanCallTarget(CommandTarget{
		Reference: "global lint",
		ModuleID:  "io.example.global",
	}), CommandScopeDecision{
		TargetCommand:  "global lint",
		TargetSource:   "io.example.global",
		TargetModuleID: "io.example.global",
		Reason:         CommandScopeDenyInaccessible,
	})

	depScope := NewCommandScope("io.example.caller")
	depScope.DirectDependencySources[""] = map[ModuleSourceID]bool{"allowed-tools": true}
	requireCommandScopeDecision(t, depScope.CanCallTarget(CommandTarget{
		Reference: "allowed-tools test",
		SourceID:  "allowed-tools",
	}), CommandScopeDecision{
		TargetCommand: "allowed-tools test",
		TargetSource:  "allowed-tools",
		Reason:        CommandScopeDenyInaccessible,
	})

	depScope.DirectDependencySources["io.example.tools"] = map[ModuleSourceID]bool{"": true}
	requireCommandScopeDecision(t, depScope.CanCallTarget(CommandTarget{
		Reference: "tools test",
		ModuleID:  "io.example.tools",
	}), CommandScopeDecision{
		TargetCommand:  "tools test",
		TargetSource:   "io.example.tools",
		TargetModuleID: "io.example.tools",
		Reason:         CommandScopeDenyInaccessible,
	})

	depScope.AddDirectDependency("io.example.tools", "allowed-tools")
	requireCommandScopeDecision(t, depScope.CanCallTarget(CommandTarget{
		Reference: "allowed-tools test",
		SourceID:  "allowed-tools",
		ModuleID:  "io.example.tools",
	}), CommandScopeDecision{
		Allowed:        true,
		TargetCommand:  "allowed-tools test",
		TargetSource:   "allowed-tools",
		TargetModuleID: "io.example.tools",
	})
}

func requireCommandScopeDecision(t *testing.T, got, want CommandScopeDecision) {
	t.Helper()

	if got != want {
		t.Fatalf("CanCallTarget() = %+v, want %+v", got, want)
	}
}
