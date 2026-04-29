// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
)

type resolveAllFunc func(context.Context, []ModuleRef, map[ModuleRefKey]ContentHash) ([]*ResolvedModule, error)

// checkMissingTransitiveDeps compares each resolved module's TransitiveDeps
// against the root requirements. Returns diagnostics for any transitive deps
// that are not explicitly declared in the root invowkmod.cue.
//
// Diagnostics are deduplicated by the missing dep's Key() — if modules B and C
// both require D, and D is not in root requirements, D is reported once
// (attributed to the first module that declares it).
func checkMissingTransitiveDeps(requirements []ModuleRef, resolved []*ResolvedModule) []MissingTransitiveDepDiagnostic {
	rootKeys := make(map[ModuleRefKey]bool, len(requirements))
	for _, req := range requirements {
		rootKeys[req.Key()] = true
	}

	var diags []MissingTransitiveDepDiagnostic
	seen := make(map[ModuleRefKey]bool)

	for _, mod := range resolved {
		for _, dep := range mod.TransitiveDeps {
			key := dep.Key()
			if !rootKeys[key] && !seen[key] {
				seen[key] = true
				diags = append(diags, MissingTransitiveDepDiagnostic{
					RequiringModule: mod.ModuleID,
					RequiringURL:    mod.ModuleRef.GitURL,
					MissingRef:      dep,
				})
			}
		}
	}

	return diags
}

// Tidy resolves all direct dependencies and returns any transitive dependencies
// that are not declared in the root invowkmod.cue. The caller (CLI) is responsible
// for adding the returned refs to invowkmod.cue via AddRequirement().
//
// Tidy does NOT write the lock file — it only identifies gaps. A subsequent
// `invowk module sync` completes the process.
func (m *Resolver) Tidy(ctx context.Context, requirements []ModuleRef) ([]ModuleRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(requirements) == 0 {
		return nil, nil
	}

	// Tidy expands the explicit-only graph to a fixed point. Sync remains
	// fail-fast, but tidy should be the one-shot repair operation users expect.
	knownHashes := m.loadExistingLockHashes()
	return tidyToFixedPoint(ctx, requirements, knownHashes, m.resolveAll)
}

func tidyToFixedPoint(ctx context.Context, requirements []ModuleRef, knownHashes map[ModuleRefKey]ContentHash, resolveAll resolveAllFunc) ([]ModuleRef, error) {
	current := append([]ModuleRef(nil), requirements...)
	known := make(map[ModuleRefKey]bool, len(current))
	for _, req := range current {
		known[req.Key()] = true
	}

	var missing []ModuleRef
	for {
		resolved, err := resolveAll(ctx, current, knownHashes)
		if err != nil {
			return nil, err
		}

		diags := checkMissingTransitiveDeps(current, resolved)
		if len(diags) == 0 {
			return missing, nil
		}

		added := false
		for _, d := range diags {
			key := d.MissingRef.Key()
			if known[key] {
				continue
			}
			known[key] = true
			current = append(current, d.MissingRef)
			missing = append(missing, d.MissingRef)
			added = true
		}
		if !added {
			return missing, nil
		}
	}
}
