#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Decide whether a Dependabot pull request may merge without a human.

`.github/workflows/dependabot-gate.yml` runs this from `main` each time a
workflow of a Dependabot pull request completes. The decision is a pure
function of one JSON snapshot and the policy tables below. The snapshot holds
the pull request, its commits, how far its head is behind `main`, its changed
files (with the base and head contents of go.mod, website/package.json and
website/package-lock.json), the check runs and commit statuses of its head
commit, and the workflow runs of that commit. Only the `gate` and
`up-to-date` commands talk to GitHub, through read-only `gh api` calls.

The gate FAILS CLOSED. Every rule appends a reason when its condition is not
exactly the expected value; a missing field, an unknown value, a rule that
raises and anything else unexpected is a reason too. The verdict is `merge`
only when no rule left a reason. `merge` requires all of:

  - the pull request: opened by dependabot[bot], base `main`, head branch in
    this repository (no fork), open, not a draft, mergeable without conflict,
    no `do-not-merge` or `hold` label;
  - the head: the SHA the triggering workflow run tested is the pull
    request's current head, and that head is not behind `main` (the `main`
    ruleset does not require up-to-date branches, so GitHub never reports
    "behind": the checks of a stale head tested an older `main`);
  - the commits: exactly one, the head, authored by dependabot[bot],
    committed by GitHub for it and with a verified signature (a push by
    anyone else, or a second commit the metadata does not describe, holds);
  - the metadata: the `updated-dependencies:` block of that commit's message
    (what dependabot/fetch-metadata reads) parses under the strict reader
    below;
  - the ecosystem: gomod or npm, by head-branch prefix, and every changed
    file is one of that ecosystem's files (GitHub Actions updates never
    merge here: pins are major tags, and PR CI does not exercise
    release.yml). A gomod update must change go.mod;
  - the file contents, compared base against head by this script, because
    nothing binds Dependabot's metadata to the files:
      - go.mod: only requirement versions change. Each step is classified
        here (update_problem): a major, a downgrade, a pre-release, a
        pseudo-version, `+incompatible`, a minor bump of a 0.x version, any
        0.0.z version, and an added or removed requirement all hold. A
        changed direct requirement must be named in the metadata, and every
        named module must be in go.mod at exactly the named version;
      - website/package.json: only the dependency sections change, only for
        dependencies the metadata names, from one plain semver range to
        another with the same operator, classified the same way;
      - website/package-lock.json: every entry that changes resolves from
        registry.npmjs.org with a sha512 integrity, and none gains an
        install script;
  - the update types: semver-patch or semver-minor for every dependency in
    the metadata as well; a minor bump whose new version is 0.x and any
    0.0.z version are treated as breaking;
  - the deny list: no sync-pair dependency (DENY_GOMOD, DENY_NPM), neither
    named in the metadata nor changed in go.mod or package.json as a side
    effect;
  - the checks: every check run and commit status of the head commit is
    completed and successful (or skipped or neutral), except IGNORED_CHECKS,
    and every REQUIRED check of the ecosystem is there and successful, so a
    check that never started cannot pass by its absence;
  - the provenance of those checks: the workflow run behind every required
    GitHub Actions check was started by dependabot[bot] (by user id), on
    `pull_request`, from the expected workflow file, for this head. A commit
    identity alone does not prove Dependabot pushed the head: GitHub signs
    the squash-merge commit of any Dependabot pull request the same way.

Usage:
  dependabot-gate.py gate --repo OWNER/NAME (--head-sha SHA | --pr NUMBER)
                          [--mode MODE] [--summary FILE] [--github-output FILE]
                          [--snapshot-out FILE]
  dependabot-gate.py decide SNAPSHOT.json [--mode MODE] [--summary FILE]
                          [--github-output FILE]
  dependabot-gate.py up-to-date --repo OWNER/NAME --head-sha SHA

