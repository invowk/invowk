// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// EnvConfigPath names the environment variable that selects the config
	// file when --ivk-config is not given. Its value must be an absolute path.
	EnvConfigPath = "INVOWK_CONFIG"

	// ConfigFlagName is the root flag that selects the config file explicitly.
	ConfigFlagName = "--ivk-config"

	// ConfigSourceFlag means the --ivk-config flag selected the config file.
	ConfigSourceFlag ConfigSourceKind = "flag"
	// ConfigSourceEnv means the INVOWK_CONFIG environment variable selected the config file.
	ConfigSourceEnv ConfigSourceKind = "env"
	// ConfigSourceUser means the per-user config file was loaded.
	ConfigSourceUser ConfigSourceKind = "user"
	// ConfigSourceDefault means no config file applied and built-in defaults were used.
	ConfigSourceDefault ConfigSourceKind = "default"
	// ConfigSourceImplicit is a selection-only kind: no explicit source was given,
	// so loading decides between ConfigSourceUser and ConfigSourceDefault by
	// checking whether the user config file exists. It is never a LoadResult source.
	ConfigSourceImplicit ConfigSourceKind = "implicit"

	errMsgConfigEnvNotAbsolute = "INVOWK_CONFIG must be an absolute path"
	errMsgExplicitConfig       = "explicit config source failed"
	errMsgInvalidSourceKind    = "invalid config source kind"
)

var (
	// ErrConfigEnvNotAbsolute marks a relative INVOWK_CONFIG value. invowk never
	// resolves it against the current directory.
	ErrConfigEnvNotAbsolute = errors.New(errMsgConfigEnvNotAbsolute)
	// ErrExplicitConfig marks any failure of an explicit config source
	// (--ivk-config or INVOWK_CONFIG). Callers use it to fail closed instead of
	// falling back to defaults.
	ErrExplicitConfig = errors.New(errMsgExplicitConfig)
	// ErrInvalidConfigSourceKind is returned when a ConfigSourceKind value is not recognized.
	ErrInvalidConfigSourceKind = errors.New(errMsgInvalidSourceKind)
)

type (
	// ConfigSourceKind identifies which config source was selected or loaded.
	//
	//nolint:revive // ConfigSourceKind reads better than SourceKind at call sites outside the package.
	ConfigSourceKind string

	// InvalidConfigSourceKindError is returned when a ConfigSourceKind value is not recognized.
	// It wraps ErrInvalidConfigSourceKind for errors.Is() compatibility.
	InvalidConfigSourceKindError struct {
		Value ConfigSourceKind
	}

	// SourceSelection is the outcome of SelectSource: which explicit source (if
	// any) applies, the raw path it names, and a Problem that makes the
	// selection unusable without touching the filesystem (a relative
	// INVOWK_CONFIG value).
	SourceSelection struct {
		Kind    ConfigSourceKind
		Path    types.FilesystemPath //goplint:ignore -- empty for the implicit selection; Validate requires it for flag and env.
		Problem error
	}

	// ExplicitConfigError wraps every failure of an explicit config source:
	// a missing file, a CUE load error, a validation failure, or a relative
	// INVOWK_CONFIG value. Unwrap exposes both the cause and ErrExplicitConfig,
	// so errors.Is(err, ErrConfigFileNotFound) keeps working.
	ExplicitConfigError struct {
		Source ConfigSourceKind
		Path   types.FilesystemPath
		Err    error
	}
)

// SelectSource picks the config source by precedence without touching the
// filesystem: --ivk-config (opts.ConfigFilePath), then INVOWK_CONFIG, then the
// implicit user-file-or-defaults choice. A lower-precedence source is never
// read once a higher one is selected, so INVOWK_CONFIG is not even looked up
// when the flag is set. An empty INVOWK_CONFIG counts as unset. A nil getenv
// behaves as an empty environment.
func SelectSource(opts LoadOptions, getenv func(string) string) SourceSelection {
	if opts.ConfigFilePath != "" {
		return SourceSelection{Kind: ConfigSourceFlag, Path: opts.ConfigFilePath}
	}
	if getenv == nil {
		return SourceSelection{Kind: ConfigSourceImplicit}
	}
	envValue := getenv(EnvConfigPath)
	if envValue == "" {
		return SourceSelection{Kind: ConfigSourceImplicit}
	}
	selection := SourceSelection{
		Kind: ConfigSourceEnv,
		Path: types.FilesystemPath(envValue), //goplint:ignore -- raw env value; absolute-path rule checked below and reported via Problem.
	}
	if !filepath.IsAbs(envValue) {
		selection.Problem = ErrConfigEnvNotAbsolute
	}
	return selection
}

