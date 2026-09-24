#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Tests for formal.py: every fail-closed path must fail, not pass."""

from __future__ import annotations

import importlib.util
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


def alloy_model(*commands: Command, name: str = "M") -> Model:
    return Model(name=name, tool="alloy", file="m.als", commands=commands, calibration="seeded")


GUARDED = (
    Command("safe", "pass", property="prop", antecedent="ante"),
    Command("ante", "instance"),
    Command("mut", "counterexample", property="prop", mutant_of="safe"),
)


class ManifestGuardTests(unittest.TestCase):
    def test_guarded_model_is_accepted(self) -> None:
        formal.validate_manifest([alloy_model(*GUARDED)])

    def test_unguarded_property_fails(self) -> None:
        model = alloy_model(Command("safe", "pass", property="prop", antecedent="ante"), Command("ante", "instance"))
        with self.assertRaisesRegex(FormalError, "unguarded property"):
            formal.validate_manifest([model])

    def test_mutant_of_other_property_fails(self) -> None:
        model = alloy_model(*GUARDED[:2], Command("mut", "counterexample", property="other", mutant_of="safe"))
        with self.assertRaisesRegex(FormalError, "mutant checks property"):
            formal.validate_manifest([model])

    def test_mutant_expecting_pass_fails(self) -> None:
        model = alloy_model(*GUARDED[:2], Command("mut", "pass", property="prop", mutant_of="safe"))
        with self.assertRaisesRegex(FormalError, "must expect a counterexample"):
            formal.validate_manifest([model])

    def test_alloy_check_without_antecedent_fails(self) -> None:
        model = alloy_model(
            Command("safe", "pass", property="prop"),
            Command("witness", "instance"),
            Command("mut", "counterexample", property="prop", mutant_of="safe"),
        )
        with self.assertRaisesRegex(FormalError, "antecedent"):
            formal.validate_manifest([model])

    def test_alloy_model_without_run_fails(self) -> None:
        model = alloy_model(Command("c", "counterexample"))
        with self.assertRaisesRegex(FormalError, "non-vacuity"):
            formal.validate_manifest([model])

    def test_unknown_verdict_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "unknown expected verdict"):
            formal.validate_manifest([alloy_model(*GUARDED, Command("x", "maybe"))])


ALLOY_SOURCE = """
pred prop { some univ }
safe: check { prop } for 3 expect 0
ante: run { some univ } for 3 expect 1
mut: check { prop } for 3 expect 1
"""


class AlloySourceTests(unittest.TestCase):
    def test_matching_source_is_accepted(self) -> None:
        formal.cross_check_alloy_source(alloy_model(*GUARDED), ALLOY_SOURCE)

    def test_expect_annotation_mismatch_fails(self) -> None:
        source = ALLOY_SOURCE.replace("safe: check { prop } for 3 expect 0", "safe: check { prop } for 3 expect 1")
        with self.assertRaisesRegex(FormalError, "source says `expect 1`"):
            formal.cross_check_alloy_source(alloy_model(*GUARDED), source)

    def test_command_missing_from_manifest_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "missing from the manifest"):
            formal.cross_check_alloy_source(alloy_model(*GUARDED), ALLOY_SOURCE + "extra: run {} for 1 expect 1\n")

    def test_check_not_referencing_property_fails(self) -> None:
        source = ALLOY_SOURCE.replace("safe: check { prop }", "safe: check { some univ }")
        with self.assertRaisesRegex(FormalError, "does not reference property"):
            formal.cross_check_alloy_source(alloy_model(*GUARDED), source)

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


def tla_model(cmd: Command) -> Model:
    return Model(name="T", tool="tla", file="t.tla", commands=(cmd,), calibration="seeded")


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
        live = Command("live", "pass", config="Live.cfg")
        model = Model(name="T", tool="tla", file="t.tla", commands=(live,), calibration="seeded")
        with self.assertRaisesRegex(FormalError, "fairness-free twin"):
            formal.check_fairness_twins(model, {"live": "SPECIFICATION Spec\nPROPERTY Live\n"})
        twin = Command("unfair", "counterexample", config="Unfair.cfg", fairness_twin_of="live")
        paired = Model(name="T", tool="tla", file="t.tla", commands=(live, twin), calibration="seeded")
        formal.check_fairness_twins(paired, {"live": "PROPERTY Live\n", "unfair": "PROPERTY Live\n"})

    def test_temporal_violation_is_attributed_to_single_property(self) -> None:
        result = formal.parse_tlc_output("Error: Temporal properties were violated.\n")
        attributed = formal.attribute_temporal_violation(result, "SPECIFICATION S\nPROPERTY NoLostBurst\n")
        self.assertEqual(attributed.violated, "NoLostBurst")
        ambiguous = formal.attribute_temporal_violation(result, "PROPERTY A\nPROPERTY B\n")
        self.assertEqual(ambiguous.violated, "<temporal>")

    def test_disabled_deadlock_check_fails(self) -> None:
        with self.assertRaisesRegex(FormalError, "deadlock checking must stay on"):
            formal.check_tlc_config_text("c", "SPECIFICATION Spec\nCHECK_DEADLOCK FALSE\n")


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
            formal.validate_manifest([alloy_model(*GUARDED, Command("w", "pass", witness=True))])

    def test_correspondence_missing_binding_fails(self) -> None:
        model = self.write_model("// | e | `RealSymbol` | `code.go` | TestGone | - |\n")
        self.assertIn("binding test TestGone", formal.check_correspondence([model], root=self.root)[0])

    def test_correspondence_without_table_fails(self) -> None:
        (self.root / "m.als").write_text("module m\n")
        self.assertIn("no correspondence table", formal.check_correspondence([alloy_model(*GUARDED)], root=self.root)[0])


class RepositoryManifestTests(unittest.TestCase):
    def test_repository_manifest_is_valid(self) -> None:
        _tools, models = formal.load_manifest()
        formal.validate_manifest(models)
        self.assertTrue(models)
        for model in models:
            if model.tool == "alloy":
                formal.cross_check_alloy_source(model, (formal.REPO_ROOT / model.file).read_text())
            else:
                spec_dir = (formal.REPO_ROOT / model.file).parent
                configs = {c.name: (spec_dir / c.config).read_text() for c in model.commands}
                formal.check_fairness_twins(model, configs)
                for name, text in configs.items():
                    formal.check_tlc_config_text(f"{model.name}.{name}", text)
            self.assertTrue(model.calibration, f"{model.name} is uncalibrated")
        self.assertEqual(formal.check_correspondence(models), [])


if __name__ == "__main__":
    unittest.main()
