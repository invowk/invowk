#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Tests for formal.py: every fail-closed path must fail, not pass."""

from __future__ import annotations

import contextlib
import dataclasses
import gzip
import importlib.util
import io
import json
import os
import re
import sys
import tempfile
import tomllib
import unittest
from pathlib import Path
from unittest import mock

sys.dont_write_bytecode = True
SCRIPT = Path(__file__).with_name("formal.py")
SPEC = importlib.util.spec_from_file_location("formal", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
formal = importlib.util.module_from_spec(SPEC)
sys.modules["formal"] = formal
SPEC.loader.exec_module(formal)

Command = formal.Command
Model = formal.Model
FormalError = formal.FormalError


def alloy_model(*commands: Command, name: str = "M", scope: str = "3") -> Model:
    return Model(name=name, tool="alloy", file="m.als", commands=commands, calibration="seeded", scope=scope,
                 budget_seconds=10, budget_source="local")


GUARDED = (
    Command("safe", "pass", property="prop", antecedent="ante", body="prop"),
    Command("ante", "instance", body="some univ"),
    Command("mut", "counterexample", property="prop", mutant_of="safe", body="prop"),
)


class ManifestGuardTests(unittest.TestCase):
    def test_guarded_model_is_accepted(self) -> None:
        formal.validate_manifest([alloy_model(*GUARDED)])

    def test_unguarded_property_fails(self) -> None:
        model = alloy_model(*GUARDED[:2])
        with self.assertRaisesRegex(FormalError, "unguarded property"):
            formal.validate_manifest([model])

    def test_mutant_of_other_property_fails(self) -> None:
        model = alloy_model(*GUARDED[:2], Command("mut", "counterexample", property="other", mutant_of="safe", body="other"))
        with self.assertRaisesRegex(FormalError, "mutant checks property"):
            formal.validate_manifest([model])

    def test_mutant_expecting_pass_fails(self) -> None:
        model = alloy_model(*GUARDED[:2], Command("mut", "pass", property="prop", mutant_of="safe", body="prop"))
        with self.assertRaisesRegex(FormalError, "must expect a counterexample"):
            formal.validate_manifest([model])

    def test_alloy_check_without_antecedent_fails(self) -> None:
        model = alloy_model(
            Command("safe", "pass", property="prop", body="prop"),
            Command("witness", "instance", body="some univ"),
            Command("mut", "counterexample", property="prop", mutant_of="safe", body="prop"),
        )
        with self.assertRaisesRegex(FormalError, "antecedent"):
            formal.validate_manifest([model])

    def test_alloy_model_without_run_fails(self) -> None:
        model = alloy_model(Command("c", "counterexample", body="x"))
        with self.assertRaisesRegex(FormalError, "non-vacuity"):
            formal.validate_manifest([model])

    def test_unknown_verdict_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "unknown expected verdict"):
            formal.validate_manifest([alloy_model(*GUARDED, Command("x", "maybe", body="x"))])

    def test_missing_budget_or_source_fails(self) -> None:
        cases = {
            (0, "local"): r"\[\[model\]\] M: budget_seconds must be a positive integer, got 0",
            (10, ""): r"\[\[model\]\] M: budget_source must be one of \['ci', 'local'\], got ''",
            (10, "guess"): "got 'guess'",
        }
        for (seconds, source), message in cases.items():
            with self.subTest(seconds=seconds, source=source):
                model = dataclasses.replace(alloy_model(*GUARDED), budget_seconds=seconds, budget_source=source)
                with self.assertRaisesRegex(FormalError, message):
                    formal.validate_manifest([model])
        suite = formal.TraceSuite(name="T", package="./t/", budget_seconds=5)
        with self.assertRaisesRegex(FormalError, r"\[\[trace\]\] T: budget_source must be one of"):
            formal.validate_manifest([alloy_model(*GUARDED)], [suite])
        formal.validate_manifest([alloy_model(*GUARDED)], [dataclasses.replace(suite, budget_source="ci")])

    def test_passing_tlc_command_without_distinct_states_fails(self) -> None:
        safe = Command("safe", "pass", property="P")
        mutant = Command("mut", "counterexample", property="P", mutant_of="safe")
        with self.assertRaisesRegex(FormalError, "T.safe: a passing TLC command must record distinct_states"):
            formal.validate_manifest([tla_model(safe, mutant)])
        formal.validate_manifest([tla_model(dataclasses.replace(safe, distinct_states=7), mutant)])


ALLOY_MODEL_SOURCE = """module M
// a line comment: check { prop } for 3 expect 0
-- a dash comment: safe: run {} for 1 expect 1
/* a block comment
   run { some univ } for 3
*/
pred prop { some univ }
"""

MANIFEST_HEAD = """
[tools.alloy]
version = "1"
jar = "a.jar"
url = "https://invalid.invalid/a.jar"
sha256 = "00"
"""


