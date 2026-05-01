// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"

	"github.com/invowk/invowk/pkg/types"
)

type (
	// ModuleScaffold is the deterministic file layout for a newly created module.
	ModuleScaffold struct {
		directoryName     ModuleScaffoldDirectoryName
		invowkmodContent  ModuleScaffoldContent
		invowkfileContent ModuleScaffoldContent
		createScriptsDir  bool
	}

	// ModuleScaffoldDirectoryName is the folder name for a generated module scaffold.
	ModuleScaffoldDirectoryName string

	// ModuleScaffoldContent is generated CUE file content for a module scaffold.
	ModuleScaffoldContent string
)

// NewModuleScaffold builds the deterministic file layout for a new module.
// Filesystem creation is owned by the application layer.
//
//goplint:ignore -- scaffold is validated before return; constructor builds generated content locally.
func NewModuleScaffold(opts CreateOptions) (ModuleScaffold, error) {
	if err := opts.Validate(); err != nil {
		return ModuleScaffold{}, err
	}

	// Validate module name
	if opts.Name == "" {
		return ModuleScaffold{}, errors.New("module name cannot be empty")
	}

	// Validate the name format
	if !moduleNameRegex.MatchString(string(opts.Name)) {
		return ModuleScaffold{}, fmt.Errorf("module name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", opts.Name)
	}

	// Use name as module identifier if not specified
	moduleID := string(opts.Module)
	if moduleID == "" {
		moduleID = string(opts.Name)
	}

	// Create description
	description := string(opts.Description)
	if description == "" {
		description = fmt.Sprintf("Commands from %s module", opts.Name)
	}

	// Create invowkmod.cue (module metadata)
	invowkmodContent := fmt.Sprintf(`// Invowkmod - Module metadata for %s
// See https://github.com/invowk/invowk for documentation

module: %q
version: "1.0.0"
description: %q

// Uncomment to add dependencies:
// requires: [
//     {
//         git_url: "https://github.com/example/utils.invowkmod.git"
//         version: "^1.0.0"
//     },
// ]
`, opts.Name, moduleID, description)

	// Create invowkfile.cue (command definitions only)
	invowkfileContent := fmt.Sprintf(`// Invowkfile - Command definitions for %s module
// See https://github.com/invowk/invowk for documentation

cmds: [
	{
		name:        "hello"
		description: "A sample command"
		implementations: [
			{
				script: "echo \"Hello from %s!\""
				runtimes: [
					{name: "native"},
					{name: "virtual"},
				]
				platforms: [
					{name: "linux"},
					{name: "macos"},
					{name: "windows"},
				]
			},
		]
	},
]
`, opts.Name, opts.Name)

	scaffold := ModuleScaffold{
		directoryName:     ModuleScaffoldDirectoryName(string(opts.Name) + ModuleSuffix), //goplint:ignore -- derived from validated module short name and fixed suffix.
		invowkmodContent:  ModuleScaffoldContent(invowkmodContent),                       //goplint:ignore -- generated non-empty content validated below.
		invowkfileContent: ModuleScaffoldContent(invowkfileContent),                      //goplint:ignore -- generated non-empty content validated below.
		createScriptsDir:  opts.CreateScriptsDir,
	}
	if err := scaffold.Validate(); err != nil {
		return ModuleScaffold{}, err
	}
	return scaffold, nil
}

// DirectoryName returns the module scaffold directory name.
func (s ModuleScaffold) DirectoryName() ModuleScaffoldDirectoryName {
	return s.directoryName
}

// InvowkmodContent returns the generated invowkmod.cue content.
func (s ModuleScaffold) InvowkmodContent() ModuleScaffoldContent {
	return s.invowkmodContent
}

// InvowkfileContent returns the generated invowkfile.cue content.
func (s ModuleScaffold) InvowkfileContent() ModuleScaffoldContent {
	return s.invowkfileContent
}

// CreateScriptsDir reports whether the scaffold includes scripts/.
func (s ModuleScaffold) CreateScriptsDir() bool {
	return s.createScriptsDir
}

// Validate returns nil if the scaffold has generated content for all required files.
func (s ModuleScaffold) Validate() error {
	var errs []error
	if err := s.directoryName.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := s.invowkmodContent.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := s.invowkfileContent.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.New(types.FormatFieldErrors("module scaffold", errs))
	}
	return nil
}

// String returns the directory name as text.
func (n ModuleScaffoldDirectoryName) String() string { return string(n) }

// Validate returns nil when the directory name is non-empty.
func (n ModuleScaffoldDirectoryName) Validate() error {
	if n == "" {
		return errors.New("module scaffold directory name must not be empty")
	}
	return nil
}

// String returns generated scaffold content as text.
func (c ModuleScaffoldContent) String() string { return string(c) }

// Validate returns nil when generated scaffold content is non-empty.
func (c ModuleScaffoldContent) Validate() error {
	if c == "" {
		return errors.New("module scaffold content must not be empty")
	}
	return nil
}
