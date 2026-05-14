// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"maps"
	"testing"
)

func TestParseContainerInspect(t *testing.T) {
	t.Parallel()

	data := []byte(`[
		{
			"Id": "abc123",
			"Name": "/my-dev",
			"State": {"Running": true, "Status": "running"},
			"Config": {"Labels": {"dev.invowk.managed": "true"}}
		}
	]`)

	info, err := parseContainerInspect(data, "fallback")
	if err != nil {
		t.Fatalf("parseContainerInspect() error = %v", err)
	}
	if info.ContainerID != "abc123" {
		t.Fatalf("ContainerID = %q, want abc123", info.ContainerID)
	}
	if info.Name != "my-dev" {
		t.Fatalf("Name = %q, want my-dev", info.Name)
	}
	if !info.Running {
		t.Fatal("Running = false, want true")
	}
	if info.Labels["dev.invowk.managed"] != "true" {
		t.Fatalf("managed label = %q", info.Labels["dev.invowk.managed"])
	}
}

func TestParseContainerInspectUsesTopLevelLabels(t *testing.T) {
	t.Parallel()

	data := []byte(`[
		{
			"Id": "abc123",
			"Name": "my-dev",
			"Labels": {"dev.invowk.managed": "true"},
			"State": {"Running": false, "Status": "exited"}
		}
	]`)

	info, err := parseContainerInspect(data, "fallback")
	if err != nil {
		t.Fatalf("parseContainerInspect() error = %v", err)
	}
	if info.Running {
		t.Fatal("Running = true, want false")
	}
	if info.Status != "exited" {
		t.Fatalf("Status = %q, want exited", info.Status)
	}
	if info.Labels["dev.invowk.managed"] != "true" {
		t.Fatalf("managed label = %q", info.Labels["dev.invowk.managed"])
	}
}

func TestParseContainerInspectDockerAndPodmanShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    []byte
		running bool
		status  string
		labels  map[string]string
	}{
		{
			name: "docker running with config labels",
			data: []byte(`[
				{
					"Id": "docker123",
					"Name": "/docker-dev",
					"State": {"Running": true, "Status": "running"},
					"Config": {"Labels": {"dev.invowk.managed": "true"}}
				}
			]`),
			running: true,
			status:  "running",
			labels:  map[string]string{"dev.invowk.managed": "true"},
		},
		{
			name: "podman stopped with top-level labels",
			data: []byte(`[
				{
					"Id": "podman123",
					"Name": "podman-dev",
					"State": {"Running": false, "Status": "exited"},
					"Labels": {"dev.invowk.persistent": "true"}
				}
			]`),
			running: false,
			status:  "exited",
			labels:  map[string]string{"dev.invowk.persistent": "true"},
		},
		{
			name: "unlabeled container",
			data: []byte(`[
				{
					"Id": "plain123",
					"Name": "plain-dev",
					"State": {"Running": true, "Status": "running"},
					"Config": {}
				}
			]`),
			running: true,
			status:  "running",
			labels:  map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info, err := parseContainerInspect(tt.data, "fallback")
			if err != nil {
				t.Fatalf("parseContainerInspect() error = %v", err)
			}
			assertContainerInfo(t, info, tt.running, tt.status, tt.labels)
		})
	}
}

func assertContainerInfo(t *testing.T, info *ContainerInfo, running bool, status string, labels map[string]string) {
	t.Helper()

	if info.Running != running {
		t.Fatalf("Running = %v, want %v", info.Running, running)
	}
	if info.Status != status {
		t.Fatalf("Status = %q, want %q", info.Status, status)
	}
	if !maps.Equal(info.Labels, labels) {
		t.Fatalf("Labels = %v, want %v", info.Labels, labels)
	}
}

func TestInspectContainerMapsMissingContainerError(t *testing.T) {
	t.Parallel()

	recorder := NewMockCommandRecorder()
	recorder.ExitCode = 1
	recorder.Stderr = "Error: No such container: missing-dev"
	engine := NewBaseCLIEngine("/usr/bin/docker", WithName("docker"), WithExecCommand(recorder.ContextCommandFunc(t)))

	_, err := engine.InspectContainer(t.Context(), "missing-dev")
	if !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("InspectContainer() error = %v, want ErrContainerNotFound", err)
	}
}

func TestCreateMapsNameConflictError(t *testing.T) {
	t.Parallel()

	recorder := NewMockCommandRecorder()
	recorder.ExitCode = 1
	recorder.Stderr = "Conflict. The container name \"my-dev\" is already in use"
	engine := NewBaseCLIEngine("/usr/bin/docker", WithName("docker"), WithExecCommand(recorder.ContextCommandFunc(t)))

	_, err := engine.Create(t.Context(), CreateOptions{
		Image:   "debian:stable-slim",
		Command: []string{"/bin/sh", "-c", "sleep infinity"},
		Name:    "my-dev",
	})
	if !errors.Is(err, ErrContainerNameConflict) {
		t.Fatalf("Create() error = %v, want ErrContainerNameConflict", err)
	}
}
