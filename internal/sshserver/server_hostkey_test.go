// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// startAndStop starts the server, fails the test on error, and stops it.
func startAndStop(t *testing.T, srv *Server) {
	t.Helper()
	if err := srv.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

//nolint:paralleltest // t.Chdir scopes the working-directory assertion and forbids t.Parallel.
func TestStartEphemeralHostKeyWritesNoKeyFile(t *testing.T) {
	// No HostKeyPath: the server must fall back to an in-memory key instead
	// of wish's default, which writes ./id_ed25519 into the process working
	// directory. t.Chdir gives this test its own directory to observe.
	workDir := t.TempDir()
	t.Chdir(workDir)

	srv, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	startAndStop(t, srv)

	if _, statErr := os.Stat(filepath.Join(workDir, "id_ed25519")); !os.IsNotExist(statErr) {
		t.Errorf("expected no id_ed25519 in working directory, stat error = %v", statErr)
	}
}

func TestStartHostKeyPathGeneratesAndReusesKey(t *testing.T) {
	t.Parallel()

	keyPath := filepath.Join(t.TempDir(), "ssh", "host_ed25519")
	cfg := DefaultConfig()
	typedKeyPath := types.FilesystemPath(keyPath)
	cfg.HostKeyPath = &typedKeyPath

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	startAndStop(t, srv)

	firstKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("host key was not generated at %s: %v", keyPath, err)
	}
	if len(firstKey) == 0 {
		t.Fatal("generated host key is empty")
	}

	srv2, err := New(cfg)
	if err != nil {
		t.Fatalf("New() (second) error = %v", err)
	}
	startAndStop(t, srv2)

	secondKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("host key missing after second start: %v", err)
	}
	if !bytes.Equal(firstKey, secondKey) {
		t.Error("host key changed between starts; expected a stable key at HostKeyPath")
	}
}
