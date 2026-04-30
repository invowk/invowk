// SPDX-License-Identifier: MPL-2.0

package config_test

import (
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestRuntimeModeValidationSyncsWithSharedTypes(t *testing.T) {
	t.Parallel()

	values := []types.RuntimeMode{
		types.RuntimeNative,
		types.RuntimeVirtual,
		types.RuntimeContainer,
		"",
		"bogus",
	}

	for _, value := range values {
		t.Run(value.String(), func(t *testing.T) {
			t.Parallel()

			sharedValid := value.Validate() == nil
			configValid := config.RuntimeMode(value).Validate() == nil
			invowkfileValid := invowkfile.RuntimeMode(value).Validate() == nil

			if configValid != sharedValid {
				t.Fatalf("config.RuntimeMode(%q).Validate() valid = %v, want shared valid %v", value, configValid, sharedValid)
			}
			if invowkfileValid != sharedValid {
				t.Fatalf("invowkfile.RuntimeMode(%q).Validate() valid = %v, want shared valid %v", value, invowkfileValid, sharedValid)
			}
		})
	}
}
