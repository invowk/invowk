// SPDX-License-Identifier: MPL-2.0

package goplint

// Flag binding variables for the analyzer's flag set. These are populated
// by the go/analysis framework during flag parsing via BoolVar/StringVar.
// The run() function never reads or mutates these directly — it reads them
// once via newRunConfig() into a local struct.
var (
	configPath           string
	baselinePath         string
	configPathExplicit   bool
	baselinePathExplicit bool

	auditExceptions             bool
	checkAll                    bool
	checkValidate               bool
	checkStringer               bool
	checkConstructors           bool
	checkConstructorSig         bool
	checkFuncOptions            bool
	checkImmutability           bool
	checkStructValidate         bool
	checkCastValidation         bool
	checkValidateUsage          bool
	checkConstructorErrUsage    bool
	checkConstructorValidates   bool
	checkValidateDelegation     bool
	checkNonZero                bool
	checkUseBeforeValidate      bool
	checkConstructorReturnError bool
	checkUseBeforeValidateCross bool
	noCFA                       bool
	auditReviewDates            bool
	checkEnumSync               bool
	suggestValidateAll          bool
)

type trackedStringFlag struct {
	value    *string
	explicit *bool
}

func (f *trackedStringFlag) Set(s string) error {
	if f.value != nil {
		*f.value = s
	}
	if f.explicit != nil {
		*f.explicit = true
	}
	return nil
}

func (f *trackedStringFlag) String() string {
	if f.value == nil {
		return ""
	}
	return *f.value
}

// modeFlagSpec is the declarative table for all bool analyzer modes.
// It binds flag registration, runConfig wiring, and --check-all expansion.
type modeFlagSpec struct {
	flagName           string
	usage              string
	defaultValue       bool
	includeInCheckAll  bool
	binding            *bool
	runConfigBoolField func(*runConfig) *bool
}

func (s modeFlagSpec) runConfigValue(rc *runConfig) bool {
	return *s.runConfigBoolField(rc)
}

var modeFlagSpecs = []modeFlagSpec{
	{
		flagName:          "audit-exceptions",
		usage:             "report exception patterns that matched zero locations (stale entries)",
		defaultValue:      false,
		includeInCheckAll: false,
		binding:           &auditExceptions,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.auditExceptions
		},
	},
	{
		flagName:          "check-validate",
		usage:             "report named non-struct types missing Validate() error method",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkValidate,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkValidate
		},
	},
	{
		flagName:          "check-stringer",
		usage:             "report named non-struct types missing String() string method",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkStringer,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkStringer
		},
	},
	{
		flagName:          "check-constructors",
		usage:             "report exported struct types missing NewXxx() constructor function",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkConstructors,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkConstructors
		},
	},
	{
		flagName:          "check-constructor-sig",
		usage:             "report NewXxx() constructors whose return type doesn't match the struct",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkConstructorSig,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkConstructorSig
		},
	},
	{
		flagName:          "check-func-options",
		usage:             "report structs that should use or complete the functional options pattern",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkFuncOptions,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkFuncOptions
		},
	},
	{
		flagName:          "check-immutability",
		usage:             "report structs with constructors that have exported mutable fields",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkImmutability,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkImmutability
		},
	},
	{
		flagName:          "check-struct-validate",
		usage:             "report exported struct types with constructors missing Validate() error method",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkStructValidate,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkStructValidate
		},
	},
	{
		flagName:          "check-cast-validation",
		usage:             "report type conversions to DDD Value Types from non-constants without Validate() check",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkCastValidation,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkCastValidation
		},
	},
	{
		flagName:          "check-validate-usage",
		usage:             "detect unused Validate() results",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkValidateUsage,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkValidateUsage
		},
	},
	{
		flagName:          "check-constructor-error-usage",
		usage:             "detect constructor calls with error return assigned to blank identifier",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkConstructorErrUsage,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkConstructorErrUsage
		},
	},
	{
		flagName:          "check-constructor-validates",
		usage:             "report NewXxx() constructors that return types with Validate() but never call it",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkConstructorValidates,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkConstructorValidates
		},
	},
	{
		flagName:          "check-validate-delegation",
		usage:             "report structs with //goplint:validate-all whose Validate() misses field delegations",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkValidateDelegation,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkValidateDelegation
		},
	},
	{
		flagName:          "check-nonzero",
		usage:             "report struct fields using nonzero-annotated types as value (non-pointer) fields where they are semantically optional",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkNonZero,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkNonZero
		},
	},
	{
		flagName:          "audit-review-dates",
		usage:             "report exception patterns with review_after dates that have passed",
		defaultValue:      false,
		includeInCheckAll: false,
		binding:           &auditReviewDates,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.auditReviewDates
		},
	},
	{
		flagName:          "check-use-before-validate",
		usage:             "report DDD Value Type variables used before Validate() in the same basic block (CFA only)",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkUseBeforeValidate,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkUseBeforeValidate
		},
	},
	{
		flagName:          "check-constructor-return-error",
		usage:             "report NewXxx() constructors for validatable types that do not return error",
		defaultValue:      false,
		includeInCheckAll: true,
		binding:           &checkConstructorReturnError,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkConstructorReturnError
		},
	},
	{
		flagName:          "check-use-before-validate-cross",
		usage:             "report DDD Value Type variables used before Validate() across CFG blocks (CFA only, opt-in)",
		defaultValue:      false,
		includeInCheckAll: false,
		binding:           &checkUseBeforeValidateCross,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkUseBeforeValidateCross
		},
	},
	{
		flagName:          "no-cfa",
		usage:             "disable control-flow analysis and use AST heuristic for cast-validation (CFA is enabled by default)",
		defaultValue:      false,
		includeInCheckAll: false,
		binding:           &noCFA,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.noCFA
		},
	},
	{
		flagName:          "check-enum-sync",
		usage:             "report mismatches between Go Validate() switch cases and CUE schema disjunction members (requires //goplint:enum-cue= directive)",
		defaultValue:      false,
		includeInCheckAll: false,
		binding:           &checkEnumSync,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkEnumSync
		},
	},
	{
		flagName:          "suggest-validate-all",
		usage:             "report structs with Validate() + validatable fields but no //goplint:validate-all directive (advisory)",
		defaultValue:      false,
		includeInCheckAll: false,
		binding:           &suggestValidateAll,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.suggestValidateAll
		},
	},
	{
		flagName:          "check-all",
		usage:             "enable all DDD compliance checks (validate + stringer + constructors + structural + cast-validation + validate-usage + constructor-error-usage + constructor-validates + nonzero + CFA)",
		defaultValue:      false,
		includeInCheckAll: false,
		binding:           &checkAll,
		runConfigBoolField: func(rc *runConfig) *bool {
			return &rc.checkAll
		},
	},
}

