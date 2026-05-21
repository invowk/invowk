// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// ModuleCollisionSourceLocal identifies a local/include/global module source.
	ModuleCollisionSourceLocal ModuleCollisionSourceKind = "local"
	// ModuleCollisionSourceVendored identifies a vendored module source.
	ModuleCollisionSourceVendored ModuleCollisionSourceKind = "vendored"
)

var (
	// ErrNoInvowkfileFound is returned when no invowkfile.cue is found in any search path.
	// Callers can check for this error using errors.Is(err, ErrNoInvowkfileFound).
	ErrNoInvowkfileFound = errors.New("no invowkfile found")

	// ErrModuleCollision is the sentinel wrapped by ModuleCollisionError.
	// Callers can check for this error using errors.Is(err, ErrModuleCollision).
	ErrModuleCollision = errors.New("module collision")
)

type (
	// ModuleCollisionSourceKind classifies the source that triggered a module
	// namespace collision.
	ModuleCollisionSourceKind string

	// ModuleCollisionError is returned when two modules publish the same command namespace.
	ModuleCollisionError struct {
		Namespace    SourceID
		FirstSource  string
		SecondSource string
		SecondKind   ModuleCollisionSourceKind
	}

	// Discovery is the stateless entry point for file discovery, command set building,
	// and single-command lookup. Each method creates fresh state from the config rather
	// than caching results, which is appropriate for a short-lived CLI process.
	//
	// The baseDir and commandsDir fields replace hardcoded "." and config.CommandsDir()
	// calls, enabling tests to inject explicit directories instead of relying on
	// process-global os.Chdir (which prevents t.Parallel()).
	//
	// The companion *Set booleans distinguish "caller did not provide a value"
	// (fall back to OS defaults) from "caller explicitly passed empty string"
	// (skip that discovery source). Init-time errors (initDiagnostics) are
	// surfaced through the standard diagnostics pipeline in discoverAllWithDiagnostics.
	Discovery struct {
		cfg     *config.Config
		baseDir types.FilesystemPath // replaces hardcoded "." — resolved once at construction
		//plint:internal -- distinguishes "not set" from "explicitly set to empty"
		baseDirSet  bool
		commandsDir types.FilesystemPath // replaces config.CommandsDir() call
		//plint:internal -- distinguishes "not set" from "explicitly set to empty"
		commandsDirSet bool
		//plint:internal -- errors from New() constructor surfaced as diagnostics
		initDiagnostics          []Diagnostic
		verifyVendoredIntegrity  bool
		provisionedModules       ProvisionedModuleEntries
		provisionedGlobalModules ProvisionedModuleEntries
	}

	//goplint:validate-all
	//
	// ProvisionedModuleEntry identifies a module copied into a provisioned
	// container layer and the command namespace it should publish.
	ProvisionedModuleEntry struct {
		// Path is the container filesystem path to a module or a directory containing modules.
		Path types.FilesystemPath
		// CommandNamespace preserves aliases/source IDs for copied module paths.
		CommandNamespace invowkmod.ModuleNamespace
	}

	// ProvisionedModuleEntries is a validated list of provisioned module entries.
	ProvisionedModuleEntries []ProvisionedModuleEntry

	// Option configures a Discovery instance via the functional options pattern.
	Option func(*Discovery)

	//goplint:ignore -- private collision-check DTO assembled from already parsed module metadata.
	commandSourceIdentity struct {
		ModuleID             invowkmod.ModuleID
		SourceID             SourceID
		SourcePath           string //goplint:ignore -- diagnostic display path may include vendored annotations.
		SourceKind           ModuleCollisionSourceKind
		ExplicitCommandScope bool
	}
)

// String returns the source kind as a string.
func (k ModuleCollisionSourceKind) String() string { return string(k) }

// Validate returns nil if the source kind is known.
func (k ModuleCollisionSourceKind) Validate() error {
	switch k {
	case ModuleCollisionSourceLocal, ModuleCollisionSourceVendored:
		return nil
	default:
		return fmt.Errorf("invalid module collision source kind %q", k)
	}
}

// Validate returns nil when the command source identity's typed fields are valid.
func (i commandSourceIdentity) Validate() error {
	return errors.Join(
		i.ModuleID.Validate(),
		i.SourceID.Validate(),
		i.SourceKind.Validate(),
	)
}

