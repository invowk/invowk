// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestDecideContainerSuiteHarness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		explicit  string
		preferred container.EngineType
		statuses  map[container.EngineType]engineProbeStatus
		wantState containerHarnessStatus
		wantType  container.EngineType
		wantText  string
	}{
		{
			name:      "explicit docker healthy",
			explicit:  "docker",
			preferred: container.EngineTypePodman,
			statuses: map[container.EngineType]engineProbeStatus{
				container.EngineTypeDocker: {present: true, healthy: true, binaryPath: "/usr/bin/docker"},
				container.EngineTypePodman: {present: true, healthy: false, reason: "unhealthy"},
			},
			wantState: containerHarnessStatusReady,
			wantType:  container.EngineTypeDocker,
		},
		{
			name:      "invalid explicit engine",
			explicit:  "nerdctl",
			preferred: container.EngineTypePodman,
			statuses:  map[container.EngineType]engineProbeStatus{},
			wantState: containerHarnessStatusFail,
			wantText:  "invalid INVOWK_TEST_CONTAINER_ENGINE",
		},
		{
			name:      "preferred missing fallback healthy",
			preferred: container.EngineTypePodman,
			statuses: map[container.EngineType]engineProbeStatus{
				container.EngineTypePodman: {present: false},
				container.EngineTypeDocker: {present: true, healthy: true, binaryPath: "/usr/bin/docker"},
			},
			wantState: containerHarnessStatusReady,
			wantType:  container.EngineTypeDocker,
		},
		{
			name:      "preferred present but unhealthy fails fast",
			preferred: container.EngineTypePodman,
			statuses: map[container.EngineType]engineProbeStatus{
				container.EngineTypePodman: {present: true, healthy: false, reason: "version probe failed"},
				container.EngineTypeDocker: {present: true, healthy: true, binaryPath: "/usr/bin/docker"},
			},
			wantState: containerHarnessStatusFail,
			wantType:  "",
			wantText:  "installed but unhealthy",
		},
		{
			name:      "preferred unsupported fallback healthy",
			preferred: container.EngineTypePodman,
			statuses: map[container.EngineType]engineProbeStatus{
				container.EngineTypePodman: {present: true, unsupported: true, reason: "workspace mount smoke failed"},
				container.EngineTypeDocker: {present: true, healthy: true, binaryPath: "/usr/bin/docker"},
			},
			wantState: containerHarnessStatusReady,
			wantType:  container.EngineTypeDocker,
		},
		{
			name:      "preferred unsupported without fallback skips",
			preferred: container.EngineTypePodman,
			statuses: map[container.EngineType]engineProbeStatus{
				container.EngineTypePodman: {present: true, unsupported: true, reason: "workspace mount smoke failed"},
				container.EngineTypeDocker: {present: false},
			},
			wantState: containerHarnessStatusSkip,
			wantText:  "cannot support the container CLI suite",
		},
		{
			name:      "explicit unsupported engine fails",
			explicit:  "podman",
			preferred: container.EngineTypePodman,
			statuses: map[container.EngineType]engineProbeStatus{
				container.EngineTypePodman: {present: true, unsupported: true, reason: "workspace mount smoke failed"},
			},
			wantState: containerHarnessStatusFail,
			wantText:  "installed but unhealthy",
		},
		{
			name:      "no installed engines skips",
			preferred: container.EngineTypePodman,
			statuses: map[container.EngineType]engineProbeStatus{
				container.EngineTypePodman: {present: false},
				container.EngineTypeDocker: {present: false},
			},
			wantState: containerHarnessStatusSkip,
			wantText:  "no installed container engine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := decideContainerSuiteHarness(tt.explicit, tt.preferred, tt.statuses)
			if got.status != tt.wantState {
				t.Fatalf("status = %v, want %v", got.status, tt.wantState)
			}
			if tt.wantType != "" && got.engineType != tt.wantType {
				t.Fatalf("engineType = %s, want %s", got.engineType, tt.wantType)
			}
			if tt.wantText != "" && !strings.Contains(got.reason, tt.wantText) {
				t.Fatalf("reason = %q, want substring %q", got.reason, tt.wantText)
			}
		})
	}
}

func TestDecideContainerSuiteHarnessForHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		host      invowkfile.PlatformType
		wantState containerHarnessStatus
		wantType  container.EngineType
		wantText  string
	}{
		{
			name:      "windows skips",
			host:      invowkfile.PlatformWindows,
			wantState: containerHarnessStatusSkip,
			wantText:  "requires a Linux host",
		},
		{
			name:      "linux delegates to harness selection",
			host:      invowkfile.PlatformLinux,
			wantState: containerHarnessStatusReady,
			wantType:  container.EngineTypePodman,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := decideContainerSuiteHarnessForHost(
				tt.host,
				"",
				container.EngineTypePodman,
				map[container.EngineType]engineProbeStatus{
					container.EngineTypePodman: {present: true, healthy: true, binaryPath: "/usr/bin/podman"},
					container.EngineTypeDocker: {present: true, healthy: true, binaryPath: "/usr/bin/docker"},
				},
			)

			if got.status != tt.wantState {
				t.Fatalf("status = %v, want %v", got.status, tt.wantState)
			}
			if tt.wantType != "" && got.engineType != tt.wantType {
				t.Fatalf("engineType = %s, want %s", got.engineType, tt.wantType)
			}
			if tt.wantText != "" && !strings.Contains(got.reason, tt.wantText) {
				t.Fatalf("reason = %q, want substring %q", got.reason, tt.wantText)
			}
		})
	}
}

func TestContainerCLISuiteSupportedHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host invowkfile.PlatformType
		want bool
	}{
		{host: invowkfile.PlatformLinux, want: true},
		{host: invowkfile.PlatformWindows, want: false},
		{host: invowkfile.PlatformMac, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.host.String(), func(t *testing.T) {
			t.Parallel()

			if got := containerCLISuiteSupportedHost(tt.host); got != tt.want {
				t.Fatalf("containerCLISuiteSupportedHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}

	current := invowkfile.CurrentPlatform()
	if got := containerCLISuiteSupportedHost(current); got != (current == invowkfile.PlatformLinux) {
		t.Fatalf("containerCLISuiteSupportedHost(CurrentPlatform=%q) = %v", current, got)
	}
}
