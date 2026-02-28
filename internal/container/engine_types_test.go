// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestContainerID_Validate(t *testing.T) {
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
			err := tt.id.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ContainerID(%q).Validate() error = %v, want valid=%v", tt.id, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ContainerID(%q).Validate() returned nil, want error", tt.id)
				}
				if !errors.Is(err, ErrInvalidContainerID) {
					t.Errorf("error should wrap ErrInvalidContainerID, got: %v", err)
				}
				var cidErr *InvalidContainerIDError
				if !errors.As(err, &cidErr) {
					t.Errorf("error should be *InvalidContainerIDError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ContainerID(%q).Validate() returned unexpected error: %v", tt.id, err)
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

func TestImageTag_Validate(t *testing.T) {
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
			err := tt.tag.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ImageTag(%q).Validate() error = %v, want valid=%v", tt.tag, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ImageTag(%q).Validate() returned nil, want error", tt.tag)
				}
				if !errors.Is(err, ErrInvalidImageTag) {
					t.Errorf("error should wrap ErrInvalidImageTag, got: %v", err)
				}
				var itErr *InvalidImageTagError
				if !errors.As(err, &itErr) {
					t.Errorf("error should be *InvalidImageTagError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ImageTag(%q).Validate() returned unexpected error: %v", tt.tag, err)
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

func TestContainerName_Validate(t *testing.T) {
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
			err := tt.cn.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ContainerName(%q).Validate() error = %v, want valid=%v", tt.cn, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ContainerName(%q).Validate() returned nil, want error", tt.cn)
				}
				if !errors.Is(err, ErrInvalidContainerName) {
					t.Errorf("error should wrap ErrInvalidContainerName, got: %v", err)
				}
				var cnErr *InvalidContainerNameError
				if !errors.As(err, &cnErr) {
					t.Errorf("error should be *InvalidContainerNameError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ContainerName(%q).Validate() returned unexpected error: %v", tt.cn, err)
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

func TestHostMapping_Validate(t *testing.T) {
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
			err := tt.hm.Validate()
			if (err == nil) != tt.want {
				t.Errorf("HostMapping(%q).Validate() error = %v, want valid=%v", tt.hm, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("HostMapping(%q).Validate() returned nil, want error", tt.hm)
				}
				if !errors.Is(err, ErrInvalidHostMapping) {
					t.Errorf("error should wrap ErrInvalidHostMapping, got: %v", err)
				}
				var hmErr *InvalidHostMappingError
				if !errors.As(err, &hmErr) {
					t.Errorf("error should be *InvalidHostMappingError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("HostMapping(%q).Validate() returned unexpected error: %v", tt.hm, err)
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

func TestNetworkPort_Validate(t *testing.T) {
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
			err := tt.port.Validate()
			if (err == nil) != tt.want {
				t.Errorf("NetworkPort(%d).Validate() error = %v, want valid=%v", tt.port, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NetworkPort(%d).Validate() returned nil, want error", tt.port)
				}
				if !errors.Is(err, ErrInvalidNetworkPort) {
					t.Errorf("error should wrap ErrInvalidNetworkPort, got: %v", err)
				}
				var npErr *InvalidNetworkPortError
				if !errors.As(err, &npErr) {
					t.Errorf("error should be *InvalidNetworkPortError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("NetworkPort(%d).Validate() returned unexpected error: %v", tt.port, err)
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

func TestHostFilesystemPath_Validate(t *testing.T) {
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
			err := tt.path.Validate()
			if (err == nil) != tt.want {
				t.Errorf("HostFilesystemPath(%q).Validate() error = %v, want valid=%v", tt.path, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("HostFilesystemPath(%q).Validate() returned nil, want error", tt.path)
				}
				if !errors.Is(err, ErrInvalidHostFilesystemPath) {
					t.Errorf("error should wrap ErrInvalidHostFilesystemPath, got: %v", err)
				}
				var hfpErr *InvalidHostFilesystemPathError
				if !errors.As(err, &hfpErr) {
					t.Errorf("error should be *InvalidHostFilesystemPathError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("HostFilesystemPath(%q).Validate() returned unexpected error: %v", tt.path, err)
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

func TestMountTargetPath_Validate(t *testing.T) {
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
			err := tt.path.Validate()
			if (err == nil) != tt.want {
				t.Errorf("MountTargetPath(%q).Validate() error = %v, want valid=%v", tt.path, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("MountTargetPath(%q).Validate() returned nil, want error", tt.path)
				}
				if !errors.Is(err, ErrInvalidMountTargetPath) {
					t.Errorf("error should wrap ErrInvalidMountTargetPath, got: %v", err)
				}
				var mtpErr *InvalidMountTargetPathError
				if !errors.As(err, &mtpErr) {
					t.Errorf("error should be *InvalidMountTargetPathError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("MountTargetPath(%q).Validate() returned unexpected error: %v", tt.path, err)
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

func TestBuildOptions_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		opts      BuildOptions
		want      bool
		wantErr   bool
		wantCount int // expected number of field errors inside InvalidBuildOptionsError
	}{
		{
			"all valid fields",
			BuildOptions{
				ContextDir: "/app",
				Dockerfile: "Dockerfile",
				Tag:        "myimage:latest",
			},
			true, false, 0,
		},
		{
			"invalid context dir (empty)",
			BuildOptions{
				ContextDir: "",
				Dockerfile: "Dockerfile",
				Tag:        "myimage:latest",
			},
			false, true, 1,
		},
		{
			"invalid dockerfile (whitespace)",
			BuildOptions{
				ContextDir: "/app",
				Dockerfile: "   ",
				Tag:        "myimage:latest",
			},
			false, true, 1,
		},
		{
			"empty tag is valid (zero-value-is-valid: means no explicit tag)",
			BuildOptions{
				ContextDir: "/app",
				Dockerfile: "Dockerfile",
				Tag:        "",
			},
			true, false, 0,
		},
		{
			"all non-empty fields invalid",
			BuildOptions{
				ContextDir: "",
				Dockerfile: "   ",
				Tag:        "\t",
			},
			false, true, 3,
		},
		{
			"zero value (only ContextDir is required)",
			BuildOptions{},
			false, true, 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.opts.Validate()
			if (err == nil) != tt.want {
				t.Errorf("BuildOptions.Validate() error = %v, want valid=%v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("BuildOptions.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidBuildOptions) {
					t.Errorf("error should wrap ErrInvalidBuildOptions, got: %v", err)
				}
				var boErr *InvalidBuildOptionsError
				if !errors.As(err, &boErr) {
					t.Fatalf("error should be *InvalidBuildOptionsError, got: %T", err)
				}
				if tt.wantCount > 0 && len(boErr.FieldErrors) != tt.wantCount {
					t.Errorf("InvalidBuildOptionsError has %d field errors, want %d: %v",
						len(boErr.FieldErrors), tt.wantCount, boErr.FieldErrors)
				}
			} else if err != nil {
				t.Errorf("BuildOptions.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestRunOptions_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		opts      RunOptions
		want      bool
		wantErr   bool
		wantCount int // expected number of field errors inside InvalidRunOptionsError
	}{
		{
			"minimal valid (image only)",
			RunOptions{
				Image: "debian:stable-slim",
			},
			true, false, 0,
		},
		{
			"fully populated valid",
			RunOptions{
				Image:      "debian:stable-slim",
				WorkDir:    "/app",
				Name:       "my-container",
				ExtraHosts: []HostMapping{"host.docker.internal:host-gateway"},
				Volumes:    []invowkfile.VolumeMountSpec{"/host:/container"},
				Ports:      []invowkfile.PortMappingSpec{"8080:80"},
			},
			true, false, 0,
		},
		{
			"empty image is invalid",
			RunOptions{
				Image: "",
			},
			false, true, 1,
		},
		{
			"whitespace-only workdir is invalid",
			RunOptions{
				Image:   "debian:stable-slim",
				WorkDir: "   ",
			},
			false, true, 1,
		},
		{
			"empty workdir is valid (zero value skipped)",
			RunOptions{
				Image:   "debian:stable-slim",
				WorkDir: "",
			},
			true, false, 0,
		},
		{
			"whitespace-only name is invalid",
			RunOptions{
				Image: "debian:stable-slim",
				Name:  "   ",
			},
			false, true, 1,
		},
		{
			"invalid extra host",
			RunOptions{
				Image:      "debian:stable-slim",
				ExtraHosts: []HostMapping{""},
			},
			false, true, 1,
		},
		{
			"invalid volume spec",
			RunOptions{
				Image:   "debian:stable-slim",
				Volumes: []invowkfile.VolumeMountSpec{""},
			},
			false, true, 1,
		},
		{
			"invalid port spec",
			RunOptions{
				Image: "debian:stable-slim",
				Ports: []invowkfile.PortMappingSpec{""},
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			RunOptions{
				Image:      "",
				WorkDir:    "\t",
				Name:       "   ",
				ExtraHosts: []HostMapping{""},
				Volumes:    []invowkfile.VolumeMountSpec{""},
				Ports:      []invowkfile.PortMappingSpec{""},
			},
			false, true, 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.opts.Validate()
			if (err == nil) != tt.want {
				t.Errorf("RunOptions.Validate() error = %v, want valid=%v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("RunOptions.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidRunOptions) {
					t.Errorf("error should wrap ErrInvalidRunOptions, got: %v", err)
				}
				var roErr *InvalidRunOptionsError
				if !errors.As(err, &roErr) {
					t.Fatalf("error should be *InvalidRunOptionsError, got: %T", err)
				}
				if tt.wantCount > 0 && len(roErr.FieldErrors) != tt.wantCount {
					t.Errorf("InvalidRunOptionsError has %d field errors, want %d: %v",
						len(roErr.FieldErrors), tt.wantCount, roErr.FieldErrors)
				}
			} else if err != nil {
				t.Errorf("RunOptions.Validate() returned unexpected error: %v", err)
			}
		})
	}
}
