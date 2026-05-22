// SPDX-License-Identifier: MPL-2.0

package acpclient

import (
	"context"
	"fmt"
)

//goplint:ignore -- ACP callback policies are adapter ports; zero-value policy grouping is valid.
type (
	// PermissionPolicy mediates ACP permission prompts.
	PermissionPolicy interface {
		RequestPermission(context.Context, PermissionRequest) (PermissionResponse, error)
	}

	// FilesystemPolicy mediates ACP text-file callbacks.
	FilesystemPolicy interface {
		ReadTextFile(context.Context, ReadTextFileRequest) (ReadTextFileResponse, error)
		WriteTextFile(context.Context, WriteTextFileRequest) (WriteTextFileResponse, error)
	}

	// TerminalPolicy mediates ACP terminal callbacks.
	TerminalPolicy interface {
		CreateTerminal(context.Context, CreateTerminalRequest) (CreateTerminalResponse, error)
		TerminalOutput(context.Context, TerminalOutputRequest) (TerminalOutputResponse, error)
		ReleaseTerminal(context.Context, ReleaseTerminalRequest) (ReleaseTerminalResponse, error)
		WaitForTerminalExit(context.Context, WaitForTerminalExitRequest) (WaitForTerminalExitResponse, error)
		KillTerminal(context.Context, KillTerminalRequest) (KillTerminalResponse, error)
	}

	// Policies groups the ACP callback policies used by one prompt run.
	Policies struct {
		Permission PermissionPolicy
		Filesystem FilesystemPolicy
		Terminal   TerminalPolicy
	}

	// RestrictivePolicy denies filesystem callbacks, cancels permission prompts,
	// and reports terminal callbacks as unsupported.
	RestrictivePolicy struct{}
)

// Normalize fills unset policies with restrictive behavior.
func (p Policies) Normalize() Policies {
	restrictive := RestrictivePolicy{}
	if p.Permission == nil {
		p.Permission = restrictive
	}
	if p.Filesystem == nil {
		p.Filesystem = restrictive
	}
	return p
}

// RequestPermission cancels permission prompts by default.
func (RestrictivePolicy) RequestPermission(context.Context, PermissionRequest) (PermissionResponse, error) {
	return PermissionResponse{Outcome: CancelledPermissionOutcome()}, nil
}

// ReadTextFile denies file reads by default.
func (RestrictivePolicy) ReadTextFile(context.Context, ReadTextFileRequest) (ReadTextFileResponse, error) {
	return ReadTextFileResponse{}, fmt.Errorf("%w: read text file", ErrPolicyDenied)
}

// WriteTextFile denies file writes by default.
func (RestrictivePolicy) WriteTextFile(context.Context, WriteTextFileRequest) (WriteTextFileResponse, error) {
	return WriteTextFileResponse{}, fmt.Errorf("%w: write text file", ErrPolicyDenied)
}

// CreateTerminal reports terminal creation as unsupported by default.
func (RestrictivePolicy) CreateTerminal(context.Context, CreateTerminalRequest) (CreateTerminalResponse, error) {
	return CreateTerminalResponse{}, fmt.Errorf("%w: create terminal", ErrUnsupportedOperation)
}

// TerminalOutput reports terminal output as unsupported by default.
func (RestrictivePolicy) TerminalOutput(context.Context, TerminalOutputRequest) (TerminalOutputResponse, error) {
	return TerminalOutputResponse{}, fmt.Errorf("%w: terminal output", ErrUnsupportedOperation)
}

// ReleaseTerminal reports terminal release as unsupported by default.
func (RestrictivePolicy) ReleaseTerminal(context.Context, ReleaseTerminalRequest) (ReleaseTerminalResponse, error) {
	return ReleaseTerminalResponse{}, fmt.Errorf("%w: release terminal", ErrUnsupportedOperation)
}

// WaitForTerminalExit reports terminal waits as unsupported by default.
func (RestrictivePolicy) WaitForTerminalExit(context.Context, WaitForTerminalExitRequest) (WaitForTerminalExitResponse, error) {
	return WaitForTerminalExitResponse{}, fmt.Errorf("%w: wait for terminal exit", ErrUnsupportedOperation)
}

// KillTerminal reports terminal kill requests as unsupported by default.
func (RestrictivePolicy) KillTerminal(context.Context, KillTerminalRequest) (KillTerminalResponse, error) {
	return KillTerminalResponse{}, fmt.Errorf("%w: kill terminal", ErrUnsupportedOperation)
}
