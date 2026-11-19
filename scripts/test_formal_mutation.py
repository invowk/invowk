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
        # Declare every row's element so the correspondence element check holds.
        elements = {cells[0].split()[0] for cells in formal.correspondence_rows(HEADER + rows)}
        (self.root / "m.als").write_text(HEADER + rows + "".join(f"pred {e} {{}}\n" for e in sorted(elements)))
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
                 [{"model": "M", "element": "decide", "killers": ["TestCrossPackage", "TestDecide"]}]),
                ("a/a.go", "Server.Stop", 11, 13, ["TestStop"], ["example.com/a"],
                 [{"model": "M", "element": "stop", "killers": ["TestStop"]}]),
                ("b/b.go", "Validate", 3, 5, ["TestValidate"], ["example.com/b"],
                 [{"model": "M", "element": "validate", "killers": ["TestValidate"]}]),
            ],
        )
        self.assertEqual([f["package"] for f in plan["functions"]], ["example.com/a", "example.com/a", "example.com/b"])
        self.assertEqual(plan["killer_packages"], {"TestCrossPackage": "example.com/b", "TestDecide": "example.com/a",
                                                   "TestStop": "example.com/a", "TestValidate": "example.com/b"})
        self.assertEqual(plan["version"], 2)
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

    def test_rows_keep_their_own_killers(self) -> None:
        rows = self.BASE_ROWS + row("decide again", "Decide", "a/a.go", "TestStop", "-")
        decide = next(f for f in self.plan(rows)["functions"] if f["symbol"] == "Decide")
        self.assertEqual(decide["killers"], ["TestCrossPackage", "TestDecide", "TestStop"])
        self.assertEqual([(r["element"], r["killers"]) for r in decide["rows"]],
                         [("decide", ["TestCrossPackage", "TestDecide"]), ("decide again", ["TestStop"])])

    def test_plan_version_1_is_rejected(self) -> None:
        path = self.root / "old.json"
        path.write_text(json.dumps({"version": 1}))
        with self.assertRaisesRegex(PlanError, "unsupported plan version 1"):
            fm.load_plan(path)

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

    def test_killer_that_is_not_a_test_fails(self) -> None:
        # A helper is not a Test function, so the shared test index (the one
        # the replay plan uses) rejects it before any package is resolved.
        (self.root / "a" / "helpers_test.go").write_text("package a\n\nfunc helperCheck(t *testing.T) {}\n")
        with self.assertRaisesRegex(PlanError, r"M: row 'helper' names binding test helperCheck that does not exist"):
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
        # A full regeneration writes only the top-level digest (design D3).
        (self.dir / "group-0" / "baseline.json").write_text(json.dumps(
            {"version": 1, "mutants": [{"id": "e1", "file": "f0.go", "mutator": "m", "line": 9, "digest": "own"}]}))
        fm.merge_baselines(self.dir, out, "d2")
        self.assertEqual(json.loads(out.read_text()),
                         {"version": 1, "digest": "d2", "mutants": [{"id": "e1", "file": "f0.go", "mutator": "m", "line": 9}]})

    def test_merge_without_groups_fails(self) -> None:
        with self.assertRaisesRegex(PlanError, "no group-"):
            fm.merge_reports(self.dir, "")


def cover(*blocks: tuple[str, int, int, int, int], mode: str = "mode: set") -> str:
    """A coverage profile: (file, start line, end line, statements, count)."""
    return "\n".join([mode] + [f"{f}:{a}.1,{b}.2 {n} {c}" for f, a, b, n, c in blocks]) + "\n"


def passing(test: str) -> str:
    return json.dumps({"Action": "pass", "Test": test}) + "\n"


