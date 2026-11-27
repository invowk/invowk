// SPDX-License-Identifier: MPL-2.0

//go:build !unix

package cmd

import "context"

func watchShutdownSignals(parent context.Context) (ctx context.Context, stop func(), preserveResume func() bool) {
	return parent, func() {}, func() bool { return false }
}
