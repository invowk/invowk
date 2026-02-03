// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"invowk-cli/pkg/invkfile"
)

// TestVirtualRuntime_Integration groups integration tests for the virtual (mvdan/sh) runtime.
// These tests execute actual shell code through the embedded interpreter.
func TestVirtualRuntime_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("CommandSubstitution", testVirtualCommandSubstitution)
	t.Run("Pipelines", testVirtualPipelines)
	t.Run("HeredocInput", testVirtualHeredocInput)
	t.Run("EnvironmentExpansion", testVirtualEnvironmentExpansion)
	t.Run("ArithmeticExpansion", testVirtualArithmeticExpansion)
	t.Run("ConditionalExecution", testVirtualConditionalExecution)
}

// testVirtualCommandSubstitution tests that command substitution works correctly.
func testVirtualCommandSubstitution(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	cmd := testCommandWithScript("subst", `RESULT=$(echo "nested output"); echo "Got: $RESULT"`, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "Got: nested output" {
		t.Errorf("Execute() output = %q, want %q", output, "Got: nested output")
	}
}

// testVirtualPipelines tests that shell pipelines work correctly.
func testVirtualPipelines(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	cmd := testCommandWithScript("pipeline", `echo -e "line1\nline2\nline3" | grep line2`, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "line2" {
		t.Errorf("Execute() output = %q, want %q", output, "line2")
	}
}

// testVirtualHeredocInput tests heredoc syntax works correctly.
func testVirtualHeredocInput(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	script := `cat <<EOF
heredoc content
with multiple lines
EOF`

	cmd := testCommandWithScript("heredoc", script, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := stdout.String()
	if !strings.Contains(output, "heredoc content") {
		t.Errorf("Execute() output missing 'heredoc content', got: %q", output)
	}
	if !strings.Contains(output, "with multiple lines") {
		t.Errorf("Execute() output missing 'with multiple lines', got: %q", output)
	}
}

// testVirtualEnvironmentExpansion tests various environment variable expansion forms.
func testVirtualEnvironmentExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	script := `MYVAR="hello"
echo "Direct: $MYVAR"
echo "Braces: ${MYVAR}"
echo "Default: ${UNSET:-default_value}"
echo "Length: ${#MYVAR}"`

	cmd := testCommandWithScript("env-expansion", script, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := stdout.String()
	if !strings.Contains(output, "Direct: hello") {
		t.Errorf("Execute() output missing 'Direct: hello', got: %q", output)
	}
	if !strings.Contains(output, "Braces: hello") {
		t.Errorf("Execute() output missing 'Braces: hello', got: %q", output)
	}
	if !strings.Contains(output, "Default: default_value") {
		t.Errorf("Execute() output missing 'Default: default_value', got: %q", output)
	}
	if !strings.Contains(output, "Length: 5") {
		t.Errorf("Execute() output missing 'Length: 5', got: %q", output)
	}
}

// testVirtualArithmeticExpansion tests arithmetic expansion in shell scripts.
func testVirtualArithmeticExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	script := `echo "Sum: $((2 + 3))"
echo "Product: $((4 * 5))"`

	cmd := testCommandWithScript("arithmetic", script, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := stdout.String()
	if !strings.Contains(output, "Sum: 5") {
		t.Errorf("Execute() output missing 'Sum: 5', got: %q", output)
	}
	if !strings.Contains(output, "Product: 20") {
		t.Errorf("Execute() output missing 'Product: 20', got: %q", output)
	}
}

// testVirtualConditionalExecution tests conditional operators (&&, ||).
func testVirtualConditionalExecution(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	script := `true && echo "AND_SUCCESS"
false || echo "OR_FALLBACK"`

	cmd := testCommandWithScript("conditional", script, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := stdout.String()
	if !strings.Contains(output, "AND_SUCCESS") {
		t.Errorf("Execute() output missing 'AND_SUCCESS', got: %q", output)
	}
	if !strings.Contains(output, "OR_FALLBACK") {
		t.Errorf("Execute() output missing 'OR_FALLBACK', got: %q", output)
	}
}

// TestVirtualRuntime_ScriptFileFromSubdir tests executing a script file from a subdirectory.
func TestVirtualRuntime_ScriptFileFromSubdir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a subdirectory with a script
	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("Failed to create scripts directory: %v", err)
	}

	scriptContent := `echo "executed from subdir"`
	scriptPath := filepath.Join(scriptsDir, "helper.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to write script file: %v", err)
	}

	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	cmd := testCommandWithScript("subdir-script", "./scripts/helper.sh", invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "executed from subdir" {
		t.Errorf("Execute() output = %q, want %q", output, "executed from subdir")
	}
}