class ReachTests(Fixture):
    """The binding reach gate over fixture profiles and a stub runner."""

    def setUp(self) -> None:
        super().setUp()
        self.plan_data = self.plan(self.BASE_ROWS)
        self.out = self.root / "reach"
        # Decide spans lines 4-9 of a/a.go, Server.Stop 11-13, Validate 3-5 of b/b.go.
        self.profiles = {
            "TestDecide": cover(("example.com/a/a.go", 4, 9, 3, 1)),
            "TestCrossPackage": cover(("example.com/a/a.go", 5, 7, 1, 1), ("example.com/a/a.go", 4, 9, 2, 0)),
            "TestStop": cover(("example.com/a/a.go", 11, 13, 1, 1)),
            "TestValidate": cover(("example.com/b/b.go", 3, 5, 1, 1)),
        }
        self.runs: dict[str, fm.KillerRun] = {}

    def runner(self, plan: dict, killer: str, coverpkg: list[str], out: Path, env: dict[str, str]) -> fm.KillerRun:
        self.coverpkg = getattr(self, "coverpkg", {})
        self.coverpkg[killer] = coverpkg
        self.env = env
        if killer in self.runs:
            return self.runs[killer]
        return fm.KillerRun(0, passing(killer), self.profiles.get(killer), 0.1)

    def gate(self) -> tuple[list[dict], list[str]]:
        triples, failures, _seconds = fm.reach(self.plan_data, self.out, runner=self.runner, env={"RAPID_SEED": "1"})
        return triples, failures

    def test_reached(self) -> None:
        triples, failures = self.gate()
        self.assertEqual(failures, [])
        self.assertEqual(sorted((t["element"], t["killer"], t["covered"], t["total"]) for t in triples), [
            ("decide", "TestCrossPackage", 1, 3), ("decide", "TestDecide", 3, 3),
            ("stop", "TestStop", 1, 1), ("validate", "TestValidate", 1, 1),
        ])
        self.assertEqual(self.coverpkg["TestCrossPackage"], ["example.com/a"])
        self.assertEqual(self.env, {"RAPID_SEED": "1"})
        report = json.loads((self.out / "reach.json").read_text())
        self.assertEqual(len(report["triples"]), 4)
        # Every unbound row is listed with its reason and abstraction cell, never gated.
        self.assertEqual(sorted({u["reason"] for u in report["unbound"]}),
                         ["characterisation-only", "no-binding", "no-symbol", "trace-only", "type-symbol"])
        self.assertIn("unbound\tM: replay\tcharacterisation-only\tServer.Stop\tcharacterisation: today's defect",
                      (self.out / "reach.txt").read_text())

    def test_zero_covered(self) -> None:
        self.profiles["TestStop"] = cover(("example.com/a/a.go", 11, 13, 4, 0))
        self.assertEqual(self.gate()[1], [
            "M: row 'stop': killer TestStop -> Server.Stop (a/a.go): executed 0 of 4 statements"])

    def test_file_not_instrumented(self) -> None:
        self.profiles["TestStop"] = cover(("example.com/b/b.go", 3, 5, 1, 1))
        self.assertIn("not instrumented", self.gate()[1][0])

    def test_empty_range(self) -> None:
        self.profiles["TestStop"] = cover(("example.com/a/a.go", 4, 9, 1, 1))
        self.assertEqual(self.gate()[1], [
            "M: row 'stop': killer TestStop -> Server.Stop (a/a.go): no coverage blocks in lines 11-13"])

    def test_missing_and_garbled_profiles(self) -> None:
        del self.profiles["TestStop"]
        self.assertEqual(self.gate()[1], ["killer TestStop (M: row 'stop'): wrote no coverage profile"])
        self.profiles["TestStop"] = "not a profile\n"
        self.assertIn("killer TestStop (M: row 'stop'): garbled coverage profile: the first line", self.gate()[1][0])
        self.profiles["TestStop"] = cover(("example.com/a/a.go", 11, 13, 1, 1)) + "junk line\n"
        self.assertIn("garbled coverage profile: line 3 'junk line'", self.gate()[1][0])

    def test_killer_fail_skip_and_not_run(self) -> None:
        for action in ("fail", "skip"):
            with self.subTest(action=action):
                self.runs["TestStop"] = fm.KillerRun(1, json.dumps({"Action": action, "Test": "TestStop"}) + "\n",
                                                     self.profiles["TestStop"], 0.1)
                self.assertEqual(self.gate()[1], [f"killer TestStop (M: row 'stop'): {action} (exit 1)"])
        self.runs["TestStop"] = fm.KillerRun(1, "# build failed\n", None, 0.1)
        self.assertEqual(self.gate()[1], ["killer TestStop (M: row 'stop'): not run (exit 1)"])

    def test_nonzero_exit_with_passing_json(self) -> None:
        self.runs["TestStop"] = fm.KillerRun(2, passing("TestStop"), self.profiles["TestStop"], 0.1)
        self.assertEqual(self.gate()[1], [
            "killer TestStop (M: row 'stop'): exited 2 although the test passed (build or package failure)"])

    def test_per_row_killers_do_not_cover_each_other(self) -> None:
        # Row "decide again" binds only TestStop, which never reaches Decide,
        # while row "decide"'s killers do: the function-level union would pass.
        self.plan_data = self.plan(self.BASE_ROWS + row("decide again", "Decide", "a/a.go", "TestStop", "-"))
        self.assertEqual(self.gate()[1], [
            "M: row 'decide again': killer TestStop -> Decide (a/a.go): no coverage blocks in lines 4-9"])

    def test_plan_failure_stops_before_any_killer(self) -> None:
        ran = []
        with mock.patch.object(fm, "run_killer", side_effect=lambda *a: ran.append(a)), \
                mock.patch.object(fm, "build_plan", side_effect=PlanError("correspondence check failed")):
            self.assertEqual(fm.main(["reach", "--out", str(self.out)]), 1)
        self.assertEqual(ran, [])

    def test_duplicate_blocks_keep_the_highest_count(self) -> None:
        blocks = fm.parse_profile(cover(("x/a.go", 1, 2, 3, 0), ("x/a.go", 1, 2, 3, 1)))
        self.assertEqual(blocks, {"x/a.go": [fm.Block(1, 2, 3, 1)]})

    def test_stub_go_command(self) -> None:
        # run_killer with a stub GO_CMD: it passes the coverage flags and the
        # pinned rapid environment, and reads back the profile it wrote.
        stub = self.root / "go"
        stub.write_text(
            "#!/bin/sh\n"
            "for arg; do case $arg in -coverprofile=*) out=${arg#-coverprofile=};; esac; done\n"
            "printf 'mode: set\\nexample.com/a/a.go:11.1,13.2 1 1\\n' > \"$out\"\n"
            "echo \"$*\" > \"$out.args\"; echo \"$RAPID_SEED $RAPID_NOFAILFILE\" >> \"$out.args\"\n"
            "printf '{\"Action\":\"pass\",\"Test\":\"TestStop\"}\\n'\n"
        )
        stub.chmod(0o755)
        self.out.mkdir()
        with mock.patch.dict(os.environ, {"GO_CMD": str(stub)}):
            run = fm.run_killer(self.plan_data, "TestStop", ["example.com/a"], self.out, {"RAPID_SEED": "7", "RAPID_NOFAILFILE": "1"})
        self.assertEqual(run.status, 0)
        self.assertEqual(fm.test_outcomes(run.output), {"TestStop": "pass"})
        self.assertTrue(run.profile.startswith("mode: set"))
        args = (self.out / "TestStop.cover.args").read_text()
        self.assertIn("-count=1 -json -covermode=set -coverpkg=example.com/a", args)
        self.assertIn("-run ^TestStop$ example.com/a", args)
        self.assertIn("7 1", args)

    def test_rapid_env_follows_the_mutation_profile(self) -> None:
        script = self.root / "mutation.sh"
        script.write_text("readonly MUTATION_RAPID_SEED=42\nreadonly MUTATION_RAPID_SHRINKTIME=2s\n")
        self.assertEqual(fm.rapid_env(script, {}), {"RAPID_SEED": "42", "RAPID_SHRINKTIME": "2s", "RAPID_NOFAILFILE": "1"})
        self.assertEqual(fm.rapid_env(script, {"RAPID_SEED": "9"})["RAPID_SEED"], "9")
        script.write_text("readonly MUTATION_RAPID_SEED=42\n")
        with self.assertRaisesRegex(PlanError, "MUTATION_RAPID_SHRINKTIME"):
            fm.rapid_env(script, {})
        # The repository's pins are found.
        self.assertEqual(set(fm.rapid_env(environ={})), {"RAPID_SEED", "RAPID_SHRINKTIME", "RAPID_NOFAILFILE"})


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
        # These fixtures carry no anchor; AnchorTests covers the staleness
        # check, so they are listed as stale to keep each test on its condition.
        triage = {"stale_survivors": [s["id"] for s in survivors if "removed" not in s]}
        return fm.check_triage(baseline, ledger, reruns, self.models, root=self.root, triage=triage)

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


ANCHORED = """
func Evaluate(x int) int {
	if x > 0 {
		x++
	}
	return x
}

// Evaluate's trailing comment.
"""


