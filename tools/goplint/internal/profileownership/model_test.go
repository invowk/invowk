// SPDX-License-Identifier: MPL-2.0

package profileownership

import (
	"testing"

	"github.com/invowk/invowk/tools/goplint/internal/soundnessgate"
)

func TestRouteConservativelyClassifiesEveryGovernedContext(t *testing.T) {
	t.Parallel()

	manifest := Manifest{FormatVersion: FormatVersion, Rules: []Rule{
		{Pattern: ".github/workflows/**", Profile: soundnessgate.ProfileSemantic},
		{Pattern: "cmd/**", Profile: soundnessgate.ProfileConsumer},
		{Pattern: "internal/**", Profile: soundnessgate.ProfileConsumer},
		{Pattern: "tools/goplint/**", Profile: soundnessgate.ProfileSemantic},
	}}
	tests := []struct {
		name  string
		input Context
		want  soundnessgate.ProfileID
	}{
		{name: "consumer", input: changed("pull_request", "cmd/invowk/main.go", "internal/config/config.go"), want: soundnessgate.ProfileConsumer},
		{name: "semantic", input: changed("pull_request", "tools/goplint/goplint/analyzer.go"), want: soundnessgate.ProfileSemantic},
		{name: "multi area", input: changed("pull_request", "cmd/invowk/main.go", "tools/goplint/spec/semantic-rules.v1.json"), want: soundnessgate.ProfileSemantic},
		{name: "rename deletion paths", input: changed("push", "cmd/old.go", "cmd/new.go"), want: soundnessgate.ProfileConsumer},
		{name: "workflow", input: changed("pull_request", ".github/workflows/lint.yml"), want: soundnessgate.ProfileSemantic},
		{name: "unknown", input: changed("pull_request", "README.md"), want: soundnessgate.ProfileSemantic},
		{name: "empty", input: changed("pull_request"), want: soundnessgate.ProfileSemantic},
		{name: "missing merge base", input: Context{Event: "pull_request", ChangedPaths: []string{"cmd/main.go"}}, want: soundnessgate.ProfileSemantic},
		{name: "shallow", input: Context{Event: "pull_request", ChangedPaths: []string{"cmd/main.go"}, MergeBaseAvailable: true, ShallowRepository: true}, want: soundnessgate.ProfileSemantic},
		{name: "dispatch", input: changed("workflow_dispatch", "cmd/main.go"), want: soundnessgate.ProfileComplete},
		{name: "schedule", input: changed("schedule", "cmd/main.go"), want: soundnessgate.ProfileComplete},
		{name: "release", input: changed("release", "cmd/main.go"), want: soundnessgate.ProfileComplete},
		{name: "unknown event", input: changed("mystery", "cmd/main.go"), want: soundnessgate.ProfileSemantic},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			decision, err := manifest.Route(testCase.input)
			if err != nil {
				t.Fatalf("Route() error = %v", err)
			}
			if decision.Profile != testCase.want {
				t.Fatalf("Route() profile = %q, want %q; reason = %s", decision.Profile, testCase.want, decision.Reason)
			}
		})
	}
}

func changed(event string, paths ...string) Context {
	return Context{Event: event, ChangedPaths: paths, MergeBaseAvailable: true}
}