// Error implements the error interface.
func (e *ModuleCollisionError) Error() string {
	return fmt.Sprintf("module name collision: %q defined in multiple sources", e.Namespace)
}

// Unwrap returns ErrModuleCollision so callers can use errors.Is for programmatic detection.
func (e *ModuleCollisionError) Unwrap() error { return ErrModuleCollision }

// WithBaseDir sets the base directory for discovery, replacing the default of
// os.Getwd(). This enables parallel tests to inject isolated temp directories
// instead of relying on the process-global working directory.
func WithBaseDir(dir types.FilesystemPath) Option {
	return func(d *Discovery) {
		d.baseDir = dir
		d.baseDirSet = true
	}
}

// WithCommandsDir sets the user commands directory, replacing the default of
// config.CommandsDir() (~/.invowk/cmds). Pass an empty string to skip user-dir
// discovery entirely.
func WithCommandsDir(dir types.FilesystemPath) Option {
	return func(d *Discovery) {
		d.commandsDir = dir
		d.commandsDirSet = true
	}
}

// WithVendoredIntegrityVerification configures whether discovery aborts on
// vendored module hash mismatches before loading vendored command definitions.
func WithVendoredIntegrityVerification(enabled bool) Option {
	return func(d *Discovery) {
		d.verifyVendoredIntegrity = enabled
	}
}

// WithInitialDiagnostics prepends adapter-supplied diagnostics to discovery results.
func WithInitialDiagnostics(diagnostics []Diagnostic) Option {
	return func(d *Discovery) {
		d.initDiagnostics = append(d.initDiagnostics, slices.Clone(diagnostics)...)
	}
}

// WithProvisionedModuleEntries configures non-global modules copied into a
// provisioned container layer.
func WithProvisionedModuleEntries(entries ProvisionedModuleEntries) Option {
	return func(d *Discovery) {
		d.provisionedModules = slices.Clone(entries)
	}
}

// WithProvisionedGlobalModuleEntries configures globally trusted modules copied
// into a provisioned container layer.
func WithProvisionedGlobalModuleEntries(entries ProvisionedModuleEntries) Option {
	return func(d *Discovery) {
		d.provisionedGlobalModules = slices.Clone(entries)
	}
}