class AnchorTests(Fixture):
    """The per-entry staleness check (refresh-anchors-mutation-ledger design D1)."""

    ROWS = row("evaluation", "Evaluate", "a/a.go", "TestDecide", "-") + row("free", "Helper", "b/b.go", "-", "-")
    GAPS = [{"model": "M", "element": "free", "symbol": "Helper"}]

    def setUp(self) -> None:
        super().setUp()
        self.write(ANCHORED)
        self.models = [self.model(self.ROWS)]

    def write(self, tail: str) -> None:
        (self.root / "a" / "a.go").write_text(CODE + tail)

    def entry(self, removed: str | None, added: str | None, mutator: str = "m", **overrides) -> dict:
        entry = {"file": "a/a.go", "symbol": "Evaluate", "model": "M", "element": "evaluation", "class": "binding-gap",
                 "reason": "r", "follow_up": "survivor triage", "mutator": mutator}
        if removed is not None:
            entry["removed"] = removed
        if added is not None:
            entry["added"] = added
        entry["id"] = fm.mutant_id("a/a.go", mutator, removed or "", added or "")
        entry.update(overrides)
        return entry

    def check(self, entries: list[dict], stale: list[str] | None = None, orphans: list[str] | None = None,
              baseline: dict | None = None, reruns: list[dict] | None = None) -> tuple[list[str], list[str]]:
        survivors = [{k: v for k, v in e.items() if k != "mutator"} for e in entries]
        if baseline is None:
            baseline = {"version": 1, "digest": "d", "mutants": [
                {"id": e["id"], "file": e["file"], "mutator": e["mutator"], "line": 1} for e in entries]}
        if reruns is None:
            reruns = [{"id": m["id"], "status": "escaped", "timestamp": "t", "digest": m.get("digest", "d")}
                      for m in baseline["mutants"] for _ in range(2)]
        triage = {"binding_gaps": self.GAPS, "orphaned_survivors": orphans or [], "stale_survivors": stale or []}
        report: list[str] = []
        failures = fm.check_triage(baseline, {"survivor": survivors}, reruns, self.models, root=self.root,
                                   triage=triage, report=report)
        return failures, [line for line in report if line.startswith("stale survivor ")]

    def test_mutant_id_matches_go_mutesting(self) -> None:
        # go-mutesting v2.8.3 MutantID of a real formal-bindings survivor.
        self.assertEqual(fm.mutant_id("internal/app/deps/deps.go", "conditional/negated",
                                      "\tif cmdInfo.ModuleID != nil {\n", "\tif cmdInfo.ModuleID == nil {\n"),
                         "e9702c74714610e1edb5145d5f921fa7")

    def test_matching_entry_passes_and_bad_hash_fails(self) -> None:
        good = self.entry("\tif x > 0 {\n", "\tif x <= 0 {\n")
        self.assertEqual(self.check([good]), ([], []))
        bad = {**good, "id": "0" * 32}
        failures, _ = self.check([bad])
        self.assertEqual(failures, [f"survivor {'0' * 32} (Evaluate): anchor mismatch: removed and added do not hash "
                                    "to the id with a/a.go and mutator m"])

    def test_missing_symbol_fails(self) -> None:
        good = self.entry("\tif x > 0 {\n", "\tif x <= 0 {\n")
        self.write(ANCHORED.replace("Evaluate(", "Assess("))
        failures, _ = self.check([good])
        self.assertIn(f"survivor {good['id']} (Evaluate): symbol missing: Evaluate no longer names a function in a/a.go",
                      failures)

    def test_removed_lines_that_are_gone_fail(self) -> None:
        good = self.entry("\tif x > 0 {\n", "\tif x <= 0 {\n")
        self.write(ANCHORED.replace("if x > 0 {", "if x > 1 {"))
        failures, _ = self.check([good])
        self.assertEqual(failures, [f"survivor {good['id']} (Evaluate): mutated code gone: the anchor lines no longer "
                                    "appear in order from inside Evaluate (a/a.go:23-28)"])

    def test_edits_elsewhere_pass(self) -> None:
        # Lines move, the indentation changes, and other body lines change.
        good = self.entry("\tif x > 0 {\n", "\tif x <= 0 {\n")
        self.write("\nfunc Before() {}\n" + ANCHORED.replace("return x", "return x * 2").replace("\tif x", "  if x"))
        self.assertEqual(self.check([good]), ([], []))

    def test_insertion_only_entry_is_anchored_by_context(self) -> None:
        inserted = self.entry("", "\t\tbreak\n", mutator="loop/range_break", context="\tif x > 0 {")
        self.assertEqual(self.check([inserted]), ([], []))
        self.write(ANCHORED.replace("if x > 0 {", "if x > 1 {"))
        failures, _ = self.check([inserted])
        self.assertIn("mutated code gone", failures[0])
        no_context = self.entry("", "\t\tbreak\n", mutator="loop/range_break")
        failures, _ = self.check([no_context])
        self.assertEqual(failures, [f"survivor {no_context['id']} (Evaluate): unanchored: it removes no line and has no context"])
        both = self.entry("\tx++\n", "", context="\tif x > 0 {")
        failures, _ = self.check([both])
        self.assertIn("unanchored: context is one line, only for a mutant that removes no line", failures[0])

    def test_non_adjacent_removed_lines_may_end_after_the_function(self) -> None:
        # go/printer re-flows a comment after the function: the removed lines
        # are matched in order, and only the first must lie inside the body.
        spread = self.entry("\tif x > 0 {\n\treturn x\n// Evaluate's trailing comment.\n", "\treturn x\n")
        self.assertEqual(self.check([spread]), ([], []))
        reordered = self.entry("\treturn x\n\tif x > 0 {\n", "")
        self.assertIn("mutated code gone", self.check([reordered])[0][0])
        outside = self.entry("// Evaluate's trailing comment.\n", "")
        self.assertIn("mutated code gone", self.check([outside])[0][0])

    def test_unanchored_entry_fails_unless_listed(self) -> None:
        bare = self.entry(None, None)
        self.assertEqual(self.check([bare])[0], [f"survivor {bare['id']} (Evaluate): unanchored: removed and added are required"])
        self.assertEqual(self.check([bare], stale=[bare["id"]]),
                         ([], [f"stale survivor {bare['id']} (Evaluate): unanchored: removed and added are required"]))

    def test_listed_orphan_is_not_anchored(self) -> None:
        orphan = self.entry(None, None, symbol="Helper", element="free", file="b/b.go")
        self.assertEqual(self.check([orphan], orphans=[orphan["id"]]), ([], []))

    def test_stale_list_entries_that_no_longer_apply_fail(self) -> None:
        good = self.entry("\tif x > 0 {\n", "\tif x <= 0 {\n")
        orphan = self.entry(None, None, symbol="Helper", element="free", file="b/b.go")
        failures, _ = self.check([good, orphan], stale=[good["id"], orphan["id"], "nowhere"], orphans=[orphan["id"]])
        self.assertEqual(failures, [
            f"[triage] stale_survivors: stale entry {good['id']} (Evaluate) passes the staleness and anchor checks; remove it",
            f"[triage] stale_survivors: {orphan['id']} (Helper) is also an orphaned survivor; list it once, in orphaned_survivors",
            "[triage] stale_survivors: stale entry nowhere is not in the ledger",
        ])

    def test_stale_departure_reasons(self) -> None:
        good = self.entry("\tif x > 0 {\n", "\tif x <= 0 {\n")
        rows = fm.model_rows(self.models, self.root)
        why = fm.departure_reasons(rows, [good], self.root, {"other": "x"})
        self.assertEqual(why(fm.STALE_LIST, good["id"]), "the entry passes the staleness and anchor checks")
        self.assertEqual(why(fm.STALE_LIST, "gone"), "the survivor left the ledger")
        why = fm.departure_reasons(rows, [good], self.root, {good["id"]: "mutated code gone: ..."})
        self.assertEqual(why(fm.STALE_LIST, good["id"]),
                         "the entry still fails the staleness or anchor check (mutated code gone: ...)")

    def test_evidence_at_a_per_mutant_digest(self) -> None:
        # Design D3: a refreshed entry's own digest replaces the baseline's.
        good = self.entry("\tif x > 0 {\n", "\tif x <= 0 {\n")
        mutant = {"id": good["id"], "file": "a/a.go", "mutator": "m", "line": 24, "digest": "own"}
        baseline = {"version": 1, "digest": "d", "mutants": [mutant]}
        self.assertEqual(self.check([good], baseline=baseline)[0], [])
        at_top = [{"id": good["id"], "status": "escaped", "timestamp": "t", "digest": "d"}] * 2
        failures, _ = self.check([good], baseline=baseline, reruns=at_top)
        self.assertEqual(failures, [f"baseline id {good['id']}: 0 recorded escaped rerun(s) at its own input digest, need 2 "
                                    f"(make mutation-formal-rerun MUTATION_MUTANT_ID={good['id']})"])
        # Every mutant has its own digest: the baseline needs none.
        self.assertEqual(self.check([good], baseline={**baseline, "digest": ""})[0], [])

    def test_staleness_check_is_static(self) -> None:
        # Spec "Staleness check is static": no go toolchain, no git history,
        # under --base too (the base manifest comes from a fake reader).
        stale = self.entry("\tif x > 0 {\n", "\tif x <= 0 {\n")
        self.write(ANCHORED.replace("if x > 0 {", "if x > 1 {"))
        calls: list = []

        def record(*args, **kwargs):
            calls.append(args[0] if args else kwargs.get("args"))
            raise AssertionError(f"subprocess called: {calls[-1]}")

        with mock.patch.object(fm.subprocess, "run", record), mock.patch.object(fm.subprocess, "check_output", record), \
                mock.patch.object(fm.subprocess, "Popen", record), mock.patch.object(fm, "git", record):
            failures, _ = self.check([stale])
            base = fm.base_manifest("deadbeef", read=lambda _path: b"[triage]\nbinding_gaps = []\n"
                                                                   b"orphaned_survivors = []\nstale_survivors = []\n"
                                                                   b"[correspondence]\nunresolved_elements = []\n")
            growth, _notices = fm.check_growth(lists_manifest([], [], [], []), base, "deadbeef")
        self.assertEqual(calls, [])
        self.assertEqual(growth, [])
        self.assertIn("mutated code gone", failures[0])


