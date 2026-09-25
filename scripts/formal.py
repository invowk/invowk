#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Fail-closed runner for Invowk's formal models (Alloy 6 and TLA+/TLC).

Every model command declares an expected verdict in formal/manifest.toml, and
the runner fails unless the checker produces exactly that verdict. A checker
that exits without a recognisable verdict is a failure (NO-VERDICT), never a
pass. Safety properties must have rejecting mutants, Alloy checks must have
satisfiable antecedents, TLC actions must be covered, and liveness
configurations must not use SYMMETRY or VIEW.

Standard library only (Python 3.11+ for tomllib).

Usage:
    scripts/formal.py fetch
    scripts/formal.py alloy [MODEL ...]
    scripts/formal.py tla [MODEL ...]
    scripts/formal.py golden [--check] [MODEL ...]
    scripts/formal.py traces [TRACE ...]
    scripts/formal.py correspondence
    scripts/formal.py all
    scripts/formal.py snapshot [--out FILE] [--compare FILE]
    scripts/formal.py affected --event EVENT [--base SHA] [--head SHA]
    scripts/formal.py replay-plan [--format text|github]
"""

from __future__ import annotations

import argparse
import concurrent.futures
import contextlib
import dataclasses
import fnmatch
import gzip
import hashlib
import io
import json
import os
import re
import shutil
import subprocess
import sys
import threading
import time
import tomllib
import urllib.request
import xml.etree.ElementTree as ET  # noqa: S405 - parses local Alloy output only
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
MANIFEST = REPO_ROOT / "formal" / "manifest.toml"
TOOL_DIR = REPO_ROOT / "bin" / "formal"
REPORT_DIR = REPO_ROOT / "artifacts" / "formal"

VERDICT_PASS = "pass"
VERDICT_COUNTEREXAMPLE = "counterexample"
VERDICT_INSTANCE = "instance"
VERDICT_NONE = "NO-VERDICT"
EXPECTED_VERDICTS = {VERDICT_PASS, VERDICT_COUNTEREXAMPLE, VERDICT_INSTANCE}
# `ci`: measured on CI runners; `local`: provisional, measured on a workstation.
BUDGET_SOURCES = {"ci", "local"}


class FormalError(Exception):
    """A fail-closed violation: the run must exit non-zero."""


@dataclasses.dataclass(frozen=True)
class Tool:
    name: str
    version: str
    jar: str
    url: str
    sha256: str

    @property
    def path(self) -> Path:
        return TOOL_DIR / self.jar


@dataclasses.dataclass(frozen=True)
class Command:
    name: str
    expect: str
    property: str = ""
    mutant_of: str = ""
    antecedent: str = ""
    witness: bool = False
    finding: str = ""
    fairness_twin_of: str = ""
    dead_actions: tuple[str, ...] = ()
    distinct_states: int = 0
    # TLC only: overrides of the model's base constants, the specification
    # operator, and whether `property` is temporal (PROPERTY, not INVARIANT).
    constants: tuple[tuple[str, str], ...] = ()
    spec: str = "Spec"
    temporal: bool = False
    # Alloy only: the command's formula and an optional scope overriding the
    # model's default; the runner renders the command from them.
    body: str = ""
    scope: str = ""


@dataclasses.dataclass(frozen=True)
class Model:
    name: str
    tool: str
    file: str
    commands: tuple[Command, ...]
    calibration: str = ""
    golden: dict[str, str | int] = dataclasses.field(default_factory=dict)
    constants: tuple[tuple[str, str], ...] = ()
    # Alloy only: the default scope of every command.
    scope: str = ""
    # Soft CI budget for the model's summed checker time (see report_timings).
    budget_seconds: int = 0
    budget_source: str = ""


@dataclasses.dataclass(frozen=True)
class TraceSuite:
    """A trace-validation suite for TLA+ model `name`: the Go test
    Test<name>_TraceHarness in `package` writes <name>Traces.tla, and
    formal/tla/<name>Trace.tla must accept every recorded trace and reject
    every targeted mutation. Constants are the model's base constants with
    `constants` overriding some of them."""

    name: str
    package: str
    constants: tuple[tuple[str, str], ...] = ()
    budget_seconds: int = 0
    budget_source: str = ""

    @property
    def label(self) -> str:
        """The suite's name in timing lines; distinct from its model's."""
        return f"trace/{self.name}"

    @property
    def spec(self) -> str:
        return f"{self.name}Trace.tla"

    @property
    def module(self) -> str:
        return f"{self.name}Traces"

    @property
    def test(self) -> str:
        return f"Test{self.name}_TraceHarness"


# ---------------------------------------------------------------------------
# Manifest


# Keys each manifest table may declare. Anything else is a typo that would
# otherwise fall back silently to a default (for example `scop =` on a golden
# command), so the loader rejects it.
TOP_KEYS = {"tools", "model", "trace", "ci"}
TOOL_KEYS = {"version", "jar", "url", "sha256"}
BUDGET_KEYS = {"budget_seconds", "budget_source"}
MODEL_KEYS = {"name", "tool", "file", "calibration", "golden", "command", "constants", "scope", *BUDGET_KEYS}
GOLDEN_KEYS = {"command", "output", "max_bytes"}
COMMAND_KEYS = {
    "name", "expect", "property", "mutant_of", "antecedent", "witness", "finding", "fairness_twin_of",
    "dead_actions", "distinct_states", "constants", "spec", "temporal", "body", "scope",
}
TRACE_KEYS = {"name", "package", "constants", *BUDGET_KEYS}
CI_KEYS = {"paths", "budget"}
CI_BUDGET_KEYS = {"soft_headroom", "local_headroom", "hard_headroom", "measured_on", "ci_runs", "command_count"}
# Keys that belong to one tool only.
TOOL_ONLY_KEYS = {"alloy": {"body", "scope"}, "tla": {"constants", "spec", "temporal"}}


def check_keys(table: str, raw: dict, allowed: set[str]) -> None:
    unknown = sorted(set(raw) - allowed)
    if unknown:
        raise FormalError(f"manifest {table}: unknown key(s) {', '.join(unknown)}")


def check_tool_keys(table: str, raw: dict, tool: str) -> None:
    for other, keys in TOOL_ONLY_KEYS.items():
        misplaced = sorted(set(raw) & keys)
        if other != tool and misplaced:
            raise FormalError(f"manifest {table}: key(s) {', '.join(misplaced)} are {other}-only, but the model's tool is {tool}")


def load_manifest(path: Path = MANIFEST) -> tuple[dict[str, Tool], list[Model]]:
    with path.open("rb") as handle:
        data = tomllib.load(handle)
    check_keys("top level", data, TOP_KEYS)
    check_keys("[ci]", data.get("ci", {}), CI_KEYS)
    check_keys("[ci.budget]", data.get("ci", {}).get("budget", {}), CI_BUDGET_KEYS)
    for key, raw in data.get("tools", {}).items():
        check_keys(f"[tools.{key}]", raw, TOOL_KEYS)
    for raw in data.get("trace", []):
        check_keys(f"[[trace]] {raw.get('name', '?')}", raw, TRACE_KEYS)
    tools = {
        key: Tool(name=key, version=v["version"], jar=v["jar"], url=v["url"], sha256=v["sha256"])
        for key, v in data.get("tools", {}).items()
    }
    models = []
    for raw in data.get("model", []):
        where = f"[[model]] {raw.get('name', '?')}"
        tool = raw.get("tool", "")
        if tool not in TOOL_ONLY_KEYS:
            raise FormalError(f"manifest {where}: unknown tool {tool!r}")
        check_tool_keys(where, raw, tool)
        check_keys(where, raw, MODEL_KEYS)
        check_keys(f"{where} [model.golden]", raw.get("golden", {}), GOLDEN_KEYS)
        for c in raw.get("command", []):
            cmd_where = f"{where} [[model.command]] {c.get('name', '?')}"
            check_tool_keys(cmd_where, c, tool)
            check_keys(cmd_where, c, COMMAND_KEYS)
            if tool == "alloy" and not c.get("body", "").strip():
                raise FormalError(f"manifest {cmd_where}: an Alloy command needs a non-empty body")
        commands = tuple(
            Command(
                name=c["name"],
                expect=c["expect"],
                property=c.get("property", ""),
                mutant_of=c.get("mutant_of", ""),
                antecedent=c.get("antecedent", ""),
                witness=c.get("witness", False),
                finding=c.get("finding", ""),
                fairness_twin_of=c.get("fairness_twin_of", ""),
                constants=tuple(c.get("constants", {}).items()),
                spec=c.get("spec", "Spec"),
                temporal=c.get("temporal", False),
                dead_actions=tuple(c.get("dead_actions", [])),
                distinct_states=c.get("distinct_states", 0),
                body=c.get("body", "").strip(),
                scope=c.get("scope", ""),
            )
            for c in raw.get("command", [])
        )
        models.append(
            Model(
                name=raw["name"],
                tool=raw["tool"],
                file=raw["file"],
                commands=commands,
                calibration=raw.get("calibration", ""),
                golden=raw.get("golden", {}),
                constants=tuple(raw.get("constants", {}).items()),
                scope=raw.get("scope", ""),
                budget_seconds=raw.get("budget_seconds", 0),
                budget_source=raw.get("budget_source", ""),
            )
        )
    return tools, models


def load_trace_suites(path: Path = MANIFEST) -> list[TraceSuite]:
    with path.open("rb") as handle:
        data = tomllib.load(handle)
    return [
        TraceSuite(
            name=raw["name"],
            package=raw["package"],
            constants=tuple(raw.get("constants", {}).items()),
            budget_seconds=raw.get("budget_seconds", 0),
            budget_source=raw.get("budget_source", ""),
        )
        for raw in data.get("trace", [])
    ]


def check_budget(where: str, seconds: object, source: str) -> None:
    if not isinstance(seconds, int) or isinstance(seconds, bool) or seconds <= 0:
        raise FormalError(f"{where}: budget_seconds must be a positive integer, got {seconds!r}")
    if source not in BUDGET_SOURCES:
        raise FormalError(f"{where}: budget_source must be one of {sorted(BUDGET_SOURCES)}, got {source!r}")


def validate_manifest(models: list[Model], suites: list[TraceSuite] = ()) -> None:
    """Static guards that need no checker run."""
    for suite in suites:
        check_budget(f"[[trace]] {suite.name}", suite.budget_seconds, suite.budget_source)
    for model in models:
        check_budget(f"[[model]] {model.name}", model.budget_seconds, model.budget_source)
        by_name = {c.name: c for c in model.commands}
        if len(by_name) != len(model.commands):
            raise FormalError(f"{model.name}: duplicate command names")
        for cmd in model.commands:
            if cmd.expect not in EXPECTED_VERDICTS:
                raise FormalError(f"{model.name}.{cmd.name}: unknown expected verdict {cmd.expect!r}")
            if cmd.mutant_of:
                target = by_name.get(cmd.mutant_of)
                if target is None:
                    raise FormalError(f"{model.name}.{cmd.name}: mutant_of names unknown command {cmd.mutant_of!r}")
                if cmd.expect != VERDICT_COUNTEREXAMPLE:
                    raise FormalError(f"{model.name}.{cmd.name}: a mutant must expect a counterexample")
                if cmd.property != target.property:
                    raise FormalError(
                        f"{model.name}.{cmd.name}: mutant checks property {cmd.property!r}, "
                        f"but {cmd.mutant_of} checks {target.property!r}"
                    )
        for cmd in model.commands:
            is_safety = cmd.expect == VERDICT_PASS and cmd.property and not cmd.mutant_of
            if not is_safety:
                continue
            # A finding records today's code; it disappears when the fix lands,
            # so it never counts as the property's vacuity guard.
            if not any(m.mutant_of == cmd.name and not m.finding for m in model.commands):
                raise FormalError(
                    f"{model.name}.{cmd.name}: unguarded property {cmd.property!r} has no rejecting mutant "
                    "(finding records do not count)"
                )
            if model.tool == "alloy":
                ante = by_name.get(cmd.antecedent)
                if ante is None or ante.expect != VERDICT_INSTANCE:
                    raise FormalError(
                        f"{model.name}.{cmd.name}: Alloy check needs an antecedent run command expecting an instance"
                    )
        for cmd in model.commands:
            if cmd.finding and cmd.expect != VERDICT_COUNTEREXAMPLE:
                raise FormalError(f"{model.name}.{cmd.name}: a finding record must expect a counterexample")
            if model.tool == "tla" and not cmd.property:
                raise FormalError(f"{model.name}.{cmd.name}: a TLC command must name the property it checks")
            # -workers 1 makes the count deterministic, so it is recorded locally.
            if model.tool == "tla" and cmd.expect == VERDICT_PASS and not cmd.distinct_states:
                raise FormalError(f"{model.name}.{cmd.name}: a passing TLC command must record distinct_states")
            if cmd.temporal and cmd.expect == VERDICT_PASS and not cmd.fairness_twin_of and not any(
                c.fairness_twin_of == cmd.name and c.expect == VERDICT_COUNTEREXAMPLE for c in model.commands
            ):
                raise FormalError(
                    f"{model.name}.{cmd.name}: liveness property needs a fairness-free twin "
                    "(fairness_twin_of) that expects a counterexample"
                )
            # Alloy witnesses are satisfiable runs; TLC witnesses are invariants ~W
            # that must be violated, naming W's situation as reachable.
            witness_verdict = VERDICT_INSTANCE if model.tool == "alloy" else VERDICT_COUNTEREXAMPLE
            if cmd.witness and cmd.expect != witness_verdict:
                raise FormalError(f"{model.name}.{cmd.name}: a {model.tool} witness must expect {witness_verdict}")
            if cmd.witness and model.tool == "tla" and not cmd.property:
                raise FormalError(f"{model.name}.{cmd.name}: a TLC witness must name its invariant as property")
        if model.tool == "alloy" and not any(c.expect == VERDICT_INSTANCE for c in model.commands):
            raise FormalError(f"{model.name}: Alloy model has no satisfiable (non-vacuity) run command")
        if model.tool == "alloy":
            render_alloy_commands(model)  # scope and property-reference guards
        if not model.calibration:
            # stderr keeps stdout data-only for `affected` and `replay-plan` ($GITHUB_OUTPUT).
            print(f"WARNING: {model.name} has no calibration record; its properties are not claimed as verified", file=sys.stderr)


# ---------------------------------------------------------------------------
# Tools


def sha256_of(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verify_tool(tool: Tool) -> Path:
    """Verify a pinned tool jar's checksum; a mismatch removes the jar."""
    path = tool.path
    if not path.exists():
        raise FormalError(f"{tool.name} {tool.version} is not installed at {path}; run: scripts/formal.py fetch")
    actual = sha256_of(path)
    if actual != tool.sha256:
        path.unlink(missing_ok=True)
        raise FormalError(f"{tool.name} {tool.version}: checksum mismatch (expected {tool.sha256}, got {actual}); removed {path}")
    print(f"tool {tool.name} {tool.version} sha256:{actual}")
    return path


