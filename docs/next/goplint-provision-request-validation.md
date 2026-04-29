# Goplint Gap: Boundary Request Validation

## Status

Yes. The provisioning request/image invariant issue is a goplint coverage gap.

The production fix is still required in the application code: `LayerProvisioner.Provision` now defaults the base image and then calls `Request.Validate()` before reading request fields for disabled provisioning, cache lookup, or layer build decisions.

## Why Existing Rules Missed It

`internal/provision.Request` already defined `Validate()` and its `BaseImage` field used the validatable `container.ImageTag` value type. The bug was that `Provision(ctx, req Request)` used `req.BaseImage` without a dominating `req.Validate()` call.

Current goplint rules cover nearby cases:

- discarded `Validate()` results
- casts to DDD value types without validation
- constructors that return validatable values without validating
- structs whose `Validate()` methods fail to delegate to validatable fields

Those rules do not prove that a method accepting an already-built request struct validates the request before using its fields. This means request structs with complete invariant definitions can still cross an application boundary unchecked.

## Proposed Analyzer Rule

Add a boundary request validation rule for structs that have `Validate() error` and are passed by value or pointer into exported or package-boundary orchestration methods.

The rule should report when a method:

- accepts a validatable request-like struct parameter
- reads fields from that parameter, passes fields into another operation, or makes control-flow decisions from those fields
- does so before a dominating successful `req.Validate()` check

The rule should allow explicit trusted-boundary exceptions for code paths where validation is guaranteed by a caller and documented with a narrow goplint directive.

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
