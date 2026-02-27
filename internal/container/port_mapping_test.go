// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"testing"
)

func TestPortMapping_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mapping PortMapping
		want    bool
		wantErr bool
	}{
		{
			"all valid fields with TCP",
			PortMapping{
				HostPort:      8080,
				ContainerPort: 80,
				Protocol:      PortProtocolTCP,
			},
			true, false,
		},
		{
			"all valid fields with UDP",
			PortMapping{
				HostPort:      5353,
				ContainerPort: 53,
				Protocol:      PortProtocolUDP,
			},
			true, false,
		},
		{
			"valid with empty protocol (defaults to TCP)",
			PortMapping{
				HostPort:      3000,
				ContainerPort: 3000,
				Protocol:      "",
			},
			true, false,
		},
		{
			"invalid host port (zero)",
			PortMapping{
				HostPort:      0,
				ContainerPort: 80,
				Protocol:      PortProtocolTCP,
			},
			false, true,
		},
		{
			"invalid container port (zero)",
			PortMapping{
				HostPort:      8080,
				ContainerPort: 0,
				Protocol:      PortProtocolTCP,
			},
			false, true,
		},
		{
			"invalid protocol",
			PortMapping{
				HostPort:      8080,
				ContainerPort: 80,
				Protocol:      PortProtocol("sctp"),
			},
			false, true,
		},
		{
			"multiple invalid fields",
			PortMapping{
				HostPort:      0,
				ContainerPort: 0,
				Protocol:      PortProtocol("bogus"),
			},
			false, true,
		},
		{
			"zero value (all fields zero/empty)",
			PortMapping{},
			false, true, // HostPort and ContainerPort invalid; Protocol "" is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.mapping.Validate()
			if (err == nil) != tt.want {
				t.Errorf("PortMapping.Validate() error = %v, want valid=%v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("PortMapping.Validate() returned nil, want error")
				}
			} else if err != nil {
				t.Errorf("PortMapping.Validate() returned unexpected error: %v", err)
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

func TestPortMapping_Validate_FieldErrorTypes(t *testing.T) {
	t.Parallel()

	// Verify that the joined error wraps the correct sentinels.
	mapping := PortMapping{
		HostPort:      0,
		ContainerPort: 0,
		Protocol:      PortProtocol("invalid"),
	}
	err := mapping.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The joined error should wrap all three sentinels
	if !errors.Is(err, ErrInvalidNetworkPort) {
		t.Errorf("error should wrap ErrInvalidNetworkPort, got: %v", err)
	}
	if !errors.Is(err, ErrInvalidPortProtocol) {
		t.Errorf("error should wrap ErrInvalidPortProtocol, got: %v", err)
	}

	// Verify individual error types via errors.As
	var npErr *InvalidNetworkPortError
	if !errors.As(err, &npErr) {
		t.Errorf("error should contain *InvalidNetworkPortError, got: %T", err)
	}

	var ppErr *InvalidPortProtocolError
	if !errors.As(err, &ppErr) {
		t.Errorf("error should contain *InvalidPortProtocolError, got: %T", err)
	}
}

func TestParsePortMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		portStr string
		want    PortMapping
		wantErr bool
	}{
		{
			"simple TCP mapping",
			"8080:80",
			PortMapping{HostPort: 8080, ContainerPort: 80},
			false,
		},
		{
			"explicit TCP protocol",
			"8080:80/tcp",
			PortMapping{HostPort: 8080, ContainerPort: 80, Protocol: PortProtocolTCP},
			false,
		},
		{
			"UDP protocol",
			"5353:53/udp",
			PortMapping{HostPort: 5353, ContainerPort: 53, Protocol: PortProtocolUDP},
			false,
		},
		{
			"same port",
			"443:443",
			PortMapping{HostPort: 443, ContainerPort: 443},
			false,
		},
		{
			"max port",
			"65535:65535",
			PortMapping{HostPort: 65535, ContainerPort: 65535},
			false,
		},
		{
			"host port out of range",
			"70000:80",
			PortMapping{},
			true,
		},
		{
			"container port out of range",
			"8080:70000",
			PortMapping{HostPort: 8080},
			true,
		},
		{
			"no colon separator",
			"8080",
			PortMapping{},
			true,
		},
		{
			"empty string",
			"",
			PortMapping{},
			true,
		},
		{
			"non-numeric host port",
			"abc:80",
			PortMapping{},
			true,
		},
		{
			"non-numeric container port",
			"8080:abc",
			PortMapping{HostPort: 8080},
			true,
		},
		{
			"zero host port",
			"0:80",
			PortMapping{HostPort: 0, ContainerPort: 80},
			true,
		},
		{
			"zero container port",
			"8080:0",
			PortMapping{HostPort: 8080, ContainerPort: 0},
			true,
		},
		{
			"invalid protocol",
			"8080:80/sctp",
			PortMapping{HostPort: 8080, ContainerPort: 80, Protocol: PortProtocol("sctp")},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParsePortMapping(tt.portStr)
			if got.HostPort != tt.want.HostPort {
				t.Errorf("HostPort = %d, want %d", got.HostPort, tt.want.HostPort)
			}
			if got.ContainerPort != tt.want.ContainerPort {
				t.Errorf("ContainerPort = %d, want %d", got.ContainerPort, tt.want.ContainerPort)
			}
			if got.Protocol != tt.want.Protocol {
				t.Errorf("Protocol = %q, want %q", got.Protocol, tt.want.Protocol)
			}
			if tt.wantErr {
				if err == nil {
					t.Error("ParsePortMapping() returned nil error, want error")
				}
			} else if err != nil {
				t.Errorf("ParsePortMapping() returned unexpected error: %v", err)
			}
		})
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
