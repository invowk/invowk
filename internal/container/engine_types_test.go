// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"testing"
)

func TestContainerID_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      ContainerID
		want    bool
		wantErr bool
	}{
		{"valid hex ID", ContainerID("abc123def456"), true, false},
		{"full SHA", ContainerID("sha256:abc123def456789"), true, false},
		{"short ID", ContainerID("abc123"), true, false},
		{"empty is invalid", ContainerID(""), false, true},
		{"whitespace only is invalid", ContainerID("   "), false, true},
		{"tab only is invalid", ContainerID("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.id.IsValid()
			if isValid != tt.want {
				t.Errorf("ContainerID(%q).IsValid() = %v, want %v", tt.id, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ContainerID(%q).IsValid() returned no errors, want error", tt.id)
				}
				if !errors.Is(errs[0], ErrInvalidContainerID) {
					t.Errorf("error should wrap ErrInvalidContainerID, got: %v", errs[0])
				}
				var cidErr *InvalidContainerIDError
				if !errors.As(errs[0], &cidErr) {
					t.Errorf("error should be *InvalidContainerIDError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ContainerID(%q).IsValid() returned unexpected errors: %v", tt.id, errs)
			}
		})
	}
}

func TestContainerID_String(t *testing.T) {
	t.Parallel()
	c := ContainerID("abc123")
	if c.String() != "abc123" {
		t.Errorf("ContainerID.String() = %q, want %q", c.String(), "abc123")
	}
}

func TestImageTag_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tag     ImageTag
		want    bool
		wantErr bool
	}{
		{"simple tag", ImageTag("debian:stable-slim"), true, false},
		{"latest tag", ImageTag("ubuntu:latest"), true, false},
		{"registry with port", ImageTag("registry.example.com:5000/myimage:v1"), true, false},
		{"no tag", ImageTag("debian"), true, false},
		{"empty is invalid", ImageTag(""), false, true},
		{"whitespace only is invalid", ImageTag("   "), false, true},
		{"tab only is invalid", ImageTag("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.tag.IsValid()
			if isValid != tt.want {
				t.Errorf("ImageTag(%q).IsValid() = %v, want %v", tt.tag, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ImageTag(%q).IsValid() returned no errors, want error", tt.tag)
				}
				if !errors.Is(errs[0], ErrInvalidImageTag) {
					t.Errorf("error should wrap ErrInvalidImageTag, got: %v", errs[0])
				}
				var itErr *InvalidImageTagError
				if !errors.As(errs[0], &itErr) {
					t.Errorf("error should be *InvalidImageTagError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ImageTag(%q).IsValid() returned unexpected errors: %v", tt.tag, errs)
			}
		})
	}
}

func TestImageTag_String(t *testing.T) {
	t.Parallel()
	it := ImageTag("debian:stable-slim")
	if it.String() != "debian:stable-slim" {
		t.Errorf("ImageTag.String() = %q, want %q", it.String(), "debian:stable-slim")
	}
}

func TestContainerName_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cn      ContainerName
		want    bool
		wantErr bool
	}{
		{"valid name", ContainerName("my-container"), true, false},
		{"name with underscore", ContainerName("my_container_1"), true, false},
		{"empty is valid (zero value)", ContainerName(""), true, false},
		{"whitespace only is invalid", ContainerName("   "), false, true},
		{"tab only is invalid", ContainerName("\t"), false, true},
		{"newline only is invalid", ContainerName("\n"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.cn.IsValid()
			if isValid != tt.want {
				t.Errorf("ContainerName(%q).IsValid() = %v, want %v", tt.cn, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ContainerName(%q).IsValid() returned no errors, want error", tt.cn)
				}
				if !errors.Is(errs[0], ErrInvalidContainerName) {
					t.Errorf("error should wrap ErrInvalidContainerName, got: %v", errs[0])
				}
				var cnErr *InvalidContainerNameError
				if !errors.As(errs[0], &cnErr) {
					t.Errorf("error should be *InvalidContainerNameError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ContainerName(%q).IsValid() returned unexpected errors: %v", tt.cn, errs)
			}
		})
	}
}

func TestContainerName_String(t *testing.T) {
	t.Parallel()
	cn := ContainerName("my-container")
	if cn.String() != "my-container" {
		t.Errorf("ContainerName.String() = %q, want %q", cn.String(), "my-container")
	}
}