func init() {
	Analyzer.Flags.Var(&trackedStringFlag{value: &configPath, explicit: &configPathExplicit}, "config",
		"path to exceptions TOML config file")
	Analyzer.Flags.Var(&trackedStringFlag{value: &baselinePath, explicit: &baselinePathExplicit}, "baseline",
		"path to baseline TOML file (suppress known findings, report only new ones)")

	for _, spec := range modeFlagSpecs {
		Analyzer.Flags.BoolVar(spec.binding, spec.flagName, spec.defaultValue, spec.usage)
	}
}

// runConfig holds the resolved flag values for a single run() invocation.
// Reading flag bindings into this struct at run() entry ensures run()
// never reads or mutates package-level state directly.
type runConfig struct {
	configPath                  string
	baselinePath                string
	auditExceptions             bool
	checkAll                    bool
	checkValidate               bool
	checkStringer               bool
	checkConstructors           bool
	checkConstructorSig         bool
	checkFuncOptions            bool
	checkImmutability           bool
	checkStructValidate         bool
	checkCastValidation         bool
	checkValidateUsage          bool
	checkConstructorErrUsage    bool
	checkConstructorValidates   bool
	checkValidateDelegation     bool
	checkNonZero                bool
	checkUseBeforeValidate      bool
	checkConstructorReturnError bool
	checkUseBeforeValidateCross bool
	noCFA                       bool
	auditReviewDates            bool
	checkEnumSync               bool
	suggestValidateAll          bool
}

// newRunConfig reads the current flag binding values into a local config
// struct and applies the --check-all expansion. The expansion happens on
// the local struct, never mutating the package-level flag variables.
func newRunConfig() runConfig {
	rc := runConfig{
		configPath:   configPath,
		baselinePath: baselinePath,
	}
	for _, spec := range modeFlagSpecs {
		*spec.runConfigBoolField(&rc) = *spec.binding
	}
	expandCheckAllModes(&rc)
	return rc
}

func expandCheckAllModes(rc *runConfig) {
	// Expand --check-all into individual supplementary checks.
	// Deliberately excludes --audit-exceptions (config maintenance tool
	// with per-package false positives). CFA is enabled by default
	// (opt out via --no-cfa).
	if !rc.checkAll {
		return
	}
	for _, spec := range modeFlagSpecs {
		if !spec.includeInCheckAll {
			continue
		}
		*spec.runConfigBoolField(rc) = true
	}
}
