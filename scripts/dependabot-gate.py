#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Decide whether a Dependabot pull request may merge without a human.

`.github/workflows/dependabot-gate.yml` runs this from `main` each time a
workflow of a Dependabot pull request completes. The decision is a pure
function of one JSON snapshot (the pull request, its commits, its changed
files, and the check runs and commit statuses of its head commit) and the
policy tables below; only the `gate` command talks to GitHub, through
read-only `gh api` calls.

The gate FAILS CLOSED. Every rule appends a reason when its condition is not
exactly the expected value; a missing field, an unknown value, a rule that
raises and anything else unexpected is a reason too. The verdict is `merge`
only when no rule left a reason. `merge` requires all of:

  - the pull request: opened by dependabot[bot], base `main`, head branch in
    this repository (no fork), open, not a draft, mergeable without conflict,
    no `do-not-merge` or `hold` label;
  - the head: the SHA the triggering workflow run tested is the pull
    request's current head;
  - the commits: exactly one, the head, authored by dependabot[bot],
    committed by GitHub for it and with a verified signature (a push by
    anyone else, or a second commit the metadata does not describe, holds);
  - the metadata: the `updated-dependencies:` block of that commit's message
    (what dependabot/fetch-metadata reads) parses under the strict reader
    below;
  - the ecosystem: gomod or npm, by head-branch prefix, and every changed
    file is one of that ecosystem's files (GitHub Actions updates never
    merge here: pins are major tags, and PR CI does not exercise
    release.yml). The manifest's diff is read as well: go.mod may change
    `require` lines only, package.json only the dependencies the metadata
    names;
  - the update types: semver-patch or semver-minor for every dependency; a
    minor bump whose new version is 0.x is treated as breaking;
  - the deny list: no sync-pair dependency (DENY_GOMOD, DENY_NPM), neither
    named in the metadata nor changed in the manifest as a side effect;
  - the checks: every check run and commit status of the head commit is
    completed and successful (or skipped or neutral), except IGNORED_CHECKS,
    and every REQUIRED check of the ecosystem is there and successful, so a
    check that never started cannot pass by its absence.

Usage:
  dependabot-gate.py gate --repo OWNER/NAME (--head-sha SHA | --pr NUMBER)
                          [--mode MODE] [--summary FILE] [--github-output FILE]
                          [--snapshot-out FILE]
  dependabot-gate.py decide SNAPSHOT.json [--mode MODE] [--summary FILE]
                          [--github-output FILE]

Both print the verdict as JSON on stdout,
`{"decision": "merge"|"hold", "reasons": [...], "dependencies": [...], ...}`,
and a readable summary on stderr. Exit 0 when a verdict was reached (merge or
hold); 1 when GitHub could not be read (the verdict printed is `hold`); 2 on
a usage error. Any other exit is a crash, and the workflow merges nothing
then: its merge steps need `decision=merge` from a step that succeeded.
Stdlib only.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable

sys.dont_write_bytecode = True

# ---------------------------------------------------------------- identities

# The bot user and GitHub's own committer, as the REST API reports them on
# PRs #189, #190 and #191 (2026-11-23). Dependabot's commits are created by
# GitHub: the author is the bot, the committer is `web-flow`, and GitHub's key
# signs them.
DEPENDABOT = ("dependabot[bot]", 49699333)
COMMITTERS = (("web-flow", 19864447), DEPENDABOT)
BASE_BRANCH = "main"
HOLD_LABELS = frozenset({"do-not-merge", "hold"})
# `mergeable_state` depends on who asks (an anonymous reader saw "unstable"
# where the maintainer's token saw "blocked"), so it only screens out what is
# never mergeable: "dirty" (conflict), "behind", "draft", "unknown" and any
# value not listed here. The conflict rule proper is `mergeable is true`.
MERGEABLE_STATES = frozenset({"clean", "unstable", "blocked", "has_hooks"})

UPDATE_PATCH = "version-update:semver-patch"
UPDATE_MINOR = "version-update:semver-minor"
ALLOWED_UPDATE_TYPES = frozenset({UPDATE_PATCH, UPDATE_MINOR})

