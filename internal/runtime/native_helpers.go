// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
)

// Type definitions (grouped for decorder compliance)
type (
	// executeOutput configures where command output is directed during execution.
	// It abstracts the difference between streaming (to ctx.Stdout/Stderr) and
	// capturing (to bytes.Buffer) execution modes.
	executeOutput struct {
		stdout io.Writer
		stderr io.Writer
		// capture indicates whether output is being captured to buffers
		capture bool
	}

	// capturedOutput holds the captured stdout and stderr buffers when capture mode is used.
	// This type is used only when executeOutput.capture is true.
	capturedOutput struct {
		stdout bytes.Buffer
		stderr bytes.Buffer
	}
)

// newStreamingOutput creates an output configuration that streams to the provided writers.
// This is used for Execute() where output goes directly to ctx.Stdout/Stderr.
func newStreamingOutput(stdout, stderr io.Writer) *executeOutput {
	return &executeOutput{
		stdout:  stdout,
		stderr:  stderr,
		capture: false,
	}
}

// newCapturingOutput creates an output configuration that captures to internal buffers.
// This is used for ExecuteCapture() where output needs to be returned as strings.
// Returns the output configuration and the buffer holder to retrieve results from.
func newCapturingOutput() (*executeOutput, *capturedOutput) {
	captured := &capturedOutput{}
	return &executeOutput{
		stdout:  &captured.stdout,
		stderr:  &captured.stderr,
		capture: true,
	}, captured
}

// extractExitCode determines the exit code from a command execution error.
// Returns a Result with exit code, output strings (if captured), and any error.
func extractExitCode(err error, captured *capturedOutput) *Result {
	result := &Result{}

	// Extract captured output if available
	if captured != nil {
		result.Output = captured.stdout.String()
		result.ErrOutput = captured.stderr.String()
	}

	if err == nil {
		result.ExitCode = 0
		return result
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		// Command executed but returned non-zero exit code
		exitCode := ExitCode(exitErr.ExitCode())
		if validateErr := exitCode.Validate(); validateErr != nil {
			result.ExitCode = 1
			result.Error = validateErr
			return result
		}
		result.ExitCode = exitCode
		return result
	}

	// Some other error (e.g., command not found, permission denied)
	result.ExitCode = 1
	result.Error = err
	return result
}
