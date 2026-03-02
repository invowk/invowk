// SPDX-License-Identifier: MPL-2.0

// Package watch provides file-watching with debounced re-execution.
//
// It monitors filesystem paths matching glob patterns and invokes a callback
// after a configurable debounce period. Events within the debounce window are
// coalesced so the callback fires once with the full set of changed paths.
package watch
