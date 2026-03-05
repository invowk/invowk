// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const semanticRulesCatalogVersion = "v1"

var semanticOutcomeDomainAllowed = []string{"safe", "unsafe", "inconclusive"}

type semanticRuleCatalog struct {
	Version                string               `json:"version"`
	Rules                  []semanticRuleSpec   `json:"rules"`
	OracleMatrix           []semanticOracleSpec `json:"oracle_matrix"`
	HistoricalMissFixtures []string             `json:"historical_miss_fixtures"`
}

type semanticRuleSpec struct {
	Category                   string   `json:"category"`
	Family                     string   `json:"family"`
	Entrypoints                []string `json:"entrypoints"`
	EnabledByFlags             []string `json:"enabled_by_flags"`
	RunControls                []string `json:"run_controls"`
	TraversalMode              string   `json:"traversal_mode"`
	StateDomain                []string `json:"state_domain"`
	OutcomeDomain              []string `json:"outcome_domain"`
	InconclusiveReasons        []string `json:"inconclusive_reasons,omitempty"`
	RequiredMetaOnInconclusive []string `json:"required_meta_on_inconclusive,omitempty"`
	BaselinePolicy             string   `json:"baseline_policy"`
}

type semanticOracleSpec struct {
	Category      string                `json:"category"`
	MustReport    []semanticOracleEntry `json:"must_report"`
	MustNotReport []semanticOracleEntry `json:"must_not_report"`
}

type semanticOracleEntry struct {
	Fixture string `json:"fixture"`
	Symbol  string `json:"symbol"`
}

func semanticRulesCatalogPath() string {
	return filepath.Join(goplintModuleRootPath(), "spec", "semantic-rules.v1.json")
}

func semanticRulesSchemaPath() string {
	return filepath.Join(goplintModuleRootPath(), "spec", "schema", "semantic-rules.schema.json")
}

func goplintModuleRootPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Clean("..")
	}
	// semantic_spec.go lives in tools/goplint/goplint/, so one level up is module root.
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func goplintPackageRootPath() string {
	return filepath.Join(goplintModuleRootPath(), "goplint")
}

func loadSemanticRuleCatalog() (semanticRuleCatalog, error) {
	path := semanticRulesCatalogPath()
	catalog := semanticRuleCatalog{}
	if err := decodeJSONFile(path, &catalog); err != nil {
		return semanticRuleCatalog{}, fmt.Errorf("loading semantic rules catalog %q: %w", path, err)
	}
	if err := validateSemanticRuleCatalog(catalog); err != nil {
		return semanticRuleCatalog{}, err
	}
	return catalog, nil
}

func loadSemanticRuleSchema() (map[string]any, error) {
	path := semanticRulesSchemaPath()
	payload := map[string]any{}
	if err := decodeJSONFile(path, &payload); err != nil {
		return nil, fmt.Errorf("loading semantic rules schema %q: %w", path, err)
	}
	return payload, nil
}

func decodeJSONFile(path string, out any) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decoding json: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return fmt.Errorf("json document has trailing data")
		}
		return fmt.Errorf("validating json eof: %w", err)
	}
	return nil
}

func validateSemanticRuleCatalog(catalog semanticRuleCatalog) error {
	if catalog.Version != semanticRulesCatalogVersion {
		return fmt.Errorf("semantic rules version must be %q (got %q)", semanticRulesCatalogVersion, catalog.Version)
	}
	if len(catalog.Rules) == 0 {
		return fmt.Errorf("semantic rules catalog must declare at least one rule")
	}

	ruleCategories := make(map[string]struct{}, len(catalog.Rules))
	for idx, rule := range catalog.Rules {
		if err := validateSemanticRuleSpec(rule); err != nil {
			return fmt.Errorf("invalid rule[%d]: %w", idx, err)
		}
		if _, exists := ruleCategories[rule.Category]; exists {
			return fmt.Errorf("duplicate rule category %q", rule.Category)
		}
		ruleCategories[rule.Category] = struct{}{}
	}

	if len(catalog.OracleMatrix) == 0 {
		return fmt.Errorf("semantic rules catalog must include oracle_matrix entries")
	}
	oracleCategories := map[string]struct{}{}
	for idx, oracle := range catalog.OracleMatrix {
		if err := validateSemanticOracleSpec(oracle); err != nil {
			return fmt.Errorf("invalid oracle_matrix[%d]: %w", idx, err)
		}
		if _, ok := ruleCategories[oracle.Category]; !ok {
			return fmt.Errorf("oracle_matrix category %q has no matching rule", oracle.Category)
		}
		if _, exists := oracleCategories[oracle.Category]; exists {
			return fmt.Errorf("duplicate oracle_matrix category %q", oracle.Category)
		}
		oracleCategories[oracle.Category] = struct{}{}
	}

	if len(catalog.HistoricalMissFixtures) == 0 {
		return fmt.Errorf("semantic rules catalog must include historical_miss_fixtures")
	}
	seenFixtures := map[string]struct{}{}
	for idx, fixture := range catalog.HistoricalMissFixtures {
		trimmed := strings.TrimSpace(fixture)
		if trimmed == "" {
			return fmt.Errorf("historical_miss_fixtures[%d] must be non-empty", idx)
		}
		if _, exists := seenFixtures[trimmed]; exists {
			return fmt.Errorf("duplicate historical_miss_fixtures entry %q", trimmed)
		}
		seenFixtures[trimmed] = struct{}{}
	}

	return nil
}

