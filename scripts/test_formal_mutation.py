#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Tests for formal_mutation.py: every fail-closed path must fail, not pass."""

from __future__ import annotations

import datetime
import importlib.util
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.dont_write_bytecode = True
SCRIPT = Path(__file__).with_name("formal_mutation.py")
SPEC = importlib.util.spec_from_file_location("formal_mutation", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
fm = importlib.util.module_from_spec(SPEC)
sys.modules["formal_mutation"] = fm
SPEC.loader.exec_module(fm)
formal = fm.formal

PlanError = fm.PlanError

CODE = """package a

// Decide is bound.
func Decide(x int) bool {
	if x > 0 {
		return true
	}
	return false
}

func (s *Server) Stop() error {
	return nil
}

func (s Server) Validate() error { return nil }

func Map[T any](xs []T) []T {
	return xs
}

type Kind int
"""

OTHER = """package b

func Validate() error {
	return nil
}

func Helper() {}
"""

HEADER = "// | model element | Go symbol | file | binding | abstraction |\n// |---|---|---|---|---|\n"


def row(*cells: str) -> str:
    return "// | " + " | ".join(cells) + " |\n"


class Fixture(unittest.TestCase):
    """A throwaway repository with two packages and one Alloy model."""

    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        (self.root / "a").mkdir()
        (self.root / "b").mkdir()
        (self.root / "a" / "a.go").write_text(CODE)
        (self.root / "b" / "b.go").write_text(OTHER)
        (self.root / "a" / "a_formal_test.go").write_text(
            "package a\n\nfunc TestDecide(t *testing.T) {}\n\nfunc TestStop(t *testing.T) {}\n\n"
            "func TestDecide_FindingF3(t *testing.T) {}\n\nfunc TestReplay(t *testing.T) {}\n"
        )
        (self.root / "a" / "a_trace_test.go").write_text("package a\n\nfunc TestM_TraceHarness(t *testing.T) {}\n")
        (self.root / "b" / "b_golden_test.go").write_text(
            "package b_test\n\nfunc TestCrossPackage(t *testing.T) {}\n\nfunc TestValidate(t *testing.T) {}\n"
        )
        self.suites = [formal.TraceSuite(name="M", package="./a/")]

    def model(self, rows: str) -> formal.Model:
        (self.root / "m.als").write_text(HEADER + rows)
        return formal.Model(name="M", tool="alloy", file="m.als", commands=(), calibration="seeded")

    def plan(self, rows: str) -> dict:
        return fm.build_plan(
            [self.model(rows)], root=self.root, suites=self.suites,
            resolve_packages=lambda dirs: {d: f"example.com/{d}" for d in dirs},
        )

    BASE_ROWS = (
        row("decide", "Decide", "a/a.go", "TestDecide, TestCrossPackage, TestM_TraceHarness", "-")
        + row("stop", "Server.Stop", "a/a.go", "TestStop", "-")
        + row("finding", "Decide", "a/a.go", "TestDecide_FindingF3", "a named characterisation test")
        + row("replay", "Server.Stop", "a/a.go", "TestReplay", "characterisation: today's defect")
        + row("trace", "Map", "a/a.go", "TestM_TraceHarness", "-")
        + row("kind", "Kind", "a/a.go", "TestDecide", "-")
        + row("free", "Helper", "b/b.go", "-", "-")
        + row("policy", "-", "m.als", "-", "policy only")
        + row("validate", "Validate", "b/b.go", "TestValidate", "-")
    )


class LineRangeTests(unittest.TestCase):
    def test_declarations_and_ranges(self) -> None:
        decls = {d.symbol: (d.start, d.end) for d in fm.function_decls(CODE)}
        self.assertEqual(decls, {"Decide": (4, 9), "Server.Stop": (11, 13), "Server.Validate": (15, 15), "Map": (17, 19)})

    def test_unclosed_function_fails(self) -> None:
        with self.assertRaisesRegex(PlanError, "no closing brace"):
            fm.function_decls("package a\n\nfunc Open() {\n\treturn\n")

    def test_bare_symbol_names_a_unique_method(self) -> None:
        decls = fm.function_decls(CODE)
        self.assertEqual(fm.find_function(decls, "Stop").symbol, "Server.Stop")
        self.assertIsNone(fm.find_function(decls, "Other.Stop"))
        self.assertIsNone(fm.find_function(decls, "Kind"))

    def test_first_changed_line(self) -> None:
        original = ["a", "b", "c"]
        self.assertIsNone(fm.first_changed_line(original, list(original)))
        self.assertEqual(fm.first_changed_line(original, ["a", "x", "c"]), 2)
        self.assertEqual(fm.first_changed_line(original, ["a", "c"]), 2)
        self.assertEqual(fm.first_changed_line(original, ["a", "b", "c", "d"]), 3)


class PlanTests(Fixture):
    def test_golden_plan(self) -> None:
        plan = self.plan(self.BASE_ROWS)
        self.assertEqual(plan["files"], ["a/a.go", "b/b.go"])
        self.assertEqual(
            [(f["file"], f["symbol"], f["start"], f["end"], f["killers"], f["packages"], f["rows"]) for f in plan["functions"]],
            [
                ("a/a.go", "Decide", 4, 9, ["TestCrossPackage", "TestDecide"], ["example.com/a", "example.com/b"],
                 [{"model": "M", "element": "decide"}]),
                ("a/a.go", "Server.Stop", 11, 13, ["TestStop"], ["example.com/a"], [{"model": "M", "element": "stop"}]),
                ("b/b.go", "Validate", 3, 5, ["TestValidate"], ["example.com/b"], [{"model": "M", "element": "validate"}]),
            ],
        )
        # a/a.go declares Server.Validate, which the leaf of b/b.go's Validate
        # would select, so the two files run in separate invocations.
        # Target files plus the files declaring their killers.
        self.assertEqual(plan["inputs"], ["a/a.go", "a/a_formal_test.go", "b/b.go", "b/b_golden_test.go"])
        self.assertEqual(plan["groups"], [
            {"match": "^(Decide|Stop)$", "files": ["a/a.go"]},
            {"match": "^(Validate)$", "files": ["b/b.go"]},
        ])
        self.assertEqual(
            [(u["element"], u["reason"]) for u in plan["unbound"]],
            [("finding", "characterisation-only"), ("free", "no-binding"), ("kind", "type-symbol"),
             ("policy", "no-symbol"), ("replay", "characterisation-only"), ("trace", "trace-only")],
        )
        self.assertEqual(plan["preflight"], {})

    def test_harnesses_and_characterisation_tests_never_kill(self) -> None:
        plan = self.plan(self.BASE_ROWS)
        killers = {t for f in plan["functions"] for t in f["killers"]}
        self.assertFalse(killers & {"TestM_TraceHarness", "TestDecide_FindingF3", "TestReplay"})

    def test_plan_files(self) -> None:
        out = self.root / "out"
        fm.write_plan(self.plan(self.BASE_ROWS), out)
        self.assertEqual((out / "resolved-targets.txt").read_text(), "a/a.go\nb/b.go\n")
        self.assertIn("M\ttrace\tMap\ta/a.go\ttrace-only\n", (out / "unbound-rows.txt").read_text())
        self.assertEqual(fm.load_plan(out / fm.PLAN_FILE)["files"], ["a/a.go", "b/b.go"])

    def test_same_file_collision_fails(self) -> None:
        rows = self.BASE_ROWS + row("validate here", "Other.Validate", "a/a.go", "TestDecide", "-")
        (self.root / "a" / "a.go").write_text(CODE + "\nfunc (o Other) Validate() error {\n\treturn nil\n}\n")
        with self.assertRaisesRegex(PlanError, r"match collision: Server.Validate in a/a.go .* M: row 'validate here'"):
            self.plan(rows)

    def test_killer_in_no_package_fails(self) -> None:
        (self.root / "a" / "helpers_test.go").write_text("package a\n\nfunc helperCheck(t *testing.T) {}\n")
        with self.assertRaisesRegex(PlanError, r"M: row 'helper' \(Decide in a/a.go\): killer test helperCheck is declared in no package"):
            self.plan(self.BASE_ROWS + row("helper", "Decide", "a/a.go", "helperCheck", "-"))

    def test_killer_in_several_packages_fails(self) -> None:
        (self.root / "b" / "dup_test.go").write_text("package b\n\nfunc TestDecide(t *testing.T) {}\n")
        with self.assertRaisesRegex(PlanError, "killer test TestDecide is declared in several packages: a, b"):
            self.plan(self.BASE_ROWS)

    def test_empty_plan_fails(self) -> None:
        rows = row("trace", "Map", "a/a.go", "TestM_TraceHarness", "-") + row(
            "rest", "Decide", "a/a.go", "TestDecide, TestStop, TestDecide_FindingF3, TestReplay, TestCrossPackage, TestValidate",
            "characterisation: all replays",
        )
        with self.assertRaisesRegex(PlanError, "the plan has no function targets"):
            self.plan(rows)

    def test_correspondence_failure_fails(self) -> None:
        with self.assertRaisesRegex(PlanError, "correspondence check failed"):
            self.plan(row("gone", "Gone", "a/a.go", "TestDecide", "-"))

    def test_untabled_binding_test_fails(self) -> None:
        with self.assertRaisesRegex(PlanError, "binding test TestReplay is not named by any correspondence row"):
            self.plan(row("decide", "Decide", "a/a.go", "TestDecide, TestStop, TestDecide_FindingF3, TestCrossPackage, TestValidate", "-"))


class ExecArgsTests(Fixture):
    def setUp(self) -> None:
        super().setUp()
        self.plan_data = self.plan(self.BASE_ROWS)
        self.plan_data["root"] = str(self.root.resolve())
        self.original = self.root / "a" / "a.go"
        self.mutant = self.root / "mutant.go"
        self.plan_data["preflight"]["a/a.go"] = {"clean_seconds": 1.0, "timeout_seconds": 12, "overridden": False}

    def mutate(self, line: int, text: str) -> str:
        lines = CODE.splitlines()
        lines[line - 1] = text
        self.mutant.write_text("\n".join(lines) + "\n")
        return str(self.mutant)

    def test_mutant_runs_only_its_functions_killers(self) -> None:
        result = fm.exec_args(self.plan_data, str(self.original), self.mutate(5, "\tif x >= 0 {"))
        self.assertEqual(result["function"], "Decide")
        self.assertEqual(result["run"], "^(TestCrossPackage|TestDecide)$")
        self.assertEqual(result["packages"], ["example.com/a", "example.com/b"])
        self.assertEqual(result["timeout"], 12)
        self.assertFalse(result["preflight"])

    def test_other_function_in_the_same_file(self) -> None:
        result = fm.exec_args(self.plan_data, str(self.original), self.mutate(12, "\treturn errors.New(\"x\")"))
        self.assertEqual((result["function"], result["run"]), ("Server.Stop", "^(TestStop)$"))

    def test_line_outside_every_function_fails(self) -> None:
        with self.assertRaisesRegex(PlanError, "outside every planned function"):
            fm.exec_args(self.plan_data, str(self.original), self.mutate(18, "\treturn nil"))

    def test_unknown_file_fails(self) -> None:
        other = self.root / "a" / "unplanned.go"
        other.write_text("package a\n")
        with self.assertRaisesRegex(PlanError, "not a formal-bindings target file"):
            fm.exec_args(self.plan_data, str(other), str(other))

    def test_preflight_runs_every_function_of_the_file(self) -> None:
        result = fm.exec_args(self.plan_data, str(self.original), str(self.original))
        self.assertTrue(result["preflight"])
        self.assertEqual(result["run"], "^(TestCrossPackage|TestDecide|TestStop)$")
        self.assertEqual(result["timeout"], fm.PREFLIGHT_TIMEOUT)

    def test_timeout_needs_a_preflight_record(self) -> None:
        del self.plan_data["preflight"]["a/a.go"]
        with self.assertRaisesRegex(PlanError, "no pre-flight record"):
            fm.exec_args(self.plan_data, str(self.original), self.mutate(5, "\tif x >= 0 {"))

    def test_timeout_override(self) -> None:
        with mock.patch.dict(os.environ, {fm.TIMEOUT_OVERRIDE_ENV: "33"}):
            self.assertEqual(fm.exec_args(self.plan_data, str(self.original), self.mutate(5, "\tif x >= 0 {"))["timeout"], 33)
        with mock.patch.dict(os.environ, {fm.TIMEOUT_OVERRIDE_ENV: "soon"}), self.assertRaisesRegex(PlanError, "positive number"):
            fm.exec_args(self.plan_data, str(self.original), self.mutate(5, "\tif x >= 0 {"))

    def test_overlay_maps_the_original_to_the_mutant(self) -> None:
        out = self.root / "overlay.json"
        fm.write_overlay(str(self.original), str(self.mutant), out)
        self.assertEqual(json.loads(out.read_text()), {"Replace": {str(self.original.resolve()): str(self.mutant.resolve())}})


def go_json(*events: tuple[str, str]) -> str:
    lines = ['{"Action":"start","Package":"p"}', "# build noise"]
    lines += [json.dumps({"Action": action, "Test": test}) for test, action in events]
    lines += [json.dumps({"Action": "pass", "Test": "TestDecide/sub"})]
    return "\n".join(lines) + "\n"


class PreflightTests(Fixture):
    def setUp(self) -> None:
        super().setUp()
        self.plan_data = self.plan(self.BASE_ROWS)

    def record(self, output: str, status: int = 1, seconds: float = 1.0) -> dict:
        return fm.preflight_record(self.plan_data, "a/a.go", output, status, seconds)

    def test_passing_preflight_records_the_timeout(self) -> None:
        with mock.patch.dict(os.environ, {fm.TIMEOUT_OVERRIDE_ENV: ""}):
            record = self.record(go_json(("TestDecide", "pass"), ("TestCrossPackage", "pass"), ("TestStop", "pass")), seconds=3.2)
        self.assertEqual(record, {"clean_seconds": 3.2, "timeout_seconds": 16, "overridden": False})
        self.assertEqual(self.plan_data["preflight"]["a/a.go"], record)
        with mock.patch.dict(os.environ, {fm.TIMEOUT_OVERRIDE_ENV: "soon"}), self.assertRaisesRegex(PlanError, "positive number"):
            self.record(go_json(("TestDecide", "pass"), ("TestCrossPackage", "pass"), ("TestStop", "pass")))
        self.assertEqual(fm.derive_timeout(0.4), fm.MIN_EXEC_TIMEOUT)

    def test_skipped_failed_or_absent_killers_fail(self) -> None:
        for outcome in ("skip", "fail"):
            with self.subTest(outcome=outcome), self.assertRaisesRegex(PlanError, rf"Server.Stop .*killer TestStop {outcome} on clean code"):
                self.record(go_json(("TestDecide", "pass"), ("TestCrossPackage", "pass"), ("TestStop", outcome)))
        with self.assertRaisesRegex(PlanError, "killer TestStop not run on clean code"):
            self.record(go_json(("TestDecide", "pass"), ("TestCrossPackage", "pass")))

    def test_executor_status_must_be_escaped(self) -> None:
        with self.assertRaisesRegex(PlanError, "exited 3, expected 1"):
            self.record(go_json(("TestDecide", "pass"), ("TestCrossPackage", "pass"), ("TestStop", "pass")), status=3)

    def test_max_timeout_is_the_largest_record(self) -> None:
        self.record(go_json(("TestDecide", "pass"), ("TestCrossPackage", "pass"), ("TestStop", "pass")))
        self.assertEqual(fm.max_timeout(self.plan_data), fm.MIN_EXEC_TIMEOUT)
        self.plan_data["preflight"]["b/b.go"] = {"clean_seconds": 9, "timeout_seconds": 45, "overridden": False}
        self.assertEqual(fm.max_timeout(self.plan_data), 45)


class RerunScopeTests(Fixture):
    def test_ids_are_scoped_to_their_file(self) -> None:
        plan = self.plan(self.BASE_ROWS)
        hint = self.root / "baseline.json"
        hint.write_text(json.dumps({"mutants": [{"id": "x1", "file": "b/b.go"}, {"id": "x2", "file": "gone.go"}]}))
        scope = fm.rerun_scope(plan, ["x1", "x2", "x3"], [hint, self.root / "absent.json"])
        self.assertEqual(scope, [("x1", "1", "b/b.go"), ("x2", "-", "-"), ("x3", "-", "-")])

    def test_max_timeout_over_recorded_files(self) -> None:
        plan = self.plan(self.BASE_ROWS)
        with self.assertRaisesRegex(PlanError, "no pre-flight record"):
            fm.max_timeout(plan)
        plan["preflight"]["b/b.go"] = {"clean_seconds": 1, "timeout_seconds": 14, "overridden": False}
        self.assertEqual(fm.max_timeout(plan), 14)


class RerunAndReportTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.dir = Path(self.tmp.name)

    def test_summary_status(self) -> None:
        self.assertEqual(fm.summary_status({"killedCount": 0, "escapedCount": 1}), "escaped")
        self.assertEqual(fm.summary_status({"killedCount": 1}), "killed")
        self.assertEqual(fm.summary_status({"escapedCount": 2}), "escaped")  # one id on two lines
        self.assertEqual(fm.summary_status({"killedCount": 1, "escapedCount": 1}), "escaped")
        for bad in ({}, {"killedCount": 1, "skippedCount": 1}):
            with self.subTest(bad=bad), self.assertRaisesRegex(PlanError, "with one status"):
                fm.summary_status(bad)

    def test_append_rerun(self) -> None:
        path = self.dir / "reruns.jsonl"
        now = datetime.datetime(2026, 9, 25, 12, 0, tzinfo=datetime.UTC)
        fm.append_rerun(path, "abc", "escaped", "d1", "c0ffee", now)
        fm.append_rerun(path, "abc", "killed", "d1", now=now)
        self.assertEqual(
            fm.read_reruns(path),
            [{"commit": "c0ffee", "digest": "d1", "id": "abc", "status": "escaped", "timestamp": "2026-09-25T12:00:00Z"},
             {"digest": "d1", "id": "abc", "status": "killed", "timestamp": "2026-09-25T12:00:00Z"}],
        )
        for bad in ('{"id": "abc", "status": "maybe", "timestamp": "t", "digest": "d"}',
                    '{"id": "abc", "status": "escaped", "timestamp": "t", "commit": "c"}'):
            path.write_text(bad + "\n")
            with self.subTest(bad=bad), self.assertRaisesRegex(PlanError, "malformed rerun record"):
                fm.read_reruns(path)

    def test_inputs_digest(self) -> None:
        files = {"b.go": b"B", "a.go": b"A"}
        digest = fm.inputs_digest(files, files.__getitem__)
        self.assertEqual(digest, fm.hashlib.sha256(b"a.go\0Ab.go\0B").hexdigest())
        self.assertNotEqual(digest, fm.inputs_digest(files, {"a.go": b"A", "b.go": b"C"}.__getitem__))
        (self.dir / "a.go").write_bytes(b"A")
        (self.dir / "b.go").write_bytes(b"B")
        self.assertEqual(fm.plan_digest({"inputs": ["b.go", "a.go"], "root": str(self.dir)}), digest)

    def write_group(self, n: int, stats: dict, escaped: list[dict], baseline: list[dict]) -> None:
        group = self.dir / f"group-{n}"
        group.mkdir()
        (group / "go-mutesting-summary.json").write_text(json.dumps(stats))
        (group / "go-mutesting-agentic.json").write_text(json.dumps({"mutants": escaped}))
        mutant = {"mutator": {"originalFilePath": f"f{n}.go"}}
        (group / "report.json").write_text(json.dumps({"killed": [mutant] * stats["killedCount"], "escaped": [mutant] * stats["escapedCount"]}))
        (group / "baseline.json").write_text(json.dumps({"version": 1, "mutants": baseline or None}))

    def test_merge_reports_and_baselines(self) -> None:
        self.write_group(0, {"killedCount": 3, "escapedCount": 1, "errorCount": 0, "skippedCount": 0}, [{"id": "e1"}],
                         [{"id": "e1", "file": "f0.go", "mutator": "m", "line": 9}])
        self.write_group(1, {"killedCount": 1, "escapedCount": 0, "errorCount": 0, "skippedCount": 1}, [], [])
        stats = fm.merge_reports(self.dir, "formal-mutation: timeout-kill file=f0.go function=F\n")
        self.assertEqual((self.dir / "run-metadata.txt").read_text(), "timeout_kills[f0.go]=1\n")
        self.assertEqual((stats["totalMutantsCount"], stats["killedCount"], stats["msi"]), (6, 4, 83.33))
        self.assertEqual(json.loads((self.dir / "go-mutesting-agentic.json").read_text())["mutants"], [{"id": "e1"}])
        self.assertEqual(
            (self.dir / "per-file.tsv").read_text(),
            "file\tkilled\tescaped\tskipped\terrored\ttimeout-kill\nf0.go\t3\t1\t0\t0\t1\nf1.go\t1\t0\t0\t0\t0\n",
        )
        out = self.dir / "baseline.json"
        self.assertEqual(fm.merge_baselines(self.dir, out, "d1"), 1)
        self.assertEqual(json.loads(out.read_text())["digest"], "d1")

    def test_merge_without_groups_fails(self) -> None:
        with self.assertRaisesRegex(PlanError, "no group-"):
            fm.merge_reports(self.dir, "")


MODEL_ROWS = (
    row("evaluation", "Evaluate", "a/a.go", "TestDecide", 'content hashing   abstracted: a single "compared" entry')
    + row("other", "Decide", "a/a.go", "TestDecide", "-")
)


class TriageTests(Fixture):
    def setUp(self) -> None:
        super().setUp()
        (self.root / "a" / "a.go").write_text(CODE + "\nfunc Evaluate() {\n}\n")
        self.models = [self.model(MODEL_ROWS)]
        self.models[0] = formal.Model(name="M", tool="alloy", file="m.als", calibration="seeded; mutation closed1: x",
                                      commands=(formal.Command("f", "counterexample", finding="F18"),))

    def survivor(self, **overrides: str) -> dict:
        entry = {"id": "s1", "file": "a/a.go", "symbol": "Evaluate", "model": "M", "element": "evaluation",
                 "class": "abstraction", "reason": "a single \"compared\"", "follow_up": ""}
        entry.update(overrides)
        return entry

    def check(self, survivors: list[dict], baseline_ids: list[str] | None = None, reruns: list[dict] | None = None,
              closed: list[dict] | None = None, digest: str = "base") -> list[str]:
        ids = [s["id"] for s in survivors] if baseline_ids is None else baseline_ids
        baseline = {"version": 1, "digest": digest, "mutants": [{"id": i, "file": "a/a.go", "mutator": "m", "line": 1} for i in ids]}
        if reruns is None:
            reruns = [{"id": i, "status": "escaped", "timestamp": "t", "digest": "base"} for i in ids for _ in range(2)]
        ledger = {"survivor": survivors, "closed": closed or []}
        return fm.check_triage(baseline, ledger, reruns, self.models, root=self.root)

    def test_valid_ledger_passes(self) -> None:
        self.assertEqual(self.check([self.survivor()], closed=[{"id": "closed1", "model": "M"}]), [])
        self.assertEqual(self.check([self.survivor(id="d", **{"class": "defect", "follow_up": "F18", "reason": "r"})]), [])

    def test_id_sets_must_match(self) -> None:
        failures = self.check([self.survivor()], baseline_ids=["other"])
        self.assertIn("baseline id other has no [[survivor]] entry in the ledger", failures)
        self.assertIn("ledger survivor s1 is not in the baseline", failures)

    def test_gaps_need_a_follow_up(self) -> None:
        for cls in ("binding-gap", "model-gap", "defect"):
            with self.subTest(cls=cls):
                self.assertIn(f"survivor s1: class {cls} needs a follow_up", self.check([self.survivor(**{"class": cls})]))
        self.assertEqual(self.check([self.survivor(**{"class": "binding-gap", "follow_up": "task 9.9"})]), [])

    def test_defect_follow_up_must_be_a_manifest_finding(self) -> None:
        failures = self.check([self.survivor(**{"class": "defect", "follow_up": "F99"})])
        self.assertIn("survivor s1: follow_up 'F99' is not a `finding` command in the manifest", failures)

    def test_abstraction_must_be_stated_in_the_row(self) -> None:
        self.assertEqual(self.check([self.survivor(reason="content  hashing abstracted")]), [])
        failures = self.check([self.survivor(reason="not in the cell")])
        self.assertIn("is not stated in the abstraction cell of M row 'evaluation'", failures[0])

    def test_row_must_exist_and_match_the_file(self) -> None:
        self.assertIn("survivor s1: no M row 'nope' names Evaluate", self.check([self.survivor(element="nope")]))
        self.assertIn("survivor s1: Evaluate is tabled in a/a.go, not b/b.go", self.check([self.survivor(file="b/b.go")]))

    def test_unknown_class_and_fields(self) -> None:
        self.assertIn("survivor s1: unknown class 'maybe'", self.check([self.survivor(**{"class": "maybe"})]))
        bad = self.survivor()
        bad["confirmed_reruns"] = "2"
        self.assertIn("fields must be", self.check([bad])[0])

    def test_reruns_are_required(self) -> None:
        one = [{"id": "s1", "status": "escaped", "timestamp": "t", "digest": "base"}]
        self.assertIn("baseline id s1: 1 recorded escaped rerun(s)", self.check([self.survivor()], reruns=one)[0])
        stale = [{"id": "s1", "status": "escaped", "timestamp": "t", "digest": "older inputs"}] * 2
        self.assertIn("baseline id s1: 0 recorded escaped rerun(s)", self.check([self.survivor()], reruns=stale)[0])
        flaky = one * 2 + [{"id": "s1", "status": "killed", "timestamp": "t", "digest": "base"}]
        self.assertIn("baseline id s1: inconsistent rerun evidence (killed as well as escaped)", self.check([self.survivor()], reruns=flaky))

    def test_baseline_needs_an_input_digest(self) -> None:
        self.assertIn("baseline: no input digest", self.check([self.survivor()], digest="")[0])

    def test_closed_ids_must_be_calibrated(self) -> None:
        failures = self.check([], closed=[{"id": "closed2", "model": "M"}, {"id": "x", "model": "Nope"}])
        self.assertIn("closed closed2: not recorded in M's calibration text", failures)
        self.assertIn("closed x: unknown model 'Nope'", failures)
        failures = self.check([self.survivor(id="closed1")], closed=[{"id": "closed1", "model": "M"}])
        self.assertIn("closed closed1: still in the baseline", failures)


class RepositoryTests(unittest.TestCase):
    def test_committed_ledger_passes(self) -> None:
        self.assertEqual(fm.main(["triage", "--check"]), 0)

    def test_repository_plan_generates(self) -> None:
        _tools, models = formal.load_manifest()
        plan = fm.build_plan(models, resolve_packages=lambda dirs: {d: d for d in dirs})
        self.assertTrue(plan["functions"])
        killers = {t for f in plan["functions"] for t in f["killers"]}
        self.assertFalse({t for t in killers if formal.is_trace_harness(t) or formal.CHARACTERISATION_NAME_RE.match(t)})


if __name__ == "__main__":
    unittest.main()