# The orphaned survivors [triage] was seeded with (design D13): Auth 8,
# unpackRejectsEscape 12.
SEEDED_ORPHANS = frozenset({
    "0087737c1ae08d3106d06a71d0eaab0d",
    "055592d8fb48880e3cec2f0f21679428",
    "0d72f8e69406dac5a30eb88c7a955e26",
    "1f1be72781b132d18a6f35b6a077ca53",
    "2de0ff368b4ec4304fb411c5ecd36cdb",
    "3112938aeab6d1eb0a0a4b90093ff80c",
    "31687863358bee2bcacc7fd033e1ea8a",
    "3400d9b396a91b8413668bbf8b19a187",
    "6e8e1d2b4a92cf5fbf66a526e76ec5d9",
    "6eaf43027e4018d328f1944f43e6c97f",
    "818a97aa16a5f2c663ff0af7e73dd31c",
    "a868872863e2850ee3eca2c336357194",
    "b5880e7045570180d24594779fc7068b",
    "b9ecd73a79f914165b1d166d467b28ae",
    "c57132fb392fb58cb0ef6ce95419fe90",
    "d16e0f309258355c0702e36be46da2eb",
    "dec0a3c2cc8cce45245ea6f8a700a4ef",
    "e5c9979e40913a5114cd855ff51ae787",
    "e6063b43d56b74bd35d1775902931352",
    "f2e95d83a50df58bb02a21dcabf4f400",
})


# The binding gaps [triage] was seeded with (design D13, decision 15): 19.
SEEDED_BINDING_GAPS = frozenset({
    ('AtomicWrite', 'caller', 'LockFile.Save'),
    ('ConcurrentModuleEdits', 'Restore', 'fileSnapshot.restore'),
    ('ConcurrentModuleEdits', 'SyncProg', 'SyncModule'),
    ('ConcurrentModuleEdits', 'TidyProg', 'TidyModule'),
    ('ConcurrentModuleEdits', 'UpdateProg', 'UpdateModule'),
    ('ConcurrentModuleEdits', 'VendorRemove, VendorCopy', 'VendorModules'),
    ('ConcurrentModuleEdits', 'WriteMod add', 'AddRequirement'),
    ('ConcurrentModuleEdits', 'WriteMod remove', 'RemoveRequirement'),
    ('DependencyClosure', 'Mod.key', 'ModuleRef.Key'),
    ('DependencyClosure', 'resolved', 'Resolver.resolveAll'),
    ('HostCallbackToken', 'Auth', 'Server.ValidateToken'),
    ('HostCallbackToken', 'End paths', 'ContainerRuntime.prepareContainerExecution'),
    ('LockIdentity', 'v2Parse', 'parseLockFile'),
    ('ModulePathContainment', 'ValidateScriptPath', 'ValidateScriptPath'),
    ('ModulePathContainment', 'checkSymlinkSafety', 'checkSymlinkSafety'),
    ('ModulePathContainment', 'unpackRejectsEscape', 'validateDestinationPath'),
    ('ScopeConstruction', 'modUnlessExplicit', 'CheckModuleCollisions'),
    ('ScopeConstruction', 'moduleSourcesHaveIDs', 'moduleIdentityFor'),
    ('ScopeConstruction', 'srcUnique', 'CheckModuleCollisions'),
})


