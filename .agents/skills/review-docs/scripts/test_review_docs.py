#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Tests for the deterministic review-docs audit tooling."""

from __future__ import annotations

import importlib.util
import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from types import SimpleNamespace


SCRIPT_PATH = Path(__file__).with_name("review_docs.py")
sys.dont_write_bytecode = True
SPEC = importlib.util.spec_from_file_location("review_docs", SCRIPT_PATH)
assert SPEC and SPEC.loader
review_docs = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = review_docs
SPEC.loader.exec_module(review_docs)


class ReviewDocsTests(unittest.TestCase):
    repo_root = review_docs.DEFAULT_REPO_ROOT

    def context(self) -> dict:
        context = {
            "schema_version": 1,
            "audit_date": "2026-07-10",
            "snapshot": {"git_head": "head", "workspace_sha256": "snapshot"},
            "contract": {},
            "inventories": {},
            "ownership": {"status": "PASS"},
            "i18n_lag": {
                "status": "PASS",
                "threshold_days": 60,
                "limit": 3,
                "candidates": [],
                "blocked": [],
            },
            "programmatic_checks": {
                name: {"status": "PASS", "detail": "", "exit_code": 0}
                for name in review_docs.REQUIRED_PROGRAMMATIC_CHECKS
            },
        }
        context["context_id"] = review_docs.canonical_sha256(context)
        return context

    def passing_result(self, surface: str, checks: dict[str, review_docs.Check]) -> dict:
        ownership_targets = review_docs.ownership_targets_by_check()
        return {
            "schema_version": 1,
            "surface": surface,
            "context_id": self.context()["context_id"],
            "items": [
                {
                    "check_id": check_id,
                    "status": "PASS",
                    "targets": sorted(ownership_targets.get(check_id, {"README.md"})),
                    "evidence": ["verified against exact source"],
                    "findings": [],
                }
                for check_id in review_docs.expected_ids(checks, surface)
            ],
            "candidates": [],
        }

    def test_current_contract_is_complete(self) -> None:
        summary = review_docs.validate_contract(self.repo_root)
        self.assertEqual(summary["check_count"], 136)
        self.assertEqual(summary["document_count"], 61)
        self.assertEqual(summary["surface_counts"]["S2"], 29)
        self.assertEqual(summary["surface_counts"]["S5"], 13)

    def test_workspace_hash_detects_same_status_content_change(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.git(root, "init")
            self.git(root, "config", "user.email", "test@example.com")
            self.git(root, "config", "user.name", "Test")
            path = root / "tracked.txt"
            path.write_text("initial", encoding="utf-8")
            self.git(root, "add", "tracked.txt")
            self.git(root, "commit", "-m", "initial")
            path.write_text("first modification", encoding="utf-8")
            first = review_docs.workspace_sha256(root)
            path.write_text("second modification", encoding="utf-8")
            second = review_docs.workspace_sha256(root)
            self.assertNotEqual(first, second)
            self.assertTrue(self.git(root, "status", "--porcelain").startswith(" M"))

    def test_i18n_lag_uses_actual_commit_dates(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.git(root, "init")
            self.git(root, "config", "user.email", "test@example.com")
            self.git(root, "config", "user.name", "Test")
            english = root / "website/docs/example.mdx"
            locale = root / (
                "website/i18n/pt-BR/docusaurus-plugin-content-docs/current/example.mdx"
            )
            english.parent.mkdir(parents=True)
            locale.parent.mkdir(parents=True)
            english.write_text("v1", encoding="utf-8")
            locale.write_text("v1", encoding="utf-8")
            self.git(root, "add", ".")
            self.git(root, "commit", "-m", "initial", date="2025-01-01T00:00:00+00:00")
            english.write_text("v2", encoding="utf-8")
            self.git(root, "add", str(english.relative_to(root)))
            self.git(root, "commit", "-m", "english update", date="2025-03-03T00:00:01+00:00")
            result = review_docs.i18n_lag(root, ["website/docs/example.mdx"], 60, 3)
            self.assertEqual(result["status"], "PASS")
            self.assertEqual(result["candidates"][0]["lag_days"], 61)

    def test_i18n_lag_excludes_exact_threshold(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.git(root, "init")
            self.git(root, "config", "user.email", "test@example.com")
            self.git(root, "config", "user.name", "Test")
            english = root / "website/docs/example.mdx"
            locale = root / (
                "website/i18n/pt-BR/docusaurus-plugin-content-docs/current/example.mdx"
            )
            english.parent.mkdir(parents=True)
            locale.parent.mkdir(parents=True)
            english.write_text("v1", encoding="utf-8")
            locale.write_text("v1", encoding="utf-8")
            self.git(root, "add", ".")
            self.git(root, "commit", "-m", "initial", date="2025-01-01T00:00:00+00:00")
            english.write_text("v2", encoding="utf-8")
            self.git(root, "add", str(english.relative_to(root)))
            self.git(root, "commit", "-m", "english update", date="2025-03-02T00:00:00+00:00")
            result = review_docs.i18n_lag(root, ["website/docs/example.mdx"], 60, 3)
            self.assertEqual(result["candidates"], [])

    def test_prepare_writes_canonical_external_context(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            output_root = Path(directory)
            checks_path = output_root / "checks.json"
            output_path = output_root / "context.json"
            checks_path.write_text(
                json.dumps(
                    {
                        name: {"status": "PASS", "detail": "ok", "exit_code": 0}
                        for name in review_docs.REQUIRED_PROGRAMMATIC_CHECKS
                    }
                ),
                encoding="utf-8",
            )
            args = SimpleNamespace(
                repo_root=self.repo_root,
                checks=checks_path,
                output=output_path,
                lag_days=60,
                lag_limit=3,
            )
            with redirect_stdout(io.StringIO()):
                self.assertEqual(review_docs.prepare_context(args), 0)
            context = json.loads(output_path.read_text(encoding="utf-8"))
            self.assertEqual(context["contract"]["check_count"], 136)
            self.assertEqual(context["ownership"]["status"], "PASS")
            self.assertEqual(len(context["snapshot"]["workspace_sha256"]), 64)
            review_docs.validate_context(self.repo_root, context, verify_snapshot=True)

    def test_result_requires_every_check(self) -> None:
        checks = review_docs.parse_checks()
        result = self.passing_result("S1", checks)
        result["items"].pop()
        with self.assertRaisesRegex(review_docs.ContractError, "checklist mismatch"):
            review_docs.validate_result(self.repo_root, self.context(), result, checks)

    def test_context_id_rejects_tampering(self) -> None:
        context = self.context()
        context["programmatic_checks"]["docs-parity"]["detail"] = "tampered"
        with self.assertRaisesRegex(review_docs.ContractError, "context_id"):
            review_docs.validate_context(self.repo_root, context, verify_snapshot=False)

    def test_fail_requires_complete_finding(self) -> None:
        checks = review_docs.parse_checks()
        result = self.passing_result("S1", checks)
        result["items"][0].update({"status": "FAIL", "findings": []})
        with self.assertRaisesRegex(review_docs.ContractError, "requires evidence and at least"):
            review_docs.validate_result(self.repo_root, self.context(), result, checks)

    def test_skip_requires_registered_simplification(self) -> None:
        checks = review_docs.parse_checks()
        result = self.passing_result("S1", checks)
        result["items"][0].update(
            {"status": "SKIP", "simplification_id": "IS-999", "findings": []}
        )
        with self.assertRaisesRegex(review_docs.ContractError, "whole-check exemption"):
            review_docs.validate_result(self.repo_root, self.context(), result, checks)

    def test_result_requires_every_owned_page_target(self) -> None:
        checks = review_docs.parse_checks()
        result = self.passing_result("S2", checks)
        core_item = next(item for item in result["items"] if item["check_id"] == "S2-C16")
        core_item["targets"].pop()
        with self.assertRaisesRegex(review_docs.ContractError, "omits owned targets"):
            review_docs.validate_result(self.repo_root, self.context(), result, checks)

    def test_blocked_result_makes_report_incomplete(self) -> None:
        checks = review_docs.parse_checks()
        results = [self.passing_result(surface, checks) for surface in review_docs.SURFACE_IDS]
        results[0]["items"][0].update(
            {"status": "BLOCKED", "reason": "tool unavailable", "evidence": []}
        )
        validated = [
            review_docs.validate_result(self.repo_root, self.context(), result, checks)
            for result in results
        ]
        report = review_docs.merge_results(self.repo_root, self.context(), validated, checks)
        self.assertEqual(report["verdict"], "INCOMPLETE")
        self.assertEqual(report["checklist_completion"]["S1"]["BLOCKED"], 1)
        self.assertEqual(report["blockers"][0]["reason"], "tool unavailable")
        self.assertIn("tool unavailable", review_docs.report_markdown(report))

    def test_merge_is_permutation_invariant(self) -> None:
        checks = review_docs.parse_checks()
        results = [self.passing_result(surface, checks) for surface in review_docs.SURFACE_IDS]
        validated = [
            review_docs.validate_result(self.repo_root, self.context(), result, checks)
            for result in results
        ]
        forward = review_docs.merge_results(self.repo_root, self.context(), validated, checks)
        reverse = review_docs.merge_results(
            self.repo_root, self.context(), list(reversed(validated)), checks
        )
        self.assertEqual(forward, reverse)

    @staticmethod
    def git(root: Path, *args: str, date: str | None = None) -> str:
        env = os.environ.copy()
        if date:
            env["GIT_AUTHOR_DATE"] = date
            env["GIT_COMMITTER_DATE"] = date
        result = subprocess.run(
            ["git", *args],
            cwd=root,
            env=env,
            check=True,
            text=True,
            capture_output=True,
        )
        return result.stdout


if __name__ == "__main__":
    unittest.main()