// Validate returns nil when the provisioned module entry is valid.
func (e ProvisionedModuleEntry) Validate() error {
	var errs []error
	if err := e.Path.Validate(); err != nil {
		errs = append(errs, err)
	}
	if e.CommandNamespace != "" {
		if err := e.CommandNamespace.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Validate returns nil when all provisioned module entries are valid.
func (e ProvisionedModuleEntries) Validate() error {
	var errs []error
	for i, entry := range e {
		if err := entry.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

// New creates a new Discovery instance. Without options, baseDir defaults to
// os.Getwd() and commandsDir defaults to config.CommandsDir(), preserving
// backward compatibility for all existing callers. If os.Getwd() fails
// (e.g., deleted working directory), baseDir is empty and current-dir
// discovery is effectively skipped.
func New(cfg *config.Config, opts ...Option) *Discovery {
	d := &Discovery{cfg: cfg, verifyVendoredIntegrity: true}
	for _, opt := range opts {
		opt(d)
	}
	if !d.baseDirSet && d.baseDir == "" {
		cwd, err := os.Getwd()
		if err == nil {
			d.baseDir = types.FilesystemPath(cwd) //goplint:ignore -- os.Getwd returns valid path or error
		} else {
			slog.Debug("failed to determine working directory for discovery, current-dir lookup will be skipped",
				"error", err)
			d.initDiagnostics = append(d.initDiagnostics, mustDiagnosticWithCause(
				SeverityWarning,
				CodeWorkingDirUnavailable,
				fmt.Sprintf("current directory unavailable, skipping local discovery: %v", err),
				"",
				err,
			))
		}
	}
	if !d.commandsDirSet && d.commandsDir == "" {
		if dir, err := config.CommandsDir(); err == nil {
			d.commandsDir = dir
		} else {
			slog.Debug("user commands directory unavailable, skipping user-dir discovery",
				"error", err)
			d.initDiagnostics = append(d.initDiagnostics, mustDiagnosticWithCause(
				SeverityWarning,
				CodeCommandsDirUnavailable,
				fmt.Sprintf("user commands directory unavailable, skipping user-dir discovery: %v", err),
				"",
				err,
			))
		}
	}
	return d
}

// LoadAll parses all discovered files into Invowkfile structs. Library-only modules
// (those without an invowkfile.cue) are skipped because they provide scripts and files
// for other modules via `requires` but contribute no commands. Module metadata is
// reattached to parsed Invowkfiles so downstream scope/dependency checks can identify
// the owning module.
func (d *Discovery) LoadAll() ([]*DiscoveredFile, error) {
	files, _, err := d.loadAllWithDiagnostics()
	if err != nil {
		return nil, err
	}

	return files, nil
}

// LoadFirst loads the first valid invowkfile found (respecting precedence).
// This method is currently used only in tests to verify single-file
// precedence behavior; production code uses LoadAll or the command set methods.
func (d *Discovery) LoadFirst() (*DiscoveredFile, error) {
	files, err := d.DiscoverAll()
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, ErrNoInvowkfileFound
	}

	file := files[0]
	var inv *invowkfile.Invowkfile
	var parseErr error

	if file.Module != nil {
		// Library-only modules can be discovered first; they are valid, but have
		// no command-bearing invowkfile to load.
		if file.Module.IsLibraryOnly || file.Path == "" {
			file.Invowkfile = nil
			return file, nil
		}

		inv, parseErr = invowkfile.ParseLoadedModuleInvowkfile(file.Module)
	} else {
		inv, parseErr = invowkfile.Parse(file.Path)
	}

	if parseErr != nil {
		file.Error = parseErr
		return file, parseErr
	}

	file.Invowkfile = inv
	return file, nil
}

// CheckModuleCollisions checks for module identity and command-source collisions
// among discovered files. Duplicate module identities remain a hard collision
// unless an explicit command namespace/alias disambiguates them; duplicate
// command source IDs always collide because they publish into the same command
// namespace.
func (d *Discovery) CheckModuleCollisions(files []*DiscoveredFile) error {
	// Map module identities separately from command source IDs so stable module
	// identity policy and command-publication namespace policy cannot drift.
	// Values are display strings (may include annotations like "vendored in ...").
	moduleSources := make(map[invowkmod.ModuleID]commandSourceIdentity)
	commandSources := make(map[SourceID]string)

	for _, file := range files {
		if file == nil || file.Error != nil {
			continue
		}

		moduleSource, hasModuleIdentity := d.moduleIdentityFor(file)
		if hasModuleIdentity {
			if existing, exists := moduleSources[moduleSource.ModuleID]; exists && !existing.ExplicitCommandScope && !moduleSource.ExplicitCommandScope {
				namespace := SourceID(moduleSource.ModuleID)
				if err := namespace.Validate(); err != nil {
					return fmt.Errorf("invalid module namespace %q: %w", namespace, err)
				}
				return &ModuleCollisionError{
					Namespace:    namespace,
					FirstSource:  existing.SourcePath,
					SecondSource: moduleSource.SourcePath,
					SecondKind:   moduleSource.SourceKind,
				}
			}
			moduleSources[moduleSource.ModuleID] = moduleSource
		}

		commandSource, ok := d.commandSourceFor(file)
		if !ok {
			continue
		}

		if existingSource, exists := commandSources[commandSource.SourceID]; exists {
			if err := commandSource.SourceID.Validate(); err != nil {
				return fmt.Errorf("invalid command namespace %q: %w", commandSource.SourceID, err)
			}
			return &ModuleCollisionError{
				Namespace:    commandSource.SourceID,
				FirstSource:  existingSource,
				SecondSource: commandSource.SourcePath,
				SecondKind:   commandSource.SourceKind,
			}
		}

		commandSources[commandSource.SourceID] = commandSource.SourcePath
	}

	return nil
}

// GetEffectiveCommandNamespace returns the command-source namespace used for
// command publication and collision checks, considering aliases from the
// includes config and vendored lock metadata.
func (d *Discovery) GetEffectiveCommandNamespace(file *DiscoveredFile) SourceID {
	source, ok := d.commandSourceFor(file)
	if !ok {
		return ""
	}
	return source.SourceID
}

func (d *Discovery) moduleIdentityFor(file *DiscoveredFile) (commandSourceIdentity, bool) {
	if file == nil {
		return commandSourceIdentity{}, false
	}

	var moduleID invowkmod.ModuleID
	sourcePath := string(file.Path)
	if file.Module != nil {
		sourcePath = string(file.Module.Path)
		if file.Module.Metadata != nil {
			moduleID = file.Module.Metadata.Module
		}
	}
	if moduleID == "" && file.Invowkfile != nil {
		moduleID = file.Invowkfile.GetModule()
	}
	if moduleID == "" {
		return commandSourceIdentity{}, false
	}

	source := commandSourceIdentity{
		ModuleID:   moduleID,
		SourceID:   SourceID(moduleID),
		SourcePath: sourcePath,
		SourceKind: ModuleCollisionSourceLocal,
	}
	if file.Module != nil {
		if file.CommandNamespace != "" {
			source.SourceID = SourceID(file.CommandNamespace)
			source.ExplicitCommandScope = true
		} else if includeAlias := d.getAliasForModulePath(file.Module.Path); includeAlias != "" {
			source.SourceID = SourceID(includeAlias)
			source.ExplicitCommandScope = true
		}
	}
	if file.ParentModule != nil {
		source.SourcePath = fmt.Sprintf("%s (vendored in %s)", source.SourcePath, file.ParentModule.Name())
		source.SourceKind = ModuleCollisionSourceVendored
	}
	return source, true
}

func (d *Discovery) commandSourceFor(file *DiscoveredFile) (commandSourceIdentity, bool) {
	source, ok := d.moduleIdentityFor(file)
	if !ok || file.Invowkfile == nil {
		return commandSourceIdentity{}, false
	}
	if file.Module != nil && !source.ExplicitCommandScope {
		source.SourceID = SourceID(getModuleShortName(file.Module.Path))
	}
	return source, true
}

// getAliasForModulePath looks up an alias for the given module directory path
// from the includes config. Paths are normalized with filepath.Clean before
// comparison to handle trailing slashes and redundant separators. Returns the
// alias if found, or empty ModuleAlias if no alias is configured.
func (d *Discovery) getAliasForModulePath(modulePath types.FilesystemPath) invowkmod.ModuleAlias {
	if d.cfg == nil {
		return ""
	}

	cleanPath := filepath.Clean(string(modulePath))

	for _, inc := range d.cfg.Includes {
		if inc.Alias != "" && filepath.Clean(string(inc.Path)) == cleanPath {
			return inc.Alias
		}
	}

	return ""
}

// loadAllWithDiagnostics parses discovered files and returns non-fatal
// discovery diagnostics (e.g., skipped includes/modules) alongside files.
func (d *Discovery) loadAllWithDiagnostics() ([]*DiscoveredFile, []Diagnostic, error) {
	files, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		return nil, diagnostics, err
	}

	for _, file := range files {
		var inv *invowkfile.Invowkfile
		var parseErr error

		if file.Module != nil {
			// Library-only modules provide scripts and files for other modules to
			// reference via `requires`, but don't contribute their own command
			// definitions to the CLI command tree.
			if file.Module.IsLibraryOnly || file.Path == "" {
				continue
			}

			// Parse module invowkfile.cue and reattach module metadata so downstream
			// logic (scope/dependency checks) can treat it as module-backed input.
			inv, parseErr = invowkfile.ParseLoadedModuleInvowkfile(file.Module)
		} else {
			inv, parseErr = invowkfile.Parse(file.Path)
		}

		if parseErr != nil {
			file.Error = parseErr
		} else {
			file.Invowkfile = inv
		}
	}

	// Detect module ID collisions after all files are parsed. This catches
	// two modules that declare the same module identifier and neither has
	// an alias to disambiguate. Callers receive a ModuleCollisionError with
	// actionable remediation (add an alias in the includes config).
	if err := d.CheckModuleCollisions(files); err != nil {
		return nil, diagnostics, err
	}

	return files, diagnostics, nil
}
