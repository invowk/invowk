// SPDX-License-Identifier: MPL-2.0

package soundnessgate

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/invowk/invowk/tools/goplint/internal/soundnessevidence"
)

func TestWorkBundleValidateRejectsStaleOrIncompleteBindings(t *testing.T) {
	t.Parallel()

	plan := validExecutionPlan()
	bundle := validWorkBundle(t, plan, plan.Commands[0].WorkUnitID)
	tests := []struct {
		name   string
		mutate func(*WorkBundle)
	}{
		{name: "plan", mutate: func(value *WorkBundle) { value.PlanID = runnerChangedDigest }},
		{name: "workspace", mutate: func(value *WorkBundle) { value.WorkspaceDigest = runnerChangedDigest }},
		{name: "command", mutate: func(value *WorkBundle) { value.CommandDigest = runnerChangedDigest }},
		{name: "binary", mutate: func(value *WorkBundle) { value.BinaryDigest = runnerChangedDigest }},
		{name: "terminal", mutate: func(value *WorkBundle) { value.TerminalStatus = "canceled" }},
		{name: "population", mutate: func(value *WorkBundle) { value.Report.Populations = nil }},
		{name: "bundle identity", mutate: func(value *WorkBundle) { value.PeakRSSBytes++ }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			mutated := bundle
			mutated.Report.Populations = slices.Clone(bundle.Report.Populations)
			testCase.mutate(&mutated)
			if err := mutated.Validate(plan); err == nil {
				t.Fatal("Validate() accepted a stale or incomplete distributed bundle")
			}
		})
	}
}

func TestWorkBundleValidateRejectsReportDigestThatDoesNotBindEmbeddedReport(t *testing.T) {
	t.Parallel()

	plan := validExecutionPlan()
	bundle := validWorkBundle(t, plan, plan.Commands[0].WorkUnitID)
	bundle.Report.Populations[0].Count++
	mutated, err := normalizeWorkBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := mutated.Validate(plan); err == nil {
		t.Fatal("Validate() accepted a recomputed bundle identity with a stale embedded-report digest")
	}
}

