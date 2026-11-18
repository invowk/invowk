#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Tests for change-flow-lenses.py."""

from __future__ import annotations

import contextlib
import importlib.util
import io
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.dont_write_bytecode = True
SCRIPT = Path(__file__).with_name("change-flow-lenses.py")
SPEC = importlib.util.spec_from_file_location("change_flow_lenses", SCRIPT)
assert SPEC and SPEC.loader
lenses = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(lenses)

ROOT = lenses.REPO_ROOT
TEMPLATE = ROOT / ".agents" / "workflows" / "change-flow.js"


def select(*lines: str, dir_has_go=None) -> dict:
    return lenses.select_lenses(lenses.parse_changes("\n".join(lines)), lenses.load_triggers(), dir_has_go)


class GlobTest(unittest.TestCase):
    def test_double_star_prefix_matches_any_depth(self) -> None:
        rx = lenses.glob_to_regex("**/*.cue")
        self.assertTrue(rx.match("invowkfile.cue"))
        self.assertTrue(rx.match("a/b/c.cue"))
        self.assertFalse(rx.match("a/b/c.cuex"))

    def test_single_star_stays_in_segment(self) -> None:
        rx = lenses.glob_to_regex("tests/cli/*.go")
        self.assertTrue(rx.match("tests/cli/cli_test.go"))
        self.assertFalse(rx.match("tests/cli/sub/x.go"))

    def test_trailing_double_star(self) -> None:
        rx = lenses.glob_to_regex("pkg/invowkfile/**")
        self.assertTrue(rx.match("pkg/invowkfile/a/b.go"))
        self.assertFalse(rx.match("pkg/invowkfilex/a.go"))


class SourceParsingTest(unittest.TestCase):
    def test_frontmatter_paths(self) -> None:
        text = '---\npaths:\n  - "a/**"\n  - \'b.go\'\nother: 1\n---\n# Body\n'
        self.assertEqual(lenses.frontmatter_paths(text, "x"), ["a/**", "b.go"])

    def test_frontmatter_errors(self) -> None:
        for text in ("# no frontmatter\n", "---\npaths:\n  - a\n", "---\nname: x\n---\n"):
            with self.subTest(text=text), self.assertRaises(lenses.LensError):
                lenses.frontmatter_paths(text, "x")

    def test_pgo_regex_only_from_its_hook(self) -> None:
        text = "  - id: other\n    files: '^nope/'\n  - id: pgo-staleness\n    files: '^(pkg/a/|internal/b/)'\n"
        rx = lenses.pgo_files_regex(text)
        self.assertTrue(rx.search("internal/b/x.go"))
        self.assertFalse(rx.search("nope/x.go"))
        with self.assertRaises(lenses.LensError):
            lenses.pgo_files_regex("  - id: other\n    files: '^x'\n")

    def test_live_sources_load(self) -> None:
        triggers = lenses.load_triggers()
        self.assertEqual(set(triggers), set(lenses.LENS_IDS) - {"architecture"})

    def test_missing_source_raises(self) -> None:
        with tempfile.TemporaryDirectory() as tmp, self.assertRaises(lenses.LensError):
            lenses.load_triggers(Path(tmp))

    def test_parse_changes(self) -> None:
        got = lenses.parse_changes("M\ta.go\nR087\told/x.go\tnew/x.go\nA\tb.md\nbare/path.txt\n\n")
        self.assertEqual(
            got,
            [("M", "a.go", "a.go"), ("R", "old/x.go", "new/x.go"), ("A", "b.md", "b.md"), ("M", "bare/path.txt", "bare/path.txt")],
        )


