// SPDX-License-Identifier: MPL-2.0

package containerargs

import (
	"errors"
	"strings"
	"testing"
)

func TestContainerNameValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   ContainerName
		wantErr bool
	}{
		{name: "empty is valid", value: ""},
		{name: "derived-style name", value: "invowk-io.example.build-a1b2c3d4e5f6"},
		{name: "external user name", value: "dev_container.1"},
		{name: "starts with digit", value: "1builder"},
		{name: "uppercase rejected", value: "Build", wantErr: true},
		{name: "space rejected", value: "dev container", wantErr: true},
		{name: "slash rejected", value: "dev/container", wantErr: true},
		{name: "colon rejected", value: "dev:container", wantErr: true},
		{name: "empty after trim rejected", value: "   ", wantErr: true},
		{name: "leading hyphen rejected", value: "-dev", wantErr: true},
		{name: "leading underscore rejected", value: "_dev", wantErr: true},
		{name: "too long rejected", value: ContainerName(strings.Repeat("a", MaxContainerNameLength+1)), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertContainerNameValidation(t, tt.value, tt.wantErr)
		})
	}
}

func TestContainerNameValidateMutationContracts(t *testing.T) {
	t.Parallel()

	validNames := []ContainerName{
		"",
		"a",
		"z",
		"0",
		"9",
		"dev.container_9-name",
		ContainerName(strings.Repeat("a", MaxContainerNameLength)),
	}
	for _, name := range validNames {
		t.Run("valid "+name.String(), func(t *testing.T) {
			t.Parallel()

			if err := name.Validate(); err != nil {
				t.Fatalf("Validate(%q) error = %v, want nil", name, err)
			}
		})
	}

	tests := []struct {
		name       string
		value      ContainerName
		wantReason string
	}{
		{
			name:       "too long preserves value and reason",
			value:      ContainerName(strings.Repeat("a", MaxContainerNameLength+1)),
			wantReason: "must be at most 128 characters",
		},
		{
			name:       "uppercase start preserves value and reason",
			value:      "Build",
			wantReason: "must start with a lowercase ASCII letter or digit",
		},
		{
			name:       "hyphen start preserves value and reason",
			value:      "-dev",
			wantReason: "must start with a lowercase ASCII letter or digit",
		},
		{
			name:       "underscore start preserves value and reason",
			value:      "_dev",
			wantReason: "must start with a lowercase ASCII letter or digit",
		},
		{
			name:       "slash body preserves value and reason",
			value:      "dev/container",
			wantReason: "must contain only lowercase ASCII letters, digits, '.', '_', or '-'",
		},
		{
			name:       "colon body preserves value and reason",
			value:      "dev:container",
			wantReason: "must contain only lowercase ASCII letters, digits, '.', '_', or '-'",
		},
		{
			name:       "space body preserves value and reason",
			value:      "dev container",
			wantReason: "must contain only lowercase ASCII letters, digits, '.', '_', or '-'",
		},
		{
			name:       "after lowercase body preserves value and reason",
			value:      "dev{container",
			wantReason: "must contain only lowercase ASCII letters, digits, '.', '_', or '-'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertInvalidContainerNameDetails(t, tt.value.Validate(), tt.value, tt.wantReason)
		})
	}
}

func TestIsLowerASCIILetterOrDigitMutationBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		char byte
		want bool
	}{
		{name: "lowercase lower boundary", char: 'a', want: true},
		{name: "lowercase upper boundary", char: 'z', want: true},
		{name: "digit lower boundary", char: '0', want: true},
		{name: "digit upper boundary", char: '9', want: true},
		{name: "before lowercase boundary", char: '`', want: false},
		{name: "after lowercase boundary", char: '{', want: false},
		{name: "before digit boundary", char: '/', want: false},
		{name: "after digit boundary", char: ':', want: false},
		{name: "uppercase is not lowercase", char: 'A', want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isLowerASCIILetterOrDigit(tt.char); got != tt.want {
				t.Fatalf("isLowerASCIILetterOrDigit(%q) = %v, want %v", tt.char, got, tt.want)
			}
		})
	}
}

func TestIsContainerNameCharacterMutationBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		char rune
		want bool
	}{
		{name: "lowercase lower boundary", char: 'a', want: true},
		{name: "lowercase upper boundary", char: 'z', want: true},
		{name: "digit lower boundary", char: '0', want: true},
		{name: "digit upper boundary", char: '9', want: true},
		{name: "dot allowed", char: '.', want: true},
		{name: "underscore allowed", char: '_', want: true},
		{name: "hyphen allowed", char: '-', want: true},
		{name: "uppercase rejected", char: 'A'},
		{name: "slash rejected", char: '/'},
		{name: "colon rejected", char: ':'},
		{name: "non ASCII rejected", char: 'é'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isContainerNameCharacter(tt.char); got != tt.want {
				t.Fatalf("isContainerNameCharacter(%q) = %v, want %v", tt.char, got, tt.want)
			}
		})
	}
}

func TestContainerNameValueFormattingMutationContracts(t *testing.T) {
	t.Parallel()

	name := ContainerName("dev.container")
	if got, want := name.String(), "dev.container"; got != want {
		t.Fatalf("ContainerName.String() = %q, want %q", got, want)
	}

	err := &InvalidContainerNameError{
		Value:  "Bad",
		Reason: "must start with a lowercase ASCII letter or digit",
	}
	if got, want := err.Error(), "invalid container name \"Bad\": must start with a lowercase ASCII letter or digit"; got != want {
		t.Fatalf("InvalidContainerNameError.Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, ErrInvalidContainerName) {
		t.Fatalf("InvalidContainerNameError should unwrap ErrInvalidContainerName, got %v", err)
	}
}

func assertContainerNameValidation(t *testing.T, value ContainerName, wantErr bool) {
	t.Helper()

	err := value.Validate()
	if wantErr {
		assertInvalidContainerNameError(t, err)
		return
	}
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func assertInvalidContainerNameError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !errors.Is(err, ErrInvalidContainerName) {
		t.Fatalf("Validate() error = %v, want ErrInvalidContainerName", err)
	}
	var invalid *InvalidContainerNameError
	if !errors.As(err, &invalid) {
		t.Fatalf("Validate() error type = %T, want *InvalidContainerNameError", err)
	}
}

func assertInvalidContainerNameDetails(
	t *testing.T,
	err error,
	wantValue ContainerName,
	wantReason string,
) {
	t.Helper()

	assertInvalidContainerNameError(t, err)
	var invalid *InvalidContainerNameError
	if !errors.As(err, &invalid) {
		t.Fatalf("Validate() error type = %T, want *InvalidContainerNameError", err)
	}
	if invalid.Value != wantValue {
		t.Fatalf("InvalidContainerNameError.Value = %q, want %q", invalid.Value, wantValue)
	}
	if invalid.Reason != wantReason {
		t.Fatalf("InvalidContainerNameError.Reason = %q, want %q", invalid.Reason, wantReason)
	}
}
