// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

// =============================================================================
// Behavioral Sync Tests — Command Domain
// =============================================================================

// TestBehavioralSync_CapabilityName verifies Go CapabilityName.Validate() agrees with
// CUE #CapabilityName disjunction.
func TestBehavioralSync_CapabilityName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#CapabilityName",
		func(s string) error { return CapabilityName(s).Validate() },
		[]behavioralSyncCase{
			{"local-area-network", true, true, ""},
			{"internet", true, true, ""},
			{"containers", true, true, ""},
			{"tty", true, true, ""},
			{"gpu", false, false, ""},
			{"", false, false, ""},
			{"TTY", false, false, ""},
		},
	)
}

// TestBehavioralSync_FlagType verifies Go FlagType.Validate() agrees with
// CUE #Flag.type disjunction ("string" | "bool" | "int" | "float").
// Note: FlagType("") is valid in Go (defaults to "string") but CUE field type?
// is optional — absent means default. The zero-value divergence is expected.
func TestBehavioralSync_FlagType(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	// type is an optional field in #Flag — use field-level lookup
	runBehavioralSyncField(t, schema, ctx, "#Flag", "type",
		func(s string) error { return FlagType(s).Validate() },
		[]behavioralSyncCase{
			{"string", true, true, ""},
			{"bool", true, true, ""},
			{"int", true, true, ""},
			{"float", true, true, ""},
			{"array", false, false, ""},
			{"STRING", false, false, ""},
			// Go accepts "" (defaults to "string"), CUE rejects "" because it doesn't match the disjunction.
			// This is expected: CUE handles optionality at the field level (type? is omitted), not value level.
			{"", true, false, "Go zero-value defaults to string; CUE uses field optionality"},
		},
	)
}

// TestBehavioralSync_ArgumentType verifies Go ArgumentType.Validate() agrees with
// CUE #Argument.type disjunction ("string" | "int" | "float").
func TestBehavioralSync_ArgumentType(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	// type is an optional field in #Argument — use field-level lookup
	runBehavioralSyncField(t, schema, ctx, "#Argument", "type",
		func(s string) error { return ArgumentType(s).Validate() },
		[]behavioralSyncCase{
			{"string", true, true, ""},
			{"int", true, true, ""},
			{"float", true, true, ""},
			{"bool", false, false, ""},
			{"", true, false, "Go zero-value defaults to string; CUE uses field optionality"},
		},
	)
}

// TestBehavioralSync_FlagName verifies Go FlagName.Validate() agrees with
// CUE #Flag.name constraint (regex + length + non-empty).
func TestBehavioralSync_FlagName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#Flag.name",
		func(s string) error { return FlagName(s).Validate() },
		[]behavioralSyncCase{
			{"verbose", true, true, ""},
			{"output-file", true, true, ""},
			{"num_retries", true, true, ""},
			{"a", true, true, ""},
			{"A", true, true, ""},
			{"a1", true, true, ""},
			{"", false, false, ""},
			{"   ", false, false, ""},
			{"123bad", false, false, ""},
			{"-starts-hyphen", false, false, ""},
			{"_starts_underscore", false, false, ""},
			{"a" + strings.Repeat("b", 255), true, true, ""},   // exactly 256 runes
			{"a" + strings.Repeat("b", 256), false, false, ""}, // 257 runes
		},
	)
}

// TestBehavioralSync_ArgumentName verifies Go ArgumentName.Validate() agrees with
// CUE #Argument.name constraint (regex + length + non-empty).
func TestBehavioralSync_ArgumentName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#Argument.name",
		func(s string) error { return ArgumentName(s).Validate() },
		[]behavioralSyncCase{
			{"file", true, true, ""},
			{"output-dir", true, true, ""},
			{"source_path", true, true, ""},
			{"a", true, true, ""},
			{"", false, false, ""},
			{"123bad", false, false, ""},
			{"-flag", false, false, ""},
			{"a" + strings.Repeat("b", 255), true, true, ""},
			{"a" + strings.Repeat("b", 256), false, false, ""},
		},
	)
}

// TestBehavioralSync_CommandName verifies Go CommandName.Validate() agrees with
// CUE #Command.name constraint (regex + length + non-empty).
// Go now enforces the same regex (^[a-zA-Z][a-zA-Z0-9_ -]*$) and MaxRunes(256)
// as CUE, so all cases are in agreement.
func TestBehavioralSync_CommandName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#Command.name",
		func(s string) error { return CommandName(s).Validate() },
		[]behavioralSyncCase{
			{"build", true, true, ""},
			{"test unit", true, true, ""},
			{"deploy-prod", true, true, ""},
			{"a", true, true, ""},
			{"", false, false, ""},
			{"   ", false, false, ""},
			// Both Go and CUE reject names not starting with a letter
			{"123bad", false, false, ""},
			{"-starts-hyphen", false, false, ""},
			{"a" + strings.Repeat("b", 255), true, true, ""},
			// Both Go and CUE reject names exceeding 256 runes
			{"a" + strings.Repeat("b", 256), false, false, ""},
		},
	)
}

// TestBehavioralSync_FlagShorthand verifies Go FlagShorthand.Validate() agrees with
// CUE #Flag.short constraint (=~"^[a-zA-Z]$").
func TestBehavioralSync_FlagShorthand(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncField(t, schema, ctx, "#Flag", "short",
		func(s string) error { return FlagShorthand(s).Validate() },
		[]behavioralSyncCase{
			{"v", true, true, ""},
			{"o", true, true, ""},
			{"Z", true, true, ""},
			{"ab", false, false, ""},
			{"1", false, false, ""},
			{"-", false, false, ""},
			// Go accepts "" (no shorthand), CUE rejects "" because short? is optional field
			{"", true, false, "Go zero-value means no shorthand; CUE uses field optionality"},
		},
	)
}

// TestBehavioralSync_WorkDir verifies Go WorkDir.Validate() agrees with
// CUE #Command.workdir constraint (#NonWhitespaceString & strings.MaxRunes(4096), optional field).
func TestBehavioralSync_WorkDir(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncField(t, schema, ctx, "#Command", "workdir",
		func(s string) error { return WorkDir(s).Validate() },
		[]behavioralSyncCase{
			{"./build", true, true, ""},
			{"/absolute/path", true, true, ""},
			{"relative", true, true, ""},
			// Go accepts "" (inherit parent workdir), CUE rejects "" when the optional field is present.
			{"", true, false, "Go zero-value means inherit parent workdir; CUE uses field optionality"},
			{"   ", false, false, ""},
		},
	)
}

// TestBehavioralSync_CommandCategory verifies Go CommandCategory.Validate() agrees with
// CUE #Command.category constraint (=~"^\\s*\\S.*$" & strings.MaxRunes(256), optional field).
func TestBehavioralSync_CommandCategory(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncField(t, schema, ctx, "#Command", "category",
		func(s string) error { return CommandCategory(s).Validate() },
		[]behavioralSyncCase{
			{"build", true, true, ""},
			{"test & verify", true, true, ""},
			{"  padded  ", true, true, ""},
			// Go accepts "" (no category), CUE rejects "" as optional field
			{"", true, false, "Go zero-value means no category; CUE uses field optionality"},
			{"   ", false, false, ""},
		},
	)
}