class AlloyCommandTests(unittest.TestCase):
    def test_rendered_commands_follow_the_manifest(self) -> None:
        model = alloy_model(*GUARDED, Command("big", "instance", body="some univ\nno none", scope="5 but 2 Key"))
        self.assertEqual(
            formal.render_alloy_commands(model),
            "safe: check {\nprop\n} for 3 expect 0\n\n"
            "ante: run {\nsome univ\n} for 3 expect 1\n\n"
            "mut: check {\nprop\n} for 3 expect 1\n\n"
            "big: run {\nsome univ\nno none\n} for 5 but 2 Key expect 1\n",
        )
        self.assertEqual(formal.render_alloy_commands(model), formal.render_alloy_commands(model))

    def test_missing_default_scope_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "M.safe: no scope, and the model declares no default scope"):
            formal.validate_manifest([alloy_model(*GUARDED, scope="")])

    def test_check_body_without_property_fails(self) -> None:
        model = alloy_model(dataclasses.replace(GUARDED[0], body="some univ"), *GUARDED[1:])
        with self.assertRaisesRegex(FormalError, "M.safe: check body does not reference property 'prop'"):
            formal.validate_manifest([model])

    def test_commands_in_comments_are_ignored(self) -> None:
        formal.reject_alloy_source_commands(alloy_model(*GUARDED), ALLOY_MODEL_SOURCE)

    def test_command_left_in_source_fails_naming_the_line(self) -> None:
        for command in ("safe: check { prop } for 3 expect 0", "run { some univ } for 3", "check prop"):
            with self.subTest(command=command):
                with self.assertRaisesRegex(FormalError, r"M: m\.als:8 declares a `(run|check)` command"):
                    formal.reject_alloy_source_commands(alloy_model(*GUARDED), ALLOY_MODEL_SOURCE + command + "\n")

    def load(self, text: str) -> list[Model]:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "manifest.toml"
            path.write_text(MANIFEST_HEAD + text)
            return formal.load_manifest(path)[1]

    def test_manifest_command_fields_are_loaded(self) -> None:
        models = self.load('''
[[model]]
name = "M"
tool = "alloy"
file = "m.als"
scope = "4"
[[model.command]]
name = "g"
body = """
some univ"""
scope = "3 but 1 Key"
expect = "instance"
''')
        self.assertEqual(models[0].scope, "4")
        self.assertEqual((models[0].commands[0].body, models[0].commands[0].scope), ("some univ", "3 but 1 Key"))

    def test_manifest_rejects_unknown_and_misplaced_keys(self) -> None:
        alloy = '[[model]]\nname = "M"\ntool = "alloy"\nfile = "m.als"\nscope = "4"\n[[model.command]]\nname = "g"\nexpect = "instance"\n'
        tla = '[[model]]\nname = "T"\ntool = "tla"\nfile = "t.tla"\n[[model.command]]\nname = "c"\nexpect = "pass"\nproperty = "P"\n'
        cases = {
            alloy + 'body = "x"\nscop = "3"\n': r"\[\[model.command\]\] g: unknown key\(s\) scop",
            alloy: r"\[\[model.command\]\] g: an Alloy command needs a non-empty body",
            alloy + 'body = "  "\n': "needs a non-empty body",
            alloy + 'body = "x"\nconstants = { N = "1" }\n': r"g: key\(s\) constants are tla-only, but the model\'s tool is alloy",
            tla + 'body = "x"\n': r"\[\[model.command\]\] c: key\(s\) body are alloy-only, but the model\'s tool is tla",
            tla.replace('file = "t.tla"\n', 'file = "t.tla"\nscope = "3"\n'): r"\[\[model\]\] T: key\(s\) scope are alloy-only",
            alloy.replace('scope = "4"\n', 'scope = "4"\nspec = "S"\n') + 'body = "x"\n': r"\[\[model\]\] M: key\(s\) spec are tla-only",
            alloy.replace('scope = "4"\n', 'scope = "4"\nfiel = "x"\n'): r"\[\[model\]\] M: unknown key\(s\) fiel",
            alloy.replace('scope = "4"\n', 'scope = "4"\n[model.golden]\ncommand = "g"\nouput = "x"\n'): r"\[model.golden\]: unknown key\(s\) ouput",
            '[[trace]]\nname = "T"\npackage = "./x/"\npkg = "y"\n': r"\[\[trace\]\] T: unknown key\(s\) pkg",
            '[tools.tla]\nversion = "1"\njar = "t"\nurl = "u"\nsha256 = "0"\nmirror = "m"\n': r"\[tools.tla\]: unknown key\(s\) mirror",
        }
        for text, message in cases.items():
            with self.subTest(message=message):
                with self.assertRaisesRegex(FormalError, message):
                    self.load(text)

    def test_trace_suite_constants_override_the_model(self) -> None:
        text = '[[trace]]\nname = "T"\npackage = "./x/"\nconstants = { Legacy = "TRUE" }\n'
        self.load(text)  # `constants` is an allowed [[trace]] key
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "manifest.toml"
            path.write_text(MANIFEST_HEAD + text)
            suite = formal.load_trace_suites(path)[0]
        model = Model(name="T", tool="tla", file="t.tla", commands=(), constants=(("Legacy", "FALSE"), ("Mutant", '"none"')))
        self.assertEqual(formal.trace_constants(suite, model), {"Legacy": "TRUE", "Mutant": '"none"'})
        self.assertEqual(formal.trace_constants(dataclasses.replace(suite, constants=()), model), dict(model.constants))
        typo = dataclasses.replace(suite, constants=(("Legacyy", "TRUE"),))
        with self.assertRaisesRegex(FormalError, r"\[\[trace\]\] T: constants override Legacyy, absent from the model's base constants"):
            formal.trace_constants(typo, model)

    def test_receipt_without_command_is_no_verdict(self) -> None:
        self.assertEqual(formal.alloy_verdicts({"commands": {}}).get("safe", formal.VERDICT_NONE), formal.VERDICT_NONE)

    def test_receipt_verdicts(self) -> None:
        receipt = {
            "commands": {
                "sat": {"type": "run", "solution": [{"instances": [{}]}]},
                "unsat": {"type": "run"},
                "cex": {"type": "check", "solution": [{"instances": [{}]}]},
                "ok": {"type": "check"},
            }
        }
        self.assertEqual(
            formal.alloy_verdicts(receipt),
            {"sat": "instance", "unsat": "pass", "cex": "counterexample", "ok": "pass"},
        )


