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
import re
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

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
        self.assertIn("parity step 'Run CLI integration tests (full)': in ci.yml only", output)
        self.assertIn("parity step 'Run CLI tests (full)': in release.yml only", output)

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
        self.assertIn("exception SPLIT_STEPS['Run internal/runtime tests (full)']: no such ci.yml parity step", output)
        self.assertIn("ci.yml excludes internal/runtime and no ci.yml step split from it runs it", output)

    def test_unparsable_file_exits_2(self) -> None:
        rc, output = self.check("jobs:\n\ttest: {}\n", self.release_text)
        self.assertEqual(rc, 2, output)

    # Review findings on #185: each test below failed before its fix.

    def test_job_and_workflow_env_are_merged(self) -> None:
        release = self.replace(
            self.release_text,
            "    strategy:\n      fail-fast: true",
            '    env:\n      INVOWK_TEST_JOB: "1"\n    strategy:\n      fail-fast: true',
        )
        ci = self.replace(
            self.ci_text,
            '  GOPROXY: "https://proxy.golang.org|direct"\n',
            '  GOPROXY: "https://proxy.golang.org|direct"\n  INVOWK_TEST_WORKFLOW: "1"\n',
        )
        rc, output = self.check(ci, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("step 'Run tests (short)': env INVOWK_TEST_JOB ('1') in release.yml only", output)
        self.assertIn("step 'Run tests (short)': env INVOWK_TEST_WORKFLOW ('1') in ci.yml only", output)

    def test_step_env_overrides_job_env(self) -> None:
        # The same value moved from step to job level is still parity.
        release = self.replace(
            self.release_text,
            '          INVOWK_TEST_REQUIRE_TMUX: "1"\n',
            '          INVOWK_TEST_REQUIRE_TMUX: "1"\n          INVOWK_TEST_X: "step"\n',
        )
        release = self.replace(
            release, "    strategy:\n      fail-fast: true", '    env:\n      INVOWK_TEST_X: "job"\n    strategy:\n      fail-fast: true'
        )
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("step 'Run CLI integration tests (full)': env INVOWK_TEST_X ('step') in release.yml only", output)
        self.assertIn("step 'Run tests (short)': env INVOWK_TEST_X ('job') in release.yml only", output)

    def test_go_test_flags_are_compared(self) -> None:
        release = self.replace(self.release_text, "-- -race -timeout 10m", "-- -timeout 10m")
        release = self.replace(release, "-- -race -short -v", "-- -race -v")
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("step 'Run CLI integration tests (full)': test command is `gotestsum [./tests/cli/...] -race -timeout 10m` in ci.yml, `gotestsum [./tests/cli/...] -timeout 10m` in release.yml", output)
        self.assertIn("step 'Run tests (short)': test command is", output)
        self.assertEqual(output.count("\n  - "), 2, output)

    def test_test_packages_are_compared(self) -> None:
        release = self.replace(self.release_text, "-- -race -timeout 10m", "-- -race -timeout 10m -run TestX")
        release = release.replace("--packages ./tests/cli/... \\\n            --rerun-fails", "--packages ./tests/cli \\\n            --rerun-fails")
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("step 'Run CLI integration tests (full)': test command is", output)

    def test_coverage_profile_names_are_ignored(self) -> None:
        ci = self.replace(self.ci_text, "-coverprofile=cli-coverage.out\n\n      # Run short", "-coverprofile other.out\n\n      # Run short")
        rc, output = self.check(ci, self.release_text)
        self.assertEqual(rc, 0, output)

    def test_unreadable_package_filter_fails_closed(self) -> None:
        release = self.replace(
            self.release_text,
            "grep -v '^github.com/invowk/invowk/tests/cli$' |",
            "grep -v '^github.com/invowk/invowk/tests/cli$' | grep -vE '/internal/app/commandadapters$' |",
        )
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 2, output)
        self.assertIn("cannot read \"grep -vE '/internal/app/commandadapters$'\"", output)

    def test_new_ci_only_test_step_is_reported(self) -> None:
        ci = self.replace(
            self.ci_text,
            "      - name: Run PowerShell install script tests (pwsh)\n",
            "      - name: Run Windows-only tests\n        if: matrix.os == 'windows'\n        run: go test ./internal/winonly/...\n\n"
            "      - name: Run PowerShell install script tests (pwsh)\n",
        )
        rc, output = self.check(ci, self.release_text)
        self.assertEqual(rc, 1, output)
        self.assertIn("parity step 'Run Windows-only tests': in ci.yml only", output)

    def test_unconditional_test_step_is_selected(self) -> None:
        ci = self.replace(
            self.ci_text,
            "      - name: Run PowerShell install script tests (pwsh)\n",
            "      - name: Run extra tests\n        run: gotestsum --packages ./internal/extra\n\n"
            "      - name: Run PowerShell install script tests (pwsh)\n",
        )
        rc, output = self.check(ci, self.release_text)
        self.assertEqual(rc, 1, output)
        self.assertIn("parity step 'Run extra tests': in ci.yml only", output)

    def test_engine_setup_steps_are_parity_steps(self) -> None:
        release = self.replace(
            self.release_text,
            "      - name: Mask Docker for Podman tests\n        if: matrix.engine == 'podman'",
            "      - name: Mask Docker for Podman tests\n        if: matrix.engine == 'none'",
        )
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("step 'Mask Docker for Podman tests': if is", output)

    def test_unreadable_test_command_fails_closed(self) -> None:
        release = self.replace(self.release_text, "          gotestsum \\\n            --format testdox \\\n            --junitfile cli-test-results.xml \\\n            --rerun-fails \\\n            --rerun-fails-max-failures 3 \\\n            --rerun-fails-report cli-rerun-report.txt \\\n            --packages ./tests/cli/... \\\n            -- -race -timeout 10m",
                               "          cd . && gotestsum --packages ./tests/cli/... -- -race -timeout 10m")
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 2, output)
        self.assertIn("readable at the start of a line", output)

    def test_no_test_step_fails_closed(self) -> None:
        release = re.sub(r"(?m)^(\s*)gotestsum \\$", r"\1true \\", self.release_text)
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 2, output)
        self.assertIn("release.yml: jobs.test has no step that runs gotestsum or go test", output)

    def test_stale_only_in_exception_is_reported(self) -> None:
        with mock.patch.dict(MODULE.ONLY_IN, {("ci.yml", "Run tests (short)"): "stale"}):
            rc, output = self.check(self.ci_text, self.release_text)
        self.assertEqual(rc, 1, output)
        self.assertIn("exception ONLY_IN[ci.yml, 'Run tests (short)']: both workflows have this step", output)

    def test_flow_collections_are_parse_errors(self) -> None:
        for old, new in (
            (
                '        env:\n          INVOWK_TEST_CONTAINER_ENGINE: ${{ matrix.engine }}\n          INVOWK_TEST_REQUIRE_TMUX: "1"\n',
                '        env: {INVOWK_TEST_CONTAINER_ENGINE: "${{ matrix.engine }}", INVOWK_TEST_REQUIRE_TMUX: "1"}\n',
            ),
            (
                '          - os: macos\n            runner: macos-15\n            engine: ""\n            test-mode: short\n',
                '          - {os: macos, runner: macos-15, engine: "", test-mode: short}\n',
            ),
        ):
            with self.subTest(new=new):
                rc, output = self.check(self.ci_text, self.replace(self.release_text, old, new))
                self.assertEqual(rc, 2, output)
                self.assertIn("flow collection", output)

    def test_split_step_compares_every_variable_it_does_not_omit(self) -> None:
        ci = self.replace(self.ci_text, "          INVOWK_TEST_TUI_PLAN_ROW: ${{ matrix.tui-plan-row }}\n",
                          '          INVOWK_TEST_TUI_PLAN_ROW: ${{ matrix.tui-plan-row }}\n          INVOWK_TEST_FOO: "1"\n')
        release = self.replace(self.release_text, "          INVOWK_TEST_TUI_PLAN_ROW: ${{ matrix.tui-plan-row }}\n",
                               '          INVOWK_TEST_TUI_PLAN_ROW: ${{ matrix.tui-plan-row }}\n          INVOWK_TEST_FOO: "1"\n')
        rc, output = self.check(ci, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("step 'Run internal/runtime tests (full)': env INVOWK_TEST_FOO ('1') in release.yml only", output)
        self.assertEqual(output.count("\n  - "), 1, output)

    def test_split_step_must_not_set_an_omitted_variable(self) -> None:
        ci = self.replace(
            self.ci_text,
            '          INVOWK_TEST_CONTAINER_PARALLEL: "2"\n        run: |\n          gotestsum',
            '          INVOWK_TEST_CONTAINER_PARALLEL: "2"\n          INVOWK_TEST_TUI_PLAN_ROW: "8"\n        run: |\n          gotestsum',
        )
        rc, output = self.check(ci, self.release_text)
        self.assertEqual(rc, 1, output)
        self.assertIn("sets INVOWK_TEST_TUI_PLAN_ROW, which its SPLIT_STEPS entry omits", output)

    def test_split_step_flags_must_match_its_origin(self) -> None:
        ci = self.replace(self.ci_text, "-- -race -timeout 15m -coverprofile=runtime-coverage.out", "-- -timeout 15m -coverprofile=runtime-coverage.out")
        rc, output = self.check(ci, self.release_text)
        self.assertEqual(rc, 1, output)
        self.assertIn("step 'Run internal/runtime tests (full)': test flags", output)

    def test_split_package_must_be_exact(self) -> None:
        ci = self.replace(self.ci_text, "--packages ./internal/runtime \\", "--packages ./internal/runtime/... \\")
        rc, output = self.check(ci, self.release_text)
        self.assertEqual(rc, 1, output)
        self.assertIn("runs './internal/runtime/...', not one ./<pkg>", output)

    def test_blame_names_release_when_it_also_excludes_the_split_package(self) -> None:
        release = self.replace(
            self.release_text,
            "grep -v '^github.com/invowk/invowk/tests/cli$' |",
            "grep -v '^github.com/invowk/invowk/tests/cli$' | grep -v '^github.com/invowk/invowk/internal/runtime$' |",
        )
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("release.yml excludes internal/runtime, which a split ci.yml step runs, so release.yml runs it nowhere", output)
        self.assertNotIn("this ci.yml step does not exclude it", output)

    def test_stale_env_and_matrix_exceptions_are_reported(self) -> None:
        with (
            mock.patch.dict(MODULE.ENV_EXCEPTIONS, {("Run tests (short)", "INVOWK_TEST_NONE"): "stale"}),
            mock.patch.dict(MODULE.MATRIX_KEY_EXCEPTIONS, {("macos", "", "runner"): "stale"}),
        ):
            rc, output = self.check(self.ci_text, self.release_text)
        self.assertEqual(rc, 1, output)
        self.assertIn("exception ENV_EXCEPTIONS[('Run tests (short)', 'INVOWK_TEST_NONE')]: matches no difference", output)
        self.assertIn("exception MATRIX_KEY_EXCEPTIONS[('macos', '', 'runner')]: matches no difference", output)

    def test_used_exceptions_are_not_stale(self) -> None:
        release = self.replace(self.release_text, "runner: macos-15", "runner: macos-26")
        release = self.replace(release, 'INVOWK_TEST_REQUIRE_TMUX: "1"', 'INVOWK_TEST_REQUIRE_TMUX: "0"')
        with (
            mock.patch.dict(MODULE.ENV_EXCEPTIONS, {("Run CLI integration tests (full)", "INVOWK_TEST_REQUIRE_TMUX"): "r"}),
            mock.patch.dict(MODULE.MATRIX_KEY_EXCEPTIONS, {("macos", "", "runner"): "r"}),
        ):
            rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 0, output)

    def test_matrix_keys_beyond_include_are_compared(self) -> None:
        release = self.replace(self.release_text, "        include:\n", '        exclude:\n          - os: macos\n            engine: ""\n        include:\n')
        rc, output = self.check(self.ci_text, release)
        self.assertEqual(rc, 1, output)
        self.assertIn("matrix key 'exclude' differs", output)


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
