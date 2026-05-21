// SPDX-License-Identifier: MPL-2.0

package llm

import (
	"errors"
	"fmt"
)

const (
	llmRequestFailedErrMsg           = "LLM request failed"
	llmMalformedResponseErrMsg       = "LLM returned malformed response"
	llmEmptyResponseErrMsg           = "LLM returned empty response"
	llmResponseContentFilteredErrMsg = "LLM response was filtered"

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
	// MalformedResponseError is returned when the LLM response cannot be parsed.
	MalformedResponseError struct {
		RawResponse string
		Err         error
	}
)

// Error implements the error interface.
func (e *MalformedResponseError) Error() string {
	if e.Err == nil {
		return llmMalformedResponseErrMsg
	}
	return fmt.Sprintf("%s: %v", llmMalformedResponseErrMsg, e.Err)
}

// RawResponsePreview returns a bounded raw provider response for explicit debug use.
//
//goplint:ignore -- explicit debug API returns bounded provider text for callers to opt into.
func (e *MalformedResponseError) RawResponsePreview() string {
	raw := e.RawResponse
	if len(raw) > maxErrorResponseLen {
		raw = raw[:maxErrorResponseLen] + "..."
	}
	return raw
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *MalformedResponseError) Unwrap() error { return ErrLLMMalformedResponse }