class UnplannedTests(Fixture):
    """The [triage] shrink-only lists (design D13 of harden-formal-instruments)."""

    ROWS = MODEL_ROWS + row("free", "Helper", "b/b.go", "-", "-")

    def setUp(self) -> None:
        super().setUp()
        (self.root / "a" / "a.go").write_text(CODE + "\nfunc Evaluate() {\n}\n")
        self.models = [self.model(self.ROWS)]

    def survivor(self, mutant: str, element: str = "free", symbol: str = "Helper", file: str = "b/b.go") -> dict:
        return {"id": mutant, "file": file, "symbol": symbol, "model": "M", "element": element,
                "class": "binding-gap", "reason": "r", "follow_up": "survivor triage"}

    def check(self, triage: dict, survivors: list[dict] | None = None) -> tuple[list[str], list[str]]:
        survivors = [self.survivor("o1")] if survivors is None else survivors
        ids = [s["id"] for s in survivors]
        baseline = {"version": 1, "digest": "d", "mutants": [{"id": i, "file": "b/b.go", "mutator": "m", "line": 1} for i in ids]}
        reruns = [{"id": i, "status": "escaped", "timestamp": "t", "digest": "d"} for i in ids for _ in range(2)]
        report: list[str] = []
        # The fixtures carry no anchor: an id that is not a listed orphan is
        # listed as stale, unless the test sets the list (AnchorTests covers it).
        orphans = set(triage.get("orphaned_survivors", []))
        triage = {"stale_survivors": [i for i in ids if i not in orphans], **triage}
        failures = fm.check_triage(baseline, {"survivor": survivors}, reruns, self.models, root=self.root,
                                   triage=triage, report=report)
        return failures, [line for line in report if not line.startswith("stale survivor ")]

    LISTED = {"binding_gaps": [{"model": "M", "element": "free", "symbol": "Helper"}], "orphaned_survivors": ["o1"]}

    def test_listed_gap_and_orphan_pass_and_are_reported(self) -> None:
        failures, report = self.check(self.LISTED)
        self.assertEqual(failures, [])
        self.assertEqual(report, ["binding gap M: 'free' (Helper)", "orphaned survivors of M: 'free' (Helper): 1"])

    def test_unlisted_gap_and_orphan_fail(self) -> None:
        failures, _ = self.check({})
        self.assertEqual(failures, [
            "[triage] binding_gaps: new binding gap M: 'free' (Helper) is not listed; the list only shrinks: bind the row instead",
            "[triage] orphaned_survivors: survivor o1 names unbound row M: 'free' (Helper) and is not listed",
        ])

    def test_new_by_design_rows_pass_and_are_reported(self) -> None:
        # A type symbol with no binding (decision 15: Serverbase State) and a
        # characterisation-only row are derived, never listed.
        self.models = [self.model(self.ROWS + row("kind", "Kind", "a/a.go", "-", "-")
                                  + row("replay", "Server.Stop", "a/a.go", "TestReplay", "characterisation: today's defect"))]
        failures, report = self.check(self.LISTED)
        self.assertEqual(failures, [])
        self.assertIn("by design M: 'kind' (Kind, type-symbol)", report)
        self.assertIn("by design M: 'replay' (Server.Stop, characterisation-only)", report)

    def test_orphan_of_a_by_design_row_must_be_listed(self) -> None:
        self.models = [self.model(self.ROWS + row("kind", "Kind", "a/a.go", "-", "-"))]
        survivors = [self.survivor("o1"), self.survivor("k1", element="kind", symbol="Kind", file="a/a.go")]
        failures, _ = self.check(self.LISTED, survivors)
        self.assertEqual(failures, ["[triage] orphaned_survivors: survivor k1 names unbound row M: 'kind' (Kind) and is not listed"])

    def test_listed_row_that_is_bound_by_design_or_names_no_row_is_stale(self) -> None:
        self.models = [self.model(self.ROWS + row("kind", "Kind", "a/a.go", "-", "-"))]
        triage = {"binding_gaps": [*self.LISTED["binding_gaps"], {"model": "M", "element": "other", "symbol": "Decide"},
                                   {"model": "M", "element": "gone", "symbol": "Gone"},
                                   {"model": "M", "element": "kind", "symbol": "Kind"}],
                  "orphaned_survivors": ["o1"]}
        failures, _ = self.check(triage)
        self.assertEqual(failures, [
            "[triage] binding_gaps: stale entry M: 'gone' (Gone) names no correspondence row",
            "[triage] binding_gaps: stale entry M: 'kind' (Kind) is unbound by design (type-symbol), which is derived, not listed; remove it",
            "[triage] binding_gaps: stale entry M: 'other' (Decide) is bound now; remove it",
        ])

    def test_listed_id_whose_row_is_bound_or_not_in_the_ledger_is_stale(self) -> None:
        triage = {**self.LISTED, "orphaned_survivors": ["o1", "b1", "nowhere"]}
        survivors = [self.survivor("o1"), self.survivor("b1", element="other", symbol="Decide", file="a/a.go")]
        failures, _ = self.check(triage, survivors)
        self.assertEqual(failures, [
            "[triage] orphaned_survivors: stale entry b1 names a bound row now; remove it",
            "[triage] orphaned_survivors: stale entry nowhere is not in the ledger",
        ])

    def test_method_symbol_classifies_as_a_function(self) -> None:
        # decision 18: the type-symbol rule must not swallow `Type.Method`.
        rows = fm.model_rows([self.model(row("stop", "Server.Stop", "a/a.go", "TestStop", "-")
                                         + row("kind", "Kind", "a/a.go", "TestStop", "-"))], self.root)
        declared, _files = fm.test_declarations(self.root)
        classified = fm.classify_rows(rows, self.root, declared)
        self.assertEqual(list(classified.functions), [("a/a.go", "Server.Stop")])
        self.assertEqual([(u["element"], u["reason"]) for u in classified.unbound], [("kind", fm.REASON_TYPE_SYMBOL)])

    def test_departure_reasons(self) -> None:
        self.models = [self.model(self.ROWS + row("kind", "Kind", "a/a.go", "-", "-"))]
        survivors = [self.survivor("o1"), self.survivor("b1", element="other", symbol="Decide", file="a/a.go"),
                     {**self.survivor("d1"), "element": "gone", "symbol": "Gone"}]
        why = fm.departure_reasons(fm.model_rows(self.models, self.root), survivors, self.root)
        cases = {
            (fm.GAPS_LIST, ("M", "other", "Decide")): "the row is bound now",
            (fm.GAPS_LIST, ("M", "kind", "Kind")): "the row was reclassified as type-symbol",
            (fm.GAPS_LIST, ("M", "gone", "Gone")): "the row was deleted",
            (fm.GAPS_LIST, ("M", "free", "Helper")): "the row is still unbound (no-binding)",
            (fm.ORPHANS_LIST, "b1"): "the row is bound now",
            (fm.ORPHANS_LIST, "d1"): "the row was deleted",
            (fm.ORPHANS_LIST, "nowhere"): "the survivor left the ledger",
            (fm.ORPHANS_LIST, "o1"): "the row is still unbound (no-binding)",
            (fm.UNRESOLVED_LIST, "M: other"): "the element resolves now",
            (fm.UNRESOLVED_LIST, "M: gone"): "the row was deleted",
        }
        for (name, entry), reason in cases.items():
            with self.subTest(name=name, entry=entry):
                self.assertEqual(why(name, entry), reason)

    def test_unknown_triage_key_and_malformed_entries(self) -> None:
        head = '[tools.alloy]\nversion = "1"\njar = "a"\nurl = "u"\nsha256 = "0"\n'
        path = self.root / "manifest.toml"
        cases = {
            '[triage]\nbinding_gaps = []\nextra = 1\n': r"\[triage\]: unknown key\(s\) extra",
            '[triage]\nunbound_rows = []\n': r"\[triage\]: unknown key\(s\) unbound_rows",
            '[triage]\nbinding_gaps = []\norphaned_survivors = []\n': "needs a stale_survivors list",
            '[triage]\nstale_survivors = []\nbinding_gaps = [{ model = "M", element = "e" }]\n': "binding_gaps must be a list of",
            '[triage]\nstale_survivors = []\norphaned_survivors = [1]\n': "orphaned_survivors must be a list of mutant ids",
            '[triage]\nstale_survivors = []\norphaned_survivors = ["a", "a"]\n': "duplicate entries",
            '[triage]\nstale_survivors = [""]\n': "stale_survivors must be a list of mutant ids",
            '[triage]\nstale_survivors = ["a", "a"]\n': "duplicate entries",
        }
        for text, message in cases.items():
            with self.subTest(message=message):
                path.write_text(head + text)
                with self.assertRaisesRegex(formal.FormalError, message):
                    formal.load_triage(path)
        path.write_text(head + '[triage]\nbinding_gaps = []\nextra = 1\n')
        with self.assertRaisesRegex(formal.FormalError, r"\[triage\]: unknown key\(s\) extra"):
            formal.load_manifest(path)
        # Deleting the key cannot reopen its seeding: validation fails.
        path.write_text(head + '[triage]\nbinding_gaps = []\norphaned_survivors = []\n')
        with self.assertRaisesRegex(formal.FormalError, "needs a stale_survivors list"):
            formal.load_manifest(path)


