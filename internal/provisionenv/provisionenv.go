// SPDX-License-Identifier: MPL-2.0

// Package provisionenv defines the environment contract between container
// provisioning and nested command discovery.
package provisionenv

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/invowkmod"
)

const (
	// ModulePathName is the fallback path-list env var for provisioned modules.
	ModulePathName Name = "INVOWK_MODULE_PATH"
	// ModuleManifestName carries provisioned module paths and namespaces.
	ModuleManifestName Name = "INVOWK_MODULE_MANIFEST"
	// GlobalModulePathName is the fallback path-list env var for global modules.
	GlobalModulePathName Name = "INVOWK_GLOBAL_MODULE_PATH"
	// GlobalModuleManifestName carries global module paths and namespaces.
	GlobalModuleManifestName Name = "INVOWK_GLOBAL_MODULE_MANIFEST"

	invalidManifestWrapFormat = "%w: %w"
)

var (
	// ErrInvalidName is returned when a provision env variable name is invalid.
	ErrInvalidName = errors.New("invalid provision env name")
	// ErrInvalidManifest is returned when a provisioned module manifest is malformed.
	ErrInvalidManifest = errors.New("invalid provisioned module manifest")
)

type (
	// Name is a provisioned-module environment variable name.
	Name string

	// Value is a provisioned-module environment variable value.
	Value string

	//goplint:validate-all
	//
	// Entry is one provisioned module manifest entry.
	Entry struct {
		// Path is the module path inside the container.
		Path container.MountTargetPath `json:"path"`
		// CommandNamespace is the command source namespace for the copied module.
		CommandNamespace invowkmod.ModuleNamespace `json:"command_namespace"` //goplint:ignore -- optional namespace has no non-empty invalid state.
	}

	// Entries is a validated provisioned-module manifest entry list.
	Entries []Entry
)

// String returns the string representation of the name.
func (n Name) String() string { return string(n) }

// Validate returns nil when the name is non-empty.
func (n Name) Validate() error {
	if n == "" {
		return ErrInvalidName
	}
	return nil
}

// String returns the string representation of the value.
func (v Value) String() string { return string(v) }

// Validate returns nil. Provision env values are parsed by contract-specific helpers.
func (v Value) Validate() error { return nil }

// IsSet reports whether the value is non-empty after trimming whitespace.
func (v Value) IsSet() bool {
	return strings.TrimSpace(v.String()) != ""
}

// Validate returns nil when all required manifest fields are valid.
func (e Entry) Validate() error {
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

// Validate returns nil when all manifest entries are valid.
func (e Entries) Validate() error {
	var errs []error
	for i, entry := range e {
		if err := entry.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

// MarshalManifest serializes entries into the provisioned-module manifest JSON.
func MarshalManifest(entries Entries) (Value, error) {
	if err := entries.Validate(); err != nil {
		return "", fmt.Errorf(invalidManifestWrapFormat, ErrInvalidManifest, err)
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf(invalidManifestWrapFormat, ErrInvalidManifest, err)
	}
	return Value(data), nil
}

// ParseManifest parses a non-empty provisioned-module manifest.
func ParseManifest(value Value) (Entries, error) {
	rawValue := strings.TrimSpace(value.String())
	if rawValue == "" {
		return nil, nil
	}
	var entries Entries
	if err := json.Unmarshal([]byte(rawValue), &entries); err != nil {
		return nil, fmt.Errorf(invalidManifestWrapFormat, ErrInvalidManifest, err)
	}
	if err := entries.Validate(); err != nil {
		return nil, fmt.Errorf(invalidManifestWrapFormat, ErrInvalidManifest, err)
	}
	return entries, nil
}

// ParseEnvironment parses the manifest when present; otherwise it falls back to
// the legacy path-list value. An invalid present manifest returns an error and
// never falls back to path-list discovery because that loses source identity.
func ParseEnvironment(manifestValue, pathListValue Value) (Entries, error) {
	if manifestValue.IsSet() {
		return ParseManifest(manifestValue)
	}
	return EntriesFromPathList(pathListValue), nil
}

// EntriesFromPathList converts a legacy module path-list env value to entries
// without explicit command namespaces.
func EntriesFromPathList(value Value) Entries {
	rawValue := strings.TrimSpace(value.String())
	var entries Entries
	for _, rawPath := range filepath.SplitList(rawValue) {
		path := strings.TrimSpace(rawPath)
		entryPath := container.MountTargetPath(path)
		if err := entryPath.Validate(); err != nil {
			continue
		}
		entries = append(entries, Entry{Path: entryPath})
	}
	return entries
}