class AntiDriftTest(unittest.TestCase):
    def test_security_paths_exist_and_are_named_in_the_agent_file(self) -> None:
        text = (ROOT / lenses.SECURITY_AGENT).read_text(encoding="utf-8")
        areas = text.split("## High-Risk Code Areas", 1)[1].split("\n## ", 1)[0]
        for path in lenses.SECURITY_PATHS:
            with self.subTest(path=path):
                self.assertTrue((ROOT / path).is_dir(), f"{path} does not exist")
                self.assertIn(f"`{path}", areas, f"{path} is not named under High-Risk Code Areas")

    def test_supply_chain_paths_exist_and_are_named_in_the_skill(self) -> None:
        text = (ROOT / lenses.MODULE_SECURITY_SKILL).read_text(encoding="utf-8")
        for path in lenses.SUPPLY_CHAIN_PATHS:
            stem = path.rstrip("*")
            with self.subTest(path=path):
                if path.endswith("*"):
                    parent, prefix = stem.rsplit("/", 1)
                    self.assertTrue(any(p.name.startswith(prefix) for p in (ROOT / parent).iterdir()), f"no {path}")
                else:
                    self.assertTrue((ROOT / stem).exists(), f"{path} does not exist")
                self.assertIn(stem, text, f"{path} is not named in the module-security skill")
        self.assertIn("lock files", text.lower())

    def test_template_lens_ids_match(self) -> None:
        if not TEMPLATE.is_file():
            self.skipTest("template not present")
        text = TEMPLATE.read_text(encoding="utf-8")
        listed = text.split("const LENS_IDS = [", 1)[1].split("]", 1)[0]
        ids = [part.strip().strip("'\"") for part in listed.split(",") if part.strip()]
        self.assertEqual(tuple(ids), lenses.LENS_IDS)


class SelectionTest(unittest.TestCase):
    def test_docs_only_change_is_fast_lane(self) -> None:
        got = select("M\t.agents/runbooks/README.md", "M\tREADME.md")
        self.assertEqual(got["lenses"], [])
        self.assertTrue(got["fast_lane_eligible"])

    def test_empty_change_is_not_fast_lane(self) -> None:
        self.assertFalse(select()["fast_lane_eligible"])

    def test_formal_and_windows_and_schema(self) -> None:
        got = select("M\tpkg/invowkfile/command.go")
        for lens in ("formal", "windows", "schema", "perf"):
            self.assertIn(lens, got["lenses"])
        self.assertEqual(got["reasons"]["formal"], ["pkg/invowkfile/command.go"])
        self.assertFalse(got["fast_lane_eligible"])

    def test_ci_lens(self) -> None:
        got = select("M\t.github/workflows/lint.yml", "M\tgo.mod")
        self.assertEqual(got["lenses"], ["ci"])
        self.assertTrue(got["fast_lane_eligible"])

    def test_security_lens(self) -> None:
        got = select("M\tinternal/sshserver/server_auth.go")
        self.assertIn("security", got["lenses"])

    def test_supply_chain_lock_file_anywhere(self) -> None:
        got = select("M\tsamples/invowkmods/x.invowkmod/invowkmod.lock.cue")
        self.assertIn("supply-chain", got["lenses"])
        self.assertIn("schema", got["lenses"])

    def test_perf_lens(self) -> None:
        self.assertIn("perf", select("M\tpkg/cueutil/parse.go")["lenses"])
        self.assertIn("perf", select("M\tinternal/benchmark/benchmark_test.go")["lenses"])

    def test_go_test_files_block_fast_lane(self) -> None:
        got = select("M\tcmd/invowk/root_test.go")
        self.assertFalse(got["fast_lane_eligible"])

    def test_cue_outside_testdata_blocks_fast_lane(self) -> None:
        self.assertFalse(select("M\tsamples/x.cue")["fast_lane_eligible"])
        got = select("M\ttests/cli/testdata/fixture.cue")
        self.assertIn("schema", got["lenses"])  # **/*.cue still triggers the lens
        self.assertFalse(got["fast_lane_eligible"])

    def test_lens_order_is_fixed(self) -> None:
        got = select("M\tinternal/sshserver/x.go", "M\t.github/workflows/ci.yml", "M\tinternal/runtime/x.go")
        self.assertEqual(got["lenses"], [lens for lens in lenses.LENS_IDS if lens in got["lenses"]])


