// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
)

var (
	// ErrInvalidBindPlan is returned when a BindPlan cannot be served safely.
	ErrInvalidBindPlan = errors.New("invalid TUI server bind plan")

	// loopbackAddr is the address every restricted bind plan starts with.
	loopbackAddr = netip.AddrFrom4([4]byte{127, 0, 0, 1})
)

type (
	// BindPlan selects the addresses the TUI server listens on.
	//
	// A restricted plan lists IPv4 addresses, with 127.0.0.1 always first, and
	// the server listens on every one of them at the same port. A wildcard plan
	// is a single 0.0.0.0 listener; it exists only for the explicit
	// container.tui_server_bind "all-interfaces" opt-in and cannot be combined
	// with other addresses, because 0.0.0.0 and 127.0.0.1 cannot share a port.
	// The zero value is the loopback-only plan.
	BindPlan struct {
		addrs    []netip.Addr
		wildcard bool
	}

	// InvalidBindPlanError describes why a bind plan was rejected.
	InvalidBindPlanError struct {
		Reason string
	}
)

// LoopbackBindPlan returns the plan that listens on 127.0.0.1 only.
func LoopbackBindPlan() BindPlan {
	return BindPlan{}
}

// WildcardBindPlan returns the plan that listens on 0.0.0.0 only.
// Callers must reach it only through an explicit user opt-in.
func WildcardBindPlan() BindPlan {
	return BindPlan{wildcard: true}
}

// NewBindPlan returns a restricted plan that listens on 127.0.0.1 and on
// each given address at one shared port. 127.0.0.1 is added first when absent,
// and duplicates are dropped. Unspecified, IPv6 and invalid addresses are
// rejected.
func NewBindPlan(extra ...netip.Addr) (BindPlan, error) {
	addrs := []netip.Addr{loopbackAddr}
	for _, addr := range extra {
		if !slices.Contains(addrs, addr) {
			addrs = append(addrs, addr)
		}
	}
	plan := BindPlan{addrs: addrs}
	if err := plan.Validate(); err != nil {
		return BindPlan{}, err
	}
	return plan, nil
}

// Error implements error.
func (e *InvalidBindPlanError) Error() string {
	return fmt.Sprintf("%s: %s", ErrInvalidBindPlan, e.Reason)
}

// Unwrap returns ErrInvalidBindPlan.
func (e *InvalidBindPlanError) Unwrap() error { return ErrInvalidBindPlan }

// Validate reports whether the plan can be served without an unintended
// wildcard listener.
func (p BindPlan) Validate() error {
	if p.wildcard {
		if len(p.addrs) != 0 {
			return &InvalidBindPlanError{Reason: "a wildcard plan cannot list addresses"}
		}
		return nil
	}
	for i, addr := range p.addrs {
		switch {
		case !addr.IsValid():
			return &InvalidBindPlanError{Reason: "invalid address"}
		case addr.IsUnspecified():
			return &InvalidBindPlanError{Reason: "unspecified address " + addr.String() + " requires the wildcard plan"}
		case !addr.Is4():
			return &InvalidBindPlanError{Reason: "address " + addr.String() + " is not IPv4"}
		case i == 0 && addr != loopbackAddr:
			return &InvalidBindPlanError{Reason: "the first address must be 127.0.0.1"}
		}
	}
	return nil
}

// IsWildcard reports whether the plan is the 0.0.0.0 opt-in plan.
func (p BindPlan) IsWildcard() bool {
	return p.wildcard
}

// Addresses returns the addresses a restricted plan listens on, loopback
// first. A wildcard plan returns 0.0.0.0.
func (p BindPlan) Addresses() []netip.Addr {
	if p.wildcard {
		return []netip.Addr{netip.IPv4Unspecified()}
	}
	if len(p.addrs) == 0 {
		return []netip.Addr{loopbackAddr}
	}
	return slices.Clone(p.addrs)
}