func TestValidateCompleteBundleSetRejectsMissingUnexpectedAndMismatchedDependencyArtifacts(t *testing.T) {
	t.Parallel()

	plan := validExecutionPlan()
	plan.Commands[0].WorkUnitID = "repository-audit"
	plan.Dependencies[0].WorkUnitID = "repository-audit"
	plan.ExpectedReports[0].WorkUnitID = "repository-audit"
	plan.Shards[0].WorkUnitID = "repository-audit"
	plan.Commands[1].WorkUnitID = "dependent"
	plan.Dependencies[1] = PlanDependencyBinding{WorkUnitID: "dependent", Requires: []string{"repository-audit"}}
	plan.ExpectedReports[1].WorkUnitID = "dependent"
	plan.Shards[1].WorkUnitID = "dependent"
	plan, err := NormalizeExecutionPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	auditDigest := soundnessevidence.DigestBytes([]byte("audit"))
	bundles := map[string]WorkBundle{
		"repository-audit": {WorkUnitID: "repository-audit", OutputRepositoryAuditDigest: auditDigest},
		"dependent":        {WorkUnitID: "dependent", InputRepositoryAuditDigest: auditDigest},
	}
	if err := validateCompleteBundleSet(plan, bundles, auditDigest); err != nil {
		t.Fatalf("validateCompleteBundleSet() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(map[string]WorkBundle)
	}{
		{name: "missing", mutate: func(values map[string]WorkBundle) { delete(values, "dependent") }},
		{name: "unexpected", mutate: func(values map[string]WorkBundle) { values["extra"] = WorkBundle{} }},
		{name: "mismatched audit", mutate: func(values map[string]WorkBundle) {
			value := values["dependent"]
			value.InputRepositoryAuditDigest = runnerChangedDigest
			values["dependent"] = value
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			values := make(map[string]WorkBundle, len(bundles))
			maps.Copy(values, bundles)
			testCase.mutate(values)
			if err := validateCompleteBundleSet(plan, values, auditDigest); err == nil {
				t.Fatal("validateCompleteBundleSet() accepted incomplete artifacts")
			}
		})
	}
}

func TestFixtureDrivenDistributedPlanWorkersAndAggregate(t *testing.T) {
	t.Parallel()

	manifest := validGateManifest()
	registry := validGateRegistry()
	root, manifestPath, _ := writeRunnerFixture(t, manifest, registry)
	plan, err := generatePlan(t.Context(), PlanOptions{
		Root: root, ManifestPath: manifestPath, Profile: ProfileSemantic,
		Resources: ResourceBudget{
			CPUUnits: 4, MemoryBytes: 16 * 1024 * 1024 * 1024,
			MaximumWorkers: 4, RunnerClass: "fixture-runner",
		},
	}, deterministicPlanDependencies())
	if err != nil {
		t.Fatal(err)
	}
	bundles := make(map[string]WorkBundle, len(plan.Commands))
	for _, command := range plan.Commands {
		bundle := validFixtureWorkBundle(t, plan, registry, command.WorkUnitID)
		if err := bundle.Validate(plan); err != nil {
			t.Fatalf("worker bundle %s: %v", command.WorkUnitID, err)
		}
		bundles[command.WorkUnitID] = bundle
	}
	if err := validateCompleteBundleSet(plan, bundles, ""); err != nil {
		t.Fatal(err)
	}
	report, err := aggregateBundleSet(plan, manifest, registry, bundles, "run-aggregate")
	if err != nil {
		t.Fatalf("aggregateBundleSet() error = %v", err)
	}
	if len(report.Subgates) != len(plan.Commands) || len(report.Observations) != len(registry.Registrations) {
		t.Fatalf("aggregate report has %d subgates and %d observations", len(report.Subgates), len(report.Observations))
	}
	telemetry, err := buildDistributedTelemetry(plan, bundles, "run-aggregate")
	if err != nil {
		t.Fatalf("buildDistributedTelemetry() error = %v", err)
	}
	wantMaximumCPU := 0
	for _, command := range plan.Commands {
		wantMaximumCPU += command.ReservedResources.CPUUnits
	}
	if len(telemetry.Subgates) != len(plan.Commands) || telemetry.MaxReservedCPUUnits != wantMaximumCPU {
		t.Fatalf("distributed telemetry = %+v", telemetry)
	}
}

func validWorkBundle(t *testing.T, plan ExecutionPlan, workUnitID string) WorkBundle {
	t.Helper()
	command, expected, _, err := distributedBindings(plan, workUnitID)
	if err != nil {
		t.Fatal(err)
	}
	binding := soundnessevidence.ObservationBinding{
		RunID: "run-worker", WorkspaceDigest: plan.Workspace.Digest,
		ManifestDigest: plan.Manifest.Digest, CommandDigest: command.CommandDigest, SubgateID: command.SubgateID,
	}
	populations := make([]Population, 0, len(expected.RequiredPopulations))
	for _, requirement := range expected.RequiredPopulations {
		populations = append(populations, Population{ID: requirement.ID, Count: requirement.Minimum})
	}
	report := Report{FormatVersion: ReportFormatVersion, Binding: binding, Status: StatusPassed, Populations: populations}
	reportDigest, err := canonicalReportDigest(report)
	if err != nil {
		t.Fatal(err)
	}
	queuedAt := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	bundle, err := normalizeWorkBundle(WorkBundle{
		FormatVersion: WorkBundleFormatVersion, PlanID: plan.PlanID, WorkUnitID: workUnitID,
		WorkspaceDigest: plan.Workspace.Digest, ManifestDigest: plan.Manifest.Digest,
		RegistryDigest: plan.Registry.Digest, ToolchainDigest: plan.Toolchain.Digest,
		CommandDigest: command.CommandDigest, BinaryDigest: command.BinaryDigest,
		TerminalStatus: distributedStatusPassed, Binding: binding,
		Report: report, ReportDigest: reportDigest, Observations: []soundnessevidence.SemanticObservation{},
		QueuedAt: queuedAt, StartedAt: queuedAt, FinishedAt: queuedAt.Add(time.Second),
		WallDurationNanoseconds: time.Second.Nanoseconds(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func validFixtureWorkBundle(
	t *testing.T,
	plan ExecutionPlan,
	registry soundnessevidence.Registry,
	workUnitID string,
) WorkBundle {
	t.Helper()
	command, expected, _, err := distributedBindings(plan, workUnitID)
	if err != nil {
		t.Fatal(err)
	}
	binding := soundnessevidence.ObservationBinding{
		RunID: "run-" + workUnitID, WorkspaceDigest: plan.Workspace.Digest,
		ManifestDigest: plan.Manifest.Digest, CommandDigest: command.CommandDigest, SubgateID: command.SubgateID,
	}
	populations := make([]Population, 0, len(expected.RequiredPopulations))
	for _, requirement := range expected.RequiredPopulations {
		populations = append(populations, Population{ID: requirement.ID, Count: requirement.Minimum})
	}
	observations := make([]soundnessevidence.SemanticObservation, 0, len(expected.RequiredRegistrationIDs))
	for _, registrationID := range expected.RequiredRegistrationIDs {
		for _, registration := range registry.Registrations {
			if registration.ID == registrationID {
				observations = append(observations, validGateObservation(registration, binding))
				break
			}
		}
	}
	report := Report{FormatVersion: ReportFormatVersion, Binding: binding, Status: StatusPassed, Populations: populations}
	reportDigest, err := canonicalReportDigest(report)
	if err != nil {
		t.Fatal(err)
	}
	queuedAt := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	bundle, err := normalizeWorkBundle(WorkBundle{
		FormatVersion: WorkBundleFormatVersion, PlanID: plan.PlanID, WorkUnitID: workUnitID,
		WorkspaceDigest: plan.Workspace.Digest, ManifestDigest: plan.Manifest.Digest,
		RegistryDigest: plan.Registry.Digest, ToolchainDigest: plan.Toolchain.Digest,
		CommandDigest: command.CommandDigest, BinaryDigest: command.BinaryDigest,
		TerminalStatus: distributedStatusPassed, Binding: binding,
		Report: report, ReportDigest: reportDigest, Observations: observations,
		QueuedAt: queuedAt, StartedAt: queuedAt, FinishedAt: queuedAt.Add(time.Second),
		WallDurationNanoseconds: time.Second.Nanoseconds(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}
