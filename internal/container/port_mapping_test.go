// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"testing"
)

func TestPortMapping_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mapping  PortMapping
		want     bool
		wantErr  bool
		wantErrs int // expected number of field errors (0 means don't check count)
	}{
		{
			"all valid fields with TCP",
			PortMapping{
				HostPort:      8080,
				ContainerPort: 80,
				Protocol:      PortProtocolTCP,
			},
			true, false, 0,
		},
		{
			"all valid fields with UDP",
			PortMapping{
				HostPort:      5353,
				ContainerPort: 53,
				Protocol:      PortProtocolUDP,
			},
			true, false, 0,
		},
		{
			"valid with empty protocol (defaults to TCP)",
			PortMapping{
				HostPort:      3000,
				ContainerPort: 3000,
				Protocol:      "",
			},
			true, false, 0,
		},
		{
			"invalid host port (zero)",
			PortMapping{
				HostPort:      0,
				ContainerPort: 80,
				Protocol:      PortProtocolTCP,
			},
			false, true, 1,
		},
		{
			"invalid container port (zero)",
			PortMapping{
				HostPort:      8080,
				ContainerPort: 0,
				Protocol:      PortProtocolTCP,
			},
			false, true, 1,
		},
		{
			"invalid protocol",
			PortMapping{
				HostPort:      8080,
				ContainerPort: 80,
				Protocol:      PortProtocol("sctp"),
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			PortMapping{
				HostPort:      0,
				ContainerPort: 0,
				Protocol:      PortProtocol("bogus"),
			},
			false, true, 3,
		},
		{
			"zero value (all fields zero/empty)",
			PortMapping{},
			false, true, 2, // HostPort and ContainerPort invalid; Protocol "" is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.mapping.IsValid()
			if isValid != tt.want {
				t.Errorf("PortMapping.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatal("PortMapping.IsValid() returned no errors, want error")
				}
				if tt.wantErrs > 0 && len(errs) != tt.wantErrs {
					t.Errorf("PortMapping.IsValid() returned %d errors, want %d: %v",
						len(errs), tt.wantErrs, errs)
				}
			} else if len(errs) > 0 {
				t.Errorf("PortMapping.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}

func TestPortMapping_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mapping PortMapping
		want    string
	}{
		{
			"tcp_explicit",
			PortMapping{HostPort: 8080, ContainerPort: 80, Protocol: PortProtocolTCP},
			"8080:80/tcp",
		},
		{
			"udp",
			PortMapping{HostPort: 53, ContainerPort: 53, Protocol: PortProtocolUDP},
			"53:53/udp",
		},
		{
			"empty_protocol_defaults_tcp",
			PortMapping{HostPort: 443, ContainerPort: 443},
			"443:443/tcp",
		},
		{
			"zero_ports",
			PortMapping{},
			"0:0/tcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.mapping.String()
			if got != tt.want {
				t.Errorf("PortMapping.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPortMapping_IsValid_FieldErrorTypes(t *testing.T) {
	t.Parallel()

	// Verify that each field error wraps the correct sentinel and has the correct type.
	mapping := PortMapping{
		HostPort:      0,
		ContainerPort: 0,
		Protocol:      PortProtocol("invalid"),
	}
	isValid, errs := mapping.IsValid()
	if isValid {
		t.Fatal("expected invalid, got valid")
	}
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors (one per invalid field), got %d: %v", len(errs), errs)
	}

	// First error: HostPort
	if !errors.Is(errs[0], ErrInvalidNetworkPort) {
		t.Errorf("first error should wrap ErrInvalidNetworkPort, got: %v", errs[0])
	}
	var npErr *InvalidNetworkPortError
	if !errors.As(errs[0], &npErr) {
		t.Errorf("first error should be *InvalidNetworkPortError, got: %T", errs[0])
	}

	// Second error: ContainerPort
	if !errors.Is(errs[1], ErrInvalidNetworkPort) {
		t.Errorf("second error should wrap ErrInvalidNetworkPort, got: %v", errs[1])
	}
	var npErr2 *InvalidNetworkPortError
	if !errors.As(errs[1], &npErr2) {
		t.Errorf("second error should be *InvalidNetworkPortError, got: %T", errs[1])
	}

	// Third error: Protocol
	if !errors.Is(errs[2], ErrInvalidPortProtocol) {
		t.Errorf("third error should wrap ErrInvalidPortProtocol, got: %v", errs[2])
	}
	var ppErr *InvalidPortProtocolError
	if !errors.As(errs[2], &ppErr) {
		t.Errorf("third error should be *InvalidPortProtocolError, got: %T", errs[2])
	}
}

func TestInvalidPortMappingError(t *testing.T) {
	t.Parallel()

	fieldErrs := []error{
		&InvalidNetworkPortError{Value: 0},
		&InvalidNetworkPortError{Value: 0},
	}
	err := &InvalidPortMappingError{
		Value:     PortMapping{HostPort: 0, ContainerPort: 0, Protocol: PortProtocolTCP},
		FieldErrs: fieldErrs,
	}

	if err.Error() == "" {
		t.Error("InvalidPortMappingError.Error() returned empty string")
	}
	if !errors.Is(err, ErrInvalidPortMapping) {
		t.Error("InvalidPortMappingError should wrap ErrInvalidPortMapping")
	}
}