SHA_RE = re.compile(r"[0-9a-f]{40}")
REPO_RE = re.compile(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+")
# A released version: no pre-release, build or pseudo-version suffix.
VERSION_RE = re.compile(r"v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)")

# -------------------------------------------------------------------- checks

ACTIONS_APP = "github-actions"
CODE_SCANNING_APP = "github-advanced-security"
OK_CONCLUSIONS = frozenset({"success", "skipped", "neutral"})

# (check name, app slug) -> why its result is not read.
IGNORED_CHECKS: dict[tuple[str, str], str] = {
    ("Track Benchmarks", ACTIONS_APP): (
        "benchmarks.yml: Dependabot pull requests get no Actions secrets, so "
        "the Bencher upload fails there by design when the job runs at all "
        "(it needs the `benchmarks: run` label); benchmarks run on main after "
        "the merge (maintainer decision 48). The other benchmark jobs need no "
        "entry: `Benchmark Fork PR` only runs for forks and is skipped here, "
        "and `Upload Fork PR Benchmarks` reports on main's commit."
    ),
    ("Dependabot merge gate", ACTIONS_APP): (
        "the gate's own job. A workflow_run job reports on main's commit, not "
        "on the pull request's; should it ever show up on the head commit, it "
        "is still running while it decides and must not block itself."
    ),
}


@dataclass(frozen=True)
class Required:
    """A check that must be present and successful, and the app reporting it."""

    name: str
    workflow: str  # the `name:` of the workflow that produces it
    app: str = ACTIONS_APP


# Reported on every pull request: lint.yml, formal-verification.yml and
# codeql.yml have no `paths:` filter on `pull_request` and no job-level `if`.
# Seen successful on #189 (npm) and #191, and present on #190 (gomod).
# scripts/test_dependabot_gate.py re-checks each name, trigger and path filter
# against the workflow files, so a renamed job fails the script tests instead
# of leaving the gate on hold for good.
REQUIRED_COMMON = (
    Required("normalized golangci-lint gates", "Lint"),
    Required("goplint consumer gates", "Lint"),
    Required("script tests (worktree-safety gate)", "Lint"),
    Required("file length gate (1000 lines)", "Lint"),
    Required("agent docs integrity", "Lint"),
    Required("Models, golden vectors, and correspondence", "Formal Verification"),
    Required("Analyze (actions)", "CodeQL"),
    Required("Analyze (go)", "CodeQL"),
    Required("Analyze (javascript-typescript)", "CodeQL"),
    Required("Analyze (python)", "CodeQL"),
    # The code-scanning result of those analyses (new alerts fail it).
    Required("CodeQL", "CodeQL", CODE_SCANNING_APP),
)
# ci.yml: its `pull_request` path filter lists go.mod and go.sum, and none of
# its jobs has an `if`. All fourteen ran on #190; website-only #189 started
# none of them, which is why they are required for gomod only.
REQUIRED_GOMOD = REQUIRED_COMMON + tuple(
    Required(name, "CI")
    for name in (
        "Test (ubuntu-latest / docker)",
        "Test (ubuntu-latest / podman)",
        "Test (windows)",
        "Test (macos)",
        "Build (linux/amd64)",
        "Build (linux/arm64)",
        "Build (darwin/amd64)",
        "Build (darwin/arm64)",
        "Build (windows/amd64)",
        "License Headers",
        "govulncheck",
        "PGO Sanity",
        "GoReleaser Check",
        "Release Dry Run",
    )
)
# test-website.yml: path filter `website/**`; the job has no `name:`, so the
# check carries its id. Ran on #189; go.mod-only #190 did not start it.
REQUIRED_NPM = REQUIRED_COMMON + (Required("test-build", "Test Website Build"),)

# ----------------------------------------------------------------- deny list

# Dependencies a pull request never bumps alone: each has a second pin or a
# manual procedure that must move in the same change. Source:
# .agents/rules/version-pinning.md and the "Required Sync Pairs" table of
# .agents/skills/ci-update/SKILL.md. `.github/dependabot.yml` keeps the same
# names out of the grouped pull requests, and the script tests fail on an
# entry that go.mod or website/package.json no longer has.
#
# Not listed, on purpose: `pgregory.net/rapid` (the rule: "a test-only Go
# dependency; go.mod is its single source of truth", so there is nothing to
# keep in step, and `make test` exercises it); `github.com/coder/acp-go-sdk`
# (no second pin: the 0.x rule holds its minor bumps, patches may merge); and
# the Go toolchain, which is not a module: a changed `go` or `toolchain` line
# in go.mod holds through the manifest rule instead.
DENY_GOMOD: dict[str, str] = {
    "github.com/invowk/goplint": (
        "pinned in go.mod and as GOPLINT_VERSION in scripts/goplint.sh, which "
        "refuses any other build; a bump also needs the baseline re-triaged "
        "(version-pinning.md, Go Tool Dependencies)"
    ),
    "github.com/golangci/golangci-lint/v2": (
        "pinned in go.mod and verified by scripts/golangci-lint.sh; a new "
        "release changes which findings `make lint` reports (ci-update, "
        "Required Sync Pairs)"
    ),
    "github.com/jonbaldie/go-mutesting/v2": (
        "pinned in go.mod and verified by scripts/mutation.sh; `mutant_id` in "
        "scripts/formal_mutation.py copies its MutantID and must be re-checked "
        "(ci-update, Required Sync Pairs)"
    ),
    "cuelang.org/go": (
        "CUE upgrades follow the 6-step process in .agents/skills/cue/SKILL.md "
        "(version-pinning.md, Go Dependencies)"
    ),
    "golang.org/x/tools": (
        "moves with the Go toolchain, golangci-lint, go-mutesting and goplint "
        "(version-pinning.md, Go Toolchain)"
    ),
    "github.com/invowk/golua": (
        "the project's own fork behind virtual-lua: a release is cut for a "
        "specific invowk change and has to build on the pinned Go toolchain "
        "(version-pinning.md, Go Toolchain)"
    ),
}
_DOCUSAURUS = (
    "a core Docusaurus package pinned to an exact version; the six move "
    "together, by hand (version-pinning.md, npm Dependencies)"
)
DENY_NPM: dict[str, str] = {
    "@docusaurus/core": _DOCUSAURUS,
    "@docusaurus/faster": _DOCUSAURUS,
    "@docusaurus/preset-classic": _DOCUSAURUS,
    "@docusaurus/module-type-aliases": _DOCUSAURUS,
    "@docusaurus/tsconfig": _DOCUSAURUS,
    "@docusaurus/types": _DOCUSAURUS,
}


@dataclass(frozen=True)
class Ecosystem:
    name: str
    branch_prefix: str  # Dependabot's head-branch prefix
    manifest: str  # the changed file whose diff is read
    files: frozenset[str]  # every file such a pull request may change
    deny: dict[str, str]
    required: tuple[Required, ...]


# Only the root module and the website have a Dependabot entry, so only their
# files are allowed (the one other go.mod in the tree is a test fixture).
ECOSYSTEMS = (
    Ecosystem(
        "gomod",
        "dependabot/go_modules/",
        "go.mod",
        frozenset({"go.mod", "go.sum"}),
        DENY_GOMOD,
        REQUIRED_GOMOD,
    ),
    Ecosystem(
        "npm",
        "dependabot/npm_and_yarn/",
        "website/package.json",
        frozenset({"website/package.json", "website/package-lock.json"}),
        DENY_NPM,
        REQUIRED_NPM,
    ),
)

# ------------------------------------------------------------------ metadata


class MetadataError(ValueError):
    """The commit message has no `updated-dependencies:` block this reads."""


METADATA_KEYS = frozenset(
    {"dependency-name", "dependency-version", "dependency-type", "update-type", "dependency-group"}
)
_METADATA_LINE = re.compile(r"(- |  )([a-z-]+): (\S.*)")
_PLAIN_SCALAR = re.compile(r"[A-Za-z0-9][A-Za-z0-9._/:+~@-]*")
_QUOTED_SCALAR = re.compile(r"'([^'\\]*)'|\"([^\"\\]*)\"")
# Plain scalars YAML would not read as strings; Dependabot quotes those.
_YAML_NON_STRING = re.compile(
    r"[-+]?(\d[\d_]*)?\.?\d*([eE][-+]?\d+)?|0x[0-9a-fA-F]+|true|false|null|yes|no|on|off|y|n",
    re.IGNORECASE,
)


def _metadata_scalar(raw: str) -> str:
    quoted = _QUOTED_SCALAR.fullmatch(raw)
    if quoted:
        value = quoted.group(1) if quoted.group(1) is not None else quoted.group(2)
    elif _PLAIN_SCALAR.fullmatch(raw) and not _YAML_NON_STRING.fullmatch(raw):
        value = raw
    else:
        raise MetadataError(f"value not understood: {raw!r}")
    if not value:
        raise MetadataError("empty value")
    if not (value.isascii() and value.isprintable()):
        raise MetadataError(f"value is not printable ASCII: {value!a}")
    return value


def parse_updated_dependencies(message: object) -> list[dict[str, str]]:
    """Read the block Dependabot appends to its commit message.

    Exactly this shape, and nothing else YAML allows:

        ---
        updated-dependencies:
        - dependency-name: react
          dependency-version: 19.3.1
          update-type: version-update:semver-minor
        ...
    """
    if not isinstance(message, str):
        raise MetadataError("no commit message")
    lines = message.split("\n")
    starts = [i for i, line in enumerate(lines) if line == "updated-dependencies:"]
    if len(starts) != 1:
        raise MetadataError(f"expected one `updated-dependencies:` line, found {len(starts)}")
    start = starts[0]
    if start == 0 or lines[start - 1] != "---":
        raise MetadataError("`updated-dependencies:` does not follow a `---` line")
    if "..." not in lines[start:]:
        raise MetadataError("the block has no closing `...` line")
    end = lines.index("...", start)
    dependencies: list[dict[str, str]] = []
    for line in lines[start + 1 : end]:
        match = _METADATA_LINE.fullmatch(line)
        if not match:
            raise MetadataError(f"line not understood: {line!r}")
        lead, key, raw = match.groups()
        if key not in METADATA_KEYS:
            raise MetadataError(f"unknown key {key!r}")
        if lead == "- ":
            dependencies.append({})
        elif not dependencies:
            raise MetadataError(f"{key!r} before the first list item")
        if key in dependencies[-1]:
            raise MetadataError(f"duplicate key {key!r}")
        dependencies[-1][key] = _metadata_scalar(raw)
    if not dependencies:
        raise MetadataError("the block lists no dependency")
    for dependency in dependencies:
        if "dependency-name" not in dependency:
            raise MetadataError("a list item has no dependency-name")
    return dependencies


# ----------------------------------------------------------- manifest diffs

_GOMOD_REQUIRE = re.compile(r"\t(\S+) v\S+( // indirect)?")
_PACKAGE_JSON_ENTRY = re.compile(r'\s+"([^"\\]+)": "[^"\\]*",?')


def changed_manifest_keys(manifest: str, patch: object) -> list[str]:
    """The modules (go.mod) or packages (package.json) whose lines a diff changes.

    Raises ValueError on any other changed line: the `go` and `toolchain`
    directives, `tool`, `replace`, a script, a key with a non-string value.
    """
    if not isinstance(patch, str) or not patch:
        raise ValueError("GitHub returned no diff for it")
    shape = _GOMOD_REQUIRE if manifest == "go.mod" else _PACKAGE_JSON_ENTRY
    keys: list[str] = []
    for line in patch.split("\n"):
        if not line.startswith(("+", "-")):
            continue
        match = shape.fullmatch(line[1:])
        if not match:
            raise ValueError(f"changed line is not a dependency entry: {_q(line)}")
        if match.group(1) not in keys:
            keys.append(match.group(1))
    return keys


# --------------------------------------------------------------------- rules


@dataclass
class Context:
    snapshot: dict
    reasons: list[str] = field(default_factory=list)
    ecosystem: Ecosystem | None = None
    dependencies: list[dict[str, str]] | None = None

    def hold(self, reason: str) -> None:
        if reason not in self.reasons:
            self.reasons.append(reason)


def _dig(value: object, *keys: str) -> object:
    """value[k1][k2]..., or None as soon as a level is not a mapping."""
    for key in keys:
        if not isinstance(value, dict):
            return None
        value = value.get(key)
    return value


def _list(value: object) -> list:
    return value if isinstance(value, list) else []


def _q(value: object) -> str:
    """An untrusted value, quoted, ASCII-only and bounded, for a reason line."""
    text = json.dumps(value) if isinstance(value, (str, int, float, bool, type(None))) else repr(value)
    return text if len(text) <= 160 else text[:157] + "..."


def _identity(user: object) -> tuple[object, object]:
    return (_dig(user, "login"), _dig(user, "id"))


def rule_pull_request(ctx: Context) -> None:
    pr = ctx.snapshot.get("pull_request")
    repo = ctx.snapshot.get("repository")
    if not isinstance(pr, dict):
        ctx.hold("the snapshot has no pull request")
        return
    if not isinstance(repo, str) or not REPO_RE.fullmatch(repo):
        ctx.hold(f"repository not understood: {_q(repo)}")
    if type(pr.get("number")) is not int or pr["number"] <= 0:
        ctx.hold(f"pull request number not understood: {_q(pr.get('number'))}")
    if _identity(pr.get("user")) != DEPENDABOT or _dig(pr, "user", "type") != "Bot":
        ctx.hold(f"the pull request author is {_q(_dig(pr, 'user', 'login'))}, not dependabot[bot]")
    if _dig(pr, "base", "ref") != BASE_BRANCH:
        ctx.hold(f"the base branch is {_q(_dig(pr, 'base', 'ref'))}, not {BASE_BRANCH}")
    if _dig(pr, "base", "repo", "full_name") != repo:
        ctx.hold(f"the base repository is {_q(_dig(pr, 'base', 'repo', 'full_name'))}, not this repository")
    if _dig(pr, "head", "repo", "full_name") != repo:
        ctx.hold(f"the head branch is in {_q(_dig(pr, 'head', 'repo', 'full_name'))} (a fork), not in this repository")
    if pr.get("state") != "open":
        ctx.hold(f"the pull request is {_q(pr.get('state'))}, not open")
    if pr.get("draft") is not False:
        ctx.hold("the pull request is a draft")
    if pr.get("mergeable") is not True:
        ctx.hold(f"GitHub does not report the pull request as mergeable (mergeable: {_q(pr.get('mergeable'))})")
    if pr.get("mergeable_state") not in MERGEABLE_STATES:
        ctx.hold(f"merge state is {_q(pr.get('mergeable_state'))} (a conflict, a stale base or an unknown state)")
    labels = pr.get("labels")
    if not isinstance(labels, list):
        ctx.hold("the pull request labels could not be read")
    for label in _list(labels):
        name = _dig(label, "name")
        if not isinstance(name, str):
            ctx.hold("a pull request label has no name")
        elif name.lower() in HOLD_LABELS:
            ctx.hold(f"the pull request has the label {_q(name)}")


def rule_head_sha(ctx: Context) -> None:
    sha = ctx.snapshot.get("head_sha")
    if not isinstance(sha, str) or not SHA_RE.fullmatch(sha):
        ctx.hold(f"the triggering head SHA is not a commit SHA: {_q(sha)}")
    elif sha != _dig(ctx.snapshot, "pull_request", "head", "sha"):
        ctx.hold(
            f"the triggering run tested {sha}, but the pull request head is now "
            f"{_q(_dig(ctx.snapshot, 'pull_request', 'head', 'sha'))}"
        )


def rule_commits(ctx: Context) -> None:
    commits = ctx.snapshot.get("commits")
    head = _dig(ctx.snapshot, "pull_request", "head", "sha")
    if not isinstance(commits, list) or len(commits) != 1:
        count = len(commits) if isinstance(commits, list) else "no"
        ctx.hold(f"expected exactly one commit (Dependabot's), found {count}")
    if _dig(ctx.snapshot, "pull_request", "commits") != len(_list(commits)):
        ctx.hold("the commit list does not match the pull request's commit count")
    for commit in _list(commits):
        sha = _q(_dig(commit, "sha"))
        if _identity(_dig(commit, "author")) != DEPENDABOT:
            ctx.hold(f"commit {sha} is authored by {_q(_dig(commit, 'author', 'login'))}, not dependabot[bot]")
        if _identity(_dig(commit, "committer")) not in COMMITTERS:
            ctx.hold(
                f"commit {sha} is committed by {_q(_dig(commit, 'committer', 'login'))}, "
                "not by GitHub for dependabot[bot]"
            )
        if _dig(commit, "commit", "verification", "verified") is not True:
            ctx.hold(f"commit {sha} has no verified signature")
    if _list(commits) and _dig(commits[-1], "sha") != head:
        ctx.hold("the last commit listed is not the pull request head")


def rule_metadata(ctx: Context) -> None:
    head = _dig(ctx.snapshot, "pull_request", "head", "sha")
    messages = [
        _dig(commit, "commit", "message")
        for commit in _list(ctx.snapshot.get("commits"))
        if head is not None and _dig(commit, "sha") == head
    ]
    if len(messages) != 1:
        ctx.hold("the head commit message could not be read")
        return
    try:
        ctx.dependencies = parse_updated_dependencies(messages[0])
    except MetadataError as exc:
        ctx.hold(f"the dependency metadata of the head commit is missing or unreadable: {exc}")


def rule_ecosystem(ctx: Context) -> None:
    ref = _dig(ctx.snapshot, "pull_request", "head", "ref")
    matches = [eco for eco in ECOSYSTEMS if isinstance(ref, str) and ref.startswith(eco.branch_prefix)]
    if len(matches) != 1:
        ctx.hold(f"the head branch {_q(ref)} is not a gomod or npm Dependabot branch (other updates never merge here)")
        return
    eco = ctx.ecosystem = matches[0]
    files = ctx.snapshot.get("files")
    if not isinstance(files, list) or not files:
        ctx.hold("the changed files could not be read")
        return
    if _dig(ctx.snapshot, "pull_request", "changed_files") != len(files):
        ctx.hold("the file list does not match the pull request's changed-file count")
    named = {dependency["dependency-name"] for dependency in ctx.dependencies or []}
    for entry in files:
        filename = _dig(entry, "filename")
        if filename not in eco.files:
            ctx.hold(f"changed file {_q(filename)} is not one a {eco.name} update may touch")
            continue
        if _dig(entry, "status") != "modified":
            ctx.hold(f"{filename} is {_q(_dig(entry, 'status'))}, not modified")
        if filename != eco.manifest:
            continue
        try:
            keys = changed_manifest_keys(eco.manifest, _dig(entry, "patch"))
        except ValueError as exc:
            ctx.hold(f"the {filename} diff is not a plain dependency update: {exc}")
            continue
        for key in keys:
            if key in eco.deny:
                ctx.hold(f"{filename} changes {key}, which is on the sync-pair deny list: {eco.deny[key]}")
            elif eco.name == "npm" and key not in named:
                ctx.hold(f"{filename} changes {_q(key)}, which the dependency metadata does not name")


def rule_update_types(ctx: Context) -> None:
    if not ctx.dependencies:
        ctx.hold("the update types could not be evaluated without dependency metadata")
    for dependency in ctx.dependencies or []:
        name = _q(dependency["dependency-name"])
        update_type = dependency.get("update-type")
        version = VERSION_RE.fullmatch(dependency.get("dependency-version", ""))
        if update_type not in ALLOWED_UPDATE_TYPES:
            ctx.hold(f"{name}: update type {_q(update_type)} is not semver-patch or semver-minor")
        if version is None:
            ctx.hold(f"{name}: new version {_q(dependency.get('dependency-version'))} is not a released x.y.z version")
        elif version.group(1) == "0" and update_type == UPDATE_MINOR:
            ctx.hold(
                f"{name}: a minor bump to {dependency['dependency-version']} is a 0.x release, "
                "treated as breaking"
            )


def rule_deny_list(ctx: Context) -> None:
    if ctx.ecosystem is None:
        ctx.hold("the deny list could not be evaluated without a known ecosystem")
        return
    for dependency in ctx.dependencies or []:
        name = dependency["dependency-name"]
        if name in ctx.ecosystem.deny:
            ctx.hold(f"{name} is on the sync-pair deny list: {ctx.ecosystem.deny[name]}")


def rule_checks(ctx: Context) -> None:
    head = _dig(ctx.snapshot, "pull_request", "head", "sha")
    runs = ctx.snapshot.get("check_runs")
    statuses = ctx.snapshot.get("statuses")
    if not isinstance(runs, list):
        ctx.hold("the check runs could not be read")
    if not isinstance(statuses, list):
        ctx.hold("the commit statuses could not be read")
    successful: set[tuple[object, object]] = set()
    present: dict[tuple[object, object], object] = {}
    for run in _list(runs):
        name, app = _dig(run, "name"), _dig(run, "app", "slug")
        label = f"check {_q(name)} ({_q(app)})"
        if not isinstance(name, str) or not isinstance(app, str):
            ctx.hold(f"a check run has no name or app: {_q(name)} / {_q(app)}")
            continue
        if head is None or _dig(run, "head_sha") != head:
            ctx.hold(f"{label} is not for the pull request head")
            continue
        if (name, app) in IGNORED_CHECKS:
            continue
        status, conclusion = _dig(run, "status"), _dig(run, "conclusion")
        if status != "completed":
            ctx.hold(f"{label} is not completed (status {_q(status)})")
        elif conclusion not in OK_CONCLUSIONS:
            ctx.hold(f"{label} concluded {_q(conclusion)}")
        elif conclusion == "success":
            successful.add((name, app))
        present[(name, app)] = conclusion if status == "completed" else status
    for status in _list(statuses):
        if _dig(status, "state") != "success":
            ctx.hold(f"commit status {_q(_dig(status, 'context'))} is {_q(_dig(status, 'state'))}")
    if ctx.ecosystem is None:
        ctx.hold("the required checks could not be evaluated without a known ecosystem")
        return
    for required in ctx.ecosystem.required:
        key = (required.name, required.app)
        if key not in present:
            ctx.hold(f"required check {_q(required.name)} ({required.workflow}) has not run on the head commit")
        elif key not in successful and present[key] in OK_CONCLUSIONS:
            ctx.hold(f"required check {_q(required.name)} concluded {present[key]}; it must succeed")


RULES: tuple[Callable[[Context], None], ...] = (
    rule_pull_request,
    rule_head_sha,
    rule_commits,
    rule_metadata,
    rule_ecosystem,
    rule_update_types,
    rule_deny_list,
    rule_checks,
)


def decide(snapshot: object) -> dict:
    """The verdict for one snapshot. Never raises: a failing rule is a reason."""
    ctx = Context(snapshot if isinstance(snapshot, dict) else {})
    if not isinstance(snapshot, dict):
        ctx.hold("the snapshot is not a JSON object")
    for rule in RULES:
        try:
            rule(ctx)
        except Exception as exc:  # noqa: BLE001 - fail closed on anything
            ctx.hold(f"{rule.__name__} could not be evaluated ({type(exc).__name__}: {_q(str(exc))})")
    if not ctx.reasons and (ctx.ecosystem is None or not ctx.dependencies):
        ctx.hold("no ecosystem or no dependency was established")
    number = _dig(ctx.snapshot, "pull_request", "number")
    sha = ctx.snapshot.get("head_sha")
    return {
        "decision": "hold" if ctx.reasons else "merge",
        "reasons": ctx.reasons,
        "dependencies": ctx.dependencies or [],
        "pull_request": number if type(number) is int else None,
        "head_sha": sha if isinstance(sha, str) and SHA_RE.fullmatch(sha) else None,
        "ecosystem": ctx.ecosystem.name if ctx.ecosystem else None,
    }


def hold_verdict(reason: str, head_sha: object = None) -> dict:
    return {
        "decision": "hold",
        "reasons": [reason],
        "dependencies": [],
        "pull_request": None,
        "head_sha": head_sha if isinstance(head_sha, str) and SHA_RE.fullmatch(head_sha) else None,
        "ecosystem": None,
    }


# --------------------------------------------------------------- GitHub I/O


class GateError(RuntimeError):
    """GitHub could not be read; the caller prints a hold verdict and exits 1."""


Api = Callable[[str], object]
PER_PAGE = 100
MAX_PAGES = 30
MERGEABLE_ATTEMPTS = 5
MERGEABLE_WAIT_SECONDS = 3


def gh_api(path: str) -> object:
    """One GET through the GitHub CLI (it reads GH_TOKEN)."""
    try:
        proc = subprocess.run(
            ["gh", "api", "--method", "GET", "-H", "Accept: application/vnd.github+json", path],
            capture_output=True,
            text=True,
            timeout=120,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise GateError(f"gh api {path}: {exc}") from exc
    if proc.returncode != 0:
        raise GateError(f"gh api {path} failed: {proc.stderr.strip()[:300]}")
    try:
        return json.loads(proc.stdout)
    except ValueError as exc:
        raise GateError(f"gh api {path} did not return JSON") from exc


def fetch_all(api: Api, path: str, key: str | None = None, *, check_total: bool = False) -> list:
    """Every page of a list endpoint (or of the list under `key`)."""
    items: list = []
    total: object = None
    separator = "&" if "?" in path else "?"
    for page in range(1, MAX_PAGES + 1):
        body = api(f"{path}{separator}per_page={PER_PAGE}&page={page}")
        if key is not None:
            if not isinstance(body, dict):
                raise GateError(f"{path}: page {page} is not an object")
            total = body.get("total_count")
            body = body.get(key)
        chunk = body
        if not isinstance(chunk, list):
            raise GateError(f"{path}: page {page} is not a list")
        items.extend(chunk)
        if len(chunk) < PER_PAGE:
            break
    else:
        raise GateError(f"{path}: more than {MAX_PAGES} pages")
    if check_total and total != len(items):
        raise GateError(f"{path}: total_count is {total!r} but {len(items)} items were read")
    return items


def resolve_pull_request(api: Api, repo: str, head_sha: str) -> tuple[int | None, str]:
    """The one open pull request of this repository whose head is head_sha."""
    pulls = fetch_all(api, f"repos/{repo}/commits/{head_sha}/pulls")
    numbers = [
        _dig(pull, "number")
        for pull in pulls
        if _dig(pull, "state") == "open"
        and _dig(pull, "head", "sha") == head_sha
        and _dig(pull, "base", "repo", "full_name") == repo
    ]
    if len(numbers) != 1 or type(numbers[0]) is not int:
        return None, f"expected exactly one open pull request with head {head_sha}, found {len(numbers)}"
    return numbers[0], ""


def _pick(value: object, *keys: str) -> dict:
    return {key: value.get(key) for key in keys} if isinstance(value, dict) else {}


def build_snapshot(api: Api, repo: str, number: int, head_sha: str | None) -> dict:
    """Read one pull request and keep only the fields the rules read."""
    pr = api(f"repos/{repo}/pulls/{number}")
    for _ in range(MERGEABLE_ATTEMPTS - 1):
        # GitHub computes mergeability in the background; null means "not yet".
        if not isinstance(pr, dict) or pr.get("mergeable") is not None or pr.get("state") != "open":
            break
        time.sleep(MERGEABLE_WAIT_SECONDS)
        pr = api(f"repos/{repo}/pulls/{number}")
    pr_sha = _dig(pr, "head", "sha")
    if not isinstance(pr_sha, str) or not SHA_RE.fullmatch(pr_sha):
        raise GateError(f"pull request {number} has no head SHA")
    commits = fetch_all(api, f"repos/{repo}/pulls/{number}/commits")
    files = fetch_all(api, f"repos/{repo}/pulls/{number}/files")
    runs = fetch_all(api, f"repos/{repo}/commits/{pr_sha}/check-runs?filter=latest", "check_runs", check_total=True)
    statuses = fetch_all(api, f"repos/{repo}/commits/{pr_sha}/status", "statuses")
    manifests = {eco.manifest for eco in ECOSYSTEMS}
    return {
        "repository": repo,
        "head_sha": head_sha if head_sha is not None else pr_sha,
        "pull_request": {
            **_pick(pr, "number", "state", "draft", "mergeable", "mergeable_state", "commits", "changed_files"),
            "user": _pick(pr.get("user"), "login", "id", "type"),
            "base": {"ref": _dig(pr, "base", "ref"), "repo": _pick(_dig(pr, "base", "repo"), "full_name")},
            "head": {
                "ref": _dig(pr, "head", "ref"),
                "sha": pr_sha,
                "repo": _pick(_dig(pr, "head", "repo"), "full_name"),
            },
            "labels": [_pick(label, "name") for label in _list(pr.get("labels"))],
        },
        "commits": [
            {
                "sha": _dig(commit, "sha"),
                "author": _pick(_dig(commit, "author"), "login", "id"),
                "committer": _pick(_dig(commit, "committer"), "login", "id"),
                "commit": {
                    "message": _dig(commit, "commit", "message"),
                    "verification": _pick(_dig(commit, "commit", "verification"), "verified", "reason"),
                },
            }
            for commit in commits
        ],
        "files": [
            _pick(entry, "filename", "status", *(("patch",) if _dig(entry, "filename") in manifests else ()))
            for entry in files
        ],
        "check_runs": [
            {**_pick(run, "name", "status", "conclusion", "head_sha"), "app": _pick(_dig(run, "app"), "slug")}
            for run in runs
        ],
        "statuses": [_pick(status, "context", "state") for status in statuses],
    }


# ----------------------------------------------------------------- reporting

MODES = {
    "decide-only": "decide-only: the workflow merges nothing in this mode, whatever the verdict",
    "merge": "merge: a `merge` verdict is merged by the workflow",
}


def render_summary(verdict: dict, mode: str | None) -> str:
    number = verdict.get("pull_request")
    lines = [
        f"### Dependabot merge gate: {str(verdict['decision']).upper()}",
        "",
        f"- Pull request: {'#' + str(number) if number is not None else 'not resolved'}",
        f"- Head: {verdict.get('head_sha') or 'unknown'}",
        f"- Ecosystem: {verdict.get('ecosystem') or 'unknown'}",
    ]
    if mode is not None:
        lines.append(f"- Mode: {MODES[mode]}")
    if verdict["reasons"]:
        lines += ["", "Reasons to hold:", ""] + [f"- {reason}" for reason in verdict["reasons"]]
    if verdict["dependencies"]:
        lines += ["", "Dependencies:", ""]
        lines += [
            f"- {dep.get('dependency-name')} {dep.get('dependency-version', '(no version)')} "
            f"({dep.get('update-type', 'no update type')})"
            for dep in verdict["dependencies"]
        ]
    # The text carries strings from the pull request: keep it inert as Markdown.
    text = "\n".join(lines)
    for char, entity in (("&", "&amp;"), ("<", "&lt;"), (">", "&gt;"), ("](", "] (")):
        text = text.replace(char, entity)
    return "".join(ch if ch == "\n" or " " <= ch <= "~" else "?" for ch in text) + "\n"


def emit(verdict: dict, args: argparse.Namespace) -> None:
    summary = render_summary(verdict, args.mode)
    print(json.dumps(verdict, indent=2))
    sys.stderr.write(summary)
    if args.summary:
        with open(args.summary, "a", encoding="utf-8") as handle:
            handle.write(summary)
    if args.github_output:
        number = verdict.get("pull_request")
        with open(args.github_output, "a", encoding="utf-8") as handle:
            handle.write(f"decision={verdict['decision']}\n")
            handle.write(f"pr_number={number if number is not None else ''}\n")
            handle.write(f"head_sha={verdict.get('head_sha') or ''}\n")


def run_gate(args: argparse.Namespace, api: Api | None = None) -> int:
    api = api or gh_api
    if not REPO_RE.fullmatch(args.repo):
        print(f"error: --repo is not OWNER/NAME: {args.repo!r}", file=sys.stderr)
        return 2
    try:
        number = args.pr
        if number is None:
            if not SHA_RE.fullmatch(args.head_sha):
                emit(hold_verdict(f"the triggering head SHA is not a commit SHA: {_q(args.head_sha)}"), args)
                return 0
            number, reason = resolve_pull_request(api, args.repo, args.head_sha)
            if number is None:
                emit(hold_verdict(reason, args.head_sha), args)
                return 0
        snapshot = build_snapshot(api, args.repo, number, args.head_sha)
    except GateError as exc:
        emit(hold_verdict(f"GitHub could not be read, so nothing was decided: {exc}", args.head_sha), args)
        return 1
    if args.snapshot_out:
        Path(args.snapshot_out).write_text(json.dumps(snapshot, indent=1) + "\n", encoding="utf-8")
    emit(decide(snapshot), args)
    return 0


def run_decide(args: argparse.Namespace) -> int:
    try:
        snapshot = json.loads(Path(args.snapshot).read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        print(f"error: cannot read {args.snapshot}: {exc}", file=sys.stderr)
        return 2
    emit(decide(snapshot), args)
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Decide whether a Dependabot pull request may merge.")
    commands = parser.add_subparsers(dest="command", required=True)
    gate = commands.add_parser("gate", help="read a pull request from GitHub and decide")
    gate.add_argument("--repo", required=True, help="OWNER/NAME")
    target = gate.add_mutually_exclusive_group(required=True)
    target.add_argument("--head-sha", help="head SHA of the workflow run that triggered the gate")
    target.add_argument("--pr", type=int, help="pull request number (local use; its current head is trusted)")
    gate.add_argument("--snapshot-out", help="also write the snapshot the decision used")
    offline = commands.add_parser("decide", help="decide from a snapshot file")
    offline.add_argument("snapshot")
    for command in (gate, offline):
        command.add_argument("--mode", choices=sorted(MODES), help="the workflow's GATE_MODE, for the summary")
        command.add_argument("--summary", help="append the summary to this file (GITHUB_STEP_SUMMARY)")
        command.add_argument("--github-output", help="append decision, pr_number and head_sha (GITHUB_OUTPUT)")
    args = parser.parse_args(argv)
    return run_gate(args) if args.command == "gate" else run_decide(args)


if __name__ == "__main__":
    sys.exit(main())
