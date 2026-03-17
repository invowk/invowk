// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/base64"
)

// base64Command wraps the u-root base64 implementation.
type base64Command struct {
	baseWrapper
}

// newBase64Command creates a new base64 command wrapper.
func newBase64Command() *base64Command {
	return &base64Command{
		baseWrapper: baseWrapper{
			name: "base64",
			flags: []FlagInfo{
				{Name: "d", Description: "decode data"},
			},
		},
	}
}

// Run executes the base64 command.
func (c *base64Command) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, base64.New(), args)
}
