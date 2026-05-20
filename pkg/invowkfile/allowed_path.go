// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const allowedPathValueMaxRunes = 4096

var (
	// ErrInvalidAllowedPathName is returned when an allowed_paths key cannot be exposed as a safe env suffix.
	ErrInvalidAllowedPathName = errors.New("invalid allowed path name")
	// ErrInvalidAllowedPathConfig is returned when an allowed_paths value is malformed.
	ErrInvalidAllowedPathConfig = errors.New("invalid allowed path config")

	allowedPathNameRegex = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
)

type (
	// AllowedPathName is a logical path key exposed as INVOWK_PATH_<NAME>.
	AllowedPathName string

	// AllowedPaths maps logical names to common or platform-specific path values.
	AllowedPaths map[AllowedPathName]any

	// InvalidAllowedPathNameError reports an invalid allowed_paths key.
	InvalidAllowedPathNameError struct {
		Value AllowedPathName
	}

	// InvalidAllowedPathConfigError reports a malformed allowed_paths value.
	InvalidAllowedPathConfigError struct {
		Name   AllowedPathName
		Reason string
	}
)

func (n AllowedPathName) String() string { return string(n) }

// Validate returns nil when the allowed path name is a safe environment suffix.
func (n AllowedPathName) Validate() error {
	if !allowedPathNameRegex.MatchString(string(n)) {
		return &InvalidAllowedPathNameError{Value: n}
	}
	return nil
}

func (e *InvalidAllowedPathNameError) Error() string {
	return fmt.Sprintf("invalid allowed_paths key %q (must match [A-Z_][A-Z0-9_]*)", e.Value)
}

func (e *InvalidAllowedPathNameError) Unwrap() error { return ErrInvalidAllowedPathName }

func (e *InvalidAllowedPathConfigError) Error() string {
	if e.Name == "" {
		return "invalid allowed_paths config: " + e.Reason
	}
	return fmt.Sprintf("invalid allowed_paths[%q]: %s", e.Name, e.Reason)
}

func (e *InvalidAllowedPathConfigError) Unwrap() error { return ErrInvalidAllowedPathConfig }

// ValidateForPlatforms returns nil when all mappings are valid for the selected implementation platforms.
func (p AllowedPaths) ValidateForPlatforms(platforms []PlatformConfig) error {
	if len(p) == 0 {
		return nil
	}
	var errs []error
	for name, raw := range p {
		if err := name.Validate(); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, common, err := allowedPathPlatformMap(name, raw); err != nil {
			errs = append(errs, err)
		} else if !common {
			for _, platform := range platforms {
				if _, ok, pathErr := p.PathForPlatform(name, platform.Name); pathErr != nil {
					errs = append(errs, pathErr)
				} else if !ok {
					errs = append(errs, &InvalidAllowedPathConfigError{
						Name:   name,
						Reason: fmt.Sprintf("missing %q mapping for implementation platform", platform.Name),
					})
				}
			}
		}
	}
	return errors.Join(errs...)
}

// PathForPlatform returns the path mapped to name for the requested platform.
func (p AllowedPaths) PathForPlatform(name AllowedPathName, platform PlatformType) (path string, ok bool, err error) {
	raw, ok := p[name]
	if !ok {
		return "", false, nil
	}
	platformMap, common, err := allowedPathPlatformMap(name, raw)
	if err != nil {
		return "", false, err
	}
	if common {
		return platformMap[""], true, nil
	}
	value, ok := platformMap[platform]
	return value, ok, nil
}

func allowedPathPlatformMap(name AllowedPathName, raw any) (paths map[PlatformType]string, common bool, err error) {
	switch value := raw.(type) {
	case string:
		if err := validateAllowedPathValue(name, value); err != nil {
			return nil, false, err
		}
		return map[PlatformType]string{"": value}, true, nil
	case map[string]any:
		return allowedPathMapFromAny(name, value)
	case map[string]string:
		converted := make(map[string]any, len(value))
		for k, v := range value {
			converted[k] = v
		}
		return allowedPathMapFromAny(name, converted)
	case map[PlatformType]string:
		converted := make(map[string]any, len(value))
		for k, v := range value {
			converted[k.String()] = v
		}
		return allowedPathMapFromAny(name, converted)
	default:
		return nil, false, &InvalidAllowedPathConfigError{Name: name, Reason: "value must be a string or platform-keyed object"}
	}
}

func allowedPathMapFromAny(name AllowedPathName, raw map[string]any) (paths map[PlatformType]string, common bool, err error) {
	if len(raw) == 0 {
		return nil, false, &InvalidAllowedPathConfigError{Name: name, Reason: "platform-keyed object must not be empty"}
	}
	paths = make(map[PlatformType]string, len(raw))
	for rawPlatform, rawPath := range raw {
		platform := PlatformType(rawPlatform)
		if err := platform.Validate(); err != nil {
			return nil, false, &InvalidAllowedPathConfigError{Name: name, Reason: err.Error()}
		}
		path, ok := rawPath.(string)
		if !ok {
			return nil, false, &InvalidAllowedPathConfigError{Name: name, Reason: rawPlatform + " path must be a string"}
		}
		if err := validateAllowedPathValue(name, path); err != nil {
			return nil, false, err
		}
		paths[platform] = path
	}
	return paths, false, nil
}

func validateAllowedPathValue(name AllowedPathName, value string) error {
	if strings.TrimSpace(value) == "" {
		return &InvalidAllowedPathConfigError{Name: name, Reason: "path must not be empty or whitespace-only"}
	}
	if len([]rune(value)) > allowedPathValueMaxRunes {
		return &InvalidAllowedPathConfigError{Name: name, Reason: fmt.Sprintf("path must be at most %d runes", allowedPathValueMaxRunes)}
	}
	return nil
}
