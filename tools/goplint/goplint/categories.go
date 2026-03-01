// SPDX-License-Identifier: MPL-2.0

package goplint

// BaselinePolicy describes how a diagnostic category is treated by
// baseline generation and suppression.
type BaselinePolicy int

const (
	// BaselineSuppressible categories are included in baseline TOML and can
	// be suppressed by baseline matching.
	BaselineSuppressible BaselinePolicy = iota
	// BaselineAlwaysVisible categories are never baselined and should always
	// be reported when encountered.
	BaselineAlwaysVisible
	// BaselineAuditOnly categories are operational/config audit findings and
	// are excluded from baseline generation.
	BaselineAuditOnly
)

// CategorySpec is the canonical metadata for one diagnostic category.
type CategorySpec struct {
	Name           string
	BaselinePolicy BaselinePolicy
	// BaselineLabel is the human-readable section label used when writing
	// baseline TOML. Only meaningful for BaselineSuppressible categories.
	BaselineLabel string
}

// diagnosticCategoryRegistry is the canonical, single-source category list.
// Keep this list in sync with category constants and diagnostic emitters.
var diagnosticCategoryRegistry = []CategorySpec{
	{Name: CategoryPrimitive, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Bare primitive type usage"},
	{Name: CategoryMissingValidate, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Named types missing Validate() method"},
	{Name: CategoryMissingStringer, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Named types missing String() method"},
	{Name: CategoryMissingConstructor, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Exported structs missing NewXxx() constructor"},
	{Name: CategoryWrongConstructorSig, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Constructors with wrong return type"},
	{Name: CategoryMissingFuncOptions, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Structs missing functional options pattern"},
	{Name: CategoryMissingImmutability, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Structs with constructor but exported mutable fields"},
	{Name: CategoryWrongValidateSig, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Named types with wrong Validate() signature"},
	{Name: CategoryWrongStringerSig, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Named types with wrong String() signature"},
	{Name: CategoryMissingStructValidate, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Structs with constructor but no Validate() method"},
	{Name: CategoryWrongStructValidateSig, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Structs with Validate() but wrong signature"},
	{Name: CategoryUnvalidatedCast, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Type conversions to DDD types without Validate() check"},
	{Name: CategoryUnusedValidateResult, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Validate() calls with result completely discarded"},
	{Name: CategoryUnusedConstructorError, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Constructor calls with error return assigned to blank identifier"},
	{Name: CategoryMissingConstructorValidate, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Constructors returning validatable types without calling Validate()"},
	{Name: CategoryIncompleteValidateDelegation, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Structs with validate-all missing field Validate() delegation"},
	{Name: CategoryNonZeroValueField, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Struct fields using nonzero types as value (non-pointer)"},
	{Name: CategoryWrongFuncOptionType, BaselinePolicy: BaselineSuppressible, BaselineLabel: "WithXxx option functions with parameter type mismatches"},
	{Name: CategoryEnumCueMissingGo, BaselinePolicy: BaselineSuppressible, BaselineLabel: "CUE disjunction members missing from Go Validate() switch"},
	{Name: CategoryEnumCueExtraGo, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Go Validate() switch cases not present in CUE disjunction"},
	{Name: CategoryUseBeforeValidate, BaselinePolicy: BaselineSuppressible, BaselineLabel: "DDD Value Type values used before Validate()"},
	{Name: CategorySuggestValidateAll, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Structs suggesting //goplint:validate-all adoption"},
	{Name: CategoryMissingConstructorErrorReturn, BaselinePolicy: BaselineSuppressible, BaselineLabel: "Constructors returning validatable types without an error return"},
	{Name: CategoryUnknownDirective, BaselinePolicy: BaselineAlwaysVisible},
	{Name: CategoryStaleException, BaselinePolicy: BaselineAuditOnly},
	{Name: CategoryOverdueReview, BaselinePolicy: BaselineAuditOnly},
}

var diagnosticCategoryByName = buildDiagnosticCategoryByName()

func buildDiagnosticCategoryByName() map[string]CategorySpec {
	out := make(map[string]CategorySpec, len(diagnosticCategoryRegistry))
	for _, spec := range diagnosticCategoryRegistry {
		out[spec.Name] = spec
	}
	return out
}

// IsKnownDiagnosticCategory reports whether category exists in the canonical
// category registry.
func IsKnownDiagnosticCategory(category string) bool {
	_, ok := diagnosticCategoryByName[category]
	return ok
}

// IsBaselineSuppressibleCategory reports whether category is included in
// baseline generation/suppression.
func IsBaselineSuppressibleCategory(category string) bool {
	spec, ok := diagnosticCategoryByName[category]
	return ok && spec.BaselinePolicy == BaselineSuppressible
}

// BaselinedCategoryNames returns all categories that participate in baseline
// generation/suppression.
func BaselinedCategoryNames() []string {
	specs := suppressibleCategorySpecs()
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.Name)
	}
	return out
}

func suppressibleCategorySpecs() []CategorySpec {
	out := make([]CategorySpec, 0, len(diagnosticCategoryRegistry))
	for _, spec := range diagnosticCategoryRegistry {
		if spec.BaselinePolicy == BaselineSuppressible {
			out = append(out, spec)
		}
	}
	return out
}