TLC_PASS = """Model checking completed. No error has been found.
<Init line 4, col 1 to line 4, col 4 of module Tiny>: 1:1
<Inc line 5, col 1 to line 5, col 3 of module Tiny>: 2:2
<Dead line 6, col 1 to line 6, col 4 of module Tiny>: 0:0
4 states generated, 3 distinct states found, 0 states left on queue.
"""
TLC_VIOLATION = """Error: Invariant NotTwo is violated.
3 states generated, 3 distinct states found, 0 states left on queue.
"""


def tla_model(*cmds: Command) -> Model:
    return Model(name="T", tool="tla", file="t.tla", commands=cmds, calibration="seeded", budget_seconds=10, budget_source="ci")


class TlcTests(unittest.TestCase):
    def test_missing_verdict_is_no_verdict(self) -> None:
        result = formal.parse_tlc_output("TLC2 Version 2.19\nStarting...\n")
        self.assertEqual(result.verdict, formal.VERDICT_NONE)
        failures = formal.evaluate_tlc_command(tla_model(Command("c", "pass")), Command("c", "pass"), result)
        self.assertTrue(failures)

    def test_wrong_verdict_fails(self) -> None:
        cmd = Command("c", "pass", property="Small")
        failures = formal.evaluate_tlc_command(tla_model(cmd), cmd, formal.parse_tlc_output(TLC_VIOLATION))
        self.assertIn("expected pass, observed counterexample", failures[0])

    def test_counterexample_must_name_guarded_property(self) -> None:
        cmd = Command("m", "counterexample", property="Small", mutant_of="c")
        failures = formal.evaluate_tlc_command(tla_model(cmd), cmd, formal.parse_tlc_output(TLC_VIOLATION))
        self.assertIn("not the guarded property", failures[0])

    def test_uncovered_action_fails_unless_declared_dead(self) -> None:
        result = formal.parse_tlc_output(TLC_PASS)
        self.assertEqual(result.zero_state_actions, ("Dead",))
        self.assertEqual(result.distinct_states, 3)
        cmd = Command("c", "pass")
        self.assertIn("zero generated states: Dead", formal.evaluate_tlc_command(tla_model(cmd), cmd, result)[0])
        dead = Command("c", "pass", dead_actions=("Dead",))
        self.assertEqual(formal.evaluate_tlc_command(tla_model(dead), dead, result), [])

    def test_uncovered_let_action_fails(self) -> None:
        # Real TLC 1.7.4 output for `Dead == LET y == x + 10 IN x > 5 /\\ x' = y`.
        output = TLC_PASS.replace(
            "<Dead line 6, col 1 to line 6, col 4 of module Tiny>: 0:0",
            "<Dead line 6, col 1 to line 6, col 4 of module Tiny (6 28 6 42)>: 0:0",
        )
        result = formal.parse_tlc_output(output)
        self.assertEqual(result.zero_state_actions, ("Dead",))
        cmd = Command("c", "pass")
        self.assertIn("zero generated states: Dead", formal.evaluate_tlc_command(tla_model(cmd), cmd, result)[0])

    def test_symmetry_in_liveness_config_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "SYMMETRY or VIEW"):
            formal.check_tlc_config_text("c", "SPECIFICATION Spec\nPROPERTY Live\nSYMMETRY Perms\n")
        formal.check_tlc_config_text("c", "SPECIFICATION Spec\nINVARIANT Safe\nSYMMETRY Perms\n")

    def test_liveness_without_fairness_twin_fails(self) -> None:
        live = Command("live", "pass", property="Live", temporal=True, distinct_states=3)
        guard = Command("mut", "counterexample", property="Live", mutant_of="live", temporal=True)
        with self.assertRaisesRegex(FormalError, "fairness-free twin"):
            formal.validate_manifest([tla_model(live, guard)])
        twin = Command("unfair", "counterexample", property="Live", temporal=True, spec="UnfairSpec",
                       mutant_of="live", fairness_twin_of="live")
        formal.validate_manifest([tla_model(live, twin)])

    def test_finding_does_not_guard_its_property(self) -> None:
        fixed = Command("fixed", "pass", property="P", distinct_states=3)
        finding = Command("current", "counterexample", property="P", mutant_of="fixed", finding="F9")
        with self.assertRaisesRegex(FormalError, "finding records do not count"):
            formal.validate_manifest([tla_model(fixed, finding)])
        mutant = Command("mut", "counterexample", property="P", mutant_of="fixed")
        formal.validate_manifest([tla_model(fixed, finding, mutant)])

    def test_generated_config_checks_one_property(self) -> None:
        model = Model(name="T", tool="tla", file="t.tla", commands=(), constants=(("N", "2"), ("M", '"none"')))
        cfg = formal.tlc_config(model, Command("c", "pass", property="Safe", constants=(("M", '"bad"'),)))
        self.assertIn('M = "bad"', cfg)
        self.assertIn("N = 2", cfg)
        self.assertEqual(cfg.count("INVARIANT Safe"), 1)
        live = formal.tlc_config(model, Command("l", "pass", property="Live", temporal=True, spec="UnfairSpec"))
        self.assertIn("PROPERTY Live", live)
        self.assertIn("SPECIFICATION UnfairSpec", live)

    def test_temporal_violation_is_attributed_to_the_checked_property(self) -> None:
        result = formal.parse_tlc_output("Error: Temporal properties were violated.\n")
        live = Command("l", "counterexample", property="NoLostBurst", temporal=True)
        self.assertEqual(formal.attribute_temporal_violation(result, live).violated, "NoLostBurst")
        safety = Command("s", "counterexample", property="Safe")
        self.assertEqual(formal.attribute_temporal_violation(result, safety).violated, "<temporal>")

    def test_disabled_deadlock_check_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "deadlock checking must stay on"):
            formal.check_tlc_config_text("c", "SPECIFICATION Spec\nCHECK_DEADLOCK FALSE\n")


