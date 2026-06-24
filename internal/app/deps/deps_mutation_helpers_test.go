// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
)

func depsMutationHostCommand(dependsOn *invowkfile.DependsOn) *invowkfile.Command {
	return &invowkfile.Command{
		Name:      depsMutationCommand,
		DependsOn: dependsOn,
		Implementations: []invowkfile.Implementation{{
			Script:   invowkfile.ImplementationScript{Content: "echo ok"},
			Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
		}},
	}
}

func depsMutationRuntimeCommand(dependsOn *invowkfile.DependsOn) *invowkfile.Command {
	return &invowkfile.Command{
		Name: depsMutationCommand,
		Implementations: []invowkfile.Implementation{{
			Script: invowkfile.ImplementationScript{Content: "echo ok"},
			Runtimes: []invowkfile.RuntimeConfig{{
				Name:      invowkfile.RuntimeContainer,
				DependsOn: dependsOn,
			}},
		}},
	}
}

func depsMutationCommandInfo(cmd *invowkfile.Command, inv *invowkfile.Invowkfile) *discovery.CommandInfo {
	if inv == nil {
		inv = &invowkfile.Invowkfile{}
	}
	return &discovery.CommandInfo{
		Name:       cmd.Name,
		Command:    cmd,
		Invowkfile: inv,
	}
}

func depsMutationRequirementAndLock() (invowkmod.ModuleRequirement, *invowkmod.LockFile) {
	req := invowkmod.ModuleRequirement{
		GitURL:  depsMutationGitURL,
		Version: depsMutationVersion,
		Alias:   invowkmod.ModuleAlias(depsMutationSource),
	}
	lock := invowkmod.NewLockFile()
	lock.Modules[invowkmod.ModuleRef(req).Key()] = invowkmod.LockedModule{
		GitURL:          req.GitURL,
		Version:         req.Version,
		ResolvedVersion: depsMutationResolvedVersion,
		GitCommit:       depsMutationGitCommit,
		Alias:           req.Alias,
		Namespace:       invowkmod.ModuleNamespace(depsMutationSource),
		ModuleID:        depsMutationModuleID,
		CommandSourceID: invowkmod.ModuleSourceID(depsMutationSource),
		ContentHash:     depsMutationContentHash,
	}
	return req, lock
}

func requireDependencyError(t *testing.T, err error) *DependencyError {
	t.Helper()

	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("error = %v, want *DependencyError", err)
	}
	return depErr
}

func requireDependencyFailureKinds(t *testing.T, failures []DependencyFailure, want ...DependencyFailureKind) {
	t.Helper()

	if len(failures) != len(want) {
		t.Fatalf("failure count = %d (%v), want %d (%v)", len(failures), failures, len(want), want)
	}
	for i := range want {
		if failures[i].Kind() != want[i] {
			t.Fatalf("failure[%d].Kind() = %q, want %q; failures=%v", i, failures[i].Kind(), want[i], failures)
		}
	}
}
