#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Decide whether a Dependabot pull request may merge without a human.

`.github/workflows/dependabot-gate.yml` runs this from `main` each time a
workflow of a Dependabot pull request completes. The decision is a pure
function of one JSON snapshot and the policy tables below. The snapshot holds
the pull request, its commits, how far its head is behind `main`, its changed
files (with the base and head contents of go.mod, website/package.json and
website/package-lock.json), the check runs and commit statuses of its head
commit, and the workflow runs of that commit with the jobs of the runs a
required check can come from. Only the `gate` and `up-to-date` commands talk
to GitHub, through read-only `gh api` calls.

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
        named module must be in go.mod at exactly the named version. Two
        relaxations, each named in the verdict's `admitted` list when it
        lets a line through:
          - `indirect-side-effect`: a requirement that is `// indirect` on
            every side where it exists and that the metadata does not name
            may be removed, may be added at a released or pseudo-version
            (unless another requirement has the same path apart from its
            `/vN` or `.vN` suffix), and may change to a released or
            pseudo-version that is strictly newer by semver precedence (a
            step from v0 to v1 is named as a crossing in the rule);
          - `allow-0x-minor`: a module under an ALLOW_0X_MINOR prefix
            (`golang.org/x/`) that is not on the deny list may take a minor
            bump of a 0.x version, direct or indirect.
        The deny list, `+incompatible`, a downgrade, a move between direct
        and indirect, a pre-release, and a pseudo-version or 0.0.z version
        of a direct requirement still hold. A require line that is not a
        comment and not `<module path> <version>[ // indirect]`, with a
        module path module.CheckPath accepts, holds;
      - website/package.json: only the dependency sections change, only for
        dependencies the metadata names, from one plain semver range to
        another with the same operator, classified the same way;
      - website/package-lock.json: it is among the changed files, and every
        entry that changes is an installed package whose `resolved` is
        exactly the registry.npmjs.org tarball of the name in its key and
        of its version, with a sha512 integrity. None is an alias (a `name`
        field), except an alias `main` already has whose only change is its
        `peer`/`dev`/`optional`/`devOptional` flags; none gains an install
        script, none goes to a version lower than the base's or to another
        major, and every dependency the metadata names is locked at exactly
        the named version;
      - no carriage return in any of those files;
  - the update types: semver-patch or semver-minor for every dependency in
    the metadata as well; a minor bump whose new version is 0.x (unless
    `allow-0x-minor` admits the module) and any 0.0.z version are treated
    as breaking;
  - the deny list: no sync-pair dependency (DENY_GOMOD, DENY_NPM), neither
    named in the metadata nor changed in go.mod or package.json as a side
    effect;
  - the checks: every check run and commit status of the head commit is
    completed and successful (or skipped or neutral), except IGNORED_CHECKS
    and the checks of a fork's runs, and every REQUIRED check of the
    ecosystem is there and successful, so a check that never started cannot
    pass by its absence;
  - the provenance of those checks: each required GitHub Actions check is
    looked up by name in the check suite of Dependabot's newest
    `pull_request` run of the expected workflow file, on this repository's
    branch, for this head (dependabot[bot] by user id). It counts only as a
    job of that run's latest attempt that ran on a runner (a check a job
    creates through the API, like `JUnit Report (...)`, is listed among the
    jobs of a run too, without a runner or steps, and never counts), and
    only when the run, the check run, the job and every step of the job
    (success or skipped) succeeded. No `pull_request` run of a required
    workflow file on this repository's branch for this head was started by
    anyone else, superseded or not; a fork's run, which anyone can start
    with the same head commit, neither satisfies nor holds anything. A
    commit identity alone does not prove Dependabot pushed the head: GitHub
    signs the squash-merge commit of any Dependabot pull request the same
    way. The control that keeps a job from forging that evidence is in the
    workflows: no job that runs pull request code may write checks
    (`scripts/test_dependabot_gate.py` enforces it). Not persisting the
    checkout token is hygiene, not that control.

Usage:
  dependabot-gate.py gate --repo OWNER/NAME (--head-sha SHA | --pr NUMBER)
                          [--mode MODE] [--summary FILE] [--github-output FILE]
                          [--snapshot-out FILE]
  dependabot-gate.py decide SNAPSHOT.json [--mode MODE] [--summary FILE]
                          [--github-output FILE]
  dependabot-gate.py up-to-date --repo OWNER/NAME --head-sha SHA

`gate` and `decide` print the verdict as JSON on stdout,
`{"decision": "merge"|"hold", "reasons": [...], "admitted": [...],
"dependencies": [...], ...}`, and a readable summary on stderr. `reasons` are
the reasons to hold; `admitted` lists the lines a relaxation let through, each
with the rule's name. Exit 0 when a verdict was reached (merge or
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
# Digits are spelled `[0-9]` everywhere: `\d` also matches the digits of other
# scripts, which `int()` reads as numbers and no registry does.
_NUMBER = r"(?:0|[1-9][0-9]*)"
_PRE_RELEASE = r"[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*"
# A released version: no pre-release, build or pseudo-version suffix.
VERSION_RE = re.compile(rf"v?({_NUMBER})\.({_NUMBER})\.({_NUMBER})")
# A semantic version with an optional pre-release (a Go pseudo-version is
# one) and no build metadata, so never `+incompatible`.
SEMVER_RE = re.compile(rf"v?({_NUMBER})\.({_NUMBER})\.({_NUMBER})(?:-({_PRE_RELEASE}))?")
# The three pseudo-version forms of go.dev/ref/mod#pseudo-versions:
# vX.0.0-yyyymmddhhmmss-abcdefabcdef, vX.Y.Z-pre.0.yyyymmddhhmmss-abcdefabcdef
# and vX.Y.(Z+1)-0.yyyymmddhhmmss-abcdefabcdef.
_PSEUDO_BASE = rf"(?:0\.0-|{_NUMBER}\.{_NUMBER}-(?:[0-9A-Za-z-]+\.)*0\.)"
PSEUDO_VERSION_RE = re.compile(rf"v{_NUMBER}\.{_PSEUDO_BASE}[0-9]{{14}}-[0-9a-f]{{12}}")

