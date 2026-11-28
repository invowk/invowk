// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"fmt"
)

const (
	// SourceTrustTierContained restricts container workspace/volume mounts to the
	// declaring module's physical root (Layer 1: vendored, remote, ~/.invowk/cmds, provisioned).
	SourceTrustTierContained SourceTrustTier = "contained"
	// SourceTrustTierLocalCwd allows container workspace/volume mounts within the
	// current working directory ceiling (Layer 2: <cwd>/invowkfile.cue and <cwd>/<m>.invowkmod).
	SourceTrustTierLocalCwd SourceTrustTier = "local-cwd"
	// SourceTrustTierInclude allows container workspace/volume mounts within the
	// module root or explicit operator-configured IncludeEntry.workspace_root (Layer 3).
	SourceTrustTierInclude SourceTrustTier = "include"
)

// ErrInvalidSourceTrustTier is returned when a SourceTrustTier value is not recognized.
var ErrInvalidSourceTrustTier = errors.New("invalid source trust tier")

type (
	// SourceTrustTier classifies the discovery trust tier used for container
	// workspace and volume mount containment.
	SourceTrustTier string

	// InvalidSourceTrustTierError is returned when a SourceTrustTier is unrecognized.
	InvalidSourceTrustTierError struct {
		Value SourceTrustTier
	}
)

// Error implements the error interface for InvalidSourceTrustTierError.
func (e *InvalidSourceTrustTierError) Error() string {
	return fmt.Sprintf("invalid source trust tier %q (valid: contained, local-cwd, include)", e.Value)
}

// Unwrap returns ErrInvalidSourceTrustTier for errors.Is() compatibility.
func (e *InvalidSourceTrustTierError) Unwrap() error { return ErrInvalidSourceTrustTier }

// String returns the string representation of SourceTrustTier.
func (t SourceTrustTier) String() string { return string(t) }

// Validate returns nil if SourceTrustTier is empty (incremental construction) or
// one of the defined 3-tier containment modes.
func (t SourceTrustTier) Validate() error {
	switch t {
	case "", SourceTrustTierContained, SourceTrustTierLocalCwd, SourceTrustTierInclude:
		return nil
	default:
		return &InvalidSourceTrustTierError{Value: t}
	}
}
