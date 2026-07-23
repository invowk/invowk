// SPDX-License-Identifier: MPL-2.0

// Package profileownership conservatively routes goplint assurance profiles.
package profileownership

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/invowk/invowk/tools/goplint/internal/soundnessgate"
)

const FormatVersion = 1

type (
	// Manifest maps governed repository paths to their minimum assurance profile.
	Manifest struct {
		FormatVersion int    `json:"format_version"`
		Rules         []Rule `json:"rules"`
	}

	// Rule owns one exact path or recursive prefix pattern.
	Rule struct {
		Pattern string                  `json:"pattern"`
		Profile soundnessgate.ProfileID `json:"profile"`
	}

	// Context is the complete event and changed-path routing input.
	Context struct {
		Event              string
		ChangedPaths       []string
		MergeBaseAvailable bool
		ShallowRepository  bool
	}

	// Decision is a deterministic profile and visible conservative reason.
	Decision struct {
		Profile soundnessgate.ProfileID `json:"profile"`
		Reason  string                  `json:"reason"`
		Paths   []string                `json:"paths"`
	}
)

// Load strictly decodes and validates an ownership manifest.
func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading ownership manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decoding ownership manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Validate checks canonical, non-ambiguous exact or recursive-prefix rules.
func (manifest Manifest) Validate() error {
	if manifest.FormatVersion != FormatVersion || len(manifest.Rules) == 0 {
		return errors.New("goplint ownership manifest has an invalid version or empty rule set")
	}
	previous := ""
	for index, rule := range manifest.Rules {
		if rule.Pattern == "" || filepath.IsAbs(rule.Pattern) || strings.ContainsAny(rule.Pattern, "\\\x00\r\n") ||
			strings.Contains(rule.Pattern, "..") {
			return fmt.Errorf("goplint ownership rules[%d] has unsafe pattern %q", index, rule.Pattern)
		}
		if strings.Contains(rule.Pattern, "*") && !strings.HasSuffix(rule.Pattern, "/**") {
			return fmt.Errorf("goplint ownership rule %q is not an exact path or recursive prefix", rule.Pattern)
		}
		if rule.Profile != soundnessgate.ProfileConsumer && rule.Profile != soundnessgate.ProfileSemantic {
			return fmt.Errorf("goplint ownership rule %q has invalid profile %q", rule.Pattern, rule.Profile)
		}
		if previous != "" && rule.Pattern <= previous {
			return errors.New("goplint ownership rules are duplicate or non-canonical")
		}
		previous = rule.Pattern
	}
	return nil
}

// Route selects completion for exhaustive events, consumer only for proven
// consumer-only path sets, and semantic for every missing or ambiguous case.
func (manifest Manifest) Route(input Context) (Decision, error) {
	if err := manifest.Validate(); err != nil {
		return Decision{}, err
	}
	paths, err := canonicalPaths(input.ChangedPaths)
	if err != nil {
		return Decision{Profile: soundnessgate.ProfileSemantic, Reason: "invalid changed-path context", Paths: []string{}}, nil
	}
	switch input.Event {
	case "completion", "release", "schedule", "workflow_dispatch":
		return Decision{Profile: soundnessgate.ProfileComplete, Reason: "event requires exhaustive completion evidence", Paths: paths}, nil
	case "pull_request", "push", "pre_commit":
	default:
		return Decision{Profile: soundnessgate.ProfileSemantic, Reason: "unknown or ambiguous event context", Paths: paths}, nil
	}
	if input.ShallowRepository || !input.MergeBaseAvailable || len(paths) == 0 {
		return Decision{Profile: soundnessgate.ProfileSemantic, Reason: "changed-path proof is missing or incomplete", Paths: paths}, nil
	}
	for _, path := range paths {
		profile, matched := manifest.profileForPath(path)
		if !matched || profile == soundnessgate.ProfileSemantic {
			reason := "semantic ownership path changed"
			if !matched {
				reason = "unknown path fails closed"
			}
			return Decision{Profile: soundnessgate.ProfileSemantic, Reason: reason, Paths: paths}, nil
		}
	}
	return Decision{
		Profile: soundnessgate.ProfileConsumer,
		Reason:  "all changed paths are proven root consumer ownership",
		Paths:   paths,
	}, nil
}

func (manifest Manifest) profileForPath(path string) (soundnessgate.ProfileID, bool) {
	for _, rule := range manifest.Rules {
		if rule.Pattern == path {
			return rule.Profile, true
		}
		if prefix, recursive := strings.CutSuffix(rule.Pattern, "/**"); recursive &&
			(path == prefix || strings.HasPrefix(path, prefix+"/")) {
			return rule.Profile, true
		}
	}
	return "", false
}

func canonicalPaths(paths []string) ([]string, error) {
	result := slices.Clone(paths)
	for index := range result {
		result[index] = filepath.ToSlash(filepath.Clean(strings.TrimSpace(result[index])))
		if result[index] == "." || result[index] == ".." || strings.HasPrefix(result[index], "../") ||
			filepath.IsAbs(result[index]) || strings.ContainsAny(result[index], "\x00\r\n") {
			return nil, fmt.Errorf("unsafe changed path %q", result[index])
		}
	}
	slices.Sort(result)
	result = slices.Compact(result)
	return result, nil
}