`gate` and `decide` print the verdict as JSON on stdout,
`{"decision": "merge"|"hold", "reasons": [...], "dependencies": [...], ...}`,
and a readable summary on stderr. Exit 0 when a verdict was reached (merge or
hold); 1 when GitHub could not be read (the verdict printed is `hold`); 2 on
a usage error. Any other exit is a crash, and the workflow merges nothing
then: its merge steps need `decision=merge` from a step that succeeded.

`up-to-date` is the workflow's last look before it merges: exit 0 only when
SHA is not behind `main` right now, 1 otherwise or when GitHub could not be
read, 2 on a usage error.
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
# signs them. No commit of those pull requests has the bot as committer, so
# that form is not accepted.
DEPENDABOT = ("dependabot[bot]", 49699333)
WEB_FLOW = ("web-flow", 19864447)
BASE_BRANCH = "main"
HOLD_LABELS = frozenset({"do-not-merge", "hold"})
# `mergeable_state` depends on who asks (an anonymous reader saw "unstable"
# where the maintainer's token saw "blocked"), so it only screens out what is
# never mergeable: "dirty" (conflict), "draft", "unknown" and any value not
# listed here. The conflict rule proper is `mergeable is true`. It is NOT the
# staleness rule: the `main` ruleset has `strict_required_status_checks_policy:
# false`, so GitHub never reports "behind" here. rule_up_to_date reads the
# compare of `main` with the head instead.
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
    # For a GitHub Actions check, the `name:` of the workflow that produces it
    # (a key of WORKFLOW_FILES). For another app's check, the service.
    workflow: str
    app: str = ACTIONS_APP


# Workflow `name:` -> the file a run of it reports as `path`. A required
# GitHub Actions check counts only when the workflow run behind it (matched by
# check suite id) ran this file; see rule_check_provenance.
WORKFLOW_FILES = {
    "CI": ".github/workflows/ci.yml",
    "Lint": ".github/workflows/lint.yml",
    "Formal Verification": ".github/workflows/formal-verification.yml",
    "CodeQL": ".github/workflows/codeql.yml",
    "Test Website Build": ".github/workflows/test-website.yml",
}

SOCKET_APP = "socket-security"
SONAR_APP = "sonarqubecloud"
# The security checks other apps report. No workflow file produces them, so
# nothing but this list makes their absence a hold. Names and app slugs read
# from `gh api repos/invowk/invowk/commits/<sha>/check-runs` for the heads of
# #189, #190 and #191 (2026-11-24): all three were there and successful on
# each. `SonarCloud` is the code-scanning result of the SonarCloud analysis
# (new alerts fail it), as `CodeQL` below is for the CodeQL analyses.
REQUIRED_EXTERNAL = (
    Required("Socket Security: Pull Request Alerts", "Socket Security", SOCKET_APP),
    Required("SonarCloud Code Analysis", "SonarCloud", SONAR_APP),
    Required("SonarCloud", "SonarCloud", CODE_SCANNING_APP),
)

