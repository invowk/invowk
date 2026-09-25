#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Tests for formal_promotion_gate.py: the gate must never PASS vacuously, and
must send GET requests only."""

from __future__ import annotations

import contextlib
import datetime as dt
import importlib.util
import io
import json
import sys
import unittest
import urllib.error
from pathlib import Path
from unittest import mock

sys.dont_write_bytecode = True
SCRIPT = Path(__file__).with_name("formal_promotion_gate.py")
SPEC = importlib.util.spec_from_file_location("formal_promotion_gate", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
gate = importlib.util.module_from_spec(SPEC)
sys.modules["formal_promotion_gate"] = gate
SPEC.loader.exec_module(gate)

NOW = dt.datetime(2026, 11, 2, 12, 0, tzinfo=dt.UTC)


def run(run_id: int, days_ago: float, attempt: int = 1, conclusion: str = "success") -> dict:
    created = (NOW - dt.timedelta(days=days_ago)).strftime("%Y-%m-%dT%H:%M:%SZ")
    return {"id": run_id, "event": "schedule", "head_branch": "main", "status": "completed", "created_at": created,
            "conclusion": conclusion, "run_attempt": attempt, "head_sha": f"{run_id:040x}"}


def formal_job(steps: dict[str, str] | None = None, conclusion: str = "success") -> dict:
    names = dict.fromkeys(("Checkout", *gate.REQUIRED_STEPS), "success") if steps is None else steps
    return {"name": gate.JOB_NAME, "conclusion": conclusion,
            "steps": [{"name": name, "conclusion": result} for name, result in names.items()]}


PRE_CHANGE_JOB = formal_job({"Checkout": "success", "Check models": "success", "Trace validation": "success"})
JUNIT_JOB = {"name": "JUnit Report (ubuntu-latest)", "conclusion": "failure", "steps": []}


def weekly(count: int = 4, newest_days_ago: float = 2) -> tuple[list[dict], dict[int, list[dict]]]:
    runs = [run(100 + k, newest_days_ago + 7 * k) for k in range(count)]
    return runs, {r["id"]: [formal_job()] for r in runs}


class EvaluateTests(unittest.TestCase):
    def evaluate(self, runs: list[dict], jobs: dict[int, list[dict]], since: dt.datetime | None = None):
        return gate.evaluate(runs, jobs.__getitem__, NOW, since)

    def assert_fails(self, verdict, message: str) -> None:
        self.assertFalse(verdict.passed)
        self.assertTrue(any(message in f for f in verdict.failures), verdict.failures)

    def test_pass(self) -> None:
        verdict = self.evaluate(*weekly())
        self.assertTrue(verdict.passed, verdict.failures)
        self.assertEqual([r["id"] for r in verdict.counted], [100, 101, 102, 103])
        text = gate.render_text(verdict)
        self.assertIn("PASS", text)
        self.assertIn(f"Require the status check context {gate.JOB_NAME!r} (source: GitHub Actions)", text)

    def test_second_attempt_success_fails(self) -> None:
        runs, jobs = weekly()
        runs[1]["run_attempt"] = 2
        self.assert_fails(self.evaluate(runs, jobs), "run 101: concluded on attempt 2, not the first attempt")

    def test_vacuous_run_with_skipped_step_fails(self) -> None:
        runs, jobs = weekly()
        jobs[102] = [formal_job(dict.fromkeys(gate.REQUIRED_STEPS, "success") | {"Deep property tests": "skipped"})]
        self.assert_fails(self.evaluate(runs, jobs), "run 102: step 'Deep property tests' concluded 'skipped'")
        missing = dict.fromkeys(gate.REQUIRED_STEPS, "success")
        del missing["Trace validation"]
        jobs[102] = [formal_job(missing)]
        self.assert_fails(self.evaluate(runs, jobs), "run 102: step 'Trace validation' is missing")

    def test_pre_change_runs_are_excluded_and_counted(self) -> None:
        runs, jobs = weekly(3)
        old = [run(90, 23), run(91, 30)]
        jobs |= {r["id"]: [PRE_CHANGE_JOB] for r in old}
        verdict = self.evaluate(runs + old, jobs)
        self.assert_fails(verdict, "found 3 current-pipeline scheduled run(s) on main, need 4")
        self.assertEqual([e["id"] for e in verdict.excluded], [90, 91])
        self.assertIn("older than pre-change run 90", verdict.excluded[1]["reason"])
        self.assertIn("excluded 2 run(s)", gate.render_text(verdict))

    def test_since_excludes_older_runs(self) -> None:
        runs, jobs = weekly()
        verdict = self.evaluate(runs, jobs, since=NOW - dt.timedelta(days=10))
        self.assert_fails(verdict, "found 2 current-pipeline")
        self.assertEqual([e["id"] for e in verdict.excluded], [102, 103])
        self.assertIn("created before --since", verdict.excluded[0]["reason"])

    def test_missed_week_fails(self) -> None:
        runs, jobs = weekly()
        runs[2:] = [run(102, 2 + 7 + 9), run(103, 2 + 7 + 9 + 7)]
        jobs = {r["id"]: [formal_job()] for r in runs}
        self.assert_fails(self.evaluate(runs, jobs), "runs 102 and 101 are 9 days, 0:00:00 apart")

    def test_too_few_runs_fails(self) -> None:
        self.assert_fails(self.evaluate(*weekly(2)), "found 2 current-pipeline")
        self.assert_fails(self.evaluate([], {}), "found 0 current-pipeline")

    def test_stale_history_fails(self) -> None:
        self.assert_fails(self.evaluate(*weekly(newest_days_ago=9)), "the schedule may be disabled")

    def test_failed_or_foreign_runs(self) -> None:
        runs, jobs = weekly()
        runs[0]["conclusion"] = "failure"
        self.assert_fails(self.evaluate(runs, jobs), "run 100: concluded 'failure'")
        runs, jobs = weekly()
        jobs[100] = [JUNIT_JOB]
        self.assert_fails(self.evaluate(runs, jobs), "run 100: no job named")

    def test_foreign_junit_job_is_ignored(self) -> None:
        runs, jobs = weekly()
        jobs = {k: [JUNIT_JOB, *v] for k, v in jobs.items()}
        self.assertTrue(self.evaluate(runs, jobs).passed)

    def test_non_schedule_runs_are_ignored(self) -> None:
        runs, jobs = weekly()
        dispatch = run(200, 1) | {"event": "workflow_dispatch", "conclusion": "failure"}
        jobs[200] = [formal_job(conclusion="failure")]
        self.assertTrue(self.evaluate([dispatch, *runs], jobs).passed)


class WorkflowContractTests(unittest.TestCase):
    """The gate matches the workflow by display names; renaming a step or the
    job would otherwise make the gate fail forever or exclude every run."""

    def test_job_and_required_steps_exist_in_the_workflow(self) -> None:
        text = (Path(__file__).resolve().parent.parent / ".github" / "workflows" / gate.WORKFLOW).read_text()
        self.assertIn(f"    name: {gate.JOB_NAME}\n", text)
        for step in gate.REQUIRED_STEPS:
            with self.subTest(step=step):
                self.assertIn(f"      - name: {step}\n", text)


class FakeAPI:
    """Serves canned payloads and records every request's method."""

    def __init__(self, runs: list[dict], jobs: dict[int, list[dict]], broken: str = "") -> None:
        self.runs, self.jobs, self.broken = runs, jobs, broken
        self.methods: list[str] = []
        self.urls: list[str] = []

    def opener(self, request, timeout: float):
        self.methods.append(request.get_method())
        self.urls.append(request.full_url)
        if self.broken == "http":
            raise urllib.error.URLError("502 Bad Gateway")
        if "/jobs" in request.full_url:
            run_id = int(request.full_url.split("/runs/")[1].split("/")[0])
            body: object = {"total_count": 1, "jobs": self.jobs[run_id]}
        else:
            body = {"total_count": len(self.runs), "workflow_runs": self.runs}
        if self.broken == "payload":
            body = ["not", "an", "object"]
        return contextlib.closing(io.BytesIO(json.dumps(body).encode()))

    def get(self, url: str, token: str | None) -> dict:
        return gate.github_get(url, token, opener=self.opener)