def ensure_tool(tool: Tool) -> Path:
    """Fetch a pinned tool jar if absent, then verify it before every use."""
    if not tool.path.exists():
        TOOL_DIR.mkdir(parents=True, exist_ok=True)
        partial = tool.path.with_suffix(tool.path.suffix + ".part")
        with urllib.request.urlopen(tool.url, timeout=120) as response, partial.open("wb") as out:  # noqa: S310
            shutil.copyfileobj(response, out)
        partial.replace(tool.path)
    return verify_tool(tool)


# ---------------------------------------------------------------------------
# Timing and soft budgets
#
# A model's time is the sum of its checker runs: one Alloy JVM plus its golden
# re-enumeration, or every TLC command's wall time. A trace suite's time is its
# harness run plus every trace check. TLC commands and traces run in parallel,
# so sums are stable where wall-clock spans would interfere with each other.
# An overrun warns and never changes the exit status.


# TLC commands and trace checks add their times from pool threads.
TIMINGS_LOCK = threading.Lock()


def add_time(timings: dict[str, float] | None, label: str, seconds: float) -> None:
    if timings is not None:
        with TIMINGS_LOCK:
            timings[label] = timings.get(label, 0.0) + seconds


@contextlib.contextmanager
def timed(timings: dict[str, float] | None, label: str):
    """Add the wall time of the `with` body to `label`, even when it raises."""
    started = time.monotonic()
    try:
        yield
    finally:
        add_time(timings, label, time.monotonic() - started)


def report_timings(timings: dict[str, float], budgets: dict[str, tuple[int, str]]) -> list[str]:
    """Print one timing line per entry, warn on soft-budget overruns, and
    append a table to $GITHUB_STEP_SUMMARY when it is set. Returns the
    overrun warnings."""
    actions = os.environ.get("GITHUB_ACTIONS") == "true"
    warnings = []
    rows = []
    for label, seconds in timings.items():
        budget, source = budgets[label]
        print(f"timing {label} {seconds:.1f}s budget {budget}s ({source})")
        over = seconds > budget
        if over:
            warning = f"{label} took {seconds:.1f}s, over its {budget}s soft budget ({source})"
            warnings.append(warning)
            print(f"WARNING: {warning}")
            if actions:
                print(f"::warning title=Formal soft budget::{warning}")
        if actions and source == "local":
            print(f"::notice title=Provisional formal budget::{label}: budget {budget}s is provisional (budget_source = local)")
        rows.append(f"| `{label}` | {seconds:.1f} | {budget} | {source} | {'over budget' if over else 'ok'} |")
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary and rows:
        with open(summary, "a", encoding="utf-8") as handle:
            handle.write("| Entry | Seconds | Budget (s) | Source | Status |\n|---|---|---|---|---|\n")
            handle.write("\n".join(rows) + "\n\n")
    return warnings


