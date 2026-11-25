// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"github.com/invowk/invowk/pkg/invowkfile"
)

// virtualWorkdir is the effective workdir of a virtual command, with what
// decides whether it is an allowed root under restricted access.
//
// A module that declares a workdir outside its own directory must not widen
// its read roots to that directory (virtual-runtime-sandbox, "The work
// directory is an allowed root unless a module declares it outside the
// module"; formal finding F15). The workdir stays the process cwd and the
// value of @work either way.
//
//goplint:ignore -- raw OS path strings, like the path walker's structs.
type virtualWorkdir struct {
	// path is the effective workdir: the cwd and the @work anchor.
	path string
	// moduleRoot is the module directory when the invowkfile is a module and
	// the workdir comes from that invowkfile's own root, command or
	// implementation workdir. It is "" for a trusted workdir: the CLI
	// override, a workdir of an invowkfile outside a module, or the default.
	moduleRoot string
}

// virtualWorkdirFor builds the workdir of a command run from the invowkfile,
// the command, the selected implementation and the CLI override.
//
// The name is bound by the VirtualPathHarness correspondence table
// (formal/alloy/VirtualPathHarness.als); keep it.
func virtualWorkdirFor(
	inv *invowkfile.Invowkfile,
	cmd *invowkfile.Command,
	impl *invowkfile.Implementation,
	cliOverride invowkfile.WorkDir,
) virtualWorkdir {
	effective := inv.ResolveEffectiveWorkDir(cmd, impl, cliOverride)
	workdir := virtualWorkdir{path: string(effective.Path)}
	if inv.IsFromModule() && effective.Source.DeclaredByInvowkfile() {
		workdir.moduleRoot = string(inv.ModulePath)
	}
	return workdir
}

// isReadRoot reports whether the workdir, and the @work anchor that points at
// it, are allowed roots. A trusted workdir always is. A module-declared one is
// only when it resolves physically inside the module: both paths are resolved
// with the strict resolver, through every link and junction, and any error, or
// a workdir on another volume, counts as outside (fail closed).
//
// The name is bound by the VirtualPathHarness correspondence table
// (formal/alloy/VirtualPathHarness.als); keep it.
func (w virtualWorkdir) isReadRoot() bool {
	if w.moduleRoot == "" {
		return true
	}
	module, err := normalizeExistingOrParent(w.moduleRoot, "")
	if err != nil {
		return false
	}
	workdir, err := normalizeExistingOrParent(w.path, "")
	if err != nil {
		return false
	}
	return pathWithin(module, workdir)
}