class TraceVerdictTests(unittest.TestCase):
    def test_consumed_trace_is_accepted(self) -> None:
        self.assertEqual(formal.trace_verdict("Error: Invariant NotFullyConsumed is violated.\n"), "accepted")

    def test_unconsumed_trace_is_rejected(self) -> None:
        self.assertEqual(formal.trace_verdict("Model checking completed. No error has been found.\n"), "rejected")

    def test_other_violation_is_no_verdict(self) -> None:
        self.assertEqual(formal.trace_verdict("Error: Invariant TypeOK is violated.\n"), formal.VERDICT_NONE)
        self.assertEqual(formal.trace_verdict("Parse error\n"), formal.VERDICT_NONE)


TRACE_SPEC = """---- MODULE MTrace ----
EXTENDS M, MTraces
CONSTANTS TraceSet, TraceIndex
VARIABLE i
INSTANCE TraceBase
\\* NotFullyConsumed == i < 2 (a comment, not a definition)
Proj == [a |-> x, b |-> y]
Settled == TRUE
TraceInit == Init /\\ TraceStart(Proj)
====
"""


class TlaSourceTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.dir = Path(self.tmp.name)
        (self.dir / "TraceBase.tla").write_text("TraceStart(p) == TRUE\nNotFullyConsumed == TRUE\n")
        (self.dir / "M.tla").write_text("---- MODULE M ----\nStart(e) == TRUE\n====\n")

    def check(self, trace_spec: str = TRACE_SPEC, model: str | None = None) -> None:
        (self.dir / "MTrace.tla").write_text(trace_spec)
        if model is not None:
            (self.dir / "M.tla").write_text(model)
        formal.check_tla_sources(self.dir)

    def test_trace_spec_with_helper_is_accepted(self) -> None:
        self.check()

    def test_trace_spec_without_instance_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, r"MTrace.tla: a trace spec must `INSTANCE TraceBase`"):
            self.check(TRACE_SPEC.replace("INSTANCE TraceBase\n", "\\* INSTANCE TraceBase\n"))

    def test_trace_spec_redefining_machinery_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "MTrace.tla: defines NotFullyConsumed, reserved by TraceBase.tla"):
            self.check(TRACE_SPEC.replace("Settled == TRUE", "NotFullyConsumed == i < 3"))

    def test_trace_spec_with_extra_variable_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "may declare only the cursor variable i, not j"):
            self.check(TRACE_SPEC.replace("VARIABLE i\n", "VARIABLES i, j\n"))

    def test_model_defining_reserved_name_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "M.tla: defines TraceStart, reserved by TraceBase.tla"):
            self.check(model="---- MODULE M ----\nTraceStart(e) == TRUE\n====\n")

    def test_proj_fields_ignore_nested_values(self) -> None:
        source = """Proj == [delivered |-> delivered, callbacks |-> callbacks,
         timer |-> IF loop = "select" THEN timer ELSE "none", returned |-> loop = "returned",
         nested |-> [x |-> 1, y |-> <<1, 2>>], sessions |-> [e \\in Execs |-> {s, t}], label |-> "a, b |-> c"]

TraceInit == Init
"""
        self.assertEqual(
            formal.proj_fields("MTrace.tla", source),
            ["callbacks", "delivered", "label", "nested", "returned", "sessions", "timer"],
        )

    def test_non_record_proj_fails(self) -> None:
        for body in ("<<a, b>>", "[e \\in Execs |-> 1]", "[a |-> 1] \\o x", "[a -> B]", "IF c THEN [a |-> 1] ELSE [a |-> 2]"):
            with self.subTest(body=body):
                with self.assertRaisesRegex(FormalError, "MTrace.tla: Proj must be a single record literal"):
                    formal.proj_fields("MTrace.tla", f"Proj == {body}\nNext == TRUE\n")
        with self.assertRaisesRegex(FormalError, "no top-level `Proj ==`"):
            formal.proj_fields("MTrace.tla", "  Proj == [a |-> 1]\n")

    def test_recorded_fields_must_match_proj(self) -> None:
        suite = formal.TraceSuite(name="M", package="./m/")
        formal.check_proj_fields(suite, ["b", "a"], TRACE_SPEC)
        with self.assertRaisesRegex(FormalError, r"harness records fields \['a', 'c'\], but MTrace.tla projects \['a', 'b'\]"):
            formal.check_proj_fields(suite, ["a", "c"], TRACE_SPEC)
        with self.assertRaisesRegex(FormalError, "records no projection fields"):
            formal.check_proj_fields(suite, None, TRACE_SPEC)

    GROUPED = """VARIABLES
    a,   \\* one
    b, c, d
aVars == <<a, b>>
cVars == <<c>>
vars == <<aVars, cVars, d>>
"""

    def test_variable_groups_partition_the_variables(self) -> None:
        formal.check_variable_groups("M", self.GROUPED)
        formal.check_variable_groups("M", "VARIABLES a, b\nvars == <<a, b>>\n")  # no groups: not checked

    def test_duplicated_or_missing_group_member_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, r"M: variable groups in vars do not partition the variables \(duplicated: b\)"):
            formal.check_variable_groups("M", self.GROUPED.replace("cVars == <<c>>", "cVars == <<c, b>>"))
        with self.assertRaisesRegex(FormalError, r"\(missing: c\)"):
            formal.check_variable_groups("M", self.GROUPED.replace("cVars == <<c>>", "cVars == <<>>"))


class ToolAndCorrespondenceTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def test_checksum_mismatch_removes_jar(self) -> None:
        original = formal.TOOL_DIR
        self.addCleanup(setattr, formal, "TOOL_DIR", original)
        formal.TOOL_DIR = self.root
        (self.root / "tool.jar").write_bytes(b"tampered")
        tool = formal.Tool(name="t", version="1", jar="tool.jar", url="https://invalid.invalid/", sha256="0" * 64)
        with self.assertRaisesRegex(FormalError, "checksum mismatch"):
            formal.verify_tool(tool)
        self.assertFalse((self.root / "tool.jar").exists())

    def test_missing_jar_without_fetch_fails(self) -> None:
        original = formal.TOOL_DIR
        self.addCleanup(setattr, formal, "TOOL_DIR", original)
        formal.TOOL_DIR = self.root
        tool = formal.Tool(name="t", version="1", jar="absent.jar", url="https://invalid.invalid/", sha256="0" * 64)
        with self.assertRaisesRegex(FormalError, "not installed"):
            formal.verify_tool(tool)

    def write_model(self, row: str) -> Model:
        (self.root / "code.go").write_text("package x\n\nfunc RealSymbol() {}\n")
        (self.root / "code_test.go").write_text("package x\n\nfunc TestBinding(t *testing.T) {}\n")
        (self.root / "m.als").write_text("// | model element | Go symbol | file | binding | abstraction |\n// |---|---|---|---|---|\n" + row)
        return alloy_model(*GUARDED)

    def test_correspondence_accepts_real_symbols(self) -> None:
        model = self.write_model("// | e | `RealSymbol` | `code.go` | TestBinding | - |\n")
        self.assertEqual(formal.check_correspondence([model], root=self.root), [])

    def test_correspondence_stale_symbol_fails(self) -> None:
        model = self.write_model("// | e | `GoneSymbol` | `code.go` | TestBinding | - |\n")
        self.assertIn("GoneSymbol, which is not declared", formal.check_correspondence([model], root=self.root)[0])

    def test_correspondence_ignores_mentions_outside_declarations(self) -> None:
        (self.root / "caller.go").write_text("package x\n\n// RealSymbol is mentioned here but declared elsewhere.\nfunc use() { RealSymbol() }\n")
        model = self.write_model("// | e | `RealSymbol` | `caller.go` | TestBinding | - |\n")
        self.assertIn("not declared in caller.go", formal.check_correspondence([model], root=self.root)[0])

    def test_witness_must_expect_instance(self) -> None:
        with self.assertRaisesRegex(FormalError, "alloy witness must expect instance"):
            formal.validate_manifest([alloy_model(*GUARDED, Command("w", "pass", witness=True, body="x"))])

    def test_correspondence_missing_binding_fails(self) -> None:
        model = self.write_model("// | e | `RealSymbol` | `code.go` | TestGone | - |\n")
        self.assertIn("binding test TestGone", formal.check_correspondence([model], root=self.root)[0])

    def test_correspondence_without_table_fails(self) -> None:
        (self.root / "m.als").write_text("module m\n")
        self.assertIn("no correspondence table", formal.check_correspondence([alloy_model(*GUARDED)], root=self.root)[0])


# An empty sig (Key), a ternary relation (lock), a relation empty in every
# instance (never), and a duplicate instance (the third repeats the first).
GOLDEN_INSTANCES = [
    {"sig": {"Caller": ["Caller$0"], "Key": []},
     "rel": {"lock": [["Caller$0", "Key$0", "Entry$0"], ["Caller$0", "Key$1", "Entry$0"]], "never": []}},
    {"sig": {"Caller": ["Caller$0"], "Key": []}, "rel": {"lock": [], "never": []}},
    {"sig": {"Caller": ["Caller$0"], "Key": []},
     "rel": {"lock": [["Caller$0", "Key$0", "Entry$0"], ["Caller$0", "Key$1", "Entry$0"]], "never": []}},
]
GOLDEN_ARITIES = {"lock": 3, "never": 2}


