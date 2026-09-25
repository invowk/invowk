#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Promotion gate for the Formal Verification lane (read-only).

Reads the run history of .github/workflows/formal-verification.yml through the
GitHub REST API, with GET requests only, and passes only when the four most
recent scheduled runs on main of the current pipeline all succeeded on their
first attempt, ran every heavy formal step, and form an unbroken weekly
streak whose newest run is at most 8 days old.

A run belongs to the current pipeline when its formal job contains the
`Classify changed paths` step, which first appeared with the
promote-formal-ci-gate change. Earlier runs, and runs created before --since,
are excluded and reported.

The script never modifies repository settings: making the check required is a
manual maintainer step (see formal/README.md, "Promoting the lane").

Standard library only. Token: GH_TOKEN, then GITHUB_TOKEN, then `gh auth
token` when gh is installed, otherwise unauthenticated (the repository is
public).

Usage:
    scripts/formal_promotion_gate.py [--repo OWNER/NAME] [--since DATE] [--now DATE] [--json]
"""

from __future__ import annotations

import argparse
import dataclasses
import datetime as dt
import json
import os
import shutil
import subprocess
import sys
import urllib.error
import urllib.request
from collections.abc import Callable

API = "https://api.github.com"
WORKFLOW = "formal-verification.yml"
JOB_NAME = "Models, golden vectors, and correspondence"
CLASSIFY_STEP = "Classify changed paths"
REQUIRED_STEPS = (
    CLASSIFY_STEP,
    "Check models",
    "Trace validation",
    "Replay golden vectors and property tests",
    "Deep property tests",
)
REQUIRED_RUNS = 4
MAX_GAP = dt.timedelta(days=8)
MAX_AGE = dt.timedelta(days=8)


class GateError(Exception):
    """The gate cannot be evaluated; it reports FAIL, never PASS."""


# ---------------------------------------------------------------------------
# GitHub API (GET only)

Opener = Callable[..., object]


def github_get(url: str, token: str | None, method: str = "GET", opener: Opener = urllib.request.urlopen) -> dict:
    """Send one GET request and return its JSON object. Any other method is
    refused before a request is built."""
    if method != "GET":
        raise GateError(f"refusing {method} {url}: the promotion gate sends GET requests only")
    headers = {"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    request = urllib.request.Request(url, headers=headers, method="GET")
    with opener(request, timeout=30) as response:  # noqa: S310 - fixed https API URL
        payload = json.load(response)
    if not isinstance(payload, dict):
        raise GateError(f"unexpected payload from {url}: {type(payload).__name__}, want an object")
    return payload


def resolve_token() -> str | None:
    for name in ("GH_TOKEN", "GITHUB_TOKEN"):
        if os.environ.get(name):
            return os.environ[name]
    if shutil.which("gh"):
        completed = subprocess.run(["gh", "auth", "token"], capture_output=True, text=True, check=False)
        if completed.returncode == 0 and completed.stdout.strip():
            return completed.stdout.strip()
    return None


def payload_list(payload: dict, key: str, url: str) -> list[dict]:
    items = payload.get(key)
    if not isinstance(items, list) or not all(isinstance(item, dict) for item in items):
        raise GateError(f"unexpected payload from {url}: no list of objects under {key!r}")
    return items


def fetch_runs(repo: str, token: str | None, get: Callable[[str, str | None], dict] = github_get) -> list[dict]:
    url = f"{API}/repos/{repo}/actions/workflows/{WORKFLOW}/runs?branch=main&event=schedule&status=completed&per_page=30"
    return payload_list(get(url, token), "workflow_runs", url)


def job_fetcher(repo: str, token: str | None, get: Callable[[str, str | None], dict] = github_get
                ) -> Callable[[int], list[dict]]:
    """A run's jobs, fetched on demand: `evaluate` asks only for the runs it
    examines."""
    def jobs_for(run_id: int) -> list[dict]:
        url = f"{API}/repos/{repo}/actions/runs/{run_id}/jobs?per_page=100"
        return payload_list(get(url, token), "jobs", url)
    return jobs_for


# ---------------------------------------------------------------------------
# Decision (pure)


@dataclasses.dataclass
class Verdict:
    passed: bool
    counted: list[dict]
    excluded: list[dict]
    failures: list[str]


def parse_time(value: str) -> dt.datetime:
    """An ISO 8601 date or datetime; naive values are UTC."""
    parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    return parsed if parsed.tzinfo else parsed.replace(tzinfo=dt.UTC)


def created(run: dict) -> dt.datetime:
    try:
        return parse_time(run["created_at"])
    except (KeyError, TypeError, ValueError) as err:
        raise GateError(f"run {run.get('id', '?')}: unexpected created_at ({err})") from err


def newest_first(runs: list[dict]) -> list[dict]:
    return sorted(runs, key=created, reverse=True)


def formal_job(jobs: list[dict]) -> dict | None:
    """The formal job, matched by exact name; check runs from other workflows
    (such as `JUnit Report (...)`) are ignored."""
    return next((job for job in jobs if job.get("name") == JOB_NAME), None)


def is_current_pipeline(job: dict) -> bool:
    return any(step.get("name") == CLASSIFY_STEP for step in job.get("steps") or [])


def run_failures(run: dict, job: dict | None) -> list[str]:
    run_id = run.get("id")
    if job is None:
        return [f"run {run_id}: no job named {JOB_NAME!r}"]
    failures = []
    if run.get("run_attempt") != 1:
        failures.append(f"run {run_id}: concluded on attempt {run.get('run_attempt')}, not the first attempt")
    if run.get("conclusion") != "success":
        failures.append(f"run {run_id}: concluded {run.get('conclusion')!r}, not 'success'")
    if job.get("conclusion") != "success":
        failures.append(f"run {run_id}: job {JOB_NAME!r} concluded {job.get('conclusion')!r}")
    steps = {step.get("name"): step.get("conclusion") for step in job.get("steps") or []}
    for name in REQUIRED_STEPS:
        if name not in steps:
            failures.append(f"run {run_id}: step {name!r} is missing")
        elif steps[name] != "success":
            failures.append(f"run {run_id}: step {name!r} concluded {steps[name]!r}, not 'success'")
    return failures


def evaluate(
    runs: list[dict], jobs_for: Callable[[int], list[dict]], now: dt.datetime, since: dt.datetime | None
) -> Verdict:
    """Walk scheduled runs on main newest first, asking `jobs_for` only for
    the runs it examines, until REQUIRED_RUNS current-pipeline runs are
    counted. The pipeline only moves forward, so the first run that predates
    it excludes every older run without fetching their jobs."""
    counted: list[dict] = []
    excluded: list[dict] = []
    failures: list[str] = []
    candidates = [
        r for r in newest_first(runs)
        if r.get("event") == "schedule" and r.get("head_branch") == "main" and r.get("status") == "completed"
    ]
    for k, run in enumerate(candidates):
        if len(counted) == REQUIRED_RUNS:
            break
        if since is not None and created(run) < since:
            excluded += [{"id": r["id"], "reason": f"created before --since {since.isoformat()}"} for r in candidates[k:]]
            break
        job = formal_job(jobs_for(run["id"]))
        if job is not None and not is_current_pipeline(job):
            excluded.append({"id": run["id"], "reason": f"predates the gate's workflow (no {CLASSIFY_STEP!r} step)"})
            excluded += [{"id": r["id"], "reason": f"older than pre-change run {run['id']}"} for r in candidates[k + 1:]]
            break
        counted.append(run)
        failures += run_failures(run, job)
    if len(counted) < REQUIRED_RUNS:
        failures.append(f"found {len(counted)} current-pipeline scheduled run(s) on main, need {REQUIRED_RUNS}")
    for newer, older in zip(counted, counted[1:]):
        gap = created(newer) - created(older)
        if gap > MAX_GAP:
            failures.append(f"runs {older['id']} and {newer['id']} are {gap} apart (more than {MAX_GAP.days} days: a missed week)")
    if counted and now - created(counted[0]) > MAX_AGE:
        failures.append(
            f"newest counted run {counted[0]['id']} is {now - created(counted[0])} old "
            f"(more than {MAX_AGE.days} days: the schedule may be disabled)"
        )
    rows = [
        {"id": r["id"], "created_at": r["created_at"], "head_sha": r.get("head_sha", ""),
         "conclusion": r.get("conclusion"), "run_attempt": r.get("run_attempt")}
        for r in counted
    ]
    return Verdict(passed=not failures, counted=rows, excluded=excluded, failures=failures)


# ---------------------------------------------------------------------------
# CLI


def render_text(verdict: Verdict) -> str:
    lines = [f"formal promotion gate: {'PASS' if verdict.passed else 'FAIL'}", ""]
    if verdict.counted:
        lines.append(f"{'run id':<14} {'created':<22} {'head sha':<12} {'conclusion':<11} attempt")
        lines += [
            f"{r['id']:<14} {r['created_at']:<22} {r['head_sha'][:12]:<12} {r['conclusion']!s:<11} {r['run_attempt']}"
            for r in verdict.counted
        ]
    else:
        lines.append("no current-pipeline scheduled runs counted")
    lines.append(f"excluded {len(verdict.excluded)} run(s)")
    lines += [f"  - {e['id']}: {e['reason']}" for e in verdict.excluded]
    if verdict.failures:
        lines.append("failures:")
        lines += [f"  - {failure}" for failure in verdict.failures]
    else:
        lines += ["", f"Require the status check context {JOB_NAME!r} (source: GitHub Actions)."]
    return "\n".join(lines)


def main(argv: list[str] | None = None, get: Callable[[str, str | None], dict] = github_get) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--repo", default="invowk/invowk")
    parser.add_argument("--since", type=parse_time, help="exclude runs created before this ISO date or datetime")
    parser.add_argument("--now", type=parse_time, help="evaluation time (default: current UTC time)")
    parser.add_argument("--json", action="store_true", help="print the verdict as JSON")
    args = parser.parse_args(argv)
    now = args.now or dt.datetime.now(dt.UTC)
    try:
        token = resolve_token()
        verdict = evaluate(fetch_runs(args.repo, token, get), job_fetcher(args.repo, token, get), now, args.since)
    except (GateError, urllib.error.URLError, OSError, ValueError, KeyError, TypeError) as err:
        verdict = Verdict(passed=False, counted=[], excluded=[], failures=[f"GitHub API: {err}"])
    if args.json:
        print(json.dumps({**dataclasses.asdict(verdict), "context": JOB_NAME}, indent=1))
    else:
        print(render_text(verdict))
    return 0 if verdict.passed else 1


if __name__ == "__main__":
    sys.exit(main())
