#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Tests for check-ci-release-parity.py.

Calibration fixtures (scripts/testdata/ci-release-parity/), so the test needs
no git history:
  - ci-567e7b00.yml, release-567e7b00.yml: both workflows at 567e7b00, before
    #183. That release.yml lacks `tui-plan-row`, INVOWK_TEST_TUI_PLAN_ROW and
    INVOWK_TEST_REQUIRE_TMUX, and the first v0.17.0 tag run failed on it: the
    check must fail there.
  - release-861e63c4.yml: release.yml as #183 fixed it: the check must pass
    against the 567e7b00 ci.yml (ci.yml did not change in #183).
The live .github/workflows files must pass too, so `make test-scripts` (lint.yml's
"script tests (worktree-safety gate)" job) fails on every PR that lets them drift.
"""

from __future__ import annotations

import contextlib
import importlib.util
import io
import sys
import tempfile
import unittest
from pathlib import Path

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
SCRIPT = HERE / "check-ci-release-parity.py"
FIXTURES = HERE / "testdata" / "ci-release-parity"
SPEC = importlib.util.spec_from_file_location("check_ci_release_parity", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def run(ci: Path, release: Path) -> tuple[int, str]:
    out, err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
        rc = MODULE.main(["--ci", str(ci), "--release", str(release)])
    return rc, out.getvalue() + err.getvalue()


class CalibrationTests(unittest.TestCase):
    def test_fails_on_release_yml_before_183(self) -> None:
        rc, output = run(FIXTURES / "ci-567e7b00.yml", FIXTURES / "release-567e7b00.yml")
        self.assertEqual(rc, 1, output)
        for line in (
            "matrix leg ubuntu-latest/docker: key 'tui-plan-row' ('8') in ci.yml only",
            "matrix leg ubuntu-latest/podman: key 'tui-plan-row' ('11') in ci.yml only",
            "step 'Run tests (full, non-CLI)': env INVOWK_TEST_TUI_PLAN_ROW",
            "step 'Run CLI integration tests (full)': env INVOWK_TEST_REQUIRE_TMUX ('1') in ci.yml only",
        ):
            self.assertIn(line, output)
        self.assertEqual(output.count("\n  - "), 4, output)

    def test_passes_on_release_yml_after_183(self) -> None:
        rc, output = run(FIXTURES / "ci-567e7b00.yml", FIXTURES / "release-861e63c4.yml")
        self.assertEqual(rc, 0, output)

    def test_passes_on_live_workflows(self) -> None:
        rc, output = run(MODULE.DEFAULT_CI, MODULE.DEFAULT_RELEASE)
        self.assertEqual(rc, 0, output)


class DriftTests(unittest.TestCase):
    """Mutations of the fixed pair, one drift each."""

    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.ci_text = (FIXTURES / "ci-567e7b00.yml").read_text(encoding="utf-8")
        self.release_text = (FIXTURES / "release-861e63c4.yml").read_text(encoding="utf-8")

    def check(self, ci_text: str, release_text: str) -> tuple[int, str]:
        ci = Path(self.tmp.name) / "ci.yml"
        release = Path(self.tmp.name) / "release.yml"
        ci.write_text(ci_text, encoding="utf-8")
        release.write_text(release_text, encoding="utf-8")
        return run(ci, release)

    def replace(self, text: str, old: str, new: str) -> str:
        self.assertEqual(text.count(old), 1, old)
        return text.replace(old, new)

    def test_env_value_drift(self) -> None:
        release = self.replace(self.release_text, 'INVOWK_TEST_REQUIRE_TMUX: "1"', 'INVOWK_TEST_REQUIRE_TMUX: "0"')
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("env INVOWK_TEST_REQUIRE_TMUX is '1' in ci.yml, '0' in release.yml", output)

    def test_env_only_in_release(self) -> None:
        release = self.replace(
            self.release_text,
            '          INVOWK_TEST_REQUIRE_TMUX: "1"\n',
            '          INVOWK_TEST_REQUIRE_TMUX: "1"\n          INVOWK_TEST_EXTRA: "1"\n',
        )
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("env INVOWK_TEST_EXTRA ('1') in release.yml only", output)

    def test_matrix_value_and_leg_drift(self) -> None:
        release = self.replace(self.release_text, 'tui-plan-row: "11"', 'tui-plan-row: "12"')
        release = self.replace(release, "runner: macos-15", "runner: macos-26")
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("matrix leg ubuntu-latest/podman: key 'tui-plan-row' is '11' in ci.yml, '12' in release.yml", output)
        self.assertIn("matrix leg macos: key 'runner' is 'macos-15' in ci.yml, 'macos-26' in release.yml", output)

    def test_missing_leg(self) -> None:
        release = self.replace(self.release_text, "          - os: macos\n", "          - os: macos-x\n")
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("matrix leg macos: in ci.yml only", output)
        self.assertIn("matrix leg macos-x: in release.yml only", output)

    def test_test_step_only_in_one_file(self) -> None:
        release = self.replace(self.release_text, "- name: Run CLI integration tests (full)", "- name: Run CLI tests (full)")
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("test step 'Run CLI integration tests (full)': in ci.yml only", output)
        self.assertIn("test step 'Run CLI tests (full)': in release.yml only", output)

    def test_condition_drift(self) -> None:
        release = self.replace(
            self.release_text,
            "      - name: Run CLI integration tests\n        if: matrix.test-mode == 'short'",
            "      - name: Run CLI integration tests\n        if: matrix.test-mode == 'short' && matrix.os != 'macos'",
        )
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("step 'Run CLI integration tests': if is", output)

    def test_split_runtime_step_must_still_run_runtime(self) -> None:
        # ci.yml keeps excluding internal/runtime but its own step no longer runs it.
        ci = self.replace(self.ci_text, "--packages ./internal/runtime \\", "--packages ./internal/other \\")
        rc, output = self.check(ci, self.release_text)
        self.assertEqual(rc, 1, output)
        self.assertIn("ci.yml excludes internal/runtime and no ci.yml step split from it runs it", output)
        self.assertIn("a split ci.yml step runs internal/other, but this ci.yml step does not exclude it", output)

    def test_split_runtime_step_env_must_match_its_origin(self) -> None:
        ci = self.replace(
            self.ci_text,
            '          INVOWK_TEST_CONTAINER_PARALLEL: "2"\n        run: |\n          gotestsum',
            '          INVOWK_TEST_CONTAINER_PARALLEL: "4"\n        run: |\n          gotestsum',
        )
        rc, output = self.check(ci, self.release_text)
        self.assertEqual(rc, 1, output)
        self.assertIn("step 'Run internal/runtime tests (full)': env INVOWK_TEST_CONTAINER_PARALLEL is '4' in ci.yml, '2' in release.yml", output)

    def test_stale_split_exception_is_reported(self) -> None:
        start = self.ci_text.index("      - name: Run internal/runtime tests (full)")
        end = self.ci_text.index("      # The full legs are Linux")
        ci = self.ci_text[:start] + self.ci_text[end:]
        rc, output = self.check(ci, self.release_text)
        self.assertEqual(rc, 1, output)
        self.assertIn("exception SPLIT_STEPS['Run internal/runtime tests (full)']: no such ci.yml test step", output)
        self.assertIn("ci.yml excludes internal/runtime and no ci.yml step split from it runs it", output)

    def test_unparsable_file_exits_2(self) -> None:
        rc, output = self.check("jobs:\n\ttest: {}\n", self.release_text)
        self.assertEqual(rc, 2, output)


class ParserTests(unittest.TestCase):
    def test_subset(self) -> None:
        doc = MODULE.load_yaml(
            "on:\n  push:\n    branches: [main]  # c\n"
            "jobs:\n  test:\n    env:\n      A: 'x # y'\n      B: \"1\"\n"
            "    steps:\n      - name: s\n        run: |\n          echo a: b\n          - x\n        if: c\n"
            "      - uses: u\n"
        )
        self.assertEqual(doc["on"]["push"]["branches"], "[main]")
        self.assertEqual(doc["jobs"]["test"]["env"], {"A": "x # y", "B": "1"})
        steps = doc["jobs"]["test"]["steps"]
        self.assertEqual(steps[0], {"name": "s", "run": "echo a: b\n- x", "if": "c"})
        self.assertEqual(steps[1], {"uses": "u"})


if __name__ == "__main__":
    unittest.main()