func validateSemanticRuleSpec(rule semanticRuleSpec) error {
	if strings.TrimSpace(rule.Category) == "" {
		return fmt.Errorf("category must be non-empty")
	}
	if strings.TrimSpace(rule.Family) == "" {
		return fmt.Errorf("family must be non-empty")
	}
	if strings.TrimSpace(rule.TraversalMode) == "" {
		return fmt.Errorf("traversal_mode must be non-empty")
	}
	if err := requireUniqueNonEmpty(rule.Entrypoints, "entrypoints"); err != nil {
		return err
	}
	if err := requireUniqueNonEmpty(rule.EnabledByFlags, "enabled_by_flags"); err != nil {
		return err
	}
	if err := requireUniqueNonEmpty(rule.RunControls, "run_controls"); err != nil {
		return err
	}
	if err := requireUniqueNonEmpty(rule.StateDomain, "state_domain"); err != nil {
		return err
	}
	if err := requireUniqueNonEmpty(rule.OutcomeDomain, "outcome_domain"); err != nil {
		return err
	}
	for _, outcome := range rule.OutcomeDomain {
		if !slices.Contains(semanticOutcomeDomainAllowed, outcome) {
			return fmt.Errorf("outcome_domain contains unsupported value %q", outcome)
		}
	}
	if err := validateSemanticBaselinePolicy(rule.BaselinePolicy); err != nil {
		return err
	}

	hasInconclusive := slices.Contains(rule.OutcomeDomain, "inconclusive")
	if hasInconclusive {
		if err := requireUniqueNonEmpty(rule.InconclusiveReasons, "inconclusive_reasons"); err != nil {
			return err
		}
		if err := requireUniqueNonEmpty(rule.RequiredMetaOnInconclusive, "required_meta_on_inconclusive"); err != nil {
			return err
		}
	} else if len(rule.InconclusiveReasons) > 0 || len(rule.RequiredMetaOnInconclusive) > 0 {
		return fmt.Errorf("inconclusive fields must be omitted when outcome_domain excludes inconclusive")
	}

	return nil
}

func validateSemanticOracleSpec(spec semanticOracleSpec) error {
	if strings.TrimSpace(spec.Category) == "" {
		return fmt.Errorf("category must be non-empty")
	}
	if len(spec.MustReport) == 0 {
		return fmt.Errorf("must_report must contain at least one entry")
	}
	if len(spec.MustNotReport) == 0 {
		return fmt.Errorf("must_not_report must contain at least one entry")
	}
	for _, entry := range spec.MustReport {
		if err := validateSemanticOracleEntry(entry, "must_report"); err != nil {
			return err
		}
	}
	for _, entry := range spec.MustNotReport {
		if err := validateSemanticOracleEntry(entry, "must_not_report"); err != nil {
			return err
		}
	}
	return nil
}

func validateSemanticOracleEntry(entry semanticOracleEntry, fieldName string) error {
	if strings.TrimSpace(entry.Fixture) == "" {
		return fmt.Errorf("%s fixture must be non-empty", fieldName)
	}
	if strings.TrimSpace(entry.Symbol) == "" {
		return fmt.Errorf("%s symbol must be non-empty", fieldName)
	}
	return nil
}

func requireUniqueNonEmpty(values []string, fieldName string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must contain at least one value", fieldName)
	}
	seen := map[string]struct{}{}
	for idx, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return fmt.Errorf("%s[%d] must be non-empty", fieldName, idx)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s contains duplicate value %q", fieldName, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateSemanticBaselinePolicy(policy string) error {
	switch strings.TrimSpace(policy) {
	case "suppressible", "always-visible", "audit-only":
		return nil
	default:
		return fmt.Errorf("baseline_policy %q is not supported", policy)
	}
}

func semanticBaselinePolicyForCategoryPolicy(policy BaselinePolicy) (string, error) {
	switch policy {
	case BaselineSuppressible:
		return "suppressible", nil
	case BaselineAlwaysVisible:
		return "always-visible", nil
	case BaselineAuditOnly:
		return "audit-only", nil
	default:
		return "", fmt.Errorf("unknown baseline policy %d", policy)
	}
}