def manifest_budgets(models: list[Model], suites: list[TraceSuite]) -> dict[str, tuple[int, str]]:
    return {m.name: (m.budget_seconds, m.budget_source) for m in models} | {
        s.label: (s.budget_seconds, s.budget_source) for s in suites
    }


# ---------------------------------------------------------------------------
# Alloy

def alloy_expected_bit(cmd: Command) -> int:
    """Alloy `expect 1` means an instance/counterexample exists."""
    return 0 if cmd.expect == VERDICT_PASS else 1


def render_alloy_command(model: Model, cmd: Command) -> str:
    """Render one manifest command as a labelled Alloy command. An instance
    command is a `run`, every other command a `check`, and the expect bit
    follows from the manifest verdict."""
    scope = cmd.scope or model.scope
    if not scope:
        raise FormalError(f"{model.name}.{cmd.name}: no scope, and the model declares no default scope")
    kind = "run" if cmd.expect == VERDICT_INSTANCE else "check"
    if kind == "check" and cmd.property and not re.search(rf"\b{re.escape(cmd.property)}\b", cmd.body):
        raise FormalError(f"{model.name}.{cmd.name}: check body does not reference property {cmd.property!r}")
    return f"{cmd.name}: {kind} {{\n{cmd.body}\n}} for {scope} expect {alloy_expected_bit(cmd)}"


def render_alloy_commands(model: Model) -> str:
    return "\n\n".join(render_alloy_command(model, cmd) for cmd in model.commands) + "\n"


def blank_matches(pattern: str, source: str) -> str:
    """Replace every match of `pattern` with spaces, keeping line numbers."""
    return re.sub(pattern, lambda m: re.sub(r"[^\n]", " ", m[0]), source, flags=re.S)


def strip_alloy_comments(source: str) -> str:
    """Blank out `//`, `--`, and `/* ... */` comments."""
    return blank_matches(r"/\*.*?\*/|//[^\n]*|--[^\n]*", source)


def reject_alloy_source_commands(model: Model, source: str) -> None:
    """Commands belong in the manifest. `run` and `check` are Alloy keywords,
    so any occurrence outside a comment declares a command, labelled or not."""
    match = re.search(r"\b(run|check)\b", strip_alloy_comments(source))
    if match:
        line = source.count("\n", 0, match.start()) + 1
        raise FormalError(
            f"{model.name}: {model.file}:{line} declares a `{match[1]}` command; commands belong in formal/manifest.toml"
        )


def stage_alloy_model(model: Model) -> Path:
    """Write artifacts/formal/<Model>/<Model>.als: the committed source, a
    generated banner, and the commands rendered from the manifest."""
    source = (REPO_ROOT / model.file).read_text()
    reject_alloy_source_commands(model, source)
    staged = REPORT_DIR / model.name / f"{model.name}.als"
    staged.parent.mkdir(parents=True, exist_ok=True)
    banner = "\n// ---- Commands generated by scripts/formal.py from formal/manifest.toml; do not edit. ----\n\n"
    staged.write_text(source.rstrip("\n") + "\n" + banner + render_alloy_commands(model))
    return staged


def alloy_verdicts(receipt: dict) -> dict[str, str]:
    """Derive a verdict per command from an Alloy exec receipt.json."""
    verdicts: dict[str, str] = {}
    for name, entry in receipt.get("commands", {}).items():
        kind = entry.get("type")
        solutions = entry.get("solution") or []
        found = any(sol.get("instances") for sol in solutions)
        if kind == "run":
            verdicts[name] = VERDICT_INSTANCE if found else VERDICT_PASS
        elif kind == "check":
            verdicts[name] = VERDICT_COUNTEREXAMPLE if found else VERDICT_PASS
    return verdicts


def alloy_exec(jar: Path, staged: Path, out_dir: Path, command: str = "*", fmt: str = "json", repeat: int = 1) -> None:
    """Run `alloy exec` on a staged model for one command (or all with "*") into a fresh out_dir."""
    if out_dir.exists():
        shutil.rmtree(out_dir)
    out_dir.mkdir(parents=True)
    args = [
        "java", "--enable-native-access=ALL-UNNAMED", "-jar", str(jar), "exec",
        "-f", "-q", "-t", fmt, "-o", str(out_dir), "-c", command, "-r", str(repeat),
        str(staged),
    ]
    completed = subprocess.run(args, capture_output=True, text=True, check=False)
    (out_dir / "alloy.log").write_text(completed.stdout + completed.stderr)


def check_alloy_model(
    jar: Path, model: Model, results: dict[str, dict] | None = None, timings: dict[str, float] | None = None
) -> list[str]:
    """Run every command of a model in one JVM and compare verdicts.

    When `results` is given, each command's observed verdict is recorded in it
    (see `snapshot`); when `timings` is given, the JVM's wall time is added to
    the model's entry."""
    staged = stage_alloy_model(model)
    out_dir = REPORT_DIR / model.name / "commands"
    with timed(timings, model.name):
        alloy_exec(jar, staged, out_dir)
    receipt_path = out_dir / "receipt.json"
    verdicts = alloy_verdicts(json.loads(receipt_path.read_text())) if receipt_path.exists() else {}
    failures = []
    for cmd in model.commands:
        observed = verdicts.get(cmd.name, VERDICT_NONE)
        if results is not None:
            results[f"{model.name}.{cmd.name}"] = {"verdict": observed}
        status = "ok" if observed == cmd.expect else "FAIL"
        print(f"  [{status}] {model.name}.{cmd.name}: expected {cmd.expect}, observed {observed}")
        if observed != cmd.expect:
            failures.append(f"{model.name}.{cmd.name}: expected {cmd.expect}, observed {observed}")
    return failures


# ---------------------------------------------------------------------------
# TLC

TLC_INVARIANT_RE = re.compile(r"^Error: Invariant (\w+) is violated", re.M)
TLC_PROPERTY_RE = re.compile(r"^Error: Temporal properties were violated", re.M)
TLC_DEADLOCK_RE = re.compile(r"^Error: Deadlock reached", re.M)
TLC_PASS_RE = re.compile(r"^Model checking completed\. No error has been found\.", re.M)
TLC_STATES_RE = re.compile(r"^(\d+) states generated, (\d+) distinct states found", re.M)
# An action whose definition starts with LET carries the LET body's location
# as a suffix: `<Dead line 6, col 1 to line 6, col 4 of module Tiny (6 28 6 42)>: 0:0`.
TLC_COVERAGE_RE = re.compile(
    r"^<(\w+) line \d+, col \d+ to line \d+, col \d+ of module \w+(?: \([\d ]+\))?>: (\d+):(\d+)", re.M
)


@dataclasses.dataclass(frozen=True)
class TlcResult:
    verdict: str
    violated: str = ""
    distinct_states: int = 0
    zero_state_actions: tuple[str, ...] = ()


def parse_tlc_output(output: str) -> TlcResult:
    states = TLC_STATES_RE.findall(output)
    distinct = int(states[-1][1]) if states else 0
    zero = tuple(name for name, _distinct, generated in TLC_COVERAGE_RE.findall(output) if int(generated) == 0)
    invariant = TLC_INVARIANT_RE.search(output)
    if invariant:
        return TlcResult(VERDICT_COUNTEREXAMPLE, invariant[1], distinct, zero)
    if TLC_PROPERTY_RE.search(output):
        return TlcResult(VERDICT_COUNTEREXAMPLE, "<temporal>", distinct, zero)
    if TLC_DEADLOCK_RE.search(output):
        return TlcResult(VERDICT_COUNTEREXAMPLE, "<deadlock>", distinct, zero)
    if TLC_PASS_RE.search(output):
        return TlcResult(VERDICT_PASS, "", distinct, zero)
    return TlcResult(VERDICT_NONE, "", distinct, zero)


def check_tlc_config_text(name: str, cfg_text: str) -> None:
    """Liveness configurations must not use SYMMETRY or VIEW."""
    keywords = set(re.findall(r"^\s*([A-Z_]+)\b", cfg_text, re.M))
    if keywords & {"PROPERTY", "PROPERTIES"} and keywords & {"SYMMETRY", "VIEW"}:
        raise FormalError(f"{name}: liveness configuration uses SYMMETRY or VIEW, which TLC does not keep sound")
    if re.search(r"^\s*CHECK_DEADLOCK\s+FALSE", cfg_text, re.M):
        raise FormalError(f"{name}: deadlock checking must stay on; use an explicit terminal stuttering action")


