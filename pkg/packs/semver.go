// SPDX-License-Identifier: EPL-2.0

package packs

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// SemverResolver handles semantic version constraint resolution.
type SemverResolver struct{}

// NewSemverResolver creates a new semver resolver.
func NewSemverResolver() *SemverResolver {
	return &SemverResolver{}
}

// Version represents a parsed semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Original   string
}

// Constraint represents a version constraint.
type Constraint struct {
	// Op is the comparison operator (=, ^, ~, >, >=, <, <=).
	Op string
	// Version is the version to compare against.
	Version *Version
	// Original is the original constraint string.
	Original string
}

// semverRegex matches semantic version strings.
var semverRegex = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([0-9A-Za-z\-\.]+))?(?:\+([0-9A-Za-z\-\.]+))?$`)

// constraintRegex matches version constraint strings.
var constraintRegex = regexp.MustCompile(`^([~^]|>=|<=|>|<|=)?v?(\d+(?:\.\d+)?(?:\.\d+)?(?:-[0-9A-Za-z\-\.]+)?)$`)

// ParseVersion parses a version string into a Version struct.
func ParseVersion(s string) (*Version, error) {
	matches := semverRegex.FindStringSubmatch(s)
	if matches == nil {
		return nil, fmt.Errorf("invalid version format: %s", s)
	}

	v := &Version{Original: s}

	var err error
	v.Major, err = strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %w", err)
	}

	if matches[2] != "" {
		v.Minor, err = strconv.Atoi(matches[2])
		if err != nil {
			return nil, fmt.Errorf("invalid minor version: %w", err)
		}
	}

	if matches[3] != "" {
		v.Patch, err = strconv.Atoi(matches[3])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version: %w", err)
		}
	}

	if matches[4] != "" {
		v.Prerelease = matches[4]
	}

	return v, nil
}

// String returns the version as a string.
func (v *Version) String() string {
	return v.Original
}

// Compare compares two versions.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Prerelease versions have lower precedence
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease != other.Prerelease {
		if v.Prerelease < other.Prerelease {
			return -1
		}
		return 1
	}

	return 0
}

// ParseConstraint parses a version constraint string.
func (r *SemverResolver) ParseConstraint(s string) (*Constraint, error) {
	s = strings.TrimSpace(s)

	matches := constraintRegex.FindStringSubmatch(s)
	if matches == nil {
		return nil, fmt.Errorf("invalid constraint format: %s", s)
	}

	op := matches[1]
	if op == "" {
		op = "="
	}

	version, err := ParseVersion(matches[2])
	if err != nil {
		return nil, fmt.Errorf("invalid version in constraint: %w", err)
	}

	return &Constraint{
		Op:       op,
		Version:  version,
		Original: s,
	}, nil
}

// Matches checks if a version satisfies the constraint.
func (c *Constraint) Matches(v *Version) bool {
	switch c.Op {
	case "=":
		return v.Compare(c.Version) == 0

	case "^":
		// Caret: allows changes that do not modify the left-most non-zero digit
		// ^1.2.3 := >=1.2.3 <2.0.0
		// ^0.2.3 := >=0.2.3 <0.3.0
		// ^0.0.3 := >=0.0.3 <0.0.4
		if v.Compare(c.Version) < 0 {
			return false
		}
		if c.Version.Major != 0 {
			return v.Major == c.Version.Major
		}
		if c.Version.Minor != 0 {
			return v.Major == 0 && v.Minor == c.Version.Minor
		}
		return v.Major == 0 && v.Minor == 0 && v.Patch == c.Version.Patch

	case "~":
		// Tilde: allows patch-level changes
		// ~1.2.3 := >=1.2.3 <1.3.0
		if v.Compare(c.Version) < 0 {
			return false
		}
		return v.Major == c.Version.Major && v.Minor == c.Version.Minor

	case ">":
		return v.Compare(c.Version) > 0

	case ">=":
		return v.Compare(c.Version) >= 0

	case "<":
		return v.Compare(c.Version) < 0

	case "<=":
		return v.Compare(c.Version) <= 0

	default:
		return false
	}
}

// Resolve finds the best matching version for a constraint.
func (r *SemverResolver) Resolve(constraintStr string, availableVersions []string) (string, error) {
	constraint, err := r.ParseConstraint(constraintStr)
	if err != nil {
		return "", err
	}

	// Parse all available versions
	var versions []*Version
	for _, vs := range availableVersions {
		v, err := ParseVersion(vs)
		if err != nil {
			continue // Skip invalid versions
		}
		versions = append(versions, v)
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no valid versions available")
	}

	// Filter versions that match the constraint
	var matching []*Version
	for _, v := range versions {
		if constraint.Matches(v) {
			matching = append(matching, v)
		}
	}

	if len(matching) == 0 {
		return "", fmt.Errorf("no version matches constraint %q (available: %v)", constraintStr, availableVersions)
	}

	// Sort by version (highest first)
	sort.Slice(matching, func(i, j int) bool {
		return matching[i].Compare(matching[j]) > 0
	})

	// Return the highest matching version
	return matching[0].Original, nil
}

// IsValidVersion checks if a string is a valid semantic version.
func IsValidVersion(s string) bool {
	_, err := ParseVersion(s)
	return err == nil
}

// IsValidConstraint checks if a string is a valid version constraint.
func IsValidConstraint(s string) bool {
	r := &SemverResolver{}
	_, err := r.ParseConstraint(s)
	return err == nil
}

// SortVersions sorts a slice of version strings in descending order (newest first).
func SortVersions(versions []string) []string {
	var parsed []*Version
	for _, vs := range versions {
		v, err := ParseVersion(vs)
		if err != nil {
			continue
		}
		parsed = append(parsed, v)
	}

	sort.Slice(parsed, func(i, j int) bool {
		return parsed[i].Compare(parsed[j]) > 0
	})

	result := make([]string, len(parsed))
	for i, v := range parsed {
		result[i] = v.Original
	}

	return result
}

// FilterVersions filters a slice of version strings by a constraint.
func FilterVersions(constraintStr string, versions []string) ([]string, error) {
	r := &SemverResolver{}
	constraint, err := r.ParseConstraint(constraintStr)
	if err != nil {
		return nil, err
	}

	var matching []string
	for _, vs := range versions {
		v, err := ParseVersion(vs)
		if err != nil {
			continue
		}
		if constraint.Matches(v) {
			matching = append(matching, vs)
		}
	}

	return matching, nil
}