def lists_manifest(unresolved: list[str] | None = None, gaps: list[tuple] | None = None,
                   orphans: list[str] | None = None, stale: list[str] | None = None) -> dict:
    """A parsed manifest with the given shrink-only lists (None: absent)."""
    data: dict = {}
    if unresolved is not None:
        data["correspondence"] = {"unresolved_elements": unresolved}
    if gaps is not None or orphans is not None or stale is not None:
        data["triage"] = {}
    if gaps is not None:
        data["triage"]["binding_gaps"] = [{"model": m, "element": e, "symbol": s} for m, e, s in gaps]
    if orphans is not None:
        data["triage"]["orphaned_survivors"] = orphans
    if stale is not None:
        data["triage"]["stale_survivors"] = stale
    return data


class BaseRowsTests(Fixture):
    """Decisions 27 and 28: receiver mismatches fail, and rows bound at the
    base cannot become by-design rows or join a listed unresolved pair."""

    BASE = row("decide", "Decide", "a/a.go", "TestDecide", "-") + row("stop", "Server.Stop", "a/a.go", "TestStop", "-")

    def rows(self, table: str) -> list:
        return fm.model_rows([self.model(table)], self.root)

    def base(self, table: str, code: str = CODE) -> tuple[list, object]:
        files = {"m.als": (HEADER + table).encode(), "a/a.go": code.encode()}

        def read(path: str) -> bytes:
            if path not in files:
                raise OSError(f"{path} is not in the base")
            return files[path]

        return fm.base_rows({"model": [{"name": "M", "file": "m.als"}, {"name": "Gone", "file": "gone.als"}]}, read)

    def compare(self, head: str, base: str = BASE, unresolved=(), seeding: bool = False) -> tuple[list[str], list[str]]:
        rows = self.rows(head)
        at_base, classified = self.base(base)
        return fm.check_base_rows(rows, fm.classify_rows(rows, self.root, {}), at_base, classified, unresolved, "B", seeding)

    def test_receiver_mismatch_is_a_problem_not_a_type_symbol(self) -> None:
        classified = fm.classify_rows(self.rows(row("stop", "Client.Stop", "a/a.go", "TestStop", "-")), self.root, {})
        self.assertEqual(classified.unbound, [])
        self.assertEqual(len(classified.problems), 1)
        self.assertIn("Client.Stop names no function: a/a.go declares Stop only with receiver Server", classified.problems[0])
        failures, _ = fm.check_unplanned(self.rows(row("stop", "Client.Stop", "a/a.go", "TestStop", "-")), [], {}, self.root)
        self.assertEqual([f for f in failures if f.startswith("row classification:")], [f"row classification: {classified.problems[0]}"])

    def test_base_rows_are_classified_from_the_base_sources(self) -> None:
        rows, classified = self.base(self.BASE + row("kind", "Kind", "a/a.go", "TestDecide", "-"))
        self.assertEqual(len(rows), 3)
        self.assertEqual(classified.bound, {("M", "decide", "Decide"), ("M", "stop", "Server.Stop")})
        _rows, without = self.base(self.BASE, code="package a\n")
        self.assertEqual(without.bound, set())

    def test_unchanged_rows_pass(self) -> None:
        self.assertEqual(self.compare(self.BASE), ([], []))

    def test_demotion_to_a_by_design_reason_fails(self) -> None:
        cases = {
            "characterisation-only": row("decide", "Decide", "a/a.go", "TestDecide", "characterisation: today"),
            "trace-only": row("decide", "Decide", "a/a.go", "TestM_TraceHarness", "-"),
            "no-symbol": row("decide", "-", "a/a.go", "TestDecide", "-"),
        }
        for reason, demoted in cases.items():
            with self.subTest(reason=reason):
                failures, notices = self.compare(demoted + row("stop", "Server.Stop", "a/a.go", "TestStop", "-"))
                self.assertEqual(failures, [f"row M: 'decide' (Decide) was bound at the base B and is unbound by design now "
                                            f"({reason}); bind it again or delete the row"])
                self.assertEqual(notices, [])

    def test_demotion_to_no_binding_is_left_to_the_gaps_list(self) -> None:
        self.assertEqual(self.compare(row("decide", "Decide", "a/a.go", "-", "-") + row("stop", "Server.Stop", "a/a.go", "TestStop", "-")),
                         ([], []))

    def test_deletion_and_re_point_are_notices(self) -> None:
        failures, notices = self.compare(row("decide", "Server.Stop", "a/a.go", "TestStop", "-"))
        self.assertEqual(failures, [])
        self.assertEqual(notices, ["row M: 'decide' (Decide), bound at the base B, was re-pointed to Server.Stop",
                                   "row M: 'stop' (Server.Stop), bound at the base B, was deleted"])

    def test_new_row_under_a_listed_unresolved_pair_fails(self) -> None:
        head = self.BASE + row("decide", "Map", "a/a.go", "TestDecide", "-")
        failures, _ = self.compare(head, unresolved=["M: decide"])
        self.assertEqual(failures, ["[correspondence] unresolved_elements: row M: 'decide' (Map) is not in the base B's tables "
                                    "and joins a listed pair; the list only shrinks: name an element that exists in the model"])
        self.assertEqual(self.compare(head), ([], []))

    def test_seeding_reports_failures_as_notices(self) -> None:
        failures, notices = self.compare(row("decide", "-", "a/a.go", "TestDecide", "-")
                                         + row("stop", "Server.Stop", "a/a.go", "TestStop", "-"), seeding=True)
        self.assertEqual(failures, [])
        self.assertEqual(len(notices), 1)
        self.assertTrue(notices[0].endswith("(reported only: the base B seeds the lists)"), notices[0])


