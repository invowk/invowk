// SPDX-License-Identifier: MPL-2.0

package container

import (
	"testing"
)

func TestSecurityOptionsReportSELinux(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "fedora daemon with selinux", output: `["name=seccomp,profile=builtin","name=selinux","name=cgroupns"]`, want: true},
		{name: "selinux with trailing newline", output: "[\"name=selinux\"]\n", want: true},
		{name: "ubuntu daemon without selinux", output: `["name=apparmor","name=seccomp,profile=builtin","name=cgroupns"]`, want: false},
		{name: "rootless daemon", output: `["name=seccomp,profile=builtin","name=rootless","name=cgroupns"]`, want: false},
		{name: "selinux as a substring only", output: `["name=seccomp,profile=selinux-like"]`, want: false},
		{name: "null", output: "null", want: false},
		{name: "malformed output", output: "Cannot connect to the Docker daemon", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := securityOptionsReportSELinux(tt.output); got != tt.want {
				t.Fatalf("securityOptionsReportSELinux(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}

// TestDockerEngine_SELinuxVolumeLabeling mirrors TestPodmanEngine_SELinuxVolumeLabeling:
// both engines must label volumes identically once SELinux is detected.
func TestDockerEngine_SELinuxVolumeLabeling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		selinux        bool
		volume         string
		expectedInArgs string
	}{
		{name: "selinux adds :z", selinux: true, volume: "/host:/container", expectedInArgs: "/host:/container:z"},
		{name: "selinux preserves :Z", selinux: true, volume: "/host:/container:Z", expectedInArgs: "/host:/container:Z"},
		{name: "selinux appends z to ro", selinux: true, volume: "/host:/container:ro", expectedInArgs: "/host:/container:ro,z"},
		{name: "no selinux leaves volume unchanged", selinux: false, volume: "/host:/container", expectedInArgs: "/host:/container"},
		{name: "no selinux preserves options", selinux: false, volume: "/host:/container:ro", expectedInArgs: "/host:/container:ro"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := NewMockCommandRecorder()
			engine := NewDockerEngineWithSELinuxCheck(
				func() bool { return tt.selinux },
				WithExecCommand(recorder.ContextCommandFunc(t)),
			)
			if _, err := engine.Run(t.Context(), RunOptions{
				Image:   "debian:stable-slim",
				Volumes: []VolumeMountSpec{VolumeMountSpec(tt.volume)},
			}); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			recorder.AssertArgsContain(t, tt.expectedInArgs)
		})
	}
}

func TestDockerEngine_DaemonReportsSELinux(t *testing.T) {
	t.Parallel()

	t.Run("asks the daemon for its security options", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stdout = `["name=seccomp,profile=builtin","name=selinux"]`
		engine := &DockerEngine{BaseCLIEngine: NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(recorder.ContextCommandFunc(t)))}

		if !engine.daemonReportsSELinux() {
			t.Fatal("daemonReportsSELinux() = false, want true")
		}
		recorder.AssertInvocationCount(t, 1)
		recorder.AssertFirstArg(t, "info")
		recorder.AssertArgsContainAll(t, []string{"--format", "{{json .SecurityOptions}}"})
	})

	t.Run("an unreachable daemon reports no selinux", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.ExitCode = 1
		recorder.Stderr = "Cannot connect to the Docker daemon"
		engine := &DockerEngine{BaseCLIEngine: NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(recorder.ContextCommandFunc(t)))}

		if engine.daemonReportsSELinux() {
			t.Fatal("daemonReportsSELinux() = true for a failing daemon, want false")
		}
	})

	t.Run("no docker binary reports no selinux", func(t *testing.T) {
		t.Parallel()

		engine := &DockerEngine{BaseCLIEngine: NewBaseCLIEngine("")}
		if engine.daemonReportsSELinux() {
			t.Fatal("daemonReportsSELinux() = true without a binary, want false")
		}
	})
}
