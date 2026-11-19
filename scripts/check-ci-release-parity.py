#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Check that release.yml's `test` job runs the same test legs as ci.yml's.

The release workflow re-runs the test matrix on the tagged commit. When a
change adds a matrix key or an INVOWK_TEST_* variable to ci.yml only, the tag
run tests something else, or fails (the first v0.17.0 tag run: #173 added
`tui-plan-row` and INVOWK_TEST_TUI_PLAN_ROW to ci.yml only; #183 fixed it).

For the `test` job of both files this compares:
  - the matrix: the `include` legs, matched by `os` + `engine` (the same legs,
    keys and values), and every other matrix key (`exclude`, extra axes);
  - the parity steps, matched by step name: every step whose `run` invokes
    `gotestsum` or `go test` (each file must have one), every step whose `if`
    names a `matrix.` value (engine masks, engine setup, image pre-pull,
    OS-only steps), and every step that sets an INVOWK_TEST_* variable. Each
    pair must have the same `if`, the same INVOWK_TEST_* names and values
    (workflow, job and step `env:` merged, the step winning), and the same
    test commands: the tool, the package selection and the `go test` flags,
    without coverage-profile file names (gotestsum's own flags, such as junit
    and rerun-report files, are not compared);
  - the `go list | grep -v` package exclusions of matched steps. Any other
    `grep` in a test step is an error, so a filter the check cannot read
    fails closed.

Intentional differences live in the exception tables below, each with its
reason; an entry that matches no difference is reported as stale. The parser
reads only the block-style YAML subset these workflows use (mappings,
sequences, scalars, `|`/`>` block scalars); a flow collection (`{...}` or
`[...]`) under `env`, the strategy or the steps is a parse error. It is
stdlib only.

Usage: check-ci-release-parity.py [--ci PATH] [--release PATH]
Exit 0 on parity, 1 on drift (each difference on its own line), 2 on a file
it cannot read or parse.
"""

from __future__ import annotations

import argparse
import re
import shlex
import sys
from pathlib import Path

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parent.parent
DEFAULT_CI = ROOT / ".github" / "workflows" / "ci.yml"
DEFAULT_RELEASE = ROOT / ".github" / "workflows" / "release.yml"

TEST_ENV_PREFIX = "INVOWK_TEST_"
EXCLUDE_RE = re.compile(r"grep -v '\^github\.com/invowk/invowk/([^'$]+)\$'")
GREP_RE = re.compile(r"(?<![\w.-])grep\b[^|;&)\n]*")
# A test command anywhere in a `run` script; `gotestsum --version` and
# `go install gotest.tools/gotestsum@...` are not test runs.
TEST_MENTION_RE = re.compile(r"(?<![\w./@-])(?:gotestsum\b(?![@\w.-])(?!\s+--version\b)|go\s+test\b)")
# A test command at the start of a logical line, after optional VAR=value words.
TEST_LINE_RE = re.compile(r"^\s*(?:[A-Za-z_][A-Za-z0-9_]*=\S*\s+)*(gotestsum|go\s+test)\b(.*)$")
VAR_REF_RE = re.compile(r"^\$(?:\{(\w+)\}|(\w+))$")
# `go test` flags that only name output files.
FILE_FLAGS = ("-coverprofile", "-test.coverprofile")

# Parity steps that exist in one file only, as (workflow, step name) -> reason.
# A step listed in SPLIT_STEPS below needs no entry here.
ONLY_IN: dict[tuple[str, str], str] = {
    ("ci.yml", "Run benchmark smoke tests"): (
        "keeps the benchmarks compiling and running in regular CI on one leg; "
        "release.yml's tag run measures nothing, and its `release` job only "
        "packages the benchmark image that Bencher runs on bare metal."
    ),
    ("ci.yml", "Merge coverage profiles"): (
        "ci.yml merges and uploads coverage profiles; release.yml collects no "
        "coverage."
    ),
    ("release.yml", "Run script tests (worktree-safety gate)"): (
        "ci.yml leaves `make test-scripts` to lint.yml's `script tests "
        "(worktree-safety gate)` job (ci-workflows rule), which runs on PRs and "
        "main pushes but not on tags, so the tag run needs its own step."
    ),
}

# A ci.yml step split out of a release.yml step: ci step name ->
#   release: the release.yml step it was split from;
#   omits: the INVOWK_TEST_* variables of that release step the split step
#          does not set (every other variable compares exactly);
#   reason: why.
# The split step must run under the same `if`, run the same `go test` flags,
# and run exactly the packages the ci counterpart of the release step excludes
# in addition to release.yml's own exclusions.
SPLIT_STEPS: dict[str, dict] = {
    "Run internal/runtime tests (full)": {
        "release": "Run tests (full, non-CLI)",
        "omits": frozenset({"INVOWK_TEST_TUI_PLAN_ROW"}),
        "reason": (
            "ci.yml runs internal/runtime in its own step (its own junit, rerun "
            "report and coverage profile, merged later for upload); release.yml "
            "keeps it in the non-CLI step and uploads no coverage. Only "
            "internal/app/commandadapters reads INVOWK_TEST_TUI_PLAN_ROW, so the "
            "runtime step does not set it."
        ),
    },
}

# Matrix leg keys allowed to differ, as (os, engine, key) -> reason.
MATRIX_KEY_EXCEPTIONS: dict[tuple[str, str, str], str] = {}

# INVOWK_TEST_* variables allowed to differ, as (step name, variable) -> reason.
ENV_EXCEPTIONS: dict[tuple[str, str], str] = {}


class ParseError(Exception):
    """The file uses YAML this minimal parser does not read."""


# ------------------------------------------------------------------ parsing


class _Flow(str):
    """An unquoted `{...}` or `[...]` value, kept as text by the parser."""


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
    if value[:1] in ("[", "{"):
        return _Flow(value)
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


def _reject_flow(value: object, where: str) -> None:
    """Fail closed on a flow collection the parser kept as opaque text."""
    if isinstance(value, _Flow):
        raise ParseError(f"{where}: flow collection {str(value)!r}; write it in block style")
    if isinstance(value, dict):
        for key, child in value.items():
            _reject_flow(child, f"{where}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            _reject_flow(child, f"{where}[{index}]")


def _mapping(value: object, where: str) -> dict:
    """A mapping-valued key; absent or empty is {}, anything else an error."""
    if value is None or value == "":
        return {}
    if not isinstance(value, dict):
        raise ParseError(f"{where} is not a mapping: {value!r}")
    return value


def _test_env(env: dict) -> dict[str, str]:
    return {str(k): str(v) for k, v in env.items() if str(k).startswith(TEST_ENV_PREFIX)}


def _test_job(doc: object, label: str) -> tuple[dict, dict[str, str]]:
    """The test job and the INVOWK_TEST_* variables its workflow and job set."""
    jobs = doc.get("jobs") if isinstance(doc, dict) else None
    job = jobs.get("test") if isinstance(jobs, dict) else None
    if not isinstance(job, dict):
        raise ParseError(f"{label}: no jobs.test mapping")
    _reject_flow(doc.get("env"), f"{label}: env")
    for key in ("env", "strategy", "steps"):
        _reject_flow(job.get(key), f"{label}: jobs.test.{key}")
    inherited = _test_env(_mapping(doc.get("env"), f"{label}: env"))
    inherited.update(_test_env(_mapping(job.get("env"), f"{label}: jobs.test.env")))
    return job, inherited


def _canonical(value: object) -> object:
    if isinstance(value, dict):
        return tuple(sorted((str(k), _canonical(v)) for k, v in value.items()))
    if isinstance(value, list):
        return tuple(sorted((_canonical(v) for v in value), key=repr))
    return str(value)


def matrix_legs(job: dict, label: str) -> tuple[dict[tuple[str, str], dict[str, str]], dict[str, object]]:
    """The `include` legs by (os, engine), and the other matrix keys."""
    strategy = _mapping(job.get("strategy"), f"{label}: jobs.test.strategy")
    matrix = _mapping(strategy.get("matrix"), f"{label}: jobs.test.strategy.matrix")
    include = matrix.get("include")
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
    others = {str(k): _canonical(v) for k, v in matrix.items() if k != "include"}
    return legs, others


def _logical_lines(run: str) -> list[str]:
    return run.replace("\\\n", " ").splitlines()


def _resolve_packages(token: str, run: str, where: str) -> str:
    """`$PACKAGES` -> `go list <args>` from its assignment in the same script."""
    ref = VAR_REF_RE.match(token)
    if not ref:
        return token
    var = ref.group(1) or ref.group(2)
    assign = re.search(rf"(?m)^\s*{var}=\"?\$\(\s*go list ([^|)]*)", run)
    if not assign:
        raise ParseError(f"{where}: cannot resolve --packages {token}: no `{var}=\"$(go list ...)\"` in the step")
    return "go list " + " ".join(assign.group(1).split())


def _go_flags(args: list[str]) -> tuple[str, ...]:
    flags: list[str] = []
    skip = False
    for arg in args:
        if skip:
            skip = False
            continue
        name = arg.split("=", 1)[0]
        if name in FILE_FLAGS:
            skip = "=" not in arg
            continue
        flags.append(arg)
    return tuple(flags)


def test_commands(run: str, where: str) -> list[tuple[str, str, tuple[str, ...]]]:
    """Each test command of a `run` script as (tool, packages, go test flags)."""
    commands = []
    for line in _logical_lines(run):
        match = TEST_LINE_RE.match(line)
        if not match:
            continue
        tool = "gotestsum" if match.group(1) == "gotestsum" else "go test"
        try:
            words = shlex.split(match.group(2))
        except ValueError as err:
            raise ParseError(f"{where}: cannot split the {tool} command: {err}") from err
        if tool == "gotestsum":
            if "--version" in words:
                continue
            own, go_args = (words[: words.index("--")], words[words.index("--") + 1 :]) if "--" in words else (words, [])
            packages = ""
            for i, word in enumerate(own):
                if word == "--packages" and i + 1 < len(own):
                    packages = own[i + 1]
                elif word.startswith("--packages="):
                    packages = word.split("=", 1)[1]
            packages = _resolve_packages(packages, run, where) if packages else ""
            commands.append((tool, packages, _go_flags(go_args)))
        else:
            positional = [w for w in words if not w.startswith("-")]
            flagged = [w for w in words if w.startswith("-")]
            commands.append((tool, " ".join(positional), _go_flags(flagged)))
    mentions = len(TEST_MENTION_RE.findall(run))
    if mentions != len(commands):
        raise ParseError(f"{where}: {mentions} test command(s) in `run`, {len(commands)} readable at the start of a line")
    return commands


def _excludes(run: str, where: str) -> set[str]:
    excludes = set()
    for grep in GREP_RE.finditer(run):
        text = grep.group(0).rstrip()
        match = EXCLUDE_RE.fullmatch(text)
        if not match:
            raise ParseError(
                f"{where}: cannot read {text!r}; a test step filters packages only with "
                "`grep -v '^github.com/invowk/invowk/<pkg>$'`"
            )
        excludes.add(match.group(1))
    return excludes


def test_steps(job: dict, label: str, inherited: dict[str, str]) -> dict[str, dict]:
    steps = job.get("steps")
    if not isinstance(steps, list):
        raise ParseError(f"{label}: jobs.test.steps is not a list")
    result: dict[str, dict] = {}
    for step in steps:
        if not isinstance(step, dict):
            continue
        name = str(step.get("name", ""))
        where = f"{label}: step {name or '(unnamed)'!r}"
        own_env = _test_env(_mapping(step.get("env"), f"{where} env"))
        condition = str(step.get("if", ""))
        run = str(step.get("run", ""))
        invokes = bool(TEST_MENTION_RE.search(run))
        if not invokes and "matrix." not in condition and not own_env:
            continue
        if not name:
            raise ParseError(f"{label}: an unnamed parity step (if: {condition!r})")
        if name in result:
            raise ParseError(f"{label}: two parity steps named {name!r}")
        result[name] = {
            "if": condition,
            "env": {**inherited, **own_env},
            "commands": test_commands(run, where),
            "excludes": _excludes(run, where) if invokes else set(),
        }
    if not any(step["commands"] for step in result.values()):
        raise ParseError(f"{label}: jobs.test has no step that runs gotestsum or go test")
    return result


def leg_name(key: tuple[str, str]) -> str:
    return key[0] + (f"/{key[1]}" if key[1] else "")


def _format_commands(commands: list) -> str:
    return "; ".join(f"{tool} [{packages}] {' '.join(flags)}".rstrip() for tool, packages, flags in commands) or "(none)"


# ------------------------------------------------------------------ comparing


def compare(ci_doc: object, release_doc: object) -> list[str]:
    ci_job, ci_inherited = _test_job(ci_doc, "ci.yml")
    release_job, release_inherited = _test_job(release_doc, "release.yml")
    drift: list[str] = []
    drift += _compare_matrix(matrix_legs(ci_job, "ci.yml"), matrix_legs(release_job, "release.yml"))
    drift += _compare_steps(
        test_steps(ci_job, "ci.yml", ci_inherited),
        test_steps(release_job, "release.yml", release_inherited),
    )
    return drift


def _compare_matrix(ci_matrix: tuple, release_matrix: tuple) -> list[str]:
    (ci, ci_other), (release, release_other) = ci_matrix, release_matrix
    drift = []
    used: set[tuple[str, str, str]] = set()
    for name in sorted(ci_other.keys() | release_other.keys()):
        if ci_other.get(name) != release_other.get(name):
            drift.append(f"matrix key {name!r} differs: {ci_other.get(name)!r} in ci.yml, {release_other.get(name)!r} in release.yml")
    for key in sorted(ci.keys() | release.keys()):
        if key not in release:
            drift.append(f"matrix leg {leg_name(key)}: in ci.yml only")
            continue
        if key not in ci:
            drift.append(f"matrix leg {leg_name(key)}: in release.yml only")
            continue
        for name in sorted(ci[key].keys() | release[key].keys()):
            a, b = ci[key].get(name), release[key].get(name)
            if a == b:
                continue
            if (key[0], key[1], name) in MATRIX_KEY_EXCEPTIONS:
                used.add((key[0], key[1], name))
                continue
            if b is None:
                drift.append(f"matrix leg {leg_name(key)}: key {name!r} ({a!r}) in ci.yml only")
            elif a is None:
                drift.append(f"matrix leg {leg_name(key)}: key {name!r} ({b!r}) in release.yml only")
            else:
                drift.append(f"matrix leg {leg_name(key)}: key {name!r} is {a!r} in ci.yml, {b!r} in release.yml")
    for entry in sorted(MATRIX_KEY_EXCEPTIONS.keys() - used):
        drift.append(f"exception MATRIX_KEY_EXCEPTIONS[{entry!r}]: matches no difference; remove the exception")
    return drift


def _compare_env(
    step: str, ci_env: dict, release_env: dict, used: set, *, omits: frozenset = frozenset(), origin_env: dict | None = None
) -> list[str]:
    drift = []
    for name in sorted(ci_env.keys() | release_env.keys()):
        a, b = ci_env.get(name), release_env.get(name)
        if name in omits:
            if a is not None:
                drift.append(f"step {step!r}: sets {name}, which its SPLIT_STEPS entry omits; remove it from `omits`")
            continue
        if a == b:
            continue
        if (step, name) in ENV_EXCEPTIONS:
            used.add((step, name))
            continue
        if b is None:
            drift.append(f"step {step!r}: env {name} ({a!r}) in ci.yml only")
        elif a is None:
            drift.append(f"step {step!r}: env {name} ({b!r}) in release.yml only")
        else:
            drift.append(f"step {step!r}: env {name} is {a!r} in ci.yml, {b!r} in release.yml")
    for name in sorted(omits - release_env.keys() - (origin_env or {}).keys()):
        drift.append(f"step {step!r}: SPLIT_STEPS omits {name}, which neither workflow's origin step sets; remove it from `omits`")
    return drift


def _split_packages(name: str, commands: list) -> tuple[set[str], list[str]]:
    packages, drift = set(), []
    for _tool, selection, _flags in commands:
        for pkg in selection.split():
            if pkg.startswith("./") and "..." not in pkg:
                packages.add(pkg[2:])
            else:
                drift.append(f"step {name!r}: runs {pkg!r}, not one ./<pkg>, so its split cannot be checked")
    if not packages and not drift:
        drift.append(f"step {name!r}: names no --packages ./<pkg>, so its split cannot be checked")
    return packages, drift


def _compare_steps(ci: dict[str, dict], release: dict[str, dict]) -> list[str]:
    drift = []
    used_env: set[tuple[str, str]] = set()
    split_into: dict[str, set[str]] = {}
    for name, split in SPLIT_STEPS.items():
        target = split["release"]
        if name not in ci:
            drift.append(f"exception SPLIT_STEPS[{name!r}]: no such ci.yml parity step; remove the exception")
            continue
        if name in release:
            drift.append(f"exception SPLIT_STEPS[{name!r}]: release.yml has this step too; remove the exception")
            continue
        if target not in release or target not in ci:
            drift.append(f"exception SPLIT_STEPS[{name!r}]: step {target!r} is missing from a workflow")
            continue
        if ci[name]["if"] != release[target]["if"]:
            drift.append(f"step {name!r}: if {ci[name]['if']!r} differs from release.yml {target!r} ({release[target]['if']!r})")
        drift += _compare_env(name, ci[name]["env"], release[target]["env"], used_env, omits=split["omits"], origin_env=ci[target]["env"])
        ci_flags = [(tool, flags) for tool, _pkgs, flags in ci[name]["commands"]]
        release_flags = [(tool, flags) for tool, _pkgs, flags in release[target]["commands"]]
        if ci_flags != release_flags:
            drift.append(f"step {name!r}: test flags {ci_flags!r} differ from release.yml {target!r} ({release_flags!r})")
        packages, package_drift = _split_packages(name, ci[name]["commands"])
        drift += package_drift
        split_into.setdefault(target, set()).update(packages)
    for (workflow, name), _reason in ONLY_IN.items():
        present, other = (ci, release) if workflow == "ci.yml" else (release, ci)
        if name not in present:
            drift.append(f"exception ONLY_IN[{workflow}, {name!r}]: no such parity step; remove the exception")
        elif name in other:
            drift.append(f"exception ONLY_IN[{workflow}, {name!r}]: both workflows have this step; remove the exception")
    for name in sorted(ci.keys() | release.keys()):
        if name in SPLIT_STEPS:
            continue
        if name not in release:
            if ("ci.yml", name) not in ONLY_IN:
                drift.append(f"parity step {name!r}: in ci.yml only (if: {ci[name]['if']!r}, env: {sorted(ci[name]['env'])})")
            continue
        if name not in ci:
            if ("release.yml", name) not in ONLY_IN:
                drift.append(f"parity step {name!r}: in release.yml only (if: {release[name]['if']!r}, env: {sorted(release[name]['env'])})")
            continue
        a, b = ci[name], release[name]
        if a["if"] != b["if"]:
            drift.append(f"step {name!r}: if is {a['if']!r} in ci.yml, {b['if']!r} in release.yml")
        drift += _compare_env(name, a["env"], b["env"], used_env)
        if a["commands"] != b["commands"]:
            drift.append(
                f"step {name!r}: test command is `{_format_commands(a['commands'])}` in ci.yml, "
                f"`{_format_commands(b['commands'])}` in release.yml"
            )
        split = split_into.get(name, set())
        extra_ci = a["excludes"] - b["excludes"]
        for pkg in sorted(extra_ci - split):
            drift.append(f"step {name!r}: ci.yml excludes {pkg} and no ci.yml step split from it runs it")
        for pkg in sorted(split - extra_ci):
            if pkg in b["excludes"]:
                drift.append(f"step {name!r}: release.yml excludes {pkg}, which a split ci.yml step runs, so release.yml runs it nowhere")
            else:
                drift.append(f"step {name!r}: a split ci.yml step runs {pkg}, but this ci.yml step does not exclude it")
        for pkg in sorted(b["excludes"] - a["excludes"]):
            drift.append(f"step {name!r}: release.yml excludes {pkg}, ci.yml does not")
    for entry in sorted(ENV_EXCEPTIONS.keys() - used_env):
        drift.append(f"exception ENV_EXCEPTIONS[{entry!r}]: matches no difference; remove the exception")
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