class GrowthTests(unittest.TestCase):
    """The base check of the shrink-only lists (decision 15)."""

    BASE = lists_manifest(["M: e1", "M: e2"], [("M", "g1", "F1"), ("M", "g2", "F2")], ["o1", "o2"], ["t1", "t2"])

    def test_growth_at_the_base_fails_for_each_list(self) -> None:
        cases = {
            "[correspondence] unresolved_elements": (lists_manifest(["M: e1", "M: new"], [("M", "g1", "F1")], ["o1"], ["t1"]),
                                                     "'M: new'"),
            "[triage] binding_gaps": (lists_manifest(["M: e1"], [("M", "g1", "F1"), ("M", "g3", "F3")], ["o1"], ["t1"]),
                                      "M: 'g3' (F3)"),
            "[triage] orphaned_survivors": (lists_manifest(["M: e1"], [("M", "g1", "F1")], ["o1", "o9"], ["t1"]), "'o9'"),
            "[triage] stale_survivors": (lists_manifest(["M: e1"], [("M", "g1", "F1")], ["o1"], ["t1", "t9"]), "'t9'"),
        }
        for name, (head, entry) in cases.items():
            with self.subTest(name=name):
                failures, notices = fm.check_growth(head, self.BASE, "abc123")
                self.assertTrue(all("left the list since the base abc123" in n for n in notices), notices)
                self.assertEqual(failures, [f"{name}: {entry} is not listed at the base abc123; "
                                            "the list only shrinks: fix the row or survivor instead of listing it"])

    def test_shrink_passes_and_explains_what_left(self) -> None:
        head = lists_manifest(["M: e2"], [], ["o2"], ["t2"])
        failures, notices = fm.check_growth(head, self.BASE, "abc123", why=lambda name, entry: f"why {entry!r}")
        self.assertEqual(failures, [])
        self.assertEqual(notices, [
            "[correspondence] unresolved_elements: 'M: e1' left the list since the base abc123: why 'M: e1'",
            "[triage] binding_gaps: M: 'g1' (F1) left the list since the base abc123: why ('M', 'g1', 'F1')",
            "[triage] binding_gaps: M: 'g2' (F2) left the list since the base abc123: why ('M', 'g2', 'F2')",
            "[triage] orphaned_survivors: 'o1' left the list since the base abc123: why 'o1'",
            "[triage] stale_survivors: 't1' left the list since the base abc123: why 't1'",
        ])
        self.assertEqual(fm.check_growth(self.BASE, self.BASE, "abc123"), ([], []))

    def test_base_without_the_lists_seeds_them(self) -> None:
        failures, notices = fm.check_growth(self.BASE, {"triage": {"unbound_rows": []}}, "main")
        self.assertEqual(failures, [])
        self.assertEqual(notices, [f"{name}: not in the base main manifest; this change seeds it"
                                   for name in ("[correspondence] unresolved_elements", "[triage] binding_gaps",
                                                "[triage] orphaned_survivors", "[triage] stale_survivors")])

    def test_base_before_the_stale_list_seeds_it(self) -> None:
        # refresh-anchors-mutation-ledger D5: the first three lists are checked
        # as usual, and stale_survivors seeds with a notice.
        base = lists_manifest(["M: e1"], [("M", "g1", "F1")], ["o1"])
        head = lists_manifest(["M: e1"], [("M", "g1", "F1")], ["o1", "o9"], ["t1"])
        failures, notices = fm.check_growth(head, base, "main")
        self.assertEqual(failures, ["[triage] orphaned_survivors: 'o9' is not listed at the base main; "
                                    "the list only shrinks: fix the row or survivor instead of listing it"])
        self.assertEqual(notices, ["[triage] stale_survivors: not in the base main manifest; this change seeds it"])

    def test_base_with_only_some_lists_fails(self) -> None:
        # decision 19: renaming or dropping one key cannot reopen seeding.
        for base, missing in ((lists_manifest(["M: e1", "M: e2"]),
                               ["[triage] binding_gaps", "[triage] orphaned_survivors", "[triage] stale_survivors"]),
                              (lists_manifest(None, [("M", "g1", "F1"), ("M", "g2", "F2")], ["o1", "o2"], ["t1", "t2"]),
                               ["[correspondence] unresolved_elements"]),
                              (lists_manifest(None, [("M", "g1", "F1"), ("M", "g2", "F2")], ["o1", "o2"]),
                               ["[correspondence] unresolved_elements", "[triage] stale_survivors"])):
            with self.subTest(missing=missing):
                failures, _notices = fm.check_growth(self.BASE, base, "main")
                self.assertEqual([f.split(":")[0] for f in failures], missing)
                for failure in failures:
                    self.assertIn("a list is seeded only from a base with none of the three", failure)

    def test_unreadable_base_fails_closed(self) -> None:
        def missing(_path: str) -> bytes:
            raise fm.subprocess.CalledProcessError(128, ["git", "show"], stderr=b"fatal: bad object deadbeef")
        with self.assertRaisesRegex(fm.PlanError, r"cannot read formal/manifest.toml at the base deadbeef "
                                                  r"\(git show deadbeef:formal/manifest.toml failed: fatal: bad object deadbeef\); "
                                                  r"fetch the base commit, set INVOWK_TRIAGE_BASE=<sha> or pass --base <sha>"):
            fm.base_manifest("deadbeef", read=missing)
        with self.assertRaisesRegex(fm.PlanError, "is not valid TOML"):
            fm.base_manifest("deadbeef", read=lambda _path: b"[triage\n")

    def test_undeterminable_base_fails_closed(self) -> None:
        def no_origin(*_args: str) -> str:
            raise fm.subprocess.CalledProcessError(128, ["git", "merge-base"], stderr="fatal: Not a valid object name origin/main")
        with self.assertRaisesRegex(fm.PlanError, r"cannot determine the base \(git merge-base HEAD origin/main failed: "
                                                  r"fatal: Not a valid object name origin/main\); fetch origin/main, "
                                                  r"set INVOWK_TRIAGE_BASE=<sha> or pass --base <sha>"):
            fm.merge_base(run=no_origin)
        self.assertEqual(fm.merge_base(run=lambda *_args: "abc123"), "abc123")

    def test_base_precedence(self) -> None:
        # decision 22: --base, then a non-empty INVOWK_TRIAGE_BASE, then the
        # merge-base. The environments are explicit, so a CI run that sets the
        # variable cannot change the result.
        merge = lambda *_args: "merged"  # noqa: E731 - a one-line stub
        self.assertEqual(fm.resolve_base("arg", {"INVOWK_TRIAGE_BASE": "env"}, merge), "arg")
        self.assertEqual(fm.resolve_base("", {"INVOWK_TRIAGE_BASE": "env"}, merge), "env")
        self.assertEqual(fm.resolve_base("", {"INVOWK_TRIAGE_BASE": ""}, merge), "merged")
        self.assertEqual(fm.resolve_base("", {"INVOWK_TRIAGE_BASE": "  "}, merge), "merged")
        self.assertEqual(fm.resolve_base("", {}, merge), "merged")

    def test_readable_base_parses(self) -> None:
        data = fm.base_manifest("abc", read=lambda path: b'[triage]\norphaned_survivors = ["o1"]\n')
        self.assertEqual(fm.shrink_lists(data)["[triage] orphaned_survivors"], {"o1"})


