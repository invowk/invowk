// SPDX-License-Identifier: MPL-2.0

package myapp

import (
	"constructorvalidates_cross/util"
)

// NewServerWithDirective calls util.ValidateServer which has the
// //goplint:validates-type=Server directive. This cross-package call
// satisfies the constructor-validates check via fact propagation.
// Should NOT be flagged.
func NewServerWithDirective(addr string) (*util.Server, error) { // want `parameter "addr" of myapp\.NewServerWithDirective uses primitive type string`
	s := &util.Server{Addr: addr}
	return s, util.ValidateServer(s)
}

// NewServerNoDirective calls util.HelperNoDirective which lacks the
// directive. The cross-package call does NOT satisfy the check.
// SHOULD be flagged.
func NewServerNoDirective(addr string) (*util.Server, error) { // want `parameter "addr" of myapp\.NewServerNoDirective uses primitive type string` `constructor myapp\.NewServerNoDirective returns util\.Server which has Validate\(\) but never calls it`
	s := &util.Server{Addr: addr}
	return s, util.HelperNoDirective(s)
}