def tlc_config(model: Model, cmd: Command) -> str:
    """Render one command's TLC configuration: the model's base constants with
    the command's overrides, its specification, TypeOK, and exactly one
    checked property."""
    constants = dict(model.constants) | dict(cmd.constants)
    lines = ["CONSTANTS", *(f"    {k} = {v}" for k, v in constants.items()), f"SPECIFICATION {cmd.spec}", "INVARIANT TypeOK"]
    lines.append(f"{'PROPERTY' if cmd.temporal else 'INVARIANT'} {cmd.property}")
    return "\n".join(lines) + "\n"


def parallel_jobs() -> int:
    """Concurrent JVMs for TLC runs; FORMAL_JOBS lowers it under memory pressure."""
    return int(os.environ.get("FORMAL_JOBS", "0")) or os.cpu_count() or 2


def run_tlc(jar: Path, spec: str, cfg_text: str, work: Path, extra_args: tuple[str, ...] = ()) -> str:
    """Run TLC on `spec` (a module staged into `work`) and return its output.

    Each JVM extracts TLC's standard modules into java.io.tmpdir, so parallel
    runs each get their own to avoid racing on that extraction.
    """
    cfg = work / "run.cfg"
    cfg.write_text(cfg_text)
    completed = subprocess.run(
        ["java", "-XX:+UseParallelGC", f"-Djava.io.tmpdir={work}", "-cp", str(jar), "tlc2.TLC",
         "-workers", "1", "-metadir", str(work / "states"), *extra_args, "-config", str(cfg), spec],
        capture_output=True, text=True, check=False, cwd=work,
    )
    output = completed.stdout + completed.stderr
    (work / "tlc.log").write_text(output)
    return output


def stage_tla_modules(work: Path, extra: tuple[Path, ...] = ()) -> None:
    """Copy the models (and any generated modules) into a fresh work directory."""
    if work.exists():
        shutil.rmtree(work)
    work.mkdir(parents=True)
    for module in (*TLA_DIR.glob("*.tla"), *extra):
        shutil.copy(module, work / module.name)


# ---------------------------------------------------------------------------
# Static checks of TLA+ sources

TLA_DIR = REPO_ROOT / "formal" / "tla"
# A top-level operator definition header: `Name ==` or `Name(args) ==`.
TLA_DEFINITION = r"^(\w+)\s*(?:\([^)]*\))?\s*=="


def strip_tla_comments(source: str) -> str:
    """Blank out `\\*` line comments and `(* ... *)` block comments."""
    return blank_matches(r"\(\*.*?\*\)|\\\*[^\n]*", source)


def tla_definitions(source: str) -> list[str]:
    """Names of the top-level operator definitions (`Name ==` or `Name(args) ==`)."""
    return re.findall(TLA_DEFINITION, strip_tla_comments(source), re.M)


def tla_variables(source: str) -> list[str]:
    """Declared variables, in order, from every VARIABLE(S) declaration."""
    names: list[str] = []
    for block in re.findall(r"^VARIABLES?\b(.*?)(?=^\S|\Z)", strip_tla_comments(source), re.M | re.S):
        names += re.findall(r"\w+", block)
    return names


def check_tla_sources(tla_dir: Path = TLA_DIR) -> None:
    """Trace specs build on TraceBase, and no other module shadows its names.

    Every definition in TraceBase.tla is reserved: trace specs instantiate it
    next to their model, so a second definition would clash."""
    reserved = set(tla_definitions((tla_dir / "TraceBase.tla").read_text()))
    for path in sorted(tla_dir.glob("*.tla")):
        if path.name == "TraceBase.tla":
            continue
        source = path.read_text()
        clashes = [name for name in tla_definitions(source) if name in reserved]
        if clashes:
            raise FormalError(f"{path.name}: defines {', '.join(clashes)}, reserved by TraceBase.tla")
        if not path.stem.endswith("Trace"):
            continue
        if not re.search(r"^INSTANCE\s+TraceBase\b", strip_tla_comments(source), re.M):
            raise FormalError(f"{path.name}: a trace spec must `INSTANCE TraceBase` instead of re-implementing it")
        extra = [name for name in tla_variables(source) if name != "i"]
        if extra:
            raise FormalError(f"{path.name}: a trace spec may declare only the cursor variable i, not {', '.join(extra)}")


def check_variable_groups(name: str, source: str) -> None:
    """When `vars` is built from named groups (`fooVars == <<...>>`), the
    groups and individual variables it names must cover every declared
    variable exactly once."""
    code = strip_tla_comments(source)
    groups = {
        group: re.findall(r"\w+", members)
        for group, members in re.findall(r"^(\w+Vars)\s*==\s*<<(.*?)>>", code, re.M | re.S)
    }
    if not groups:
        return
    match = re.search(r"^vars\s*==\s*<<(.*?)>>", code, re.M | re.S)
    if match is None:
        raise FormalError(f"{name}: defines variable groups but no `vars == <<...>>`")
    members = [v for item in re.findall(r"\w+", match[1]) for v in groups.get(item, [item])]
    declared = tla_variables(source)
    duplicates = sorted({v for v in members if members.count(v) > 1})
    missing = [v for v in declared if v not in members]
    unknown = [v for v in members if v not in declared]
    if duplicates or missing or unknown:
        details = [f"{label}: {', '.join(vs)}" for label, vs in
                   (("duplicated", duplicates), ("missing", missing), ("not declared", unknown)) if vs]
        raise FormalError(f"{name}: variable groups in vars do not partition the variables ({'; '.join(details)})")


def attribute_temporal_violation(result: TlcResult, cmd: Command) -> TlcResult:
    """TLC does not name a violated temporal property. Generated configurations
    check exactly one property, so a temporal violation belongs to it."""
    if result.violated == "<temporal>" and cmd.temporal:
        return dataclasses.replace(result, violated=cmd.property)
    return result


def evaluate_tlc_command(model: Model, cmd: Command, result: TlcResult) -> list[str]:
    failures = []
    if result.verdict != cmd.expect:
        failures.append(f"{model.name}.{cmd.name}: expected {cmd.expect}, observed {result.verdict}")
    elif cmd.expect == VERDICT_COUNTEREXAMPLE and cmd.property and result.violated != cmd.property:
        failures.append(
            f"{model.name}.{cmd.name}: counterexample names {result.violated!r}, not the guarded property {cmd.property!r}"
        )
    uncovered = [a for a in result.zero_state_actions if a not in cmd.dead_actions]
    if cmd.expect == VERDICT_PASS and uncovered:
        failures.append(f"{model.name}.{cmd.name}: actions with zero generated states: {', '.join(uncovered)}")
    if cmd.distinct_states and result.distinct_states != cmd.distinct_states:
        print(
            f"WARNING: {model.name}.{cmd.name}: distinct states {result.distinct_states} "
            f"differ from recorded {cmd.distinct_states}"
        )
    return failures


def check_tlc_models(
    jar: Path, models: list[Model], results: dict[str, dict] | None = None, timings: dict[str, float] | None = None
) -> list[str]:
    """Check every command of every TLC model in parallel, one JVM each, and
    print each result as it completes.

    When `results` is given, each command's verdict, violated property,
    distinct-state count, and zero-coverage actions are recorded in it; when
    `timings` is given, each command's wall time is added to its model's."""
    check_tla_sources()
    jobs = []
    for model in models:
        check_variable_groups(model.name, (REPO_ROOT / model.file).read_text())
        for cmd in model.commands:
            cfg_text = tlc_config(model, cmd)
            check_tlc_config_text(f"{model.name}.{cmd.name}", cfg_text)
            jobs.append((model, cmd, cfg_text, REPORT_DIR / model.name / cmd.name))

    def run(job: tuple[Model, Command, str, Path]) -> tuple[str, TlcResult, list[str]]:
        model, cmd, cfg_text, work = job
        with timed(timings, model.name):
            stage_tla_modules(work)
            output = run_tlc(jar, Path(model.file).name, cfg_text, work, ("-coverage", "1"))
        result = attribute_temporal_violation(parse_tlc_output(output), cmd)
        cmd_failures = evaluate_tlc_command(model, cmd, result)
        status = "ok" if not cmd_failures else "FAIL"
        line = f"  [{status}] {model.name}.{cmd.name}: expected {cmd.expect}, observed {result.verdict} {result.violated}"
        return line.rstrip(), result, cmd_failures

    # Failures keep manifest order; result lines appear in completion order.
    failures_by_job: list[list[str]] = [[] for _ in jobs]
    with concurrent.futures.ThreadPoolExecutor(max_workers=parallel_jobs()) as pool:
        futures = {pool.submit(run, job): k for k, job in enumerate(jobs)}
        for future in concurrent.futures.as_completed(futures):
            k = futures[future]
            model, cmd, _cfg, _work = jobs[k]
            line, result, failures_by_job[k] = future.result()
            print(line)
            print(f"  states {model.name}.{cmd.name} {result.distinct_states}")
            if results is not None:
                results[f"{model.name}.{cmd.name}"] = {
                    "verdict": result.verdict,
                    "violated": result.violated,
                    "distinct_states": result.distinct_states,
                    "zero_coverage": sorted(result.zero_state_actions),
                }
    return [failure for job_failures in failures_by_job for failure in job_failures]