class RepositoryTests(unittest.TestCase):
    def test_committed_ledger_passes(self) -> None:
        # The tree is its own base: CI checkouts are shallow, and the real
        # base comparison is the formal lane's `Triage lists` step.
        self.assertEqual(fm.main(["triage", "--check", "--base", "HEAD"]), 0)

    def test_triage_lists_only_shrink(self) -> None:
        # Seeded on 2026-11-14 with 19 binding gaps and 20 orphans (design
        # D13); a committed list may lose entries but never gain one.
        triage = formal.load_triage()
        self.assertLessEqual({(r["model"], r["element"], r["symbol"]) for r in triage["binding_gaps"]}, SEEDED_BINDING_GAPS)
        self.assertEqual(len(SEEDED_BINDING_GAPS), 19)
        self.assertEqual(len(SEEDED_ORPHANS), 20)
        self.assertLessEqual(set(triage["orphaned_survivors"]), SEEDED_ORPHANS)
        # Seeded empty on 2026-11-19 (refresh-anchors-mutation-ledger D5): the
        # fresh run anchored every survivor outside the refresh and the orphans.
        self.assertEqual(triage["stale_survivors"], [])

    def test_removing_a_listed_entry_fails(self) -> None:
        _tools, models = formal.load_manifest()
        triage = formal.load_triage()
        with fm.LEDGER.open("rb") as handle:
            ledger = fm.tomllib.load(handle)
        baseline = fm.read_baseline(fm.BASELINE)
        reruns = fm.read_reruns(fm.RERUNS)
        for part in ("binding_gaps", "orphaned_survivors"):
            with self.subTest(part=part):
                shrunk = {**triage, part: triage[part][1:]}
                failures = fm.check_triage(baseline, ledger, reruns, models, triage=shrunk)
                # An orphan that leaves the list loses its anchor exemption too.
                self.assertEqual(len(failures), 2 if part == "orphaned_survivors" else 1, failures)
                self.assertIn("is not listed", failures[0])
                if part == "orphaned_survivors":
                    self.assertIn("unanchored: removed and added are required", failures[1])

    def test_repository_plan_generates(self) -> None:
        _tools, models = formal.load_manifest()
        plan = fm.build_plan(models, resolve_packages=lambda dirs: {d: d for d in dirs})
        self.assertTrue(plan["functions"])
        killers = {t for f in plan["functions"] for t in f["killers"]}
        self.assertFalse({t for t in killers if formal.is_trace_harness(t) or formal.CHARACTERISATION_NAME_RE.match(t)})


if __name__ == "__main__":
    unittest.main()
