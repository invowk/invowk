#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Tests for formal.py: every fail-closed path must fail, not pass."""

from __future__ import annotations

import dataclasses
import gzip
import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path

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
    return Model(name=name, tool="alloy", file="m.als", commands=commands, calibration="seeded", scope=scope)


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
    return Model(name="T", tool="tla", file="t.tla", commands=cmds, calibration="seeded")


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

    def test_symmetry_in_liveness_config_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "SYMMETRY or VIEW"):
            formal.check_tlc_config_text("c", "SPECIFICATION Spec\nPROPERTY Live\nSYMMETRY Perms\n")
        formal.check_tlc_config_text("c", "SPECIFICATION Spec\nINVARIANT Safe\nSYMMETRY Perms\n")

    def test_liveness_without_fairness_twin_fails(self) -> None:
        live = Command("live", "pass", property="Live", temporal=True)
        guard = Command("mut", "counterexample", property="Live", mutant_of="live", temporal=True)
        with self.assertRaisesRegex(FormalError, "fairness-free twin"):
            formal.validate_manifest([tla_model(live, guard)])
        twin = Command("unfair", "counterexample", property="Live", temporal=True, spec="UnfairSpec",
                       mutant_of="live", fairness_twin_of="live")
        formal.validate_manifest([tla_model(live, twin)])

    def test_finding_does_not_guard_its_property(self) -> None:
        fixed = Command("fixed", "pass", property="P")
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


class RepositoryManifestTests(unittest.TestCase):
    def test_repository_manifest_is_valid(self) -> None:
        _tools, models = formal.load_manifest()
        formal.validate_manifest(models)
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
        for suite in formal.load_trace_suites():
            self.assertTrue(formal.proj_fields(suite.spec, (formal.TLA_DIR / suite.spec).read_text()))

    def test_golden_headers_match_the_manifest(self) -> None:
        """Java-free freshness: a manifest-only edit of a golden command's body
        or scope must be caught before `make formal` runs."""
        _tools, models = formal.load_manifest()
        golden = [m for m in models if m.golden]
        self.assertTrue(golden)
        for model in golden:
            with self.subTest(model=model.name):
                header, _ = formal.decode_golden(formal.REPO_ROOT / model.golden["output"])
                self.assertEqual(formal.golden_header_problems(model, header), [])

    def test_manifest_only_golden_scope_edit_fails_the_digest_check(self) -> None:
        _tools, models = formal.load_manifest()
        model = next(m for m in models if m.golden)
        header, _ = formal.decode_golden(formal.REPO_ROOT / model.golden["output"])
        commands = tuple(
            dataclasses.replace(c, scope="2") if c.name == model.golden["command"] else c for c in model.commands
        )
        problems = formal.golden_header_problems(dataclasses.replace(model, commands=commands), header)
        self.assertEqual(len(problems), 1)
        self.assertIn(f"{model.name}: the golden command in formal/manifest.toml changed", problems[0])


if __name__ == "__main__":
    unittest.main()