# ---------------------------------------------------------------------------
# Golden vectors (Alloy instance enumeration)
#
# The XML solution format is used because it is Alloy's canonical instance
# encoding: it lists every sig (including subset sigs) with its atoms. The JSON
# receipt omits subset-sig membership and is only trusted for SAT/UNSAT.

BUILTIN_SIGS = {"univ", "Int", "seq/Int", "String", "none"}
# Bump when the exported vector format changes.
GOLDEN_FORMAT = 2
# Compressed size budget for one golden file; [model.golden] max_bytes may lower it.
GOLDEN_MAX_BYTES = 600_000


def golden_command(model: Model) -> Command:
    name = model.golden.get("command")
    cmd = next((c for c in model.commands if c.name == name), None)
    if cmd is None or cmd.expect != VERDICT_INSTANCE:
        raise FormalError(f"{model.name}: [model.golden] command {name!r} must name a command expecting an instance")
    return cmd


def golden_command_digest(model: Model) -> str:
    return hashlib.sha256(render_alloy_command(model, golden_command(model)).encode()).hexdigest()


def golden_fingerprint(tools: dict[str, Tool], model: Model) -> str:
    """Identify what produced a golden file: format, model source, solver jar,
    golden command name, and the rendered golden command.

    Go tests recompute the model-source part (see internal/testutil/alloygolden),
    so a model edit without regeneration fails plain `make test`. The rendered
    command lives in the manifest; scripts/test_formal.py checks it without Java.
    """
    source = sha256_of(REPO_ROOT / model.file)
    return (
        f"format={GOLDEN_FORMAT};source={source};alloy={tools['alloy'].sha256};"
        f"command={model.golden['command']};golden={golden_command_digest(model)}"
    )


def golden_staleness(tools: dict[str, Tool], model: Model) -> str | None:
    """Java-free freshness: the committed header must carry the fingerprint the
    current model source, jar, and manifest golden command produce. Returns a
    message naming the model when it does not."""
    output = model.golden["output"]
    target = REPO_ROOT / output
    if not target.exists():
        return f"{model.name}: {output} is missing; generate it with: scripts/formal.py golden {model.name}"
    if read_golden_header(target).get("fingerprint") != golden_fingerprint(tools, model):
        return f"{model.name}: {output} is stale; regenerate with: scripts/formal.py golden {model.name}"
    return None


def xml_instance(text: str) -> ET.Element:
    instance = ET.fromstring(text).find("instance")
    if instance is None:
        raise FormalError("Alloy XML solution has no <instance>")
    return instance


def parse_alloy_xml_arities(text: str) -> dict[str, int]:
    """Each relation's declared arity, from its field's <types> (a relation
    empty in every instance has no tuples to count)."""
    arities = {}
    for field in xml_instance(text).findall("field"):
        types = field.find("types")
        if types is None or not types.findall("type"):
            raise FormalError(f"Alloy XML field {field.get('label')!r} declares no <types>")
        arities[field.get("label", "")] = len(types.findall("type"))
    return arities


def parse_alloy_xml_instance(text: str) -> dict:
    """Return {"sig": {name: [atoms]}, "rel": {name: [[atoms...]]}} for one solution."""
    instance = xml_instance(text)
    sigs: dict[str, list[str]] = {}
    rels: dict[str, list[list[str]]] = {}
    for sig in instance.findall("sig"):
        name = sig.get("label", "").removeprefix("this/")
        if name in BUILTIN_SIGS:
            continue
        sigs[name] = sorted(atom.get("label", "") for atom in sig.findall("atom"))
    for field in instance.findall("field"):
        name = field.get("label", "")
        rels[name] = sorted(
            [atom.get("label", "") for atom in tup.findall("atom")] for tup in field.findall("tuple")
        )
    return {"sig": dict(sorted(sigs.items())), "rel": dict(sorted(rels.items()))}


def export_golden(jar: Path, tools: dict[str, Tool], model: Model, check: bool = False) -> Path:
    command = model.golden.get("command")
    output = model.golden.get("output")
    if not command or not output:
        raise FormalError(f"{model.name}: golden export needs [model.golden] command and output")
    out_dir = REPORT_DIR / model.name / "golden"
    alloy_exec(jar, stage_alloy_model(model), out_dir, command=command, fmt="xml", repeat=0)
    files = sorted(out_dir.glob(f"{command}-solution-*.xml"), key=lambda p: int(p.stem.rsplit("-", 1)[1]))
    if not files:
        raise FormalError(f"{model.name}: golden command {command!r} produced no instances")
    texts = [f.read_text() for f in files]
    instances = [parse_alloy_xml_instance(text) for text in texts]
    vectors = {
        "model": model.name,
        "command": command,
        "fingerprint": golden_fingerprint(tools, model),
        "count": len(instances),
        **encode_golden(instances, parse_alloy_xml_arities(texts[0])),
    }
    target = REPO_ROOT / output
    payload = (json.dumps(vectors, sort_keys=True, separators=(",", ":")) + "\n").encode()
    shutil.rmtree(out_dir)  # one XML file per instance; the .gz is the artefact
    if check:
        committed = gzip.decompress(target.read_bytes()) if target.exists() else b""
        if committed != payload:
            raise FormalError(f"{model.name}: {output} is stale; regenerate with: scripts/formal.py golden {model.name}")
        print(f"  golden {model.name}: {len(instances)} instances match {output}")
        return target
    # mtime=0 keeps the archive byte-identical across regenerations.
    compressed = gzip.compress(payload, compresslevel=9, mtime=0)
    check_golden_size(model, output, len(compressed))
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(compressed)
    duplicates = len(instances) - len({json.dumps(i, sort_keys=True) for i in instances})
    print(f"  golden {model.name}: {len(instances)} instances ({duplicates} duplicates), {len(compressed)} bytes -> {output}")
    return target


def check_golden_size(model: Model, output: str, size: int) -> None:
    budget = min(GOLDEN_MAX_BYTES, model.golden.get("max_bytes", GOLDEN_MAX_BYTES))
    if size > budget:
        raise FormalError(f"{model.name}: {output} would be {size} compressed bytes, over the {budget}-byte budget")


# Golden format 2 is columnar and dictionary-coded:
#
#   atoms: every atom label in the file, sorted;
#   sig:   {name: {"values": [[atom index, ...], ...], "index": [value per instance]}};
#   rel:   {name: {"arity": n, "values": [[flat row-major atom indices], ...], "index": [...]}}.
#
# Each column stores its distinct values once. Atoms are sorted, so sorted
# index lists decode to the same sorted label lists as format 1. Instance order
# and duplicates are kept.


def encode_golden(instances: list[dict], arities: dict[str, int]) -> dict:
    """Encode instances (format-1 dicts) as format-2 atoms, sig, and rel columns."""
    sig_names = sorted(instances[0]["sig"]) if instances else []
    rel_names = sorted(instances[0]["rel"]) if instances else []
    for k, inst in enumerate(instances):
        if sorted(inst["sig"]) != sig_names or sorted(inst["rel"]) != rel_names:
            raise FormalError(f"golden instance {k} has a different set of sigs or relations")
    if set(rel_names) - set(arities):
        raise FormalError(f"golden relations without a declared arity: {sorted(set(rel_names) - set(arities))}")
    atoms = sorted(
        {a for inst in instances for v in inst["sig"].values() for a in v}
        | {a for inst in instances for v in inst["rel"].values() for t in v for a in t}
    )
    index_of = {atom: i for i, atom in enumerate(atoms)}

    def column(cells: list[tuple[int, ...]]) -> dict:
        values = sorted(set(cells))
        position = {value: i for i, value in enumerate(values)}
        return {"values": [list(v) for v in values], "index": [position[c] for c in cells]}

    sig = {name: column([tuple(index_of[a] for a in inst["sig"][name]) for inst in instances]) for name in sig_names}
    rel = {}
    for name in rel_names:
        arity = arities[name]
        cells = []
        for inst in instances:
            if any(len(t) != arity for t in inst["rel"][name]):
                raise FormalError(f"golden relation {name!r} has a tuple whose arity is not {arity}")
            cells.append(tuple(index_of[a] for t in inst["rel"][name] for a in t))
        rel[name] = {"arity": arity, **column(cells)}
    return {"atoms": atoms, "sig": sig, "rel": rel}