// String returns the string representation of the ConfigSourceKind.
func (k ConfigSourceKind) String() string { return string(k) }

// Validate returns an error if the ConfigSourceKind is not one of the defined kinds.
func (k ConfigSourceKind) Validate() error {
	switch k {
	case ConfigSourceFlag, ConfigSourceEnv, ConfigSourceUser, ConfigSourceDefault, ConfigSourceImplicit:
		return nil
	default:
		return &InvalidConfigSourceKindError{Value: k}
	}
}

// IsExplicit reports whether the kind names a source the user selected on
// purpose (--ivk-config or INVOWK_CONFIG). Explicit sources fail closed.
func (k ConfigSourceKind) IsExplicit() bool {
	return k == ConfigSourceFlag || k == ConfigSourceEnv
}

// Label returns the user-facing name of an explicit source ("--ivk-config" or
// "INVOWK_CONFIG"), or the kind itself for the other kinds.
//
//goplint:ignore -- returns display text for error messages and CLI output.
func (k ConfigSourceKind) Label() string {
	switch k {
	case ConfigSourceFlag:
		return ConfigFlagName
	case ConfigSourceEnv:
		return EnvConfigPath
	case ConfigSourceUser, ConfigSourceDefault, ConfigSourceImplicit:
		return string(k)
	default:
		return string(k)
	}
}

// Error implements the error interface for InvalidConfigSourceKindError.
func (e *InvalidConfigSourceKindError) Error() string {
	return fmt.Sprintf("invalid config source kind %q", e.Value)
}

// Unwrap returns ErrInvalidConfigSourceKind for errors.Is() compatibility.
func (e *InvalidConfigSourceKindError) Unwrap() error { return ErrInvalidConfigSourceKind }

// Validate returns nil when the selection kind is known, explicit selections
// carry a path, and an implicit selection carries neither a path nor a problem.
func (s SourceSelection) Validate() error {
	var errs []error
	if err := s.Kind.Validate(); err != nil {
		errs = append(errs, err)
	}
	switch s.Kind {
	case ConfigSourceFlag, ConfigSourceEnv:
		if err := s.Path.Validate(); err != nil {
			errs = append(errs, err)
		}
	case ConfigSourceImplicit:
		if s.Path != "" || s.Problem != nil {
			errs = append(errs, errors.New("implicit config selection must not carry a path or problem"))
		}
	case ConfigSourceUser, ConfigSourceDefault:
		errs = append(errs, fmt.Errorf("config source kind %q is not a selection kind", s.Kind))
	}
	return errors.Join(errs...)
}

// IsExplicit reports whether the selection names --ivk-config or INVOWK_CONFIG.
func (s SourceSelection) IsExplicit() bool { return s.Kind.IsExplicit() }

// Error names the explicit source and its path before the cause.
func (e *ExplicitConfigError) Error() string {
	return fmt.Sprintf("config from %s %q: %v", e.Source.Label(), e.Path, e.Err)
}

// Unwrap returns the cause and ErrExplicitConfig for errors.Is() compatibility.
func (e *ExplicitConfigError) Unwrap() []error {
	return []error{e.Err, ErrExplicitConfig}
}

// wrapExplicitSourceError wraps err in ExplicitConfigError when the selection
// is explicit. Implicit selections return err unchanged so the caller keeps
// the user-file fallback behaviour.
func wrapExplicitSourceError(selection SourceSelection, err error) error {
	if err == nil || !selection.IsExplicit() {
		return err
	}
	if existing, ok := errors.AsType[*ExplicitConfigError](err); ok && existing != nil {
		return err
	}
	return &ExplicitConfigError{Source: selection.Kind, Path: selection.Path, Err: err}
}
