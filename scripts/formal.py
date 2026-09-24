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
    scripts/formal.py correspondence
    scripts/formal.py all
"""

from __future__ import annotations

import argparse
import dataclasses
import gzip
import hashlib
import json
import re
import shutil
import subprocess
import sys
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
    config: str = ""
    fairness_twin_of: str = ""
    dead_actions: tuple[str, ...] = ()
    distinct_states: int = 0


@dataclasses.dataclass(frozen=True)
class Model:
    name: str
    tool: str
    file: str
    commands: tuple[Command, ...]
    calibration: str = ""
    golden: dict[str, str] = dataclasses.field(default_factory=dict)


# ---------------------------------------------------------------------------
# Manifest


def load_manifest(path: Path = MANIFEST) -> tuple[dict[str, Tool], list[Model]]:
    with path.open("rb") as handle:
        data = tomllib.load(handle)
    tools = {
        key: Tool(name=key, version=v["version"], jar=v["jar"], url=v["url"], sha256=v["sha256"])
        for key, v in data.get("tools", {}).items()
    }
    models = []
    for raw in data.get("model", []):
        commands = tuple(
            Command(
                name=c["name"],
                expect=c["expect"],
                property=c.get("property", ""),
                mutant_of=c.get("mutant_of", ""),
                antecedent=c.get("antecedent", ""),
                witness=c.get("witness", False),
                config=c.get("config", ""),
                fairness_twin_of=c.get("fairness_twin_of", ""),
                dead_actions=tuple(c.get("dead_actions", [])),
                distinct_states=c.get("distinct_states", 0),
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
            )
        )
    return tools, models


def validate_manifest(models: list[Model]) -> None:
    """Static guards that need no checker run."""
    for model in models:
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
            if not any(m.mutant_of == cmd.name for m in model.commands):
                raise FormalError(f"{model.name}.{cmd.name}: unguarded property {cmd.property!r} has no rejecting mutant")
            if model.tool == "alloy":
                ante = by_name.get(cmd.antecedent)
                if ante is None or ante.expect != VERDICT_INSTANCE:
                    raise FormalError(
                        f"{model.name}.{cmd.name}: Alloy check needs an antecedent run command expecting an instance"
                    )
        for cmd in model.commands:
            # Alloy witnesses are satisfiable runs; TLC witnesses are invariants ~W
            # that must be violated, naming W's situation as reachable.
            witness_verdict = VERDICT_INSTANCE if model.tool == "alloy" else VERDICT_COUNTEREXAMPLE
            if cmd.witness and cmd.expect != witness_verdict:
                raise FormalError(f"{model.name}.{cmd.name}: a {model.tool} witness must expect {witness_verdict}")
            if cmd.witness and model.tool == "tla" and not cmd.property:
                raise FormalError(f"{model.name}.{cmd.name}: a TLC witness must name its invariant as property")
        if model.tool == "alloy" and not any(c.expect == VERDICT_INSTANCE for c in model.commands):
            raise FormalError(f"{model.name}: Alloy model has no satisfiable (non-vacuity) run command")
        if not model.calibration:
            print(f"WARNING: {model.name} has no calibration record; its properties are not claimed as verified")


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
# Alloy

ALLOY_COMMAND_RE = re.compile(r"^\s*(?P<label>\w+)\s*:\s*(?P<kind>run|check)\b(?P<body>.*?)\bexpect\s+(?P<expect>[01])", re.S | re.M)


def alloy_source_commands(source: str) -> dict[str, tuple[str, int, str]]:
    """Map labelled command -> (kind, expect, body) from Alloy source text."""
    commands: dict[str, tuple[str, int, str]] = {}
    for match in ALLOY_COMMAND_RE.finditer(source):
        commands[match["label"]] = (match["kind"], int(match["expect"]), match["body"])
    return commands


def alloy_expected_bit(cmd: Command) -> int:
    """Alloy `expect 1` means an instance/counterexample exists."""
    return 0 if cmd.expect == VERDICT_PASS else 1


def cross_check_alloy_source(model: Model, source: str) -> None:
    declared = alloy_source_commands(source)
    for cmd in model.commands:
        if cmd.name not in declared:
            raise FormalError(f"{model.name}.{cmd.name}: no labelled command with an `expect` annotation in {model.file}")
        kind, expect_bit, body = declared[cmd.name]
        if expect_bit != alloy_expected_bit(cmd):
            raise FormalError(
                f"{model.name}.{cmd.name}: source says `expect {expect_bit}` but the manifest expects {cmd.expect}"
            )
        if cmd.expect == VERDICT_INSTANCE and kind != "run":
            raise FormalError(f"{model.name}.{cmd.name}: an instance-expecting command must be a run")
        if kind == "check" and cmd.property and not re.search(rf"\b{re.escape(cmd.property)}\b", body):
            raise FormalError(f"{model.name}.{cmd.name}: check body does not reference property {cmd.property!r}")
    for label in declared:
        if label not in {c.name for c in model.commands}:
            raise FormalError(f"{model.name}: command {label!r} in {model.file} is missing from the manifest")


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


def alloy_exec(jar: Path, model: Model, out_dir: Path, command: str = "*", fmt: str = "json", repeat: int = 1) -> None:
    """Run `alloy exec` for one command (or all with "*") into a fresh out_dir."""
    if out_dir.exists():
        shutil.rmtree(out_dir)
    out_dir.mkdir(parents=True)
    args = [
        "java", "--enable-native-access=ALL-UNNAMED", "-jar", str(jar), "exec",
        "-f", "-q", "-t", fmt, "-o", str(out_dir), "-c", command, "-r", str(repeat),
        str(REPO_ROOT / model.file),
    ]
    completed = subprocess.run(args, capture_output=True, text=True, check=False)
    (out_dir / "alloy.log").write_text(completed.stdout + completed.stderr)


def check_alloy_model(jar: Path, model: Model) -> list[str]:
    """Run every command of a model in one JVM and compare verdicts."""
    source = (REPO_ROOT / model.file).read_text()
    cross_check_alloy_source(model, source)
    out_dir = REPORT_DIR / model.name / "commands"
    alloy_exec(jar, model, out_dir)
    receipt_path = out_dir / "receipt.json"
    verdicts = alloy_verdicts(json.loads(receipt_path.read_text())) if receipt_path.exists() else {}
    failures = []
    for cmd in model.commands:
        observed = verdicts.get(cmd.name, VERDICT_NONE)
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
TLC_COVERAGE_RE = re.compile(r"^<(\w+) line \d+, col \d+ to line \d+, col \d+ of module \w+>: (\d+):(\d+)", re.M)


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


def check_fairness_twins(model: Model, config_texts: dict[str, str]) -> None:
    """Every liveness configuration needs a fairness-free twin expected to fail."""
    for cmd in model.commands:
        if not re.search(r"^\s*PROPERT(Y|IES)\b", config_texts.get(cmd.name, ""), re.M) or cmd.fairness_twin_of:
            continue
        twins = [c for c in model.commands if c.fairness_twin_of == cmd.name]
        if not twins or any(t.expect != VERDICT_COUNTEREXAMPLE for t in twins):
            raise FormalError(
                f"{model.name}.{cmd.name}: liveness configuration needs a fairness-free twin "
                "(fairness_twin_of) that expects a counterexample"
            )


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


def check_tlc_model(jar: Path, model: Model) -> list[str]:
    spec = REPO_ROOT / model.file
    config_texts = {cmd.name: (spec.parent / cmd.config).read_text() for cmd in model.commands}
    check_fairness_twins(model, config_texts)
    failures = []
    for cmd in model.commands:
        cfg = spec.parent / cmd.config
        check_tlc_config_text(f"{model.name}.{cmd.name}", config_texts[cmd.name])
        out_dir = REPORT_DIR / model.name / cmd.name
        out_dir.mkdir(parents=True, exist_ok=True)
        args = [
            "java", "-XX:+UseParallelGC", "-cp", str(jar), "tlc2.TLC", "-coverage", "1",
            "-workers", "auto", "-metadir", str(out_dir / "states"), "-config", str(cfg), str(spec),
        ]
        completed = subprocess.run(args, capture_output=True, text=True, check=False, cwd=spec.parent)
        output = completed.stdout + completed.stderr
        (out_dir / "tlc.log").write_text(output)
        result = parse_tlc_output(output)
        cmd_failures = evaluate_tlc_command(model, cmd, result)
        status = "ok" if not cmd_failures else "FAIL"
        print(f"  [{status}] {model.name}.{cmd.name}: expected {cmd.expect}, observed {result.verdict} {result.violated}".rstrip())
        failures.extend(cmd_failures)
    return failures


# ---------------------------------------------------------------------------
# Golden vectors (Alloy instance enumeration)
#
# The XML solution format is used because it is Alloy's canonical instance
# encoding: it lists every sig (including subset sigs) with its atoms. The JSON
# receipt omits subset-sig membership and is only trusted for SAT/UNSAT.

BUILTIN_SIGS = {"univ", "Int", "seq/Int", "String", "none"}
# Bump when the exported vector format changes.
GOLDEN_FORMAT = 1


def golden_fingerprint(tools: dict[str, Tool], model: Model) -> str:
    """Identify what produced a golden file: model source, solver jar, command, format.

    Go tests recompute the model-source part (see internal/testutil/alloygolden),
    so a model edit without regeneration fails plain `make test`.
    """
    source = sha256_of(REPO_ROOT / model.file)
    return f"format={GOLDEN_FORMAT};source={source};alloy={tools['alloy'].sha256};command={model.golden['command']}"


def golden_is_fresh(tools: dict[str, Tool], model: Model) -> bool:
    target = REPO_ROOT / model.golden["output"]
    if not target.exists():
        return False
    header = json.loads(gzip.decompress(target.read_bytes()))
    return header.get("fingerprint") == golden_fingerprint(tools, model)


def parse_alloy_xml_instance(text: str) -> dict:
    """Return {"sig": {name: [atoms]}, "rel": {name: [[atoms...]]}} for one solution."""
    root = ET.fromstring(text)
    instance = root.find("instance")
    if instance is None:
        raise FormalError("Alloy XML solution has no <instance>")
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
    alloy_exec(jar, model, out_dir, command=command, fmt="xml", repeat=0)
    files = sorted(out_dir.glob(f"{command}-solution-*.xml"), key=lambda p: int(p.stem.rsplit("-", 1)[1]))
    if not files:
        raise FormalError(f"{model.name}: golden command {command!r} produced no instances")
    instances = [parse_alloy_xml_instance(f.read_text()) for f in files]
    vectors = {
        "model": model.name,
        "command": command,
        "fingerprint": golden_fingerprint(tools, model),
        "count": len(instances),
        "instances": instances,
    }
    target = REPO_ROOT / output
    payload = (json.dumps(vectors, sort_keys=True, separators=(",", ":")) + "\n").encode()
    if check:
        committed = gzip.decompress(target.read_bytes()) if target.exists() else b""
        if committed != payload:
            raise FormalError(f"{model.name}: {output} is stale; regenerate with: scripts/formal.py golden {model.name}")
        print(f"  golden {model.name}: {len(instances)} instances match {output}")
        return target
    target.parent.mkdir(parents=True, exist_ok=True)
    # mtime=0 keeps the archive byte-identical across regenerations.
    target.write_bytes(gzip.compress(payload, mtime=0))
    shutil.rmtree(out_dir)  # one XML file per instance; the .gz is the artefact
    print(f"  golden {model.name}: {len(instances)} instances -> {output}")
    return target


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


def test_function_index(root: Path) -> set[str]:
    """Names of every Go test function in the repository, built once."""
    names: set[str] = set()
    for path in root.rglob("*_test.go"):
        if SKIP_DIRS.intersection(path.relative_to(root).parts):
            continue
        names.update(re.findall(r"^func (\w+)\(", path.read_text(), re.M))
    return names


def check_correspondence(models: list[Model], root: Path = REPO_ROOT) -> list[str]:
    """Every named Go symbol must be declared in its file; every binding test must exist."""
    failures = []
    tests = test_function_index(root)
    for model in models:
        rows = correspondence_rows((root / model.file).read_text())
        if not rows:
            failures.append(f"{model.name}: no correspondence table in {model.file}")
        for element, symbol, file, binding, _abstraction in rows:
            if symbol in {"", "-"}:
                continue
            path = root / file
            if not path.is_file():
                failures.append(f"{model.name}: row {element!r} names missing file {file}")
                continue
            leaf = symbol.split(".")[-1]
            if not re.search(GO_DECL_RE.format(name=re.escape(leaf)), path.read_text(), re.M):
                failures.append(f"{model.name}: row {element!r} names {symbol}, which is not declared in {file}")
            if binding not in {"", "-"} and binding not in tests:
                failures.append(f"{model.name}: row {element!r} names binding test {binding} that does not exist")
    return failures


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
    parser.add_argument("action", choices=["fetch", "alloy", "tla", "golden", "correspondence", "all"])
    parser.add_argument("models", nargs="*")
    parser.add_argument("--check", action="store_true", help="golden: verify committed vectors instead of rewriting them")
    args = parser.parse_args(argv)

    try:
        tools, models = load_manifest()
        validate_manifest(models)
        failures: list[str] = []
        if args.action == "fetch":
            for tool in tools.values():
                ensure_tool(tool)
            return 0
        if args.action in {"correspondence", "all"}:
            failures += check_correspondence(models)
        if args.action in {"alloy", "all"}:
            chosen = select(models, "alloy", args.models)
            if chosen:
                jar = ensure_tool(tools["alloy"])
                for model in chosen:
                    failures += check_alloy_model(jar, model)
        if args.action in {"tla", "all"}:
            chosen = select(models, "tla", args.models)
            if not chosen:
                print("no TLA+ models selected")
            else:
                jar = ensure_tool(tools["tla"])
                for model in chosen:
                    failures += check_tlc_model(jar, model)
        if args.action == "all":
            for model in select(models, "alloy", args.models):
                if model.golden and not golden_is_fresh(tools, model):
                    failures.append(
                        f"{model.name}: {model.golden['output']} is stale; regenerate with: scripts/formal.py golden {model.name}"
                    )
        if args.action == "golden":
            jar = ensure_tool(tools["alloy"])
            for model in select(models, "alloy", args.models):
                if model.golden:
                    export_golden(jar, tools, model, check=args.check)
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