def decode_columns(data: dict) -> list[dict]:
    """Decode format-2 columns back into format-1 instance dicts."""
    atoms, count = data["atoms"], data["count"]
    for kind in ("sig", "rel"):
        for name, col in data[kind].items():
            if len(col["index"]) != count:
                raise FormalError(f"golden {kind} column {name!r} has {len(col['index'])} indices, want {count}")
    sig_values = {name: [[atoms[a] for a in v] for v in col["values"]] for name, col in data["sig"].items()}
    rel_values = {}
    for name, col in data["rel"].items():
        arity = col["arity"]
        rel_values[name] = [
            [[atoms[a] for a in v[i:i + arity]] for i in range(0, len(v), arity)] for v in col["values"]
        ]
    return [
        {
            "sig": {name: sig_values[name][col["index"][k]] for name, col in sorted(data["sig"].items())},
            "rel": {name: rel_values[name][col["index"][k]] for name, col in sorted(data["rel"].items())},
        }
        for k in range(count)
    ]


GOLDEN_HEADER_KEYS = ("model", "command", "fingerprint", "count")


def read_golden(path: Path) -> dict:
    return json.loads(gzip.decompress(path.read_bytes()))


def read_golden_header(path: Path) -> dict:
    """A golden file's model, command, fingerprint, and count."""
    data = read_golden(path)
    return {k: data.get(k) for k in GOLDEN_HEADER_KEYS}


def decode_golden(path: Path) -> tuple[dict, list[dict]]:
    """Return a golden file's header (model, command, fingerprint, count) and
    its instances as {"sig": {name: [atoms]}, "rel": {name: [[atoms...]]}}."""
    data = read_golden(path)
    header = {k: data[k] for k in GOLDEN_HEADER_KEYS}
    fmt = golden_file_format(header)
    if fmt != GOLDEN_FORMAT:
        raise FormalError(f"{path}: golden format {fmt} is not {GOLDEN_FORMAT}; regenerate with: make formal-golden")
    return header, decode_columns(data)


def golden_file_format(header: dict) -> int:
    match = re.match(r"format=(\d+);", header.get("fingerprint", ""))
    if not match:
        raise FormalError(f"golden file for {header.get('model')} has no format in its fingerprint")
    return int(match[1])