# -------------------------------------------------------------------- checks

ACTIONS_APP = "github-actions"
CODE_SCANNING_APP = "github-advanced-security"
OK_CONCLUSIONS = frozenset({"success", "skipped", "neutral"})
# What each step of a job behind a required check may conclude.
STEP_CONCLUSIONS = frozenset({"success", "skipped"})

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
# check suite id) ran this file and lists the check among its jobs; see
# rule_check_provenance.
WORKFLOW_FILES = {
    "CI": ".github/workflows/ci.yml",
    "Lint": ".github/workflows/lint.yml",
    "Formal Verification": ".github/workflows/formal-verification.yml",
    "CodeQL": ".github/workflows/codeql.yml",
    "Test Website Build": ".github/workflows/test-website.yml",
}
# The runs whose jobs are read, and that only Dependabot may have started on
# `pull_request` for the head commit.
REQUIRED_WORKFLOW_PATHS = frozenset(WORKFLOW_FILES.values())

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
# its required jobs has an `if` (test-report and pgo-commit have one and are
# not required). All fourteen ran on #190; website-only #189 started
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


# --------------------------------------------------------------- relaxations

# The two rules that let through a go.mod line the classifier would hold. The
# maintainer approved both on 2026-11-24, after the strict rules were measured
# against the repository's history: they merged 2 of 16 Go updates shaped like
# a Dependabot group, and 49 of the 57 "0.x minor" reasons were golang.org/x
# modules. Each line a rule admits is named in the verdict's `admitted` list.
RULE_INDIRECT = "indirect-side-effect"
RULE_0X_MINOR = "allow-0x-minor"

# Module path prefix -> why a minor bump of a 0.x version is not treated as
# breaking there. Only that one reason is lifted, for direct and indirect
# requirements alike: a 0.0.z version, a pseudo-version, a pre-release, a
# major and a downgrade hold as everywhere else. A module on the deny list is
# never admitted, whatever its path (golang.org/x/tools).
ALLOW_0X_MINOR: dict[str, str] = {
    "golang.org/x/": (
        "the Go project keeps every golang.org/x module at v0 and tags a new "
        "minor for each routine release, so the 0.x minor carries no "
        "breaking-change signal there; `make test` and the required checks "
        "exercise the ones this module imports"
    ),
}


@dataclass(frozen=True)
class Ecosystem:
    name: str
    branch_prefix: str  # Dependabot's head-branch prefix
    manifest: str
    # The file every update must change, where the named dependencies are
    # bound to a version. A Go update always rewrites go.mod, which lists
    # indirect requirements too. An npm update inside the declared range
    # changes only the lockfile, and one that moves a range changes the
    # lockfile too (its root entry mirrors package.json).
    required_file: str
    files: frozenset[str]  # every file such a pull request may change
    deny: dict[str, str]
    required: tuple[Required, ...]
    allow_0x_minor: dict[str, str] = field(default_factory=dict)