class MainTests(unittest.TestCase):
    def main(self, api: FakeAPI, *args: str) -> tuple[int, str]:
        out = io.StringIO()
        with mock.patch.dict("os.environ", {"GH_TOKEN": "t"}), contextlib.redirect_stdout(out):
            code = gate.main(["--now", NOW.isoformat(), *args], get=api.get)
        self.assertTrue(api.methods, "no request was sent")
        self.assertEqual(set(api.methods), {"GET"})
        return code, out.getvalue()

    def test_pass_end_to_end_uses_get_only(self) -> None:
        runs, jobs = weekly(6)
        api = FakeAPI(runs, jobs)
        code, out = self.main(api)
        self.assertEqual(code, 0, out)
        self.assertIn("formal promotion gate: PASS", out)
        self.assertIn("event=schedule", api.urls[0])
        self.assertIn("branch=main", api.urls[0])
        self.assertEqual(len(api.urls), 1 + 4, "jobs are fetched only until four current-pipeline runs are found")

    def test_json_output(self) -> None:
        code, out = self.main(FakeAPI(*weekly(2)), "--json")
        self.assertEqual(code, 1)
        verdict = json.loads(out)
        self.assertFalse(verdict["passed"])
        self.assertEqual(verdict["context"], gate.JOB_NAME)
        self.assertEqual(len(verdict["counted"]), 2)

    def test_api_error_fails(self) -> None:
        code, out = self.main(FakeAPI(*weekly(), broken="http"))
        self.assertEqual(code, 1)
        self.assertIn("FAIL", out)
        self.assertIn("GitHub API: <urlopen error 502 Bad Gateway>", out)

    def test_malformed_payload_fails(self) -> None:
        code, out = self.main(FakeAPI(*weekly(), broken="payload"))
        self.assertEqual(code, 1)
        self.assertIn("unexpected payload", out)
        runs, jobs = weekly()
        del runs[0]["created_at"]
        code, out = self.main(FakeAPI(runs, jobs))
        self.assertEqual(code, 1)
        self.assertIn("unexpected created_at", out)
        code, out = self.main(FakeAPI([{"id": 1}], {}), "--json")
        self.assertEqual(code, 1)

    def test_non_get_methods_are_refused_before_sending(self) -> None:
        opener = mock.Mock()
        for method in ("POST", "PUT", "PATCH", "DELETE", "HEAD"):
            with self.subTest(method=method):
                with self.assertRaisesRegex(gate.GateError, f"refusing {method} .*GET requests only"):
                    gate.github_get("https://api.github.com/repos/invowk/invowk/rulesets/1", "t", method=method, opener=opener)
        opener.assert_not_called()

    def test_token_chain(self) -> None:
        with mock.patch.dict("os.environ", {"GH_TOKEN": "a", "GITHUB_TOKEN": "b"}):
            self.assertEqual(gate.resolve_token(), "a")
        with mock.patch.dict("os.environ", {"GH_TOKEN": "", "GITHUB_TOKEN": "b"}):
            self.assertEqual(gate.resolve_token(), "b")
        with mock.patch.dict("os.environ", {"GH_TOKEN": "", "GITHUB_TOKEN": ""}), mock.patch.object(gate.shutil, "which", return_value=None):
            self.assertIsNone(gate.resolve_token())


if __name__ == "__main__":
    unittest.main()
