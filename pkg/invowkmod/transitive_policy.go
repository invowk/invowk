// SPDX-License-Identifier: MPL-2.0

package invowkmod

// CheckMissingTransitiveDeps compares each resolved module's TransitiveDeps
// against the root requirements. It returns diagnostics for transitive
// dependencies that are not explicitly declared in the root invowkmod.cue.
func CheckMissingTransitiveDeps(requirements []ModuleRef, resolved []*ResolvedModule) []MissingTransitiveDepDiagnostic {
	rootKeys := make(map[ModuleRefKey]bool, len(requirements))
	for _, req := range requirements {
		rootKeys[req.Key()] = true
	}

	var diags []MissingTransitiveDepDiagnostic
	seen := make(map[ModuleRefKey]bool)
	for _, mod := range resolved {
		if mod == nil {
			continue
		}
		appendMissingTransitiveDeps(&diags, seen, rootKeys, mod.ModuleID, mod.ModuleRef.GitURL, mod.TransitiveDeps)
	}
	return diags
}

// CheckMissingVendoredTransitiveDeps compares each vendored module's declared
// requirements against the root requirements. It uses the same explicit-only
// policy as module sync, but works from parsed vendored module metadata.
func CheckMissingVendoredTransitiveDeps(requirements []ModuleRequirement, vendored []*Module) []MissingTransitiveDepDiagnostic {
	rootKeys := make(map[ModuleRefKey]bool, len(requirements))
	for _, req := range requirements {
		rootKeys[ModuleRef(req).Key()] = true
	}

	var diags []MissingTransitiveDepDiagnostic
	seen := make(map[ModuleRefKey]bool)
	for _, mod := range vendored {
		if mod == nil || mod.Metadata == nil {
			continue
		}
		deps := make([]ModuleRef, 0, len(mod.Metadata.Requires))
		for _, req := range mod.Metadata.Requires {
			deps = append(deps, ModuleRef(req))
		}
		appendMissingTransitiveDeps(&diags, seen, rootKeys, mod.Metadata.Module, "", deps)
	}
	return diags
}

func appendMissingTransitiveDeps(
	diags *[]MissingTransitiveDepDiagnostic,
	seen map[ModuleRefKey]bool,
	rootKeys map[ModuleRefKey]bool,
	requiringModule ModuleID,
	requiringURL GitURL,
	deps []ModuleRef,
) {
	for _, dep := range deps {
		key := dep.Key()
		if rootKeys[key] || seen[key] {
			continue
		}
		seen[key] = true
		*diags = append(*diags, MissingTransitiveDepDiagnostic{
			RequiringModule: requiringModule,
			RequiringURL:    requiringURL,
			MissingRef:      dep,
		})
	}
}