# Only the root module and the website have a Dependabot entry, so only their
# files are allowed (the one other go.mod in the tree is a test fixture).
# CONTENT_RULES, further down, names the files whose contents are read.
ECOSYSTEMS = (
    Ecosystem(
        "gomod",
        "dependabot/go_modules/",
        "go.mod",
        "go.mod",
        frozenset({"go.mod", "go.sum"}),
        DENY_GOMOD,
        REQUIRED_GOMOD,
        ALLOW_0X_MINOR,
    ),
    Ecosystem(
        "npm",
        "dependabot/npm_and_yarn/",
        "website/package.json",
        "website/package-lock.json",
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
    r"[-+]?([0-9][0-9_]*)?\.?[0-9]*([eE][-+]?[0-9]+)?|0x[0-9a-fA-F]+|true|false|null|yes|no|on|off|y|n",
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


PROBLEM_0X_MINOR = "a minor bump of a 0.x version, treated as breaking"


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
        return PROBLEM_0X_MINOR
    return None


def semver_key(version: object) -> tuple | None:
    """A sort key in semver precedence order, or None when `version` is not `[v]x.y.z[-pre]`.

    A release sorts after its pre-releases; pre-release identifiers compare
    one by one, a numeric one below an alphanumeric one, and a shorter list
    below a longer one it starts. A Go pseudo-version is a pre-release, so it
    sorts by its base version and then by its timestamp. Build metadata
    (`+incompatible`) is not read: such a version has no key.
    """
    match = SEMVER_RE.fullmatch(version) if isinstance(version, str) else None
    if match is None:
        return None
    release = (int(match.group(1)), int(match.group(2)), int(match.group(3)))
    if match.group(4) is None:
        return (release, (1,))
    identifiers: list[tuple[int, int, str]] = []
    for identifier in match.group(4).split("."):
        if not identifier.isdigit():
            identifiers.append((1, 0, identifier))
        elif identifier != "0" and identifier.startswith("0"):
            return None  # a numeric identifier has no leading zero
        else:
            identifiers.append((0, int(identifier), ""))
    return (release, (0, tuple(identifiers)))


_GOMOD_REQUIRE = re.compile(r"(\S+) (v\S+)( // indirect)?")
# A module path as golang.org/x/mod/module.CheckPath accepts it, as far as a
# pattern says it. Each element: ASCII letters, digits and `-._~`, no leading
# or trailing dot (so never `.` or `..`), not a Windows reserved name and not
# a Windows short name (a tilde and digits) before its first dot. The first
# element is a lower-case domain with a dot. A quoted path is not one: Go
# reads `"golang.org/x/tools"` as golang.org/x/tools, this reader would not.
_GO_WINDOWS_NAMES = r"(?!(?i:con|prn|aux|nul|com[1-9]|lpt[1-9])(?:[./]|\Z))(?![^/.]*~[0-9]+(?:[./]|\Z))"
_GO_FIRST_ELEMENT = rf"(?=[^/]*\.){_GO_WINDOWS_NAMES}[0-9a-z](?:[0-9a-z.-]*[0-9a-z-])?"
_GO_ELEMENT = rf"{_GO_WINDOWS_NAMES}[0-9A-Za-z_~-](?:[0-9A-Za-z._~-]*[0-9A-Za-z_~-])?"
_GO_MODULE_PATH = re.compile(rf"{_GO_FIRST_ELEMENT}(?:/{_GO_ELEMENT})*")
# module.SplitPathVersion: a final `/vN` element is a major version from v2
# on, without a leading zero or a dot; a gopkg.in path ends in `.vN`.
_GO_BAD_MAJOR = re.compile(r".*/v(?:[0-9.]*\.[0-9.]*|0[0-9.]*|1)")
_GOPKG_IN = re.compile(r"gopkg\.in/.+\.v(?:0|[1-9][0-9]*)(?:-unstable)?")


def go_module_path_ok(path: str) -> bool:
    """Whether `path` is a module path golang.org/x/mod/module.CheckPath would accept."""
    if not _GO_MODULE_PATH.fullmatch(path) or _GO_BAD_MAJOR.fullmatch(path):
        return False
    return not path.startswith("gopkg.in/") or _GOPKG_IN.fullmatch(path) is not None


def _requirement(line: str) -> tuple[str, str, bool] | None:
    """(module, version, indirect) of a requirement line, None for a comment; ValueError otherwise."""
    if line.strip().startswith("//"):
        return None
    match = _GOMOD_REQUIRE.fullmatch(line)
    if match is None or not go_module_path_ok(match.group(1)):
        raise ValueError(f"a require line this gate does not read: {line.strip()!a}")
    return match.group(1), match.group(2), match.group(3) is not None


def parse_go_mod(text: str) -> tuple[dict[str, tuple[str, bool]], list[str]]:
    """({module: (version, indirect)}, every other non-blank line) of a go.mod.

    Requirements are read from `require ( ... )` blocks and one-line
    `require` directives only. A line of the same shape in another block
    (`exclude`) stays in the second value, with the `go`, `toolchain`, `tool`
    and `replace` lines and the comments, so the caller can demand that
    nothing there changes. Raises ValueError for a module required twice and
    for a line of a require directive that is neither a comment nor
    `<module path> <version>[ // indirect]` with a valid module path.
    """
    requirements: dict[str, tuple[str, bool]] = {}
    others: list[str] = []
    in_block = False
    for line in text.split("\n"):
        requirement = None
        if in_block and line == ")":
            in_block = False
            continue
        if in_block and line.strip():
            requirement = _requirement(line.removeprefix("\t"))
        elif line == "require (":
            in_block = True
            continue
        elif line.startswith("require "):
            requirement = _requirement(line[len("require ") :])
        if requirement is None:
            if line.strip():
                others.append(line)
            continue
        module, version, indirect = requirement
        if module in requirements:
            raise ValueError(f"{module} is required twice")
        requirements[module] = (version, indirect)
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
_NPM_RANGE = re.compile(rf"([~^]?)({_NUMBER}\.{_NUMBER}\.{_NUMBER})")
_NPM_INTEGRITY = re.compile(r"sha512-[A-Za-z0-9+/]{86}==")
NPM_REGISTRY = "https://registry.npmjs.org/"
# A lockfile key: `node_modules/<name>`, nested any number of times. The one
# group is the name of the package the entry installs. No part of a name is
# `.` or `..`: `node_modules/..` names a directory, not a package.
_NPM_NAME_PART = r"(?!\.\.?(?:/|\Z))[A-Za-z0-9._~-]+"
_NPM_NAME = rf"(?:@{_NPM_NAME_PART}/)?{_NPM_NAME_PART}"
_NPM_LOCK_KEY = re.compile(rf"(?:node_modules/{_NPM_NAME}/)*node_modules/({_NPM_NAME})")


def npm_version_key(version: object) -> tuple | None:
    """semver_key for a lockfile version, which has no `v` prefix."""
    if not isinstance(version, str) or version.startswith("v"):
        return None
    return semver_key(version)


def registry_tarball(name: str, version: object) -> str | None:
    """The one URL the npm registry serves `name@version` from, or None for a version that is not plain semver.

    https://registry.npmjs.org/<name>/-/<name without its scope>-<version>.tgz
    is how the registry names every tarball; all 1386 registry entries of
    the lockfile on main had that form on 2026-11-24.
    """
    if npm_version_key(version) is None:
        return None
    return f"{NPM_REGISTRY}{name}/-/{name.rsplit('/', 1)[-1]}-{version}.tgz"


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
    admitted: list[str] = field(default_factory=list)
    ecosystem: Ecosystem | None = None
    dependencies: list[dict[str, str]] | None = None

    def hold(self, reason: str) -> None:
        if reason not in self.reasons:
            self.reasons.append(reason)

    def admit(self, rule: str, line: str) -> None:
        """Record a line that only a relaxation let through, with the rule's name."""
        note = f"{line}: admitted by {rule}"
        if note not in self.admitted:
            self.admitted.append(note)


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
    if eco.required_file not in seen:
        ctx.hold(
            f"{eco.required_file} is not among the changed files "
            f"(nothing else binds the dependencies a {eco.name} update names to a version)"
        )


def _allows_0x_minor(eco: Ecosystem | None, module: str) -> str | None:
    """The ALLOW_0X_MINOR prefix that admits `module`, or None."""
    if eco is None or module in eco.deny:
        return None
    return next((prefix for prefix in eco.allow_0x_minor if module.startswith(prefix)), None)


def _step_problem(ctx: Context, eco: Ecosystem, line: str, module: str, old: str, new: str) -> str | None:
    """update_problem for one go.mod requirement, after the `allow-0x-minor` relaxation."""
    problem = update_problem(old, new)
    prefix = _allows_0x_minor(eco, module) if problem == PROBLEM_0X_MINOR else None
    if prefix is None:
        return problem
    ctx.admit(RULE_0X_MINOR, f"{line} (a minor bump of a 0.x version under {prefix})")
    return None


# The major-version suffix of a module path: example.com/mod/v2, gopkg.in/yaml.v3.
_GOMOD_MAJOR_SUFFIX = re.compile(r"[/.]v[0-9]+$")


def _major_sibling(module: str, requirements: set[str]) -> str | None:
    """Another requirement with the same path apart from its `/vN` or `.vN` suffix, or None."""
    stem = _GOMOD_MAJOR_SUFFIX.sub("", module)
    siblings = (other for other in sorted(requirements) if other != module and _GOMOD_MAJOR_SUFFIX.sub("", other) == stem)
    return next(siblings, None)


Requirements = dict[str, tuple[str, bool]]


def _check_indirect_side_effect(ctx: Context, eco: Ecosystem, filename: str, module: str, old: Requirements, new: Requirements) -> None:
    """The `indirect-side-effect` relaxation: an indirect requirement the metadata does not name.

    The Go command rewrites these lines when a named module moves: it drops
    one nothing needs any more, adds one a new version needs, and raises one
    to the minimum another module asks for. None of them is an update anyone
    chose, so the patch-or-minor classification does not apply; what holds
    is a step that cannot be such a side effect (a downgrade, a version Go
    does not order, a second major of a module already required).
    """
    if module not in new:
        ctx.admit(RULE_INDIRECT, f"{filename} removes the indirect requirement {_q(module + ' ' + old[module][0])}")
        return
    version = new[module][0]
    key = semver_key(version)
    if key is None:
        ctx.hold(
            f"{filename}: the indirect requirement {_q(module)} is at {_q(version)}, which is not a semantic version "
            "without build metadata (`+incompatible` never merges here)"
        )
    elif module not in old:
        _check_indirect_addition(ctx, filename, module, version, set(old) | set(new))
    else:
        _check_indirect_change(ctx, eco, filename, module, old[module][0], version)


def _check_indirect_addition(ctx: Context, filename: str, module: str, version: str, requirements: set[str]) -> None:
    line = f"{filename} adds the indirect requirement {_q(module + ' ' + version)}"
    sibling = _major_sibling(module, requirements)
    if not VERSION_RE.fullmatch(version) and not PSEUDO_VERSION_RE.fullmatch(version):
        ctx.hold(f"{line}, which is neither a released x.y.z version nor a pseudo-version")
    elif sibling is not None:
        ctx.hold(f"{line}, another major version of the requirement {_q(sibling)}")
    else:
        ctx.admit(RULE_INDIRECT, line)


def _check_indirect_change(ctx: Context, eco: Ecosystem, filename: str, module: str, old: str, new: str) -> None:
    """A changed unnamed indirect requirement: strictly forward, to a released or pseudo-version.

    A pre-release holds, as it does for an added requirement: the Go command
    raises an indirect requirement to what another module asks for, and a
    module that asks for a release candidate is not a routine update. A step
    from v0 to v1 is forward by semver precedence and is admitted, with the
    crossing named in the rule so the job summary shows it.
    """
    line = f"{filename}: {_q(module)} {_q(old)} -> {_q(new)}"
    old_key, new_key = semver_key(old), semver_key(new)
    if old_key is None or new_key is None or new_key <= old_key:
        ctx.hold(f"{line} is not a step forward by semver precedence (a downgrade, or a version with build metadata)")
    elif not VERSION_RE.fullmatch(new) and not PSEUDO_VERSION_RE.fullmatch(new):
        ctx.hold(f"{line} is a step to a pre-release, which is neither a released x.y.z version nor a pseudo-version")
    elif _step_problem(ctx, eco, line, module, old, new) is not None:
        # Forward, but not a patch or minor update of a released version.
        rule = RULE_INDIRECT
        if new_key[0][0] != old_key[0][0]:
            rule = f"{RULE_INDIRECT} (crosses v{old_key[0][0]} to v{new_key[0][0]})"
        ctx.admit(rule, f"{line} (an indirect requirement the metadata does not name, strictly newer)")


def _check_requirement(
    ctx: Context, eco: Ecosystem, filename: str, module: str, old: Requirements, new: Requirements, named: bool
) -> None:
    """One changed go.mod requirement under the strict rules (a direct or a named one)."""
    if module not in old:
        ctx.hold(f"{filename} adds the requirement {_q(module + ' ' + new[module][0])}")
        return
    if module not in new:
        ctx.hold(f"{filename} removes the requirement {_q(module + ' ' + old[module][0])}")
        return
    (old_version, old_indirect), (new_version, new_indirect) = old[module], new[module]
    if old_indirect != new_indirect:
        ctx.hold(f"{filename} moves {_q(module)} between direct and indirect")
    if old_version != new_version:
        line = f"{filename}: {_q(module)} {_q(old_version)} -> {_q(new_version)}"
        problem = _step_problem(ctx, eco, line, module, old_version, new_version)
        if problem:
            ctx.hold(f"{line} is {problem}")
    if not new_indirect and not named:
        ctx.hold(f"{filename} changes the direct requirement {_q(module)}, which the dependency metadata does not name")


def check_go_mod(ctx: Context, eco: Ecosystem, filename: str, base: str, head: str) -> None:
    """go.mod: requirement versions only, each step classified here.

    go.sum is not read. The Go command checks every module it downloads
    against the line go.sum has for it, so a line that does not match what
    the module proxy serves fails the build, and an extra line selects
    nothing. It consults the checksum database only for a module go.sum has
    no line for: the lines themselves are taken as the pull request wrote
    them.
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
            # Nothing else is said about it: no step of a sync pair merges here.
            ctx.hold(f"{filename} changes {module}, which is on the sync-pair deny list: {eco.deny[module]}")
            continue
        indirect_everywhere = all(side[module][1] for side in (old, new) if module in side)
        if indirect_everywhere and module not in named:
            _check_indirect_side_effect(ctx, eco, filename, module, old, new)
        else:
            _check_requirement(ctx, eco, filename, module, old, new, module in named)
    for name, dependency in named.items():
        # Every named module is in go.mod, changed, at exactly the named version.
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


def _check_lock_root(ctx: Context, filename: str, entry: dict, before: object) -> None:
    """The root entry is the project itself: its dependency sections mirror package.json."""
    if split_dependency_sections(entry)[1] != split_dependency_sections(before if isinstance(before, dict) else {})[1]:
        ctx.hold(f"{filename}: the root entry changes something other than its dependency ranges")


# The flags npm sets on a lockfile entry from where the package sits in the
# dependency graph (a dev-only, optional or peer dependency). Moving another
# package can flip them on an entry whose package does not change.
NPM_LOCK_FLAGS = ("peer", "dev", "optional", "devOptional")


def _alias_flags_only(entry: dict, before: object) -> bool:
    """An alias `main` already has, of which nothing but the graph flags changed.

    Its entry minus NPM_LOCK_FLAGS equals the base entry minus the same keys,
    and each flag is absent or `true` (the only values npm writes). The
    package, its version, tarball and integrity are what `main` installs
    already. Anything else about an alias holds.
    """
    if "name" not in entry or not isinstance(before, dict):
        return False
    if any(entry.get(flag, True) is not True for flag in NPM_LOCK_FLAGS):
        return False

    def unflagged(value: dict) -> dict:
        return {name: field for name, field in value.items() if name not in NPM_LOCK_FLAGS}

    return unflagged(entry) == unflagged(before)


def _check_lock_entry(ctx: Context, filename: str, key: str, entry: dict, before: object) -> None:
    """One changed lockfile entry other than the root: a registry package, as npm writes it."""
    match = _NPM_LOCK_KEY.fullmatch(key)
    if match is None:
        ctx.hold(f"{filename}: the entry {_q(key)} is not an installed package under node_modules/")
        return
    if _alias_flags_only(entry, before):
        return
    if "name" in entry:
        alias = _q(entry["name"])
        ctx.hold(f"{filename}: {_q(key)} is an alias of {alias} (a `name` field): it installs another package under this name")
    version, resolved, integrity = entry.get("version"), entry.get("resolved"), entry.get("integrity")
    expected = registry_tarball(match.group(1), version)
    if expected is None:
        ctx.hold(f"{filename}: {_q(key)} has the version {_q(version)}, which is not a plain semantic version")
    if expected is None or resolved != expected:
        ctx.hold(f"{filename}: {_q(key)} resolves from {_q(resolved)}, not from {expected or NPM_REGISTRY}")
    if not isinstance(integrity, str) or not _NPM_INTEGRITY.fullmatch(integrity):
        ctx.hold(f"{filename}: {_q(key)} has no sha512 integrity")
    if entry.get("hasInstallScript", False) is not False and _dig(before, "hasInstallScript") is not True:
        ctx.hold(f"{filename}: {_q(key)} gains an install script")
    if before is not None:
        _check_lock_step(ctx, filename, key, _dig(before, "version"), version)


def _check_lock_step(ctx: Context, filename: str, key: str, old: object, new: object) -> None:
    """An entry main already has never goes to a lower version or to another major.

    Nothing else about the step is judged: most changed entries are
    transitive packages no one named, and a minor bump of a 0.x one (react-dom
    19.3.1 brings scheduler 0.27.0 -> 0.28.0) is ordinary there.
    """
    old_key, new_key = npm_version_key(old), npm_version_key(new)
    if old_key is None:
        ctx.hold(f"{filename}: {_q(key)} was at {_q(old)} on the base, which is not a plain semantic version to compare with")
    elif new_key is None:
        return  # held by _check_lock_entry: the version is not plain semver
    elif new_key < old_key:
        ctx.hold(f"{filename}: {_q(key)} {_q(old)} -> {_q(new)} is a downgrade")
    elif new_key[0][0] != old_key[0][0]:
        ctx.hold(f"{filename}: {_q(key)} {_q(old)} -> {_q(new)} is a major update")


def _check_lock_binding(ctx: Context, filename: str, old_packages: dict, new_packages: dict) -> None:
    """Every dependency the metadata names is locked, at the top level, at exactly the named version."""
    for dependency in ctx.dependencies or []:
        name, version = dependency["dependency-name"], dependency.get("dependency-version", "")
        key = f"node_modules/{name}"
        locked = _dig(new_packages.get(key), "version")
        if locked is None or locked == _dig(old_packages.get(key), "version"):
            ctx.hold(f"the dependency metadata names {_q(name)}, but {filename} does not change the version of {_q(key)}")
        elif locked != version:
            ctx.hold(f"the dependency metadata gives {_q(name)} {_q(version)}, but {filename} locks {_q(locked)}")


def check_package_lock(ctx: Context, eco: Ecosystem, filename: str, base: str, head: str) -> None:
    """package-lock.json: what `npm ci` on main would install after the merge.

    Only the entries that differ from the base are read: the others are on
    main already. Each one must be the registry tarball of the package its
    key names, at its own version, with a sha512 integrity. None may be an
    alias, gain an install script (it would run in deploy-website.yml), go
    back to an older version or move to another major, and the dependencies
    the metadata names must be locked at the versions it gives.
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
            _check_lock_root(ctx, filename, entry, before)
        else:
            _check_lock_entry(ctx, filename, key, entry, before)
    _check_lock_binding(ctx, filename, old_packages, new_packages)


# The changed files whose base and head contents are read, and by what.
CONTENT_RULES: dict[str, Callable[[Context, Ecosystem, str, str, str], None]] = {
    "go.mod": check_go_mod,
    "website/package.json": check_package_json,
    "website/package-lock.json": check_package_lock,
}


def _file_texts(ctx: Context, filename: str, entry: object) -> tuple[str, str] | None:
    """The base and head text of a changed file, or None (with a reason) when they cannot be judged."""
    base, head = _dig(entry, "base"), _dig(entry, "head")
    if not isinstance(base, str) or not isinstance(head, str) or not base or not head:
        ctx.hold(f"the base and head contents of {filename} could not be read")
        return None
    if "\r" in base or "\r" in head:
        # Go reads a bare carriage return as a space and this reader does
        # not split lines on it: the two would see different files.
        ctx.hold(f"{filename} contains a carriage return")
        return None
    return base, head


def rule_contents(ctx: Context) -> None:
    eco = ctx.ecosystem
    if eco is None:
        ctx.hold("the file contents could not be evaluated without a known ecosystem")
        return
    for entry in _list(ctx.snapshot.get("files")):
        filename = _dig(entry, "filename")
        if filename not in eco.files or filename not in CONTENT_RULES:
            continue
        texts = _file_texts(ctx, filename, entry)
        if texts is None:
            continue
        try:
            CONTENT_RULES[filename](ctx, eco, filename, *texts)
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
        elif version.group(1) == "0" and update_type == UPDATE_MINOR and not _allows_0x_minor(ctx.ecosystem, dependency["dependency-name"]):
            # check_go_mod names the rule when it admits the same step in go.mod.
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


def _readable_run(run: object) -> bool:
    """Whether a workflow run of the snapshot has every field the rules read, with its type.

    `head_repository.id` is None for a run whose fork was deleted: such a run
    is not on a branch of this repository (_own_run), and counts as a fork's
    run (fork_suites).
    """
    numbers = (
        _dig(run, "id"),
        _dig(run, "check_suite_id"),
        _dig(run, "actor", "id"),
        _dig(run, "repository", "id"),
        _dig(run, "run_attempt"),
    )
    head_repository = _dig(run, "head_repository")
    return (
        all(type(number) is int for number in numbers)
        and isinstance(_dig(run, "path"), str)
        and isinstance(_dig(run, "event"), str)
        and "conclusion" in run
        and isinstance(head_repository, dict)
        and "id" in head_repository
        and (head_repository["id"] is None or type(head_repository["id"]) is int)
    )


def _own_run(run: dict) -> bool:
    """A run of a branch of this repository; a fork's run is not one. Needs _readable_run."""
    return run["head_repository"]["id"] == run["repository"]["id"]


def fork_suites(snapshot: dict) -> set[int]:
    """The check suites of the head's workflow runs that a fork's pull request started.

    Anyone can open a pull request from a fork with the head commit of a
    Dependabot pull request, and a maintainer may approve its runs: they run
    on the same commit, so their checks are listed with Dependabot's. They
    neither satisfy nor hold anything here. A run whose head repository is
    gone (`head_repository: null`, a deleted fork) is one of them: a branch of
    this repository always has this repository as its head repository.
    """
    return {run["check_suite_id"] for run in _list(snapshot.get("workflow_runs")) if _readable_run(run) and not _own_run(run)}


def rule_checks(ctx: Context) -> None:
    head = _dig(ctx.snapshot, "pull_request", "head", "sha")
    runs = ctx.snapshot.get("check_runs")
    statuses = ctx.snapshot.get("statuses")
    if not isinstance(runs, list):
        ctx.hold("the check runs could not be read")
    if not isinstance(statuses, list):
        ctx.hold("the commit statuses could not be read")
    forks = fork_suites(ctx.snapshot)
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
        suite = _dig(run, "check_suite", "id")
        if (name, app) in IGNORED_CHECKS or (type(suite) is int and suite in forks):
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
        # A required GitHub Actions check is looked up in Dependabot's own
        # run instead (rule_check_provenance): a fork's later check of the
        # same name may stand in its place in this listing.
        if not results and required.app != ACTIONS_APP:
            ctx.hold(f"required check {_q(required.name)} ({required.workflow}) has not run on the head commit")
        # Every run of that name: a second one that was skipped proves nothing.
        for outcome in results:
            if outcome != "success" and outcome in OK_CONCLUSIONS:
                ctx.hold(f"required check {_q(required.name)} concluded {outcome}; it must succeed")


def _hold_unless_dependabot_started(ctx: Context, run: dict) -> None:
    if run["actor"]["id"] != DEPENDABOT[1]:
        ctx.hold(
            f"a {_q(run['event'])} run of {_q(run['path'])} for the head commit was started by user id "
            f"{run['actor']['id']}, not by dependabot[bot] ({DEPENDABOT[1]})"
        )


def _head_runs(ctx: Context, head: object) -> list[dict]:
    """The workflow runs of the head commit; a run that cannot be read holds.

    A `pull_request` run of a required workflow file, on a branch of this
    repository, that someone else started holds here, whether or not a
    required check still points at it: it exists only because that person
    pushed the head or reopened the pull request, and Dependabot's later runs
    do not undo that. A fork's run is not held on (fork_suites).
    """
    runs = ctx.snapshot.get("workflow_runs")
    if not isinstance(runs, list):
        ctx.hold("the workflow runs could not be read")
    readable: list[dict] = []
    for run in _list(runs):
        path = _dig(run, "path")
        if not _readable_run(run):
            ctx.hold(f"a workflow run of the head commit could not be read ({_q(path)})")
        elif head is None or _dig(run, "head_sha") != head:
            ctx.hold(f"the workflow run of {_q(path)} is not for the pull request head")
        else:
            readable.append(run)
            if _own_run(run) and path in REQUIRED_WORKFLOW_PATHS and run["event"] == "pull_request":
                _hold_unless_dependabot_started(ctx, run)
    return readable


def _bound_run(runs: list[dict], path: str) -> dict | None:
    """Dependabot's newest `pull_request` run of `path` on this repository's branch, or None."""
    candidates = [
        run
        for run in runs
        if run["path"] == path and run["event"] == "pull_request" and _own_run(run) and run["actor"]["id"] == DEPENDABOT[1]
    ]
    return max(candidates, key=lambda run: run["id"]) if candidates else None


def _ran_on_a_runner(job: dict) -> bool:
    """A job that a runner ran has a runner id and steps; a check created through the API has neither."""
    runner, steps = job.get("runner_id"), job.get("steps")
    return type(runner) is int and runner > 0 and isinstance(steps, list) and len(steps) > 0


def _latest_attempt_checks(run: dict, name: str) -> tuple[list[tuple[dict, dict]], str | None]:
    """The (check run, job) pairs of `name` in the run's latest attempt, or why they cannot be told.

    The check runs are those of the run's own check suite, every attempt
    (`filter=all`). Each must be a job of the run (same id). A job of an
    earlier attempt is superseded by the re-run and skipped. A check of that
    name in the latest attempt that no runner ran is one a job created
    through the API (GitHub lists those among the jobs too, with no runner
    and no steps): it is never a required check, and it holds.
    """
    jobs, checks = run.get("jobs"), run.get("check_runs")
    if not isinstance(jobs, list) or not isinstance(checks, list):
        return [], "the jobs or the check runs of that run could not be read"
    pairs: list[tuple[dict, dict]] = []
    for check in checks:
        if _dig(check, "check_suite", "id") != run["check_suite_id"]:
            return [], "a check run listed for the run's check suite is of another suite"
        if _dig(check, "name") != name or _dig(check, "app", "slug") != ACTIONS_APP:
            continue
        check_id = _dig(check, "id")
        matches = [job for job in jobs if type(check_id) is int and type(_dig(job, "id")) is int and job["id"] == check_id]
        if len(matches) != 1:
            return [], f"{len(matches)} of its jobs have the id of a check of that name"
        attempt = _dig(matches[0], "run_attempt")
        if type(attempt) is int and attempt < run["run_attempt"]:
            continue
        if type(attempt) is not int or attempt != run["run_attempt"]:
            return [], f"a job of that name has the attempt {_q(attempt)}, not one of the run's"
        if not _ran_on_a_runner(matches[0]):
            return [], "a check of that name has no runner or no steps, as a check created through the API"
        pairs.append((check, matches[0]))
    return pairs, None


def _job_problem(check: dict, job: dict, head: object) -> str | None:
    """Why a bound check run and its job do not prove the check passed, or None.

    The check run and the job are one record (same id) and a job with
    `checks: write` can PATCH the conclusion of a check run, so neither
    conclusion alone is the evidence. The steps are reported by the runner
    and no checks API call writes them.
    """
    if head is None or _dig(check, "head_sha") != head:
        return "its check run is not for the pull request head"
    if _dig(check, "status") != "completed" or _dig(check, "conclusion") != "success":
        return f"its check run concluded {_q(_dig(check, 'conclusion'))} (status {_q(_dig(check, 'status'))})"
    if job.get("conclusion") != "success":
        return f"its job concluded {_q(job.get('conclusion'))}"
    if not all(step in STEP_CONCLUSIONS for step in job["steps"]):
        return f"a step of its job concluded {_q(next(step for step in job['steps'] if step not in STEP_CONCLUSIONS))}"
    return None


def _check_origin(ctx: Context, required: Required, runs: list[dict], head: object) -> tuple[int, int] | None:
    """Bind one required GitHub Actions check to a job of Dependabot's run; (check run id, check suite id), or None."""
    path = WORKFLOW_FILES[required.workflow]
    label = f"required check {_q(required.name)} ({required.workflow})"
    run = _bound_run(runs, path)
    if run is None:
        ctx.hold(f"{label}: dependabot[bot] started no pull_request run of {path} on this repository's branch for the head commit")
        return None
    where = f"in Dependabot's run {run['id']} of {path} (attempt {run['run_attempt']})"
    if run["conclusion"] != "success":
        ctx.hold(f"Dependabot's run {run['id']} of {path}, behind required checks, concluded {_q(run['conclusion'])}")
    pairs, problem = _latest_attempt_checks(run, required.name)
    if problem is None and len(pairs) != 1:
        problem = "it has not run" if not pairs else f"{len(pairs)} jobs of the latest attempt report it"
    if problem is None:
        problem = _job_problem(*pairs[0], head)
    if problem is not None:
        ctx.hold(f"{label} {where}: {problem}")
        return None
    return pairs[0][0]["id"], run["check_suite_id"]


def rule_check_provenance(ctx: Context) -> None:
    """Who started the workflow run behind each required GitHub Actions check, and which job reported it.

    Only what Dependabot does itself (opening the pull request, pushing to
    its branch) makes dependabot[bot] the `actor` of a `pull_request` run.
    Someone who force-pushes a Dependabot-authored, GitHub-signed commit onto
    the branch (the squash-merge commit of another Dependabot pull request has
    that shape) is the actor of the runs that push starts, and so is someone
    who closes and reopens the pull request. A person who re-runs a job only
    becomes `triggering_actor`, which is not read: the run still tested
    Dependabot's push.

    Each required check is looked up by name among the check runs of
    Dependabot's own run of its workflow file (_check_origin), never in a
    fork's run. It counts only as a job of that run's latest attempt that a
    runner ran, with the run, the check run, the job and every step
    successful. The control that keeps a job from forging that evidence is
    that no job running pull request code may write checks (ci.yml publishes
    its JUnit reports from a job that runs none); this is defence in depth.
    A check elsewhere on the head, outside a fork's run, named like a
    required one and not the bound job holds too.
    """
    head = _dig(ctx.snapshot, "pull_request", "head", "sha")
    runs = _head_runs(ctx, head)
    checks = _list(ctx.snapshot.get("check_runs"))
    for check in checks:
        if type(_dig(check, "id")) is not int:
            ctx.hold(f"check {_q(_dig(check, 'name'))} has no id")
        if type(_dig(check, "check_suite", "id")) is not int:
            ctx.hold(f"check {_q(_dig(check, 'name'))} has no check suite id")
    if ctx.ecosystem is None:
        ctx.hold("the origin of the required checks could not be evaluated without a known ecosystem")
        return
    # Another app's required check is reported by the app itself; its slug is part of the check key.
    bound = {required: _check_origin(ctx, required, runs, head) for required in ctx.ecosystem.required if required.app == ACTIONS_APP}
    _check_listing(ctx, checks, bound)


def _check_listing(ctx: Context, checks: list, bound: dict[Required, tuple[int, int] | None]) -> None:
    """The head's check runs (`filter=latest`) must show each bound job under its name, and nothing else.

    Outside a fork's runs, the checks named like a required GitHub Actions
    check must all be the bound check run, in the bound run's check suite:
    another one is a second check of that name (a re-run Dependabot's run
    does not explain, or one created through the API). None may be listed
    only when a fork's later check of the same name stands in its place.
    """
    forks = fork_suites(ctx.snapshot)
    names = {required.name for required in bound}
    listed: dict[str, set[object]] = {}
    hidden: set[str] = set()
    for check in checks:
        name, suite = _dig(check, "name"), _dig(check, "check_suite", "id")
        if not isinstance(name, str) or name not in names or _dig(check, "app", "slug") != ACTIONS_APP:
            continue
        if type(suite) is int and suite in forks:
            hidden.add(name)
        else:
            listed.setdefault(name, set()).add((_dig(check, "id"), suite))
    for required, origin in bound.items():
        ids = listed.get(required.name, set())
        if origin is None or ids == {origin} or (not ids and required.name in hidden):
            continue
        shown = ", ".join(sorted(f"{_q(other)} (suite {_q(suite)})" for other, suite in ids)) or "none"
        ctx.hold(
            f"required check {_q(required.name)} ({required.workflow}): the check runs of the head commit list {shown} "
            f"under its name, not only the job {origin[0]} of Dependabot's run (suite {origin[1]})"
        )


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
        "admitted": ctx.admitted,
        "dependencies": ctx.dependencies or [],
        "pull_request": number if type(number) is int else None,
        "head_sha": sha if isinstance(sha, str) and SHA_RE.fullmatch(sha) else None,
        "ecosystem": ctx.ecosystem.name if ctx.ecosystem else None,
    }


def hold_verdict(reason: str, head_sha: object = None) -> dict:
    return {
        "decision": "hold",
        "reasons": [reason],
        "admitted": [],
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
    The output is read as bytes and decoded strictly: text mode would turn a
    bare carriage return into a line feed, and the rules would then read a
    file Go does not read (rule_contents holds on a carriage return).
    """
    accept = "application/vnd.github.raw+json" if raw else "application/vnd.github+json"
    try:
        proc = subprocess.run(
            ["gh", "api", "--method", "GET", "-H", f"Accept: {accept}", path],
            capture_output=True,
            timeout=120,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise GateError(f"gh api {path}: {exc}") from exc
    if proc.returncode != 0:
        raise GateError(f"gh api {path} failed: {proc.stderr.decode('utf-8', 'replace').strip()[:300]}")
    try:
        text = proc.stdout.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise GateError(f"gh api {path} did not return UTF-8 text") from exc
    if raw:
        return text
    try:
        return json.loads(text)
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


def _check_run_fields(run: object) -> dict:
    return {
        **_pick(run, "id", "name", "status", "conclusion", "head_sha"),
        "app": _pick(_dig(run, "app"), "slug"),
        "check_suite": _pick(_dig(run, "check_suite"), "id"),
    }


def _job_fields(job: object) -> dict:
    """One job of a workflow run: its id, runner, attempt, conclusion and the conclusion of each step."""
    return {
        **_pick(job, "id", "runner_id", "run_attempt", "conclusion"),
        "steps": [_dig(step, "conclusion") for step in _list(_dig(job, "steps"))],
    }


def _workflow_run_fields(api: Api, repo: str, run: object) -> dict:
    """One workflow run of the head commit, with its jobs and check runs when a required check can come from it.

    `filter=all` lists the jobs, and the check runs of the run's check suite,
    of every attempt of the run: a re-run keeps the check suite, and every
    job of the new attempt gets a new id. The rules bind a required check to
    the latest attempt. `head_repository` is null for a run whose fork was
    deleted, and kept as an id of None. A page that cannot be read, or a
    listing shorter than its `total_count`, raises GateError.
    """
    fields = {
        **_pick(run, "id", "path", "event", "head_sha", "check_suite_id", "run_attempt", "conclusion"),
        "actor": _pick(_dig(run, "actor"), "id"),
        "repository": _pick(_dig(run, "repository"), "id"),
        "head_repository": {"id": _dig(run, "head_repository", "id")},
    }
    run_id, suite = fields.get("id"), fields.get("check_suite_id")
    if fields.get("path") in REQUIRED_WORKFLOW_PATHS and type(run_id) is int and type(suite) is int:
        jobs = fetch_all(api, f"repos/{repo}/actions/runs/{run_id}/jobs?filter=all", "jobs", check_total=True)
        fields["jobs"] = [_job_fields(job) for job in jobs]
        checks = fetch_all(api, f"repos/{repo}/check-suites/{suite}/check-runs?filter=all", "check_runs", check_total=True)
        fields["check_runs"] = [_check_run_fields(check) for check in checks]
    return fields


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
    labels = pr.get("labels")
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
            # None when GitHub sent no list, so that rule_pull_request holds.
            "labels": [_pick(label, "name") for label in labels] if isinstance(labels, list) else None,
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
        "check_runs": [_check_run_fields(run) for run in runs],
        "statuses": [_pick(status, "context", "state") for status in statuses],
        "workflow_runs": [_workflow_run_fields(api, repo, run) for run in workflow_runs],
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
    if verdict.get("admitted"):
        lines += ["", "Admitted by a relaxation (rule named at the end of each line):", ""]
        lines += [f"- {line}" for line in verdict["admitted"]]
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
