# Goplint Gap: Boundary Request Validation

## Status

Implemented.

`LayerProvisioner.Provision` defaults the base image and then calls `Request.Validate()` before reading request fields for disabled provisioning, cache lookup, or layer build decisions. The adjacent cache-inspection boundary, `LayerProvisioner.GetProvisionedImageTag`, now validates its `container.ImageTag` argument before cache-key generation so `IsImageProvisioned` inherits the same image invariant.

goplint now includes `--check-boundary-request-validation`, included by `--check-all`, for exported functions and methods that accept validatable `*Request` / `*Options` structs and return `error`. It reports `unvalidated-boundary-request` when the parameter is read before a checked `Validate()` guard. The mode allows defaulting and nil-guard prologues before validation, and it supports a narrow `//goplint:trusted-boundary` directive for wrappers that delegate to an already-validating boundary or validate a documented partial option shape.

## Why Existing Rules Missed It

`internal/provision.Request` already defined `Validate()` and its `BaseImage` field used the validatable `container.ImageTag` value type. The bug was that `Provision(ctx, req Request)` used `req.BaseImage` without a dominating `req.Validate()` call.

Current goplint rules cover nearby cases:

- discarded `Validate()` results
- casts to DDD value types without validation
- constructors that return validatable values without validating
- structs whose `Validate()` methods fail to delegate to validatable fields

Those rules do not prove that a method accepting an already-built request struct validates the request before using its fields. This means request structs with complete invariant definitions can still cross an application boundary unchecked.

## Analyzer Rule

The boundary request validation rule applies to structs that have `Validate() error`, whose type name ends in `Request` or `Options`, and that are passed by value or pointer into exported error-returning boundary methods.

The rule reports when a method:

- accepts a validatable request-like struct parameter
- reads fields from that parameter, passes fields into another operation, or makes control-flow decisions from those fields
- does so before a dominating successful `req.Validate()` check

The rule allows explicit trusted-boundary exceptions for code paths where validation is guaranteed by a caller and documented with the narrow `//goplint:trusted-boundary -- reason` directive.

## Example Shape

```go
func (p *LayerProvisioner) Provision(ctx context.Context, req Request) (*Result, error) {
	if req.BaseImage == "" {
		req.BaseImage = DefaultBaseImage
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	// Field use after this point is guarded by Request.Validate().
}
```

This fills a distinct gap from constructor validation: request values may be assembled by CLI, tests, application services, or other adapters, and boundary methods remain the last reliable place to enforce their invariants.