func TestHostMapping_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		hm      HostMapping
		want    bool
		wantErr bool
	}{
		{"docker internal", HostMapping("host.docker.internal:host-gateway"), true, false},
		{"ip mapping", HostMapping("myhost:192.168.1.1"), true, false},
		{"simple mapping", HostMapping("localhost:127.0.0.1"), true, false},
		{"empty is invalid", HostMapping(""), false, true},
		{"whitespace only is invalid", HostMapping("   "), false, true},
		{"tab only is invalid", HostMapping("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.hm.IsValid()
			if isValid != tt.want {
				t.Errorf("HostMapping(%q).IsValid() = %v, want %v", tt.hm, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("HostMapping(%q).IsValid() returned no errors, want error", tt.hm)
				}
				if !errors.Is(errs[0], ErrInvalidHostMapping) {
					t.Errorf("error should wrap ErrInvalidHostMapping, got: %v", errs[0])
				}
				var hmErr *InvalidHostMappingError
				if !errors.As(errs[0], &hmErr) {
					t.Errorf("error should be *InvalidHostMappingError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("HostMapping(%q).IsValid() returned unexpected errors: %v", tt.hm, errs)
			}
		})
	}
}

func TestHostMapping_String(t *testing.T) {
	t.Parallel()
	hm := HostMapping("host.docker.internal:host-gateway")
	if hm.String() != "host.docker.internal:host-gateway" {
		t.Errorf("HostMapping.String() = %q, want %q", hm.String(), "host.docker.internal:host-gateway")
	}
}

func TestNetworkPort_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		port    NetworkPort
		want    bool
		wantErr bool
	}{
		{"standard HTTP port", NetworkPort(80), true, false},
		{"standard HTTPS port", NetworkPort(443), true, false},
		{"max port", NetworkPort(65535), true, false},
		{"port 1", NetworkPort(1), true, false},
		{"zero is invalid", NetworkPort(0), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.port.IsValid()
			if isValid != tt.want {
				t.Errorf("NetworkPort(%d).IsValid() = %v, want %v", tt.port, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("NetworkPort(%d).IsValid() returned no errors, want error", tt.port)
				}
				if !errors.Is(errs[0], ErrInvalidNetworkPort) {
					t.Errorf("error should wrap ErrInvalidNetworkPort, got: %v", errs[0])
				}
				var npErr *InvalidNetworkPortError
				if !errors.As(errs[0], &npErr) {
					t.Errorf("error should be *InvalidNetworkPortError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("NetworkPort(%d).IsValid() returned unexpected errors: %v", tt.port, errs)
			}
		})
	}
}

func TestNetworkPort_String(t *testing.T) {
	t.Parallel()
	p := NetworkPort(8080)
	if p.String() != "8080" {
		t.Errorf("NetworkPort.String() = %q, want %q", p.String(), "8080")
	}
}

func TestHostFilesystemPath_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    HostFilesystemPath
		want    bool
		wantErr bool
	}{
		{"absolute path", HostFilesystemPath("/home/user/data"), true, false},
		{"relative path", HostFilesystemPath("./data"), true, false},
		{"dot path", HostFilesystemPath("."), true, false},
		{"empty is invalid", HostFilesystemPath(""), false, true},
		{"whitespace only is invalid", HostFilesystemPath("   "), false, true},
		{"tab only is invalid", HostFilesystemPath("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.path.IsValid()
			if isValid != tt.want {
				t.Errorf("HostFilesystemPath(%q).IsValid() = %v, want %v", tt.path, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("HostFilesystemPath(%q).IsValid() returned no errors, want error", tt.path)
				}
				if !errors.Is(errs[0], ErrInvalidHostFilesystemPath) {
					t.Errorf("error should wrap ErrInvalidHostFilesystemPath, got: %v", errs[0])
				}
				var hfpErr *InvalidHostFilesystemPathError
				if !errors.As(errs[0], &hfpErr) {
					t.Errorf("error should be *InvalidHostFilesystemPathError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("HostFilesystemPath(%q).IsValid() returned unexpected errors: %v", tt.path, errs)
			}
		})
	}
}

func TestHostFilesystemPath_String(t *testing.T) {
	t.Parallel()
	p := HostFilesystemPath("/home/user/data")
	if p.String() != "/home/user/data" {
		t.Errorf("HostFilesystemPath.String() = %q, want %q", p.String(), "/home/user/data")
	}
}

func TestMountTargetPath_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    MountTargetPath
		want    bool
		wantErr bool
	}{
		{"absolute container path", MountTargetPath("/app/data"), true, false},
		{"root path", MountTargetPath("/"), true, false},
		{"workspace path", MountTargetPath("/workspace"), true, false},
		{"empty is invalid", MountTargetPath(""), false, true},
		{"whitespace only is invalid", MountTargetPath("   "), false, true},
		{"tab only is invalid", MountTargetPath("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.path.IsValid()
			if isValid != tt.want {
				t.Errorf("MountTargetPath(%q).IsValid() = %v, want %v", tt.path, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("MountTargetPath(%q).IsValid() returned no errors, want error", tt.path)
				}
				if !errors.Is(errs[0], ErrInvalidMountTargetPath) {
					t.Errorf("error should wrap ErrInvalidMountTargetPath, got: %v", errs[0])
				}
				var mtpErr *InvalidMountTargetPathError
				if !errors.As(errs[0], &mtpErr) {
					t.Errorf("error should be *InvalidMountTargetPathError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("MountTargetPath(%q).IsValid() returned unexpected errors: %v", tt.path, errs)
			}
		})
	}
}

func TestMountTargetPath_String(t *testing.T) {
	t.Parallel()
	p := MountTargetPath("/app/data")
	if p.String() != "/app/data" {
		t.Errorf("MountTargetPath.String() = %q, want %q", p.String(), "/app/data")
	}
}
