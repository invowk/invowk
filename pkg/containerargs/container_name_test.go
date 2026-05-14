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
