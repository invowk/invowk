// SPDX-License-Identifier: MPL-2.0

package goplint

import "testing"

func TestSemanticSpecSchemaAndCatalogParse(t *testing.T) {
	t.Parallel()

	schema, err := loadSemanticRuleSchema()
	if err != nil {
		t.Fatalf("loadSemanticRuleSchema() error: %v", err)
	}
	if got := schema["$schema"]; got == nil {
		t.Fatal("semantic schema is missing $schema")
	}
	if got := schema["properties"]; got == nil {
		t.Fatal("semantic schema is missing properties")
	}

	_ = mustLoadSemanticRuleCatalog(t)
}

func TestSemanticSpecRegistrySync(t *testing.T) {
	t.Parallel()

	catalog := mustLoadSemanticRuleCatalog(t)
	rulesByCategory := make(map[string]semanticRuleSpec, len(catalog.Rules))
	for _, rule := range catalog.Rules {
		rulesByCategory[rule.Category] = rule
	}

	expectedCFACategories := semanticSpecExpectedCFACategories()
	for _, category := range expectedCFACategories {
		rule, ok := rulesByCategory[category]
		if !ok {
			t.Fatalf("semantic catalog is missing required CFA category %q", category)
		}
		if !IsKnownDiagnosticCategory(category) {
			t.Fatalf("semantic catalog category %q is not known in diagnostic registry", category)
		}
		spec, ok := diagnosticCategorySpec(category)
		if !ok {
			t.Fatalf("diagnosticCategorySpec(%q) not found", category)
		}
		wantPolicy, err := semanticBaselinePolicyForCategoryPolicy(spec.BaselinePolicy)
		if err != nil {
			t.Fatalf("semanticBaselinePolicyForCategoryPolicy(%q) error: %v", category, err)
		}
		if rule.BaselinePolicy != wantPolicy {
			t.Fatalf("rule %q baseline policy = %q, want %q", category, rule.BaselinePolicy, wantPolicy)
		}
	}
}

func TestSemanticSpecOracleCoverageSync(t *testing.T) {
	t.Parallel()

	catalog := mustLoadSemanticRuleCatalog(t)
	oracleByCategory := make(map[string]struct{}, len(catalog.OracleMatrix))
	for _, oracle := range catalog.OracleMatrix {
		oracleByCategory[oracle.Category] = struct{}{}
	}

	for _, category := range semanticSpecExpectedCFACategories() {
		if _, ok := oracleByCategory[category]; !ok {
			t.Fatalf("oracle_matrix is missing required CFA category %q", category)
		}
	}
}

func semanticSpecExpectedCFACategories() []string {
	return []string{
		CategoryUnvalidatedCast,
		CategoryUnvalidatedCastInconclusive,
		CategoryUseBeforeValidateSameBlock,
		CategoryUseBeforeValidateCrossBlock,
		CategoryUseBeforeValidateInconclusive,
		CategoryMissingConstructorValidate,
		CategoryMissingConstructorValidateInc,
	}
}