def golden_digest(instances: list[dict]) -> str:
    """Encoding-independent digest of an instance sequence (canonical JSON)."""
    return hashlib.sha256(json.dumps(instances, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


# ---------------------------------------------------------------------------
# Correspondence


CORRESPONDENCE_ROW_RE = re.compile(r"^\s*(?://|\\\*)\s*\|(?P<row>.+)\|\s*$", re.M)


def correspondence_rows(source: str) -> list[list[str]]:
    rows = []
    for match in CORRESPONDENCE_ROW_RE.finditer(source):
        cells = [cell.strip().strip("`") for cell in match["row"].split("|")]
        if len(cells) == 5 and cells[0] and not set(cells[0]) <= {"-"} and cells[0].lower() != "model element":
            rows.append(cells)
    return rows


SKIP_DIRS = {".git", "node_modules", "bin", "artifacts", "website"}
GO_DECL_RE = r"^(?:func (?:\([^)]*\) )?{name}\b|type {name}\b|\s+{name}\s+(?:=|[A-Za-z*\[])|(?:var|const) {name}\b)"
TEST_FUNC_RE = re.compile(r"^func (Test\w+)\(", re.M)
# Files whose tests bind a model; every test in them must be accounted for.
BINDING_TEST_SUFFIXES = ("_formal_test.go", "_golden_test.go", "_rapid_test.go", "_trace_test.go")
TRACE_HARNESS_SUFFIX = "_TraceHarness"
# A characterisation test asserts today's (defective) behaviour, so it can
# never kill a mutant of the property. It is marked by its name or by an
# abstraction note beginning `characterisation:` on the row that tables it.
CHARACTERISATION_NAME_RE = re.compile(r"^Test\w*?_(?:Characterisation$|FindingF\d+(?:_|$))")
CHARACTERISATION_NOTE = "characterisation:"


def binding_tests(cell: str) -> list[str]:
    """The test names in a binding cell: `-` or empty for none, otherwise a
    comma-separated list. An empty list item is returned as "" so the
    correspondence check can reject it."""
    if cell in {"", "-"}:
        return []
    return [name.strip().strip("`") for name in cell.split(",")]


def is_trace_harness(test: str) -> bool:
    return test.startswith("Test") and test.endswith(TRACE_HARNESS_SUFFIX)


def test_function_files(root: Path) -> dict[str, list[Path]]:
    """Each Go test function name, with the repository-relative files that
    declare it."""
    files: dict[str, list[Path]] = {}
    for path in go_test_files(root):
        for name in TEST_FUNC_RE.findall(path.read_text()):
            files.setdefault(name, []).append(path.relative_to(root))
    return files


def characterisation_tests(rows: list[list[str]]) -> set[str]:
    """Tests tabled on a row whose abstraction note marks them as characterisation."""
    return {
        test
        for _element, _symbol, _file, binding, abstraction in rows
        if abstraction.startswith(CHARACTERISATION_NOTE)
        for test in binding_tests(binding)
    }


def is_characterisation_test(test: str, noted: set[str]) -> bool:
    return test in noted or CHARACTERISATION_NAME_RE.match(test) is not None


def go_test_files(root: Path):
    for path in sorted(root.rglob("*_test.go")):
        parts = path.relative_to(root).parts
        if SKIP_DIRS.intersection(parts) or "testdata" in parts:
            continue
        yield path


def test_function_index(root: Path) -> set[str]:
    """Names of every Go test function in the repository, built once."""
    names: set[str] = set()
    for path in go_test_files(root):
        names.update(re.findall(r"^func (\w+)\(", path.read_text(), re.M))
    return names


def check_binding_completeness(tabled: set[str], suites: list[TraceSuite], root: Path) -> list[str]:
    """Every test in a binding-test file is either a trace harness in a
    `[[trace]]` suite's package or named by some correspondence row."""
    failures = []
    suite_dirs = {os.path.normpath(suite.package) for suite in suites}
    for path in go_test_files(root):
        if not path.name.endswith(BINDING_TEST_SUFFIXES):
            continue
        rel = path.relative_to(root)
        for test in TEST_FUNC_RE.findall(path.read_text()):
            if is_trace_harness(test):
                if os.path.normpath(rel.parent.as_posix()) not in suite_dirs:
                    failures.append(f"{rel}: trace harness {test} is not in a package declared by a [[trace]] suite")
            elif test not in tabled:
                failures.append(f"{rel}: binding test {test} is not named by any correspondence row")
    return failures


def check_correspondence(models: list[Model], root: Path = REPO_ROOT, suites: list[TraceSuite] | None = None) -> list[str]:
    """Every named Go symbol must be declared in its file; every binding test
    must exist; every test in a binding-test file must be tabled."""
    failures = []
    tests = test_function_index(root)
    tabled: set[str] = set()
    for model in models:
        rows = correspondence_rows((root / model.file).read_text())
        if not rows:
            failures.append(f"{model.name}: no correspondence table in {model.file}")
        for element, symbol, file, binding, _abstraction in rows:
            for test in binding_tests(binding):
                tabled.add(test)
                if not test:
                    failures.append(f"{model.name}: row {element!r} has an empty name in binding cell {binding!r}")
                elif test not in tests:
                    failures.append(f"{model.name}: row {element!r} names binding test {test} that does not exist")
            if symbol in {"", "-"}:
                continue
            path = root / file
            if not path.is_file():
                failures.append(f"{model.name}: row {element!r} names missing file {file}")
                continue
            leaf = symbol.split(".")[-1]
            if not re.search(GO_DECL_RE.format(name=re.escape(leaf)), path.read_text(), re.M):
                failures.append(f"{model.name}: row {element!r} names {symbol}, which is not declared in {file}")
    suites = load_trace_suites() if suites is None else suites
    return failures + check_binding_completeness(tabled, suites, root)


# ---------------------------------------------------------------------------
# Replay plan: the binding tests the CI replay step runs


def go_package(path: Path) -> str:
    """The `go test` package argument (`./dir/`) of a repository-relative file."""
    return f"./{path.parent.as_posix()}/"


def replay_plan(
    models: list[Model], root: Path = REPO_ROOT, files: dict[str, list[Path]] | None = None
) -> tuple[list[str], list[str]]:
    """The declaring packages (`./dir/`) and names of every binding test in
    the correspondence tables, minus the trace harnesses that trace validation
    runs. A binding that no `_test.go` file declares fails, naming the model
    and row. `files` is a prebuilt `test_function_files(root)`."""
    files = test_function_files(root) if files is None else files
    tests: set[str] = set()
    unknown = []
    for model in models:
        for element, _symbol, _file, binding, _abstraction in correspondence_rows((root / model.file).read_text()):
            for test in binding_tests(binding):
                if is_trace_harness(test):
                    continue
                if test not in files:
                    unknown.append(f"{model.name}: row {element!r} names binding test {test!r}, which no _test.go file declares")
                    continue
                tests.add(test)
    if unknown:
        raise FormalError("replay plan: " + "; ".join(unknown))
    packages = sorted({go_package(path) for test in tests for path in files[test]})
    return packages, sorted(tests)


def replay_pattern(tests: list[str]) -> str:
    return f"^({'|'.join(tests)})$"


# ---------------------------------------------------------------------------
# CI classification (`affected`)
#
# Runs before the manifest is validated and fails closed: every non-PR event,
# every error, an empty diff, and a manifest that does not load or validate
# all answer run=true. Patterns use fnmatch.fnmatchcase, where `*` also
# matches `/`, so they can only over-match GitHub `paths` globs.


def ci_paths(path: Path = MANIFEST) -> list[str]:
    with path.open("rb") as handle:
        patterns = tomllib.load(handle)["ci"]["paths"]
    if not patterns or not all(isinstance(p, str) and p for p in patterns):
        raise FormalError("[ci] paths must be a non-empty list of patterns")
    return patterns


def changed_paths(base: str, head: str) -> list[str]:
    if not base or not head:
        raise FormalError("affected: a pull_request needs --base and --head")
    completed = subprocess.run(
        ["git", "diff", "--name-only", base, head], capture_output=True, text=True, check=True, cwd=REPO_ROOT
    )
    return [line for line in completed.stdout.splitlines() if line]


def matches_ci_paths(path: str, patterns: list[str]) -> bool:
    return any(fnmatch.fnmatchcase(path, pattern) for pattern in patterns)


def affected(event: str, base: str, head: str, manifest: Path = MANIFEST, diff=changed_paths) -> str:
    """`run=true` or `run=false` for $GITHUB_OUTPUT; diagnostics go to stderr."""
    if event != "pull_request":
        print(f"affected: event {event!r} runs the full lane", file=sys.stderr)
        return "run=true"
    try:
        # A manifest that does not load or validate must reach `make formal`.
        validate_manifest(load_manifest(manifest)[1], load_trace_suites(manifest))
        patterns = ci_paths(manifest)
        changed = diff(base, head)
        if not changed:
            print("affected: empty diff; running the full lane", file=sys.stderr)
            return "run=true"
        hits = [path for path in changed if matches_ci_paths(path, patterns)]
        print(f"affected: {len(hits)} of {len(changed)} changed paths match [ci] paths {hits[:10]}", file=sys.stderr)
        return f"run={'true' if hits else 'false'}"
    except Exception as err:  # noqa: BLE001 - fail closed on anything
        print(f"affected: {type(err).__name__}: {err}; running the full lane", file=sys.stderr)
        return "run=true"


# ---------------------------------------------------------------------------
# Trace validation
#
# A trace is ACCEPTED when TLC violates the trace spec's NotFullyConsumed
# invariant: some behaviour of the model consumes every record. Deadlock
# checking is off for trace specs, because a trace that cannot continue is the
# expected way to reject it.

TRACE_DIR = REPORT_DIR / "traces"
TRACE_ACCEPTED = "accepted"
TRACE_REJECTED = "rejected"


def trace_verdict(output: str) -> str:
    if re.search(r"^Error: Invariant NotFullyConsumed is violated", output, re.M):
        return TRACE_ACCEPTED
    if TLC_PASS_RE.search(output):
        return TRACE_REJECTED
    return VERDICT_NONE


def proj_fields(spec: str, source: str) -> list[str]:
    """The sorted top-level field names of the `Proj ==` record literal.

    The definition runs to the next top-level definition. Fields are the
    `name |->` segments at bracket depth 1, so nested records, function
    constructors, tuples, and multi-line IF/THEN/ELSE values do not count.
    Anything but a record literal fails closed."""
    match = re.search(rf"^Proj\s*==(.*?)(?={TLA_DEFINITION}|^====|\Z)", strip_tla_comments(source), re.M | re.S)
    if match is None:
        raise FormalError(f"{spec}: no top-level `Proj ==` definition")
    # Blank string literals, and turn << >> into single bracket characters.
    body = re.sub(r'"[^"]*"', '""', match[1].strip()).replace("<<", "(").replace(">>", ")")
    not_record = FormalError(f"{spec}: Proj must be a single record literal [field |-> ..., ...]")
    if not body.startswith("["):
        raise not_record
    segments, start, depth = [], 1, 0
    for k, char in enumerate(body):
        if char in "[({":
            depth += 1
        elif char in "])}":
            depth -= 1
            if depth == 0:
                if body[k + 1:].strip():
                    raise not_record
                segments.append(body[start:k])
                break
        elif char == "," and depth == 1:
            segments.append(body[start:k])
            start = k + 1
    else:
        raise FormalError(f"{spec}: unbalanced brackets in Proj")
    fields = [re.match(r"\s*(\w+)\s*\|->", segment) for segment in segments]
    if not all(fields):
        raise not_record
    return sorted(field[1] for field in fields)


def record_traces(suite: TraceSuite) -> dict[str, int]:
    """Run the Go harness that writes <module>.tla and <module>.json."""
    env = dict(os.environ, **{"INVOWK_FORMAL_TRACE_DIR": str(TRACE_DIR)})
    completed = subprocess.run(
        ["go", "test", "-count=1", "-run", f"^{suite.test}$", suite.package],
        capture_output=True, text=True, check=False, env=env, cwd=REPO_ROOT,
    )
    if completed.returncode != 0:
        raise FormalError(f"trace harness {suite.test} failed:\n{completed.stdout}{completed.stderr}")
    counts_path = TRACE_DIR / f"{suite.module}.json"
    if not counts_path.exists():
        raise FormalError(f"trace harness {suite.test} wrote no {counts_path.name} (skipped?)")
    counts = json.loads(counts_path.read_text())
    if counts.get(TRACE_ACCEPTED, 0) == 0 or counts.get(TRACE_REJECTED, 0) == 0:
        raise FormalError(f"{suite.name}: a trace suite needs accepted traces and targeted mutations, got {counts}")
    check_proj_fields(suite, counts.get("fields"), (TLA_DIR / suite.spec).read_text())
    return counts


def check_proj_fields(suite: TraceSuite, recorded: list[str] | None, spec_source: str) -> None:
    """The harness's record keys must be exactly the trace spec's Proj fields;
    otherwise a misspelled key makes a targeted mutation vacuously rejected."""
    if not recorded:
        raise FormalError(f"{suite.name}: {suite.module}.json records no projection fields (use tlatrace.WriteSuite)")
    parsed = proj_fields(suite.spec, spec_source)
    if sorted(recorded) != parsed:
        raise FormalError(f"{suite.name}: harness records fields {sorted(recorded)}, but {suite.spec} projects {parsed}")


def trace_constants(suite: TraceSuite, model: Model) -> dict[str, str]:
    """The model's base constants with the suite's overrides; an override of a
    constant the model does not declare is a typo and fails closed."""
    base = dict(model.constants)
    unknown = sorted(k for k, _ in suite.constants if k not in base)
    if unknown:
        raise FormalError(f"[[trace]] {suite.name}: constants override {', '.join(unknown)}, absent from the model's base constants")
    return base | dict(suite.constants)


def check_trace(jar: Path, suite: TraceSuite, constants: dict[str, str], trace_set: str, index: int) -> tuple[str, str | None]:
    """Return the observed verdict of one trace, and a failure when it differs."""
    work = TRACE_DIR / suite.name / f"{trace_set}-{index}"
    stage_tla_modules(work, (TRACE_DIR / f"{suite.module}.tla",))
    constants = constants | {"TraceSet": f'"{trace_set}"', "TraceIndex": str(index)}
    cfg_text = "\n".join(
        ["CONSTANTS", *(f"    {k} = {v}" for k, v in constants.items()), "SPECIFICATION TraceSpec", "INVARIANT NotFullyConsumed"]
    ) + "\n"
    verdict = trace_verdict(run_tlc(jar, suite.spec, cfg_text, work, ("-deadlock",)))
    if verdict != trace_set:
        return verdict, f"{suite.name} {trace_set} trace {index}: expected {trace_set}, observed {verdict} (see {work / 'tlc.log'})"
    return verdict, None


def check_trace_suites(
    jar: Path,
    suites: list[TraceSuite],
    models: list[Model],
    results: dict[str, dict] | None = None,
    timings: dict[str, float] | None = None,
) -> list[str]:
    """Record and check every suite. When `results` is given, each suite's
    counts and per-trace verdicts are recorded in it; when `timings` is given,
    each suite's harness and trace-check wall times are added to its entry."""
    failures: list[str] = []
    check_tla_sources()
    if TRACE_DIR.exists():
        shutil.rmtree(TRACE_DIR)
    TRACE_DIR.mkdir(parents=True)
    by_name = {m.name: m for m in models}
    jobs = []
    for suite in suites:
        if suite.name not in by_name:
            raise FormalError(f"trace suite {suite.name} names no TLA+ model")
        constants = trace_constants(suite, by_name[suite.name])  # fails before recording
        with timed(timings, suite.label):
            counts = record_traces(suite)
        print(f"  {suite.name}: {counts[TRACE_ACCEPTED]} recorded traces, {counts[TRACE_REJECTED]} targeted mutations")
        if results is not None:
            results[suite.name] = {
                TRACE_ACCEPTED: counts[TRACE_ACCEPTED], TRACE_REJECTED: counts[TRACE_REJECTED], "verdicts": {}
            }
        for trace_set in (TRACE_ACCEPTED, TRACE_REJECTED):
            jobs.extend((suite, constants, trace_set, index) for index in range(1, counts[trace_set] + 1))
    def timed_check(job: tuple[TraceSuite, dict[str, str], str, int]) -> tuple[str, str | None]:
        with timed(timings, job[0].label):
            return check_trace(jar, *job)

    with concurrent.futures.ThreadPoolExecutor(max_workers=parallel_jobs()) as pool:
        for (suite, _constants, trace_set, index), (verdict, failure) in zip(jobs, pool.map(timed_check, jobs)):
            if results is not None:
                results[suite.name]["verdicts"][f"{trace_set}-{index}"] = verdict
            if failure:
                failures.append(failure)
    print(f"  checked {len(jobs)} traces, {len(failures)} unexpected")
    return failures


# ---------------------------------------------------------------------------
# Snapshot: before/after evidence for infrastructure refactors
#
# A snapshot records every observable result of the formal suite: each
# command's verdict (and, for TLC, violated property, distinct states, and
# zero-coverage actions), each trace suite's counts and per-trace verdicts, and
# the digest of each golden file's decoded instances. A refactor of the models
# or the runner must leave every entry identical.

SNAPSHOT_DEFAULT = REPORT_DIR / "snapshot.json"


def take_snapshot(tools: dict[str, Tool], models: list[Model]) -> tuple[dict, list[str]]:
    commands: dict[str, dict] = {}
    traces: dict[str, dict] = {}
    failures: list[str] = []
    alloy_models = [m for m in models if m.tool == "alloy"]
    if alloy_models:
        jar = ensure_tool(tools["alloy"])
        for model in alloy_models:
            failures += check_alloy_model(jar, model, commands)
    tla_jar = ensure_tool(tools["tla"])
    failures += check_tlc_models(tla_jar, [m for m in models if m.tool == "tla"], commands)
    failures += check_trace_suites(tla_jar, load_trace_suites(), models, traces)
    golden = {
        m.name: golden_digest(decode_golden(REPO_ROOT / m.golden["output"])[1]) for m in alloy_models if m.golden
    }
    return {"commands": commands, "traces": traces, "golden": golden}, failures


def flatten(record: object, prefix: str = "") -> dict[str, object]:
    """Flatten nested dicts into dotted-path leaves; lists stay leaves."""
    if not isinstance(record, dict):
        return {prefix: record}
    flat: dict[str, object] = {}
    for key, value in record.items():
        flat.update(flatten(value, f"{prefix}/{key}" if prefix else str(key)))
    return flat


def compare_snapshots(old: dict, new: dict) -> list[str]:
    """Every differing entry, with its old and new values."""
    before, after = flatten(old), flatten(new)
    return [
        f"{key}: {before.get(key, '<absent>')!r} -> {after.get(key, '<absent>')!r}"
        for key in sorted(before.keys() | after.keys())
        if before.get(key, "<absent>") != after.get(key, "<absent>")
    ]


def snapshot(tools: dict[str, Tool], models: list[Model], out: Path, compare: Path | None) -> int:
    record, failures = take_snapshot(tools, models)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(record, indent=1, sort_keys=True) + "\n")
    print(f"\nsnapshot written to {out}")
    for failure in failures:
        print(f"  note: {failure}")
    if compare is None:
        return 0
    diffs = compare_snapshots(json.loads(compare.read_text()), record)
    if diffs:
        print(f"\nsnapshot DIFFERS from {compare}:")
        for diff in diffs:
            print(f"  - {diff}")
        return 1
    print(f"snapshot identical to {compare} ({len(flatten(record))} entries)")
    return 0


# ---------------------------------------------------------------------------
# CLI


def select(models: list[Model], tool: str, names: list[str]) -> list[Model]:
    chosen = [m for m in models if m.tool == tool and (not names or m.name in names)]
    unknown = set(names) - {m.name for m in models}
    if unknown:
        raise FormalError(f"unknown model(s): {', '.join(sorted(unknown))}")
    return chosen


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument(
        "action",
        choices=["fetch", "alloy", "tla", "golden", "traces", "correspondence", "all", "snapshot", "affected", "replay-plan"],
    )
    parser.add_argument("models", nargs="*")
    parser.add_argument("--check", action="store_true", help="golden: verify committed vectors instead of rewriting them")
    parser.add_argument("--out", type=Path, default=SNAPSHOT_DEFAULT, help="snapshot: output file")
    parser.add_argument("--compare", type=Path, help="snapshot: fail unless every entry matches this snapshot")
    parser.add_argument("--event", default="", help="affected: the GitHub event name")
    parser.add_argument("--base", default="", help="affected: the pull request's base SHA")
    parser.add_argument("--head", default="", help="affected: the commit under test")
    parser.add_argument("--format", choices=["text", "github"], default="text", help="replay-plan: output format")
    args = parser.parse_args(argv)
    # Result lines must reach CI logs as they are produced, not when a pipe's
    # block buffer fills.
    if isinstance(sys.stdout, io.TextIOWrapper):
        sys.stdout.reconfigure(line_buffering=True)

    if args.action == "affected":
        # Before manifest validation: classification must answer even when
        # the manifest is broken.
        print(affected(args.event, args.base, args.head))
        return 0

    timings: dict[str, float] = {}
    try:
        tools, models = load_manifest()
        suites = load_trace_suites()
        validate_manifest(models, suites)
        failures: list[str] = []
        if args.action == "replay-plan":
            packages, tests = replay_plan(models)
            if args.format == "text":
                print("\n".join(f"  {test}" for test in tests))
            print(f"packages={' '.join(packages)}")
            print(f"run={replay_pattern(tests)}")
            return 0
        if args.action == "fetch":
            for tool in tools.values():
                ensure_tool(tool)
            return 0
        if args.action == "snapshot":
            return snapshot(tools, models, args.out, args.compare)
        if args.action in {"correspondence", "all"}:
            failures += check_correspondence(models)
        if args.action in {"alloy", "all"}:
            chosen = select(models, "alloy", args.models)
            if chosen:
                jar = ensure_tool(tools["alloy"])
                for model in chosen:
                    failures += check_alloy_model(jar, model, timings=timings)
        if args.action in {"tla", "all"}:
            chosen = select(models, "tla", args.models)
            if not chosen:
                print("no TLA+ models selected")
            else:
                failures += check_tlc_models(ensure_tool(tools["tla"]), chosen, timings=timings)
        if args.action == "all":
            # The fingerprint pre-check gives a fast, clear message; otherwise
            # re-enumerate and compare byte for byte.
            for model in select(models, "alloy", args.models):
                if not model.golden:
                    continue
                stale = golden_staleness(tools, model)
                if stale:
                    failures.append(stale)
                    continue
                try:
                    with timed(timings, model.name):
                        export_golden(ensure_tool(tools["alloy"]), tools, model, check=True)
                except FormalError as err:
                    failures.append(str(err))
        if args.action == "traces":
            chosen_suites = [t for t in suites if not args.models or t.name in args.models]
            if not chosen_suites:
                raise FormalError("no trace suites selected")
            failures += check_trace_suites(ensure_tool(tools["tla"]), chosen_suites, models, timings=timings)
        if args.action == "golden":
            jar = ensure_tool(tools["alloy"])
            for model in select(models, "alloy", args.models):
                if model.golden:
                    export_golden(jar, tools, model, check=args.check)
        report_timings(timings, manifest_budgets(models, suites))
        if failures:
            print("\nformal verification FAILED:")
            for failure in failures:
                print(f"  - {failure}")
            return 1
        print("\nformal verification passed")
        return 0
    except FormalError as err:
        print(f"formal verification FAILED: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
