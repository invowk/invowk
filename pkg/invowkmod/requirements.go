// SPDX-License-Identifier: MPL-2.0

package invowkmod

// ModuleRefsFromRequirements converts parsed invowkmod.cue requirements into
// resolver module refs while preserving source identity fields.
func ModuleRefsFromRequirements(requirements []ModuleRequirement) []ModuleRef {
	refs := make([]ModuleRef, 0, len(requirements))
	for _, req := range requirements {
		refs = append(refs, ModuleRef(req))
	}
	return refs
}
