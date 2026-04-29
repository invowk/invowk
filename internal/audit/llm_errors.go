// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"errors"
	"fmt"
)

const (
	llmRequestFailedErrMsg           = "LLM request failed"
	llmMalformedResponseErrMsg       = "LLM returned malformed response"
	llmEmptyResponseErrMsg           = "LLM returned empty response"
	llmResponseContentFilteredErrMsg = "LLM response was filtered"

	// DefaultLLMConcurrency limits parallel LLM requests.
	DefaultLLMConcurrency = 2

	maxErrorResponseLen = 200
)

var (
	// ErrLLMRequestFailed is the sentinel for general LLM request failures.
	ErrLLMRequestFailed = errors.New(llmRequestFailedErrMsg)
	// ErrLLMMalformedResponse is the sentinel for unparseable LLM responses.
	ErrLLMMalformedResponse = errors.New(llmMalformedResponseErrMsg)
	// ErrLLMEmptyResponse is the sentinel for empty LLM responses.
	ErrLLMEmptyResponse = errors.New(llmEmptyResponseErrMsg)
	// ErrLLMResponseContentFiltered is the sentinel for content-filtered responses.
	ErrLLMResponseContentFiltered = errors.New(llmResponseContentFilteredErrMsg)
)

type (
	// LLMMalformedResponseError is returned when the LLM response cannot be parsed.
	LLMMalformedResponseError struct {
		RawResponse string
		Err         error
	}
)

// Error implements the error interface.
func (e *LLMMalformedResponseError) Error() string {
	raw := e.RawResponse
	if len(raw) > maxErrorResponseLen {
		raw = raw[:maxErrorResponseLen] + "..."
	}
	return fmt.Sprintf("%s: %v (response: %q)", llmMalformedResponseErrMsg, e.Err, raw)
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMMalformedResponseError) Unwrap() error { return ErrLLMMalformedResponse }