# Reported on every pull request: lint.yml, formal-verification.yml and
# codeql.yml have no `paths:` filter on `pull_request` and no job-level `if`.
# Seen successful on #189 (npm) and #191, and present on #190 (gomod).
# scripts/test_dependabot_gate.py re-checks each name, trigger and path filter
# against the workflow files, so a renamed job fails the script tests instead
# of leaving the gate on hold for good.
REQUIRED_COMMON = REQUIRED_EXTERNAL + (
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
# in go.mod holds through check_go_mod instead.
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
    manifest: str
    # Whether every update changes the manifest. A Go update always rewrites
    # go.mod, which lists indirect requirements too; an npm update inside the
    # declared range changes only the lockfile.
    manifest_required: bool
    files: frozenset[str]  # every file such a pull request may change
    deny: dict[str, str]
    required: tuple[Required, ...]


# Only the root module and the website have a Dependabot entry, so only their
# files are allowed (the one other go.mod in the tree is a test fixture).
# CONTENT_RULES, further down, names the files whose contents are read.
ECOSYSTEMS = (
    Ecosystem(
        "gomod",
        "dependabot/go_modules/",
        "go.mod",
        True,
        frozenset({"go.mod", "go.sum"}),
        DENY_GOMOD,
        REQUIRED_GOMOD,
    ),
    Ecosystem(
        "npm",
        "dependabot/npm_and_yarn/",
        "website/package.json",
        False,
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
# What a quoted value may hold: a plain value, or one YAML makes Dependabot
# quote (a leading `@`, a number). Nothing with a space: `'@docusaurus/core '`
# would otherwise be a name the deny list does not know.
_QUOTED_TEXT = re.compile(r"[A-Za-z0-9@][A-Za-z0-9._/:+~@-]*")
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
    if not _QUOTED_TEXT.fullmatch(value):
        raise MetadataError(f"value is not a plain name or version: {value!a}")
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


# ------------------------------------------------------------ file contents


def update_problem(old: str, new: str) -> str | None:
    """Why `old -> new` is not a patch or minor update, or None when it is one.

    The classification is made here, from the versions in the files. The
    update type in Dependabot's metadata covers only the dependencies it
    names, and nothing binds it to what the files change.
    """
    before, after = VERSION_RE.fullmatch(old), VERSION_RE.fullmatch(new)
    if after is None:
        return "not an update to a released x.y.z version (a pre-release, a pseudo-version or `+incompatible`)"
    if before is None:
        return "an update from a version that is not a released x.y.z version"
    was, now = tuple(map(int, before.groups())), tuple(map(int, after.groups()))
    if now < was:
        return "a downgrade"
    if now == was:
        return "not a version change"
    if now[0] != was[0]:
        return "a major update"
    if now[:2] == (0, 0):
        return "a change to a 0.0.z version, treated as breaking"
    if now[0] == 0 and now[1] != was[1]:
        return "a minor bump of a 0.x version, treated as breaking"
    return None


_GOMOD_REQUIRE = re.compile(r"(\S+) (v\S+)( // indirect)?")


def parse_go_mod(text: str) -> tuple[dict[str, tuple[str, bool]], list[str]]:
    """({module: (version, indirect)}, every other non-blank line) of a go.mod.

    Requirements are read from `require ( ... )` blocks and one-line
    `require` directives only. A line of the same shape in another block
    (`exclude`) stays in the second value, with the `go`, `toolchain`, `tool`
    and `replace` lines and the comments, so the caller can demand that
    nothing there changes. Raises ValueError for a module required twice.
    """
    requirements: dict[str, tuple[str, bool]] = {}
    others: list[str] = []
    in_block = False
    for line in text.split("\n"):
        match = None
        if in_block:
            if line == ")":
                in_block = False
                continue
            if line.startswith("\t"):
                match = _GOMOD_REQUIRE.fullmatch(line[1:])
        elif line == "require (":
            in_block = True
            continue
        elif line.startswith("require "):
            match = _GOMOD_REQUIRE.fullmatch(line[len("require ") :])
        if match is None:
            if line.strip():
                others.append(line)
            continue
        module, version, indirect = match.groups()
        if module in requirements:
            raise ValueError(f"{module} is required twice")
        requirements[module] = (version, indirect is not None)
    if in_block:
        raise ValueError("a require block is not closed")
    return requirements, others


def parse_json_object(text: str) -> dict:
    """A JSON object, read strictly: a repeated key or a non-JSON number is an error.

    npm keeps the last of two equal keys; a reader that kept another one
    would compare a different file from the one npm installs.
    """

    def pairs(items: list[tuple[str, object]]) -> dict:
        keys = [key for key, _ in items]
        if len(keys) != len(set(keys)):
            raise ValueError("a JSON object repeats a key")
        return dict(items)

    def constant(name: str) -> object:
        raise ValueError(f"{name} is not JSON")

    value = json.loads(text, object_pairs_hook=pairs, parse_constant=constant)
    if not isinstance(value, dict):
        raise ValueError("not a JSON object")
    return value


# The sections of a package.json (and of the root entry of a lockfile) that
# hold dependency ranges. Everything else must be the same in base and head.
DEPENDENCY_SECTIONS = ("dependencies", "devDependencies", "optionalDependencies", "peerDependencies")
# `^1.2.3`, `~1.2.3` or `1.2.3`: no URL, git or file reference, alias, tag,
# wildcard or compound range.
_NPM_RANGE = re.compile(r"([~^]?)((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))")
_NPM_RESOLVED = re.compile(r"https://registry\.npmjs\.org/[A-Za-z0-9@._~/+-]+\.tgz")
_NPM_INTEGRITY = re.compile(r"sha512-[A-Za-z0-9+/]{86}==")


def split_dependency_sections(package: dict) -> tuple[dict[str, dict[str, str]], dict]:
    """({section: {name: range}}, the other fields) of a package.json object."""
    sections: dict[str, dict[str, str]] = {}
    for section in DEPENDENCY_SECTIONS:
        value = package.get(section, {})
        if not isinstance(value, dict) or not all(isinstance(spec, str) for spec in value.values()):
            raise ValueError(f"`{section}` is not a map of names to version ranges")
        sections[section] = value
    return sections, {key: value for key, value in package.items() if key not in DEPENDENCY_SECTIONS}


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


def rule_up_to_date(ctx: Context) -> None:
    """The head must contain all of `main`, or its checks tested an older base."""
    behind = _dig(ctx.snapshot, "compare", "behind_by")
    merge_base = _dig(ctx.snapshot, "compare", "merge_base_sha")
    if type(behind) is not int:
        ctx.hold(f"how far the head is behind {BASE_BRANCH} could not be read")
    elif behind != 0:
        ctx.hold(
            f"the head is behind {BASE_BRANCH} (by {behind}), so its checks tested an older base: "
            "wait for Dependabot's rebase or comment `@dependabot rebase`"
        )
    if not isinstance(merge_base, str) or not SHA_RE.fullmatch(merge_base):
        ctx.hold(f"the merge base with {BASE_BRANCH} (where the base file contents are read) is not a commit SHA")


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
        if _identity(_dig(commit, "committer")) != WEB_FLOW:
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
    seen: set[object] = set()
    for entry in files:
        filename = _dig(entry, "filename")
        if filename not in eco.files:
            ctx.hold(f"changed file {_q(filename)} is not one a {eco.name} update may touch")
            continue
        if filename in seen:
            ctx.hold(f"{filename} is listed twice among the changed files")
        seen.add(filename)
        if _dig(entry, "status") != "modified":
            ctx.hold(f"{filename} is {_q(_dig(entry, 'status'))}, not modified")
    if eco.manifest_required and eco.manifest not in seen:
        ctx.hold(f"{eco.manifest} is not among the changed files (a lockfile-only {eco.name} change never merges)")


def check_go_mod(ctx: Context, eco: Ecosystem, filename: str, base: str, head: str) -> None:
    """go.mod: requirement versions only, each step classified here.

    go.sum is not read: the Go toolchain verifies every module against it and
    the checksum database, so a wrong line fails the build and an extra line
    selects nothing.
    """
    old, old_others = parse_go_mod(base)
    new, new_others = parse_go_mod(head)
    if old_others != new_others:
        ctx.hold(
            f"{filename} changes something other than a requirement's version "
            "(a `go`, `toolchain`, `tool`, `replace` or `exclude` line, or a comment)"
        )
    named = {dependency["dependency-name"]: dependency for dependency in ctx.dependencies or []}
    changed = [module for module in sorted(set(old) | set(new)) if old.get(module) != new.get(module)]
    for module in changed:
        if module in eco.deny:
            ctx.hold(f"{filename} changes {module}, which is on the sync-pair deny list: {eco.deny[module]}")
        if module not in old:
            ctx.hold(f"{filename} adds the requirement {_q(module + ' ' + new[module][0])}")
            continue
        if module not in new:
            ctx.hold(f"{filename} removes the requirement {_q(module + ' ' + old[module][0])}")
            continue
        (old_version, old_indirect), (new_version, new_indirect) = old[module], new[module]
        if old_indirect != new_indirect:
            ctx.hold(f"{filename} moves {_q(module)} between direct and indirect")
        if old_version != new_version:
            problem = update_problem(old_version, new_version)
            if problem:
                ctx.hold(f"{filename}: {_q(module)} {_q(old_version)} -> {_q(new_version)} is {problem}")
        if not new_indirect and module not in named:
            ctx.hold(f"{filename} changes the direct requirement {_q(module)}, which the dependency metadata does not name")
    for name, dependency in named.items():
        version = dependency.get("dependency-version", "")
        if name not in changed or name not in new:
            ctx.hold(f"the dependency metadata names {_q(name)}, but {filename} does not change its version")
        elif new[name][0] != "v" + version.removeprefix("v"):
            ctx.hold(f"the dependency metadata gives {_q(name)} {_q(version)}, but {filename} requires {_q(new[name][0])}")


def check_package_json(ctx: Context, eco: Ecosystem, filename: str, base: str, head: str) -> None:
    """package.json: only named dependencies change, from one plain range to another."""
    old, old_others = split_dependency_sections(parse_json_object(base))
    new, new_others = split_dependency_sections(parse_json_object(head))
    if old_others != new_others:
        ctx.hold(
            f"{filename} changes something other than a dependency's version range "
            "(a script, `overrides`, `engines` or another field)"
        )
    named = {dependency["dependency-name"] for dependency in ctx.dependencies or []}
    for section in DEPENDENCY_SECTIONS:
        before, after = old[section], new[section]
        for name in sorted(set(before) | set(after)):
            if before.get(name) == after.get(name):
                continue
            if name in eco.deny:
                ctx.hold(f"{filename} changes {name}, which is on the sync-pair deny list: {eco.deny[name]}")
            if name not in before or name not in after:
                ctx.hold(f"{filename} adds or removes {_q(name)} in `{section}`")
                continue
            if name not in named:
                ctx.hold(f"{filename} changes {_q(name)}, which the dependency metadata does not name")
            was, now = _NPM_RANGE.fullmatch(before[name]), _NPM_RANGE.fullmatch(after[name])
            if now is None or was is None:
                ctx.hold(
                    f"{filename}: {_q(name)} {_q(before[name])} -> {_q(after[name])} is not a change between "
                    "plain semver ranges (a URL, a git or file reference, an alias or a tag never merges here)"
                )
                continue
            if was.group(1) != now.group(1):
                ctx.hold(f"{filename}: {_q(name)} {_q(before[name])} -> {_q(after[name])} changes the range operator")
            problem = update_problem(was.group(2), now.group(2))
            if problem:
                ctx.hold(f"{filename}: {_q(name)} {_q(before[name])} -> {_q(after[name])} is {problem}")


def check_package_lock(ctx: Context, eco: Ecosystem, filename: str, base: str, head: str) -> None:
    """package-lock.json: what `npm ci` on main would install after the merge.

    Only the entries that differ from the base are read: the others are on
    main already. Each one must come from the npm registry with a sha512
    integrity, and none may gain an install script, which would run in
    deploy-website.yml.
    """
    old, new = parse_json_object(base), parse_json_object(head)
    old_packages, new_packages = old.get("packages"), new.get("packages")
    if not isinstance(old_packages, dict) or not isinstance(new_packages, dict):
        ctx.hold(f"{filename} has no `packages` map (lockfileVersion 2 or 3)")
        return
    if {key: value for key, value in old.items() if key != "packages"} != {
        key: value for key, value in new.items() if key != "packages"
    }:
        ctx.hold(f"{filename} changes a field outside `packages` (its name, version or lockfileVersion)")
    for key in sorted(new_packages):
        entry, before = new_packages[key], old_packages.get(key)
        if entry == before:
            continue
        if not isinstance(entry, dict):
            ctx.hold(f"{filename}: the entry {_q(key)} is not an object")
        elif key == "":
            # The project itself: its dependency sections mirror package.json.
            if split_dependency_sections(entry)[1] != split_dependency_sections(before if isinstance(before, dict) else {})[1]:
                ctx.hold(f"{filename}: the root entry changes something other than its dependency ranges")
        else:
            if not key.startswith("node_modules/"):
                ctx.hold(f"{filename}: the entry {_q(key)} is not an installed package under node_modules/")
            resolved, integrity = entry.get("resolved"), entry.get("integrity")
            if not isinstance(resolved, str) or not _NPM_RESOLVED.fullmatch(resolved):
                ctx.hold(f"{filename}: {_q(key)} resolves from {_q(resolved)}, not from https://registry.npmjs.org/")
            if not isinstance(integrity, str) or not _NPM_INTEGRITY.fullmatch(integrity):
                ctx.hold(f"{filename}: {_q(key)} has no sha512 integrity")
            if entry.get("hasInstallScript", False) is not False and _dig(before, "hasInstallScript") is not True:
                ctx.hold(f"{filename}: {_q(key)} gains an install script")


# The changed files whose base and head contents are read, and by what.
CONTENT_RULES: dict[str, Callable[[Context, Ecosystem, str, str, str], None]] = {
    "go.mod": check_go_mod,
    "website/package.json": check_package_json,
    "website/package-lock.json": check_package_lock,
}


def rule_contents(ctx: Context) -> None:
    eco = ctx.ecosystem
    if eco is None:
        ctx.hold("the file contents could not be evaluated without a known ecosystem")
        return
    for entry in _list(ctx.snapshot.get("files")):
        filename = _dig(entry, "filename")
        if filename not in eco.files or filename not in CONTENT_RULES:
            continue
        base, head = _dig(entry, "base"), _dig(entry, "head")
        if not isinstance(base, str) or not isinstance(head, str) or not base or not head:
            ctx.hold(f"the base and head contents of {filename} could not be read")
            continue
        try:
            CONTENT_RULES[filename](ctx, eco, filename, base, head)
        except ValueError as exc:
            ctx.hold(f"{filename} is not a file this gate can read: {_q(str(exc))}")


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
        elif version.group(1) == "0" and version.group(2) == "0":
            ctx.hold(f"{name}: {dependency['dependency-version']} is a 0.0.z version, where any change is treated as breaking")
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
    outcomes: dict[tuple[object, object], list[object]] = {}
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
        outcomes.setdefault((name, app), []).append(conclusion if status == "completed" else status)
    for status in _list(statuses):
        if _dig(status, "state") != "success":
            ctx.hold(f"commit status {_q(_dig(status, 'context'))} is {_q(_dig(status, 'state'))}")
    if ctx.ecosystem is None:
        ctx.hold("the required checks could not be evaluated without a known ecosystem")
        return
    for required in ctx.ecosystem.required:
        results = outcomes.get((required.name, required.app), [])
        if not results:
            ctx.hold(f"required check {_q(required.name)} ({required.workflow}) has not run on the head commit")
        # Every run of that name: a second one that was skipped proves nothing.
        for outcome in results:
            if outcome != "success" and outcome in OK_CONCLUSIONS:
                ctx.hold(f"required check {_q(required.name)} concluded {outcome}; it must succeed")


def rule_check_provenance(ctx: Context) -> None:
    """Who started the workflow run behind each required GitHub Actions check.

    Only what Dependabot does itself (opening the pull request, pushing to
    its branch) makes dependabot[bot] the `actor` of a `pull_request` run.
    Someone who force-pushes a Dependabot-authored, GitHub-signed commit onto
    the branch (the squash-merge commit of another Dependabot pull request has
    that shape) is the actor of the runs that push starts, and so is someone
    who closes and reopens the pull request. A person who re-runs a job only
    becomes `triggering_actor`, which is not read: the run still tested
    Dependabot's push.
    """
    head = _dig(ctx.snapshot, "pull_request", "head", "sha")
    runs = ctx.snapshot.get("workflow_runs")
    if not isinstance(runs, list):
        ctx.hold("the workflow runs could not be read")
    by_suite: dict[int, list[dict]] = {}
    for run in _list(runs):
        suite, actor = _dig(run, "check_suite_id"), _dig(run, "actor", "id")
        path, event = _dig(run, "path"), _dig(run, "event")
        if type(suite) is not int or type(actor) is not int or not isinstance(path, str) or not isinstance(event, str):
            ctx.hold(f"a workflow run of the head commit could not be read ({_q(path)})")
        elif head is None or _dig(run, "head_sha") != head:
            ctx.hold(f"the workflow run of {_q(path)} is not for the pull request head")
        else:
            by_suite.setdefault(suite, []).append(run)
    for check in _list(ctx.snapshot.get("check_runs")):
        if type(_dig(check, "check_suite", "id")) is not int:
            ctx.hold(f"check {_q(_dig(check, 'name'))} has no check suite id")
    if ctx.ecosystem is None:
        ctx.hold("the origin of the required checks could not be evaluated without a known ecosystem")
        return
    for required in ctx.ecosystem.required:
        if required.app != ACTIONS_APP:
            continue  # reported by the app itself; its slug is part of the check key
        expected = WORKFLOW_FILES[required.workflow]
        for check in _list(ctx.snapshot.get("check_runs")):
            if (_dig(check, "name"), _dig(check, "app", "slug")) != (required.name, required.app):
                continue
            suite = _dig(check, "check_suite", "id")
            behind = by_suite.get(suite, []) if type(suite) is int else []
            if len(behind) != 1:
                ctx.hold(f"required check {_q(required.name)}: {len(behind)} workflow runs of the head commit stand behind it, not one")
                continue
            run = behind[0]
            if run["actor"]["id"] != DEPENDABOT[1]:
                ctx.hold(
                    f"the workflow run of {_q(run['path'])} behind required checks was started by user id "
                    f"{run['actor']['id']}, not by dependabot[bot] ({DEPENDABOT[1]})"
                )
            if run["event"] != "pull_request":
                ctx.hold(f"the workflow run of {_q(run['path'])} behind required checks ran on {_q(run['event'])}, not on pull_request")
            if run["path"] != expected:
                ctx.hold(f"required check {_q(required.name)} was reported by a run of {_q(run['path'])}, not of {expected}")


RULES: tuple[Callable[[Context], None], ...] = (
    rule_pull_request,
    rule_head_sha,
    rule_up_to_date,
    rule_commits,
    rule_metadata,
    rule_ecosystem,
    rule_contents,
    rule_update_types,
    rule_deny_list,
    rule_checks,
    rule_check_provenance,
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


# api(path) returns the decoded JSON body; api(path, raw=True) the file text.
Api = Callable[..., object]
PER_PAGE = 100
MAX_PAGES = 30
MERGEABLE_ATTEMPTS = 5
MERGEABLE_WAIT_SECONDS = 3


def gh_api(path: str, raw: bool = False) -> object:
    """One GET through the GitHub CLI (it reads GH_TOKEN).

    `raw` asks the contents API for the file itself, as text: the JSON form
    carries no content for a file over 1 MB, and the lockfile is close to it.
    """
    accept = "application/vnd.github.raw+json" if raw else "application/vnd.github+json"
    try:
        proc = subprocess.run(
            ["gh", "api", "--method", "GET", "-H", f"Accept: {accept}", path],
            capture_output=True,
            text=True,
            encoding="utf-8",
            timeout=120,
            check=False,
        )
    except (OSError, UnicodeError, subprocess.TimeoutExpired) as exc:
        raise GateError(f"gh api {path}: {exc}") from exc
    if proc.returncode != 0:
        raise GateError(f"gh api {path} failed: {proc.stderr.strip()[:300]}")
    if raw:
        return proc.stdout
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
    """The one open Dependabot pull request of this repository whose head is head_sha.

    Anyone can open a pull request from a fork with the same head commit. It
    is not a candidate (the head repository is another one, the author is not
    the bot, both by id), so it cannot make the count two and hold the real
    pull request for good.
    """
    pulls = fetch_all(api, f"repos/{repo}/commits/{head_sha}/pulls")
    numbers = [
        _dig(pull, "number")
        for pull in pulls
        if _dig(pull, "state") == "open"
        and _dig(pull, "head", "sha") == head_sha
        and _dig(pull, "base", "repo", "full_name") == repo
        and type(_dig(pull, "base", "repo", "id")) is int
        and _dig(pull, "head", "repo", "id") == _dig(pull, "base", "repo", "id")
        and _dig(pull, "user", "id") == DEPENDABOT[1]
    ]
    if len(numbers) != 1 or type(numbers[0]) is not int:
        return None, (
            f"expected exactly one open Dependabot pull request of this repository with head {head_sha}, "
            f"found {len(numbers)}"
        )
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
    # `behind_by` counts the commits of main the head lacks. With none, the
    # merge base is main's tip, and the base file contents are main's.
    compare = api(f"repos/{repo}/compare/{BASE_BRANCH}...{pr_sha}?per_page=1")
    merge_base = _dig(compare, "merge_base_commit", "sha")
    commits = fetch_all(api, f"repos/{repo}/pulls/{number}/commits")
    files = fetch_all(api, f"repos/{repo}/pulls/{number}/files")
    runs = fetch_all(api, f"repos/{repo}/commits/{pr_sha}/check-runs?filter=latest", "check_runs", check_total=True)
    statuses = fetch_all(api, f"repos/{repo}/commits/{pr_sha}/status", "statuses", check_total=True)
    workflow_runs = fetch_all(api, f"repos/{repo}/actions/runs?head_sha={pr_sha}", "workflow_runs", check_total=True)

    def contents(entry: object) -> dict:
        """Base and head text of a file the content rules read."""
        filename = _dig(entry, "filename")
        if filename not in CONTENT_RULES or _dig(entry, "status") != "modified":
            return {}
        if not isinstance(merge_base, str) or not SHA_RE.fullmatch(merge_base):
            return {}
        path = f"repos/{repo}/contents/{filename}"
        return {"base": api(f"{path}?ref={merge_base}", raw=True), "head": api(f"{path}?ref={pr_sha}", raw=True)}

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
        "compare": {"behind_by": _dig(compare, "behind_by"), "merge_base_sha": merge_base},
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
        "files": [{**_pick(entry, "filename", "status"), **contents(entry)} for entry in files],
        "check_runs": [
            {
                **_pick(run, "name", "status", "conclusion", "head_sha"),
                "app": _pick(_dig(run, "app"), "slug"),
                "check_suite": _pick(_dig(run, "check_suite"), "id"),
            }
            for run in runs
        ],
        "statuses": [_pick(status, "context", "state") for status in statuses],
        "workflow_runs": [
            {**_pick(run, "path", "event", "head_sha", "check_suite_id"), "actor": _pick(_dig(run, "actor"), "id")}
            for run in workflow_runs
        ],
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


def run_up_to_date(args: argparse.Namespace, api: Api | None = None) -> int:
    """The workflow's last look before `gh pr merge`: is the head still not behind main?

    The verdict was reached some seconds earlier, and another pull request
    may have merged since (the workflow's concurrency group is per head).
    """
    api = api or gh_api
    if not REPO_RE.fullmatch(args.repo) or not SHA_RE.fullmatch(args.head_sha):
        print("error: --repo is not OWNER/NAME or --head-sha is not a commit SHA", file=sys.stderr)
        return 2
    try:
        behind = _dig(api(f"repos/{args.repo}/compare/{BASE_BRANCH}...{args.head_sha}?per_page=1"), "behind_by")
    except GateError as exc:
        print(f"hold: GitHub could not be read: {exc}", file=sys.stderr)
        return 1
    if type(behind) is not int or behind != 0:
        print(f"hold: {args.head_sha} is behind {BASE_BRANCH} (behind_by: {_q(behind)}); nothing is merged", file=sys.stderr)
        return 1
    print(f"{args.head_sha} is not behind {BASE_BRANCH}")
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
    current = commands.add_parser("up-to-date", help="exit 0 only when a head commit is not behind main")
    current.add_argument("--repo", required=True, help="OWNER/NAME")
    current.add_argument("--head-sha", required=True, help="the head SHA of the verdict")
    args = parser.parse_args(argv)
    return {"gate": run_gate, "decide": run_decide, "up-to-date": run_up_to_date}[args.command](args)


if __name__ == "__main__":
    sys.exit(main())