class GoldenFormatTests(unittest.TestCase):
    def encoded(self) -> dict:
        return {"model": "M", "command": "golden", "fingerprint": "format=2;x", "count": len(GOLDEN_INSTANCES),
                **formal.encode_golden(GOLDEN_INSTANCES, GOLDEN_ARITIES)}

    def test_round_trip_keeps_order_duplicates_and_empty_keys(self) -> None:
        self.assertEqual(formal.decode_columns(self.encoded()), GOLDEN_INSTANCES)

    def test_decode_golden_reads_format_2(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "g.json.gz"
            path.write_bytes(gzip.compress(json.dumps(self.encoded()).encode(), mtime=0))
            header, instances = formal.decode_golden(path)
        self.assertEqual(instances, GOLDEN_INSTANCES)
        self.assertEqual(header["count"], 3)

    def test_decode_golden_rejects_format_1(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "g.json.gz"
            old = {"model": "M", "command": "golden", "fingerprint": "format=1;x", "count": 1, "instances": []}
            path.write_bytes(gzip.compress(json.dumps(old).encode(), mtime=0))
            with self.assertRaisesRegex(FormalError, "golden format 1 is not 2; regenerate with: make formal-golden"):
                formal.decode_golden(path)

    def test_digest_is_encoding_independent(self) -> None:
        self.assertEqual(formal.golden_digest(formal.decode_columns(self.encoded())), formal.golden_digest(GOLDEN_INSTANCES))

    def test_arity_comes_from_declaration(self) -> None:
        self.assertEqual(self.encoded()["rel"]["never"], {"arity": 2, "values": [[]], "index": [0, 0, 0]})

    def test_undeclared_arity_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "without a declared arity"):
            formal.encode_golden(GOLDEN_INSTANCES, {"lock": 3})

    def test_tuple_of_wrong_arity_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "arity is not 2"):
            formal.encode_golden(GOLDEN_INSTANCES, {"lock": 2, "never": 2})

    def test_truncated_column_fails(self) -> None:
        data = self.encoded()
        data["sig"]["Key"]["index"].pop()
        with self.assertRaisesRegex(FormalError, "has 2 indices, want 3"):
            formal.decode_columns(data)

    def test_export_over_budget_fails(self) -> None:
        model = Model(name="M", tool="alloy", file="m.als", commands=(), golden={"max_bytes": 100})
        with self.assertRaisesRegex(FormalError, r"g\.json\.gz would be 101 compressed bytes, over the 100-byte budget"):
            formal.check_golden_size(model, "g.json.gz", 101)
        formal.check_golden_size(model, "g.json.gz", 100)
        with self.assertRaisesRegex(FormalError, "over the 600000-byte budget"):
            formal.check_golden_size(dataclasses.replace(model, golden={}), "g.json.gz", 600_001)


SNAPSHOT = {
    "commands": {
        "A.safe": {"verdict": "pass"},
        "T.c": {"verdict": "pass", "violated": "", "distinct_states": 12, "zero_coverage": ["Dead"]},
    },
    "traces": {"T": {"accepted": 1, "rejected": 1, "verdicts": {"accepted-1": "accepted", "rejected-1": "rejected"}}},
    "golden": {"A": "0" * 64},
}


class SnapshotTests(unittest.TestCase):
    def changed(self, path: tuple[str, ...], value: object) -> dict:
        record = json.loads(json.dumps(SNAPSHOT))
        target = record
        for key in path[:-1]:
            target = target[key]
        target[path[-1]] = value
        return record

    def test_identical_records_compare_equal(self) -> None:
        self.assertEqual(formal.compare_snapshots(SNAPSHOT, json.loads(json.dumps(SNAPSHOT))), [])

    def test_changed_entries_are_named_with_old_and_new_values(self) -> None:
        cases = {
            ("commands", "A.safe", "verdict"): ("counterexample", "commands/A.safe/verdict: 'pass' -> 'counterexample'"),
            ("commands", "T.c", "distinct_states"): (13, "commands/T.c/distinct_states: 12 -> 13"),
            ("traces", "T", "verdicts", "rejected-1"): ("accepted", "traces/T/verdicts/rejected-1: 'rejected' -> 'accepted'"),
            ("golden", "A"): ("1" * 64, "golden/A: '" + "0" * 64 + "' -> '" + "1" * 64 + "'"),
        }
        for path, (value, message) in cases.items():
            with self.subTest(path=path):
                self.assertEqual(formal.compare_snapshots(SNAPSHOT, self.changed(path, value)), [message])

    def test_missing_entry_differs(self) -> None:
        record = json.loads(json.dumps(SNAPSHOT))
        del record["commands"]["A.safe"]
        self.assertEqual(formal.compare_snapshots(SNAPSHOT, record), ["commands/A.safe/verdict: 'pass' -> '<absent>'"])


