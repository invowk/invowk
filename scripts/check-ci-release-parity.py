#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Check that release.yml's `test` job runs the same test legs as ci.yml's.

The release workflow re-runs the test matrix on the tagged commit. When a
change adds a matrix key or an INVOWK_TEST_* variable to ci.yml only, the tag
run tests something else, or fails (the first v0.17.0 tag run: #173 added
`tui-plan-row` and INVOWK_TEST_TUI_PLAN_ROW to ci.yml only; #183 fixed it).

For the `test` job of both files this compares:
  - the matrix legs, matched by `os` + `engine`: the same legs, the same keys,
    the same values;
  - the test steps (steps whose `if` names matrix.test-mode, or that set an
    INVOWK_TEST_* variable), matched by step name: the same steps, each with
    the same `if` and the same INVOWK_TEST_* names and values;
  - the `go list | grep -v` package exclusions of matched steps.

Intentional differences live in the exception tables below, each with its
reason. The parser reads only the block-style YAML subset these workflows use
(mappings, sequences, scalars, `|`/`>` block scalars); it is stdlib only.

Usage: check-ci-release-parity.py [--ci PATH] [--release PATH]
Exit 0 on parity, 1 on drift (each difference on its own line), 2 on a file
it cannot read or parse.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parent.parent
DEFAULT_CI = ROOT / ".github" / "workflows" / "ci.yml"
DEFAULT_RELEASE = ROOT / ".github" / "workflows" / "release.yml"

TEST_ENV_PREFIX = "INVOWK_TEST_"
EXCLUDE_RE = re.compile(r"grep -v '\^github\.com/invowk/invowk/([^'$]+)\$'")
PACKAGES_RE = re.compile(r"--packages\s+\./([A-Za-z0-9_./-]+?)(?:/\.\.\.)?(?:\s|$)")

# Test steps that exist in one file only, as (workflow, step name) -> reason.
# A step listed in SPLIT_STEPS below needs no entry here.
ONLY_IN: dict[tuple[str, str], str] = {}

# A ci.yml step split out of a release.yml step: ci step name ->
# (release step name, reason). The split step must run under the same `if`,
# set a subset of the release step's INVOWK_TEST_* variables with the same
# values, and run exactly the packages the ci counterpart of the release step
# excludes in addition to release.yml's own exclusions.
SPLIT_STEPS: dict[str, tuple[str, str]] = {
    "Run internal/runtime tests (full)": (
        "Run tests (full, non-CLI)",
        "ci.yml runs internal/runtime in its own step (its own junit, rerun "
        "report and coverage profile, merged later for upload); release.yml "
        "keeps it in the non-CLI step and uploads no coverage. Only "
        "internal/app/commandadapters reads INVOWK_TEST_TUI_PLAN_ROW, so the "
        "runtime step does not set it.",
    ),
}

# Matrix keys allowed to differ, as (os, engine, key) -> reason.
MATRIX_KEY_EXCEPTIONS: dict[tuple[str, str, str], str] = {}

# INVOWK_TEST_* variables allowed to differ, as (step name, variable) -> reason.
ENV_EXCEPTIONS: dict[tuple[str, str], str] = {}


class ParseError(Exception):
    """The file uses YAML this minimal parser does not read."""


# ------------------------------------------------------------------ parsing


def _strip_comment(value: str) -> str:
    """Drop a trailing ` # comment` outside quotes."""
    quote = ""
    for i, ch in enumerate(value):
        if quote:
            if ch == quote:
                quote = ""
        elif ch in "'\"":
            quote = ch
        elif ch == "#" and (i == 0 or value[i - 1] in " \t"):
            return value[:i].rstrip()
    return value.rstrip()


def _scalar(value: str) -> str:
    value = _strip_comment(value.strip())
    if len(value) >= 2 and value[0] == value[-1] and value[0] in "'\"":
        return value[1:-1]
    return value


def _split_key(text: str) -> tuple[str, str] | None:
    """Split `key: rest` (or `key:`), honouring quoted keys."""
    if text[:1] in "'\"":
        end = text.find(text[0], 1)
        if end < 0 or text[end + 1 : end + 2] != ":":
            return None
        return text[1:end], text[end + 2 :]
    match = re.match(r"([^\s:#][^:#]*?):(?:\s+(.*)|$)", text)
    if not match:
        return None
    return match.group(1), match.group(2) or ""


class _Parser:
    def __init__(self, text: str) -> None:
        self.lines: list[tuple[int, str, int]] = []
        for number, raw in enumerate(text.splitlines(), 1):
            if "\t" in raw[: len(raw) - len(raw.lstrip())]:
                raise ParseError(f"line {number}: tab indentation")
            stripped = raw.strip()
            if not stripped or stripped.startswith("#") or stripped in ("---", "..."):
                continue
            self.lines.append((len(raw) - len(raw.lstrip(" ")), raw.strip(), number))
        self.i = 0

    def parse(self) -> object:
        if not self.lines:
            return {}
        value = self.block(self.lines[0][0])
        if self.i != len(self.lines):
            raise ParseError(f"line {self.lines[self.i][2]}: unexpected indentation")
        return value

    def block(self, indent: int) -> object:
        if self.lines[self.i][1].startswith("- ") or self.lines[self.i][1] == "-":
            return self.sequence(indent)
        return self.mapping(indent)

    def skip_block_scalar(self, indent: int) -> str:
        """Consume a `|`/`>` block scalar (raw lines more indented than indent).

        Comment-only lines inside it were dropped at load time; the content is
        used only for regex scans of `run` scripts, which ignore comments.
        """
        body = []
        while self.i < len(self.lines) and self.lines[self.i][0] > indent:
            body.append(self.lines[self.i][1])
            self.i += 1
        return "\n".join(body)

    def value_after_key(self, rest: str, indent: int) -> object:
        rest = _strip_comment(rest)
        if re.fullmatch(r"[|>][-+0-9]*", rest):
            return self.skip_block_scalar(indent)
        if rest:
            return _scalar(rest)
        if self.i < len(self.lines):
            child_indent, child_text, _ = self.lines[self.i]
            if child_indent > indent or (child_indent == indent and child_text.startswith("- ")):
                return self.block(child_indent)
        return ""

    def mapping(self, indent: int, first: str | None = None) -> dict[str, object]:
        result: dict[str, object] = {}
        while self.i < len(self.lines):
            line_indent, text, number = self.lines[self.i]
            if first is not None:
                text, first = first, None
            elif line_indent < indent:
                break
            elif line_indent > indent:
                raise ParseError(f"line {number}: unexpected indentation")
            elif text.startswith("- "):
                break
            split = _split_key(text)
            if split is None:
                raise ParseError(f"line {number}: not a `key: value` line: {text!r}")
            key, rest = split
            self.i += 1
            if key in result:
                raise ParseError(f"line {number}: duplicate key {key!r}")
            result[key] = self.value_after_key(rest, indent)
        return result

    def sequence(self, indent: int) -> list[object]:
        result: list[object] = []
        while self.i < len(self.lines):
            line_indent, text, number = self.lines[self.i]
            if line_indent < indent or not (text.startswith("- ") or text == "-"):
                break
            if line_indent > indent:
                raise ParseError(f"line {number}: unexpected indentation")
            item = text[1:].lstrip()
            item_indent = indent + (len(text) - len(item))
            if not item:
                self.i += 1
                result.append(self.block(self.lines[self.i][0]) if self.i < len(self.lines) else "")
            elif _split_key(item) is not None and not item.startswith(("'", '"', "[", "{")):
                result.append(self.mapping(item_indent, first=item))
            else:
                self.i += 1
                result.append(_scalar(item))
        return result


def load_yaml(text: str) -> object:
    return _Parser(text).parse()


# ----------------------------------------------------------------- extraction


def _test_job(doc: object, label: str) -> dict:
    jobs = doc.get("jobs") if isinstance(doc, dict) else None
    job = jobs.get("test") if isinstance(jobs, dict) else None
    if not isinstance(job, dict):
        raise ParseError(f"{label}: no jobs.test mapping")
    return job


def matrix_legs(job: dict, label: str) -> dict[tuple[str, str], dict[str, str]]:
    include = job.get("strategy", {}).get("matrix", {}).get("include")
    if not isinstance(include, list):
        raise ParseError(f"{label}: jobs.test.strategy.matrix.include is not a list")
    legs: dict[tuple[str, str], dict[str, str]] = {}
    for leg in include:
        if not isinstance(leg, dict) or "os" not in leg:
            raise ParseError(f"{label}: a matrix leg without `os`: {leg!r}")
        key = (str(leg["os"]), str(leg.get("engine", "")))
        if key in legs:
            raise ParseError(f"{label}: two matrix legs for {leg_name(key)}")
        legs[key] = {str(k): str(v) for k, v in leg.items()}
    return legs


def test_steps(job: dict, label: str) -> dict[str, dict]:
    steps = job.get("steps")
    if not isinstance(steps, list):
        raise ParseError(f"{label}: jobs.test.steps is not a list")
    result: dict[str, dict] = {}
    for step in steps:
        if not isinstance(step, dict):
            continue
        env = step.get("env") if isinstance(step.get("env"), dict) else {}
        test_env = {k: str(v) for k, v in env.items() if k.startswith(TEST_ENV_PREFIX)}
        condition = str(step.get("if", ""))
        if "matrix.test-mode" not in condition and not test_env:
            continue
        name = str(step.get("name", ""))
        if not name:
            raise ParseError(f"{label}: an unnamed test step (if: {condition!r})")
        if name in result:
            raise ParseError(f"{label}: two test steps named {name!r}")
        run = str(step.get("run", ""))
        result[name] = {
            "if": condition,
            "env": test_env,
            "excludes": set(EXCLUDE_RE.findall(run)),
            "packages": set(PACKAGES_RE.findall(run)),
        }
    return result


def leg_name(key: tuple[str, str]) -> str:
    return key[0] + (f"/{key[1]}" if key[1] else "")


# ------------------------------------------------------------------ comparing


def compare(ci_doc: object, release_doc: object) -> list[str]:
    ci_job = _test_job(ci_doc, "ci.yml")
    release_job = _test_job(release_doc, "release.yml")
    drift: list[str] = []
    drift += _compare_matrix(matrix_legs(ci_job, "ci.yml"), matrix_legs(release_job, "release.yml"))
    drift += _compare_steps(test_steps(ci_job, "ci.yml"), test_steps(release_job, "release.yml"))
    return drift


def _compare_matrix(ci: dict, release: dict) -> list[str]:
    drift = []
    for key in sorted(ci.keys() | release.keys()):
        if key not in release:
            drift.append(f"matrix leg {leg_name(key)}: in ci.yml only")
            continue
        if key not in ci:
            drift.append(f"matrix leg {leg_name(key)}: in release.yml only")
            continue
        for name in sorted(ci[key].keys() | release[key].keys()):
            if (key[0], key[1], name) in MATRIX_KEY_EXCEPTIONS:
                continue
            a, b = ci[key].get(name), release[key].get(name)
            if a == b:
                continue
            if b is None:
                drift.append(f"matrix leg {leg_name(key)}: key {name!r} ({a!r}) in ci.yml only")
            elif a is None:
                drift.append(f"matrix leg {leg_name(key)}: key {name!r} ({b!r}) in release.yml only")
            else:
                drift.append(f"matrix leg {leg_name(key)}: key {name!r} is {a!r} in ci.yml, {b!r} in release.yml")
    return drift


def _compare_env(step: str, ci_env: dict, release_env: dict, *, subset: bool = False) -> list[str]:
    drift = []
    for name in sorted(ci_env.keys() | release_env.keys()):
        if (step, name) in ENV_EXCEPTIONS:
            continue
        a, b = ci_env.get(name), release_env.get(name)
        if a == b or (subset and a is None):
            continue
        if b is None:
            drift.append(f"step {step!r}: env {name} ({a!r}) in ci.yml only")
        elif a is None:
            drift.append(f"step {step!r}: env {name} ({b!r}) in release.yml only")
        else:
            drift.append(f"step {step!r}: env {name} is {a!r} in ci.yml, {b!r} in release.yml")
    return drift


def _compare_steps(ci: dict[str, dict], release: dict[str, dict]) -> list[str]:
    drift = []
    split_into: dict[str, set[str]] = {}
    for name, (target, _reason) in SPLIT_STEPS.items():
        if name not in ci:
            drift.append(f"exception SPLIT_STEPS[{name!r}]: no such ci.yml test step; remove the exception")
            continue
        if name in release:
            drift.append(f"exception SPLIT_STEPS[{name!r}]: release.yml has this step too; remove the exception")
            continue
        if target not in release or target not in ci:
            drift.append(f"exception SPLIT_STEPS[{name!r}]: step {target!r} is missing from a workflow")
            continue
        if ci[name]["if"] != release[target]["if"]:
            drift.append(f"step {name!r}: if {ci[name]['if']!r} differs from release.yml {target!r} ({release[target]['if']!r})")
        drift += _compare_env(name, ci[name]["env"], release[target]["env"], subset=True)
        split_into.setdefault(target, set()).update(ci[name]["packages"])
        if not ci[name]["packages"]:
            drift.append(f"step {name!r}: names no --packages ./<pkg>, so its split cannot be checked")
    for (workflow, name), _reason in ONLY_IN.items():
        present = ci if workflow == "ci.yml" else release
        if name not in present:
            drift.append(f"exception ONLY_IN[{workflow}, {name!r}]: no such test step; remove the exception")
    for name in sorted(ci.keys() | release.keys()):
        if name in SPLIT_STEPS:
            continue
        if name not in release:
            if ("ci.yml", name) not in ONLY_IN:
                drift.append(f"test step {name!r}: in ci.yml only (if: {ci[name]['if']!r}, env: {sorted(ci[name]['env'])})")
            continue
        if name not in ci:
            if ("release.yml", name) not in ONLY_IN:
                drift.append(f"test step {name!r}: in release.yml only (if: {release[name]['if']!r}, env: {sorted(release[name]['env'])})")
            continue
        a, b = ci[name], release[name]
        if a["if"] != b["if"]:
            drift.append(f"step {name!r}: if is {a['if']!r} in ci.yml, {b['if']!r} in release.yml")
        drift += _compare_env(name, a["env"], b["env"])
        split = split_into.get(name, set())
        extra_ci = a["excludes"] - b["excludes"]
        if extra_ci != split:
            for pkg in sorted(extra_ci - split):
                drift.append(f"step {name!r}: ci.yml excludes {pkg} and no ci.yml step split from it runs it")
            for pkg in sorted(split - extra_ci):
                drift.append(f"step {name!r}: a split ci.yml step runs {pkg}, but this ci.yml step does not exclude it")
        for pkg in sorted(b["excludes"] - a["excludes"]):
            drift.append(f"step {name!r}: release.yml excludes {pkg}, ci.yml does not")
    return drift


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--ci", type=Path, default=DEFAULT_CI)
    parser.add_argument("--release", type=Path, default=DEFAULT_RELEASE)
    args = parser.parse_args(argv)
    try:
        ci_doc = load_yaml(args.ci.read_text(encoding="utf-8"))
        release_doc = load_yaml(args.release.read_text(encoding="utf-8"))
        drift = compare(ci_doc, release_doc)
    except (OSError, ParseError) as err:
        print(f"check-ci-release-parity: {err}", file=sys.stderr)
        return 2
    if drift:
        print(f"check-ci-release-parity: {args.release} test job drifted from {args.ci}:", file=sys.stderr)
        for line in drift:
            print(f"  - {line}", file=sys.stderr)
        print(
            "Mirror the change in both workflows, or record an intentional difference with its reason "
            "in the exception tables of scripts/check-ci-release-parity.py.",
            file=sys.stderr,
        )
        return 1
    print("check-ci-release-parity: ci.yml and release.yml test legs match")
    return 0


if __name__ == "__main__":
    sys.exit(main())
