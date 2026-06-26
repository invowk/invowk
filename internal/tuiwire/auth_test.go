// SPDX-License-Identifier: MPL-2.0

package tuiwire

import (
	"errors"
	"testing"
)

func TestAuthTokenMutationContracts(t *testing.T) {
	t.Parallel()

	token := AuthToken("secret-token")
	if got := token.String(); got != "secret-token" {
		t.Fatalf("AuthToken.String() = %q, want secret-token", got)
	}
	if err := token.Validate(); err != nil {
		t.Fatalf("AuthToken.Validate() error = %v, want nil", err)
	}

	invalid := AuthToken(" \t")
	err := invalid.Validate()
	if !errors.Is(err, ErrInvalidAuthToken) {
		t.Fatalf("AuthToken.Validate() error = %v, want ErrInvalidAuthToken", err)
	}
	var invalidErr *InvalidAuthTokenError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("AuthToken.Validate() error type = %T, want *InvalidAuthTokenError", err)
	}
	if invalidErr.Value != invalid {
		t.Fatalf("InvalidAuthTokenError.Value = %q, want %q", invalidErr.Value, invalid)
	}
	if got, want := invalidErr.Error(), `invalid auth token " \t": must be non-empty`; got != want {
		t.Fatalf("InvalidAuthTokenError.Error() = %q, want %q", got, want)
	}
}
