// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/issue"
)

func TestNewServiceError_PanicsOnNilErr(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on nil Err, got none")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if msg != "ServiceError: Err must not be nil" {
			t.Fatalf("unexpected panic message: %s", msg)
		}
	}()

	newServiceError(nil, 0, "")
}

func TestNewServiceError_ValidConstruction(t *testing.T) {
	t.Parallel()

	err := errors.New("test error")
	svcErr := newServiceError(err, issue.CommandNotFoundId, "styled message")

	if !errors.Is(svcErr.Err, err) {
		t.Errorf("Err = %v, want %v", svcErr.Err, err)
	}
	if svcErr.IssueID != issue.CommandNotFoundId {
		t.Errorf("IssueID = %d, want %d", svcErr.IssueID, issue.CommandNotFoundId)
	}
	if svcErr.StyledMessage != "styled message" {
		t.Errorf("StyledMessage = %q, want %q", svcErr.StyledMessage, "styled message")
	}
}

func TestServiceError_ErrorAndUnwrap(t *testing.T) {
	t.Parallel()

	underlying := errors.New("underlying error")
	svcErr := newServiceError(underlying, 0, "")

	if svcErr.Error() != "underlying error" {
		t.Errorf("Error() = %q, want %q", svcErr.Error(), "underlying error")
	}
	if !errors.Is(svcErr, underlying) {
		t.Error("errors.Is should find underlying error via Unwrap")
	}
}

func TestRenderServiceError_NilServiceError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderServiceError(&buf, nil)

	if buf.Len() != 0 {
		t.Errorf("expected no output for nil ServiceError, got %q", buf.String())
	}
}

func TestRenderServiceError_StyledMessageOnly(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	svcErr := newServiceError(errors.New("test"), 0, "styled output\n")
	renderServiceError(&buf, svcErr)

	if buf.String() != "styled output\n" {
		t.Errorf("output = %q, want %q", buf.String(), "styled output\n")
	}
}

func TestRenderServiceError_WithIssueID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	svcErr := newServiceError(errors.New("test"), issue.CommandNotFoundId, "")
	renderServiceError(&buf, svcErr)

	// Issue catalog entry should be rendered (contains the issue template content)
	output := buf.String()
	if output == "" {
		t.Error("expected non-empty output when IssueID is set")
	}
}

func TestRenderServiceError_StyledMessageAndIssueID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	svcErr := newServiceError(errors.New("test"), issue.CommandNotFoundId, "styled: ")
	renderServiceError(&buf, svcErr)

	output := buf.String()
	// Should contain both the styled message prefix and the issue catalog content
	if len(output) <= len("styled: ") {
		t.Errorf("expected styled message + issue content, got only %q", output)
	}
}

func TestRenderServiceError_ZeroIssueIDSkipsCatalog(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	svcErr := newServiceError(errors.New("test"), 0, "only this")
	renderServiceError(&buf, svcErr)

	if buf.String() != "only this" {
		t.Errorf("output = %q, want %q", buf.String(), "only this")
	}
}