class ArchitectureTest(unittest.TestCase):
    def test_conservative_without_git(self) -> None:
        got = select("A\tinternal/newpkg/a.go")
        self.assertIn("architecture", got["lenses"])

    def test_added_file_in_existing_package(self) -> None:
        got = select("A\tinternal/runtime/extra.go", dir_has_go=lambda side, d: True)
        self.assertNotIn("architecture", got["lenses"])

    def test_new_package(self) -> None:
        got = select("A\tinternal/newpkg/a.go", dir_has_go=lambda side, d: False)
        self.assertEqual(got["reasons"]["architecture"], ["internal/newpkg/a.go"])

    def test_removed_package(self) -> None:
        got = select("D\tpkg/oldpkg/a.go", dir_has_go=lambda side, d: False)
        self.assertIn("architecture", got["lenses"])

    def test_test_files_and_other_roots_never_count(self) -> None:
        got = select("A\tinternal/newpkg/a_test.go", "A\ttools/x/a.go", "A\tinternal/x/testdata/a.go")
        self.assertNotIn("architecture", got["lenses"])

    def test_rename_within_dir_is_not_architecture(self) -> None:
        got = select("R100\tinternal/runtime/a.go\tinternal/runtime/b.go")
        self.assertNotIn("architecture", got["lenses"])

    def test_rename_across_dirs(self) -> None:
        got = select("R100\tinternal/a/x.go\tinternal/b/x.go", dir_has_go=lambda side, d: False)
        self.assertIn("architecture", got["lenses"])

    def test_git_dir_has_go(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)

            def run(*argv: str) -> str:
                return subprocess.run(["git", "-C", tmp, *argv], check=True, capture_output=True, text=True).stdout

            run("init", "-q")
            run("config", "user.email", "t@example.invalid")
            run("config", "user.name", "t")
            run("config", "commit.gpgsign", "false")
            (repo / "internal" / "old").mkdir(parents=True)
            (repo / "internal" / "old" / "a.go").write_text("package old\n")
            (repo / "internal" / "old" / "a_test.go").write_text("package old\n")
            run("add", "-A")
            run("commit", "-q", "-m", "base")
            base = run("rev-parse", "HEAD").strip()
            check = lenses.git_dir_has_go({"base": base, "head": base}, root=repo)
            self.assertTrue(check("base", "internal/old"))
            self.assertFalse(check("base", "internal/missing"))


class CliTest(unittest.TestCase):
    def run_main(self, *argv: str) -> tuple[int, dict]:
        out = io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(io.StringIO()):
            code = lenses.main(list(argv))
        return code, json.loads(out.getvalue())

    def test_paths_from(self) -> None:
        with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as handle:
            handle.write("M\tinternal/sshserver/x.go\n")
        try:
            code, got = self.run_main("--paths-from", handle.name)
        finally:
            Path(handle.name).unlink()
        self.assertEqual(code, 0)
        self.assertIn("security", got["lenses"])
        self.assertEqual(set(got), {"lenses", "fast_lane_eligible", "reasons"})

    def test_git_mode_empty_diff(self) -> None:
        code, got = self.run_main("--base", "HEAD", "--head", "HEAD")
        self.assertEqual(code, 0)
        self.assertEqual(got["lenses"], [])
        self.assertFalse(got["fast_lane_eligible"])

    def test_unreadable_sources_select_all(self) -> None:
        original = lenses.load_triggers

        def broken():
            raise lenses.LensError("boom")

        lenses.load_triggers = broken
        try:
            code, got = self.run_main("--paths-from", "/nonexistent")
        finally:
            lenses.load_triggers = original
        self.assertEqual(code, 0)
        self.assertEqual(got["lenses"], list(lenses.LENS_IDS))
        self.assertFalse(got["fast_lane_eligible"])


if __name__ == "__main__":
    unittest.main()
