// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"os"

	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	// ScriptFileReader reads script files for execution-time script resolution.
	ScriptFileReader func(path string) ([]byte, error)

	// ScriptResolver resolves the selected implementation script for a command
	// execution. Filesystem access lives here, at the runtime boundary, rather
	// than on the invowkfile schema model.
	ScriptResolver struct {
		readFile ScriptFileReader
	}
)

// NewScriptResolver creates a script resolver that uses the provided reader.
func NewScriptResolver(readFile ScriptFileReader) (ScriptResolver, error) {
	if readFile == nil {
		readFile = os.ReadFile
	}
	resolver := ScriptResolver{readFile: readFile}
	if err := resolver.Validate(); err != nil {
		return ScriptResolver{}, err
	}
	return resolver, nil
}

// Validate returns nil when the script resolver has a reader.
func (r ScriptResolver) Validate() error {
	if r.readFile == nil {
		return invowkfile.ErrScriptReaderRequired
	}
	return nil
}

// ResolveSelected resolves the selected implementation script for execution.
//
//goplint:ignore -- runtime APIs consume script bodies as strings for shell/interpreter calls.
func (r ScriptResolver) ResolveSelected(ctx *ExecutionContext) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	return ctx.SelectedImpl.ResolveScriptWithFSAndModule(
		ctx.Invowkfile.FilePath,
		ctx.Invowkfile.ModulePath,
		func(path string) ([]byte, error) {
			return r.readFile(path)
		},
	)
}

// ScriptPath returns the selected implementation's script path using the
// invowkfile's module boundary when present.
func (r ScriptResolver) ScriptPath(ctx *ExecutionContext) invowkfile.FilesystemPath {
	return ctx.SelectedImpl.GetScriptFilePathWithModule(ctx.Invowkfile.FilePath, ctx.Invowkfile.ModulePath)
}
