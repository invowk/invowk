// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
)

var (
	dockerInfoKey    = "info --format " + dockerInfoNetworkFmt
	dockerContextKey = "context inspect --format " + dockerContextHostFmt
	dockerBridgeKey  = "network inspect bridge --format " + dockerBridgeGatewayFmt
	dockerVersionKey = "version --format " + dockerServerComponentsFmt
	podmanInfoKey    = "info --format " + podmanInfoNetworkFmt
)

// fakeQueryEngine answers RunQuery from a map keyed by the joined arguments.
type fakeQueryEngine struct {
	Engine
	name    string
	answers map[string]string
	fail    map[string]error
	calls   []string
}

func (f *fakeQueryEngine) Name() string { return f.name }

func (f *fakeQueryEngine) RunQuery(_ context.Context, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err := f.fail[key]; err != nil {
		return nil, err
	}
	out, ok := f.answers[key]
	if !ok {
		return nil, errors.New("unexpected query " + key)
	}
	return []byte(out), nil
}

func noEnv(string) string { return "" }

func TestQueryEngineNetworkFactsDocker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		answers map[string]string
		getenv  func(string) string
		want    EngineNetworkFacts
	}{
		{
			name: "rootful local",
			answers: map[string]string{
				dockerInfoKey:    `Fedora Linux 45|["name=seccomp,profile=builtin","name=selinux","name=cgroupns"]` + "\n",
				dockerContextKey: "unix:///var/run/docker.sock\n",
				dockerVersionKey: "Engine;containerd;runc;tini-static;\n",
				dockerBridgeKey:  "172.17.0.1 \n",
			},
			getenv: noEnv,
			want:   EngineNetworkFacts{Engine: EngineTypeDocker, BridgeGateway: netip.MustParseAddr("172.17.0.1")},
		},
		{
			// Probed 2026-11-15: Docker CLI 29.7.2 against a rootless Podman
			// 6.1.3 socket; a rootful socket differs only in name=rootless.
			name: "rootful podman behind the docker CLI skips the bridge",
			answers: map[string]string{
				dockerInfoKey:    `fedora|["name=seccomp,profile=default","name=selinux"]`,
				dockerVersionKey: "Podman Engine;Conmon;OCI Runtime (crun);Engine;",
			},
			getenv: func(key string) string {
				if key == "DOCKER_HOST" {
					return "unix:///run/podman/podman.sock"
				}
				return ""
			},
			want: EngineNetworkFacts{Engine: EngineTypeDocker, PodmanBackend: true},
		},
		{
			name:    "rootless",
			answers: map[string]string{dockerInfoKey: `Ubuntu|["name=seccomp","name=rootless"]`, dockerContextKey: "unix:///run/user/1000/docker.sock", dockerBridgeKey: "172.17.0.1"},
			getenv:  noEnv,
			want:    EngineNetworkFacts{Engine: EngineTypeDocker, Rootless: true, BridgeGateway: netip.MustParseAddr("172.17.0.1")},
		},
		{
			name:    "docker desktop",
			answers: map[string]string{dockerInfoKey: `Docker Desktop|["name=seccomp"]`, dockerContextKey: "unix:///Users/me/.docker/run/docker.sock"},
			getenv:  noEnv,
			want:    EngineNetworkFacts{Engine: EngineTypeDocker, DockerDesktop: true},
		},
		{
			name:    "remote DOCKER_HOST skips context and bridge",
			answers: map[string]string{dockerInfoKey: `Debian|[]`},
			getenv: func(key string) string {
				if key == "DOCKER_HOST" {
					return "ssh://builder"
				}
				return ""
			},
			want: EngineNetworkFacts{Engine: EngineTypeDocker, RemoteEndpoint: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine := &fakeQueryEngine{name: "docker", answers: tt.answers}
			got, err := QueryEngineNetworkFacts(t.Context(), engine, tt.getenv)
			if err != nil {
				t.Fatalf("QueryEngineNetworkFacts(): %v", err)
			}
			if got != tt.want {
				t.Fatalf("facts = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestQueryEngineNetworkFactsPodman(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		out  string
		want EngineNetworkFacts
	}{
		{name: "rootless pasta", out: "true|pasta|false|/usr/bin/pasta\n", want: EngineNetworkFacts{Engine: EngineTypePodman, Rootless: true, PastaAvailable: true}},
		{name: "rootless without pasta", out: "true|slirp4netns|false|", want: EngineNetworkFacts{Engine: EngineTypePodman, Rootless: true}},
		{name: "podman-remote", out: "true|pasta|true|/usr/bin/pasta", want: EngineNetworkFacts{Engine: EngineTypePodman, Rootless: true, RemoteEndpoint: true, PastaAvailable: true}},
		{name: "rootful", out: "false|||", want: EngineNetworkFacts{Engine: EngineTypePodman}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine := &fakeQueryEngine{name: "podman", answers: map[string]string{podmanInfoKey: tt.out}}
			got, err := QueryEngineNetworkFacts(t.Context(), engine, noEnv)
			if err != nil {
				t.Fatalf("QueryEngineNetworkFacts(): %v", err)
			}
			if got != tt.want {
				t.Fatalf("facts = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestQueryEngineNetworkFactsFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		engine *fakeQueryEngine
	}{
		{name: "docker info fails", engine: &fakeQueryEngine{name: "docker", fail: map[string]error{dockerInfoKey: errors.New("daemon down")}}},
		{name: "docker version fails", engine: &fakeQueryEngine{
			name:    "docker",
			answers: map[string]string{dockerInfoKey: `Debian|[]`, dockerContextKey: "unix:///var/run/docker.sock"},
			fail:    map[string]error{dockerVersionKey: errors.New("daemon down")},
		}},
		{name: "podman docker shim template", engine: &fakeQueryEngine{name: "docker", answers: map[string]string{dockerInfoKey: "<no value>"}}},
		{name: "podman info fails", engine: &fakeQueryEngine{name: "podman", fail: map[string]error{podmanInfoKey: errors.New("template: no field Pasta")}}},
		{name: "podman short output", engine: &fakeQueryEngine{name: "podman", answers: map[string]string{podmanInfoKey: "true|pasta"}}},
		{name: "unknown engine", engine: &fakeQueryEngine{name: "nerdctl"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := QueryEngineNetworkFacts(t.Context(), tt.engine, noEnv); !errors.Is(err, ErrNetworkFactsUnavailable) {
				t.Fatalf("error = %v, want ErrNetworkFactsUnavailable", err)
			}
		})
	}
}

func TestFlatpakInfoSharesNetwork(t *testing.T) {
	t.Parallel()

	withNet := "[Application]\nname=org.example\n\n[Context]\nshared=network;ipc;\nsockets=x11;\n"
	without := "[Application]\nname=org.example\n\n[Context]\nshared=ipc;\n[Instance]\nshared=network;\n"
	if !flatpakInfoSharesNetwork(withNet) {
		t.Error("shared=network;ipc; not detected")
	}
	if flatpakInfoSharesNetwork(without) {
		t.Error("network outside [Context] counted")
	}
}

func TestIsRemoteDockerEndpoint(t *testing.T) {
	t.Parallel()

	for endpoint, want := range map[string]bool{
		"":                            false,
		"unix:///var/run/docker.sock": false,
		"npipe:////./pipe/docker":     false,
		"fd://":                       false,
		"tcp://10.0.0.5:2376":         true,
		"ssh://builder":               true,
		// The Docker CLI dials a scheme-less host:port as TCP.
		"build-box:2375":   true,
		"127.0.0.1:1":      true,
		"http://dind:2375": true,
	} {
		if got := isRemoteDockerEndpoint(endpoint); got != want {
			t.Errorf("isRemoteDockerEndpoint(%q) = %v, want %v", endpoint, got, want)
		}
	}
}

// TestQueryEngineNetworkFactsLocalEngine runs the real query against the
// local engine, through the sandbox-aware wrapper production uses.
func TestQueryEngineNetworkFactsLocalEngine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	engine, err := AutoDetectEngine()
	if err != nil {
		t.Skipf("no container engine available: %v", err)
	}
	testutil.AcquireContainerSemaphore(t)

	facts, err := QueryEngineNetworkFacts(t.Context(), NewSandboxAwareEngine(engine), nil)
	if err != nil {
		t.Fatalf("QueryEngineNetworkFacts(%s): %v", engine.Name(), err)
	}
	if string(facts.Engine) != engine.Name() {
		t.Fatalf("facts.Engine = %q, want %q", facts.Engine, engine.Name())
	}
	t.Logf("local %s network facts: %+v", engine.Name(), facts)
}