class TimingTests(unittest.TestCase):
    def report(self, env: dict[str, str], timings: dict[str, float], budgets: dict) -> tuple[list[str], str]:
        out = io.StringIO()
        with mock.patch.dict(os.environ, env, clear=True), contextlib.redirect_stdout(out):
            warnings = formal.report_timings(timings, budgets)
        return warnings, out.getvalue()

    def test_overrun_warns_without_failing(self) -> None:
        warnings, out = self.report({}, {"M": 12.34, "trace/T": 1.0}, {"M": (10, "ci"), "trace/T": (5, "local")})
        self.assertEqual(warnings, ["M took 12.3s, over its 10s soft budget (ci)"])
        self.assertIn("timing M 12.3s budget 10s (ci)\n", out)
        self.assertIn("timing trace/T 1.0s budget 5s (local)\n", out)
        self.assertIn("WARNING: M took 12.3s", out)
        self.assertNotIn("::", out)  # annotations only under GitHub Actions

    def test_github_annotations_and_step_summary(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            summary = Path(tmp) / "summary.md"
            env = {"GITHUB_ACTIONS": "true", "GITHUB_STEP_SUMMARY": str(summary)}
            _warnings, out = self.report(env, {"M": 12.0, "L": 1.0}, {"M": (10, "ci"), "L": (5, "local")})
            table = summary.read_text()
        self.assertIn("::warning title=Formal soft budget::M took 12.0s", out)
        self.assertIn("::notice title=Provisional formal budget::L: budget 5s is provisional", out)
        self.assertNotIn("::notice title=Provisional formal budget::M", out)
        self.assertIn("| `M` | 12.0 | 10 | ci | over budget |", table)
        self.assertIn("| `L` | 1.0 | 5 | local | ok |", table)

    def test_add_time_sums_per_label(self) -> None:
        timings: dict[str, float] = {}
        formal.add_time(timings, "M", 1.5)
        formal.add_time(timings, "M", 2.0)
        formal.add_time(None, "M", 9.0)  # no collector: ignored
        self.assertEqual(timings, {"M": 3.5})


def no_git(_base: str, _head: str) -> list[str]:
    raise AssertionError("affected must not diff for this event")


class AffectedTests(unittest.TestCase):
    def affected(self, event: str, diff=no_git, manifest: Path = formal.MANIFEST, base: str = "b", head: str = "h") -> str:
        with contextlib.redirect_stderr(io.StringIO()):
            return formal.affected(event, base, head, manifest=manifest, diff=diff)

    def test_non_pull_request_events_run_without_git(self) -> None:
        for event in ("schedule", "workflow_dispatch", "push", ""):
            with self.subTest(event=event):
                self.assertEqual(self.affected(event), "run=true")

    def test_match_and_miss(self) -> None:
        self.assertEqual(self.affected("pull_request", lambda b, h: ["README.md", "internal/sshserver/server.go"]), "run=true")
        self.assertEqual(self.affected("pull_request", lambda b, h: ["README.md", "website/docs/intro.md"]), "run=false")
        # fnmatchcase: no case folding, and `*` crosses `/`.
        self.assertEqual(self.affected("pull_request", lambda b, h: ["FORMAL/manifest.toml"]), "run=false")
        self.assertEqual(self.affected("pull_request", lambda b, h: ["internal/container/retry/x.go"]), "run=true")

    def test_empty_diff_runs(self) -> None:
        self.assertEqual(self.affected("pull_request", lambda b, h: []), "run=true")

    def test_bad_sha_runs_the_full_lane(self) -> None:
        self.assertEqual(self.affected("pull_request", formal.changed_paths, base="0" * 40, head="HEAD"), "run=true")
        self.assertEqual(self.affected("pull_request", formal.changed_paths, base="", head="HEAD"), "run=true")

    def test_malformed_manifest_runs_the_full_lane(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            broken = Path(tmp) / "manifest.toml"
            for text in ("[ci\npaths = [", '[ci]\npaths = ["docs/**"]\n[[model]]\nname = "M"\ntool = "alloy"\nfiel = "x"\n',
                         '[ci]\npaths = []\n'):
                with self.subTest(text=text):
                    broken.write_text(text)
                    self.assertEqual(self.affected("pull_request", lambda b, h: ["docs/x.md"], manifest=broken), "run=true")

    def test_main_prints_run_and_exits_zero_before_validation(self) -> None:
        out = io.StringIO()
        with mock.patch.object(formal, "load_manifest", side_effect=AssertionError("validated")), \
                contextlib.redirect_stdout(out), contextlib.redirect_stderr(io.StringIO()):
            code = formal.main(["affected", "--event", "schedule"])
        self.assertEqual((code, out.getvalue()), (0, "run=true\n"))


class ReplayPlanTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        for rel, tests in {"a/a_test.go": ["TestA", "TestM_TraceHarness"], "b/c/b_test.go": ["TestB"],
                           "a/testdata/x_test.go": ["TestIgnored"]}.items():
            (self.root / rel).parent.mkdir(parents=True, exist_ok=True)
            (self.root / rel).write_text("package x\n\n" + "".join(f"func {t}(t *testing.T) {{}}\n" for t in tests))

    def plan(self, binding: str) -> tuple[list[str], list[str]]:
        (self.root / "m.als").write_text(f"// | e | `S` | `a/a.go` | {binding} | - |\n")
        return formal.replay_plan([alloy_model(*GUARDED)], root=self.root)

    def test_comma_separated_cells_and_trace_harnesses(self) -> None:
        packages, tests = self.plan("TestB, `TestA`, TestM_TraceHarness")
        self.assertEqual((packages, tests), (["./a/", "./b/c/"], ["TestA", "TestB"]))
        self.assertEqual(formal.replay_pattern(tests), "^(TestA|TestB)$")
        self.assertEqual(self.plan("-"), ([], []))

    def test_unknown_test_fails_naming_model_and_row(self) -> None:
        for binding in ("TestA, TestGone", "TestIgnored"):
            with self.subTest(binding=binding):
                with self.assertRaisesRegex(FormalError, r"M: row 'e' names binding test 'Test(Gone|Ignored)', which no _test.go file declares"):
                    self.plan(binding)


WORKFLOW = formal.REPO_ROOT / ".github" / "workflows" / "formal-verification.yml"
# Tests the pre-change hand-written replay regex missed.
PREVIOUSLY_UNREPLAYED = (
    "TestSyncFreshCacheRejectsChangedContent", "TestSyncRejectsRepointedTag", "TestSyncExistingCacheRejectsTamperedContent",
    "TestRevokeTokenClosesAuthenticatedConnection", "TestRevocationRacesAuthenticationSafely",
)


class CiContractTests(unittest.TestCase):
    """The repository's [ci] paths, replay plan, and workflow timeouts."""

    @classmethod
    def setUpClass(cls) -> None:
        cls.tools, cls.models = formal.load_manifest()
        cls.patterns = formal.ci_paths()
        cls.rows = [(m, row) for m in cls.models for row in formal.correspondence_rows((formal.REPO_ROOT / m.file).read_text())]
        cls.files = formal.test_function_files(formal.REPO_ROOT)

    def assert_covered(self, kind: str, paths: set[str]) -> None:
        self.assertTrue(paths, kind)
        missing = sorted(p for p in paths if not formal.matches_ci_paths(p, self.patterns))
        self.assertEqual(missing, [], f"[ci] paths do not match these {kind}")

    def test_ci_paths_cover_correspondence_files(self) -> None:
        self.assert_covered("correspondence files", {row[2] for _m, row in self.rows if row[2] not in {"", "-"}} | {m.file for m in self.models})

    def test_ci_paths_cover_binding_test_files(self) -> None:
        tests = {t for _m, row in self.rows for t in formal.binding_tests(row[3])}
        self.assert_covered("binding-test files", {p.as_posix() for t in tests for p in self.files.get(t, [])})

    def test_ci_paths_cover_trace_packages(self) -> None:
        packages = {os.path.normpath(s.package).replace(os.sep, "/") + "/**" for s in formal.load_trace_suites()}
        self.assert_covered("normalised trace packages", packages)

    def test_ci_paths_cover_golden_outputs(self) -> None:
        self.assert_covered("golden outputs", {str(m.golden["output"]) for m in self.models if m.golden})

    def test_replay_plan_covers_every_binding_test(self) -> None:
        packages, tests = formal.replay_plan(self.models, files=self.files)
        pattern = re.compile(formal.replay_pattern(tests))
        for _model, row in self.rows:
            for test in formal.binding_tests(row[3]):
                if formal.is_trace_harness(test):
                    self.assertIsNone(pattern.match(test), test)
                    continue
                self.assertTrue(pattern.match(test), test)
                for path in self.files[test]:
                    self.assertIn(formal.go_package(path), packages, test)
        for test in PREVIOUSLY_UNREPLAYED:
            self.assertIn(test, tests)

    def test_budget_record_counts_every_command(self) -> None:
        data = tomllib.loads(formal.MANIFEST.read_text())
        self.assertEqual(data["ci"]["budget"]["command_count"], sum(len(m.commands) for m in self.models))

    def test_workflow_job_timeout_covers_step_timeouts(self) -> None:
        text = WORKFLOW.read_text()
        job = re.search(r"^    timeout-minutes: (\d+)$", text, re.M)
        steps = {name: int(minutes) for name, minutes in
                 re.findall(r"^      - name: (.+)\n(?:        (?!timeout-minutes).*\n)*?        timeout-minutes: (\d+)$", text, re.M)}
        self.assertIsNotNone(job)
        self.assertEqual(
            sorted(steps), ["Check models", "Deep property tests", "Replay golden vectors and property tests", "Trace validation"]
        )
        self.assertGreater(int(job[1]), sum(steps.values()) + 3, steps)

    def test_workflow_replays_the_generated_plan(self) -> None:
        text = WORKFLOW.read_text()
        self.assertIn("python3 scripts/formal.py replay-plan --format github", text)
        self.assertNotIn("paths:", text.split("concurrency:")[0])
        heavy = ("Setup Go", "Setup Java", "Cache formal tool jars", "Fetch and verify pinned tools", "Runner self-tests",
                 "Check models", "Trace validation", "Replay golden vectors and property tests", "Deep property tests")
        for name in heavy:
            with self.subTest(step=name):
                block = text.split(f"      - name: {name}\n", 1)[1].split("\n      - name: ", 1)[0]
                self.assertIn("if: steps.affected.outputs.run != 'false'", block)


class RepositoryManifestTests(unittest.TestCase):
    def test_repository_manifest_is_valid(self) -> None:
        _tools, models = formal.load_manifest()
        formal.validate_manifest(models, formal.load_trace_suites())
        self.assertTrue(models)
        for model in models:
            if model.tool == "alloy":
                formal.reject_alloy_source_commands(model, (formal.REPO_ROOT / model.file).read_text())
            else:
                formal.check_variable_groups(model.name, (formal.REPO_ROOT / model.file).read_text())
                for cmd in model.commands:
                    formal.check_tlc_config_text(f"{model.name}.{cmd.name}", formal.tlc_config(model, cmd))
            self.assertTrue(model.calibration, f"{model.name} is uncalibrated")
        self.assertEqual(formal.check_correspondence(models), [])
        formal.check_tla_sources()
        by_name = {m.name: m for m in models}
        for suite in formal.load_trace_suites():
            self.assertTrue(formal.proj_fields(suite.spec, (formal.TLA_DIR / suite.spec).read_text()))
            formal.trace_constants(suite, by_name[suite.name])

    def test_golden_headers_match_the_manifest(self) -> None:
        """Java-free freshness: a manifest-only edit of a golden command's body
        or scope must be caught before `make formal` runs."""
        tools, models = formal.load_manifest()
        golden = [m for m in models if m.golden]
        self.assertTrue(golden)
        for model in golden:
            with self.subTest(model=model.name):
                self.assertIsNone(formal.golden_staleness(tools, model))

    def test_manifest_only_golden_scope_edit_fails_the_digest_check(self) -> None:
        tools, models = formal.load_manifest()
        model = next(m for m in models if m.golden)
        commands = tuple(
            dataclasses.replace(c, scope="2") if c.name == model.golden["command"] else c for c in model.commands
        )
        stale = formal.golden_staleness(tools, dataclasses.replace(model, commands=commands))
        self.assertEqual(
            stale, f"{model.name}: {model.golden['output']} is stale; regenerate with: scripts/formal.py golden {model.name}"
        )

if __name__ == "__main__":
    unittest.main()
