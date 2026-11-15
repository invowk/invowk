#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Formal-bindings mutation profile: plan, executor lookup, pre-flight, reruns,
and the survivor triage ledger.

The plan is generated at run time from the correspondence tables named by
formal/manifest.toml, so no hand-kept target list can drift from the models.
For each Go function a bound row names, it records the function's line range,
the rows naming it, and its killer tests (binding tests minus trace harnesses
and characterisation tests) with the packages that declare them. The plan fails
closed on a correspondence failure, a function-name collision, a killer test
declared in zero or several packages, or an empty plan.

scripts/mutation-formal-exec.sh asks `exec-args` which function a mutant
changed and runs only that function's killer tests, across packages, through a
`go test -overlay`. scripts/mutation.sh drives the pre-flight
(`preflight-record`), rerun evidence (`rerun-record`), and the ledger check
(`triage --check`).

Standard library only (Python 3.11+ for tomllib).

Usage:
    scripts/formal_mutation.py plan --out DIR
    scripts/formal_mutation.py show --plan FILE [--group N] {match|files|group-count|max-timeout}
    scripts/formal_mutation.py exec-args --plan FILE --original PATH --changed PATH [--overlay-out FILE]
    scripts/formal_mutation.py preflight-record --plan FILE --file PATH --json FILE --status N --seconds S
    scripts/formal_mutation.py rerun-scope --plan FILE [--hint REPORT ...] ID ...
    scripts/formal_mutation.py rerun-record --plan FILE --summary FILE --id ID [--reruns FILE]
    scripts/formal_mutation.py merge-reports --report-dir DIR
    scripts/formal_mutation.py merge-baselines --report-dir DIR [--baseline FILE]
    scripts/formal_mutation.py triage --check [--baseline FILE] [--ledger FILE] [--reruns FILE] [--base SHA]
        (without --base: $INVOWK_TRIAGE_BASE when non-empty, else git merge-base HEAD origin/main)
    scripts/formal_mutation.py reach [--out DIR]

`reach` is the binding reach gate: it runs every killer test of the plan alone
under a coverage profile and fails unless each killer of each bound row
executes at least one statement of the row's function.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import dataclasses
import datetime
import hashlib
import io
import json
import math
import os
import re
import subprocess
import sys
import time
import tomllib
from collections.abc import Callable, Iterable
from pathlib import Path
from typing import NamedTuple

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))

import formal  # noqa: E402 - sibling module, imported after the path fix

REPO_ROOT = formal.REPO_ROOT
PLAN_FILE = "formal-plan.json"
PLAN_VERSION = 2
BASELINE = REPO_ROOT / "tools" / "mutation" / "baselines" / "formal-bindings-baseline.json"
LEDGER = REPO_ROOT / "tools" / "mutation" / "triage" / "formal-bindings.toml"
RERUNS = REPO_ROOT / "tools" / "mutation" / "triage" / "formal-bindings-reruns.jsonl"
REACH_DIR = formal.REPORT_DIR / "reach"
# The mutation profile's rapid determinism pins live in scripts/mutation.sh;
# the reach gate reads them from there so the two cannot drift.
MUTATION_SH = REPO_ROOT / "scripts" / "mutation.sh"

# Per-file exec timeout: max(MIN_EXEC_TIMEOUT, TIMEOUT_FACTOR x clean wall time).
MIN_EXEC_TIMEOUT = 10
TIMEOUT_FACTOR = 5
TIMEOUT_OVERRIDE_ENV = "MUTATION_FORMAL_EXEC_TIMEOUT"
# The pre-flight itself runs before any timeout is measured.
PREFLIGHT_TIMEOUT = 900

REASON_NO_SYMBOL = "no-symbol"
REASON_NO_BINDING = "no-binding"
REASON_TYPE_SYMBOL = "type-symbol"
REASON_TRACE_ONLY = "trace-only"
REASON_CHARACTERISATION_ONLY = "characterisation-only"

TRIAGE_CLASSES = {"equivalent", "abstraction", "binding-gap", "model-gap", "defect"}
FOLLOW_UP_CLASSES = {"binding-gap", "model-gap", "defect"}
SURVIVOR_KEYS = {"id", "file", "symbol", "model", "element", "class", "reason", "follow_up"}
CLOSED_KEYS = {"id", "model"}
REQUIRED_ESCAPED_RERUNS = 2
# go-mutesting's result classes, as named in report.json and the rerun evidence.
RERUN_STATUSES = ("killed", "escaped", "skipped", "errored")
RERUN_REQUIRED_KEYS = {"id", "status", "timestamp", "digest"}

# A gofmt'd top-level function declaration: optional receiver (named or not,
# pointer or not, generic or not), the name, optional type parameters, then `(`.
# Shared with the correspondence check's receiver test.
FUNC_DECL_RE = formal.FUNC_DECL_RE


class PlanError(Exception):
    """A fail-closed plan, pre-flight, or triage condition."""


class Row(NamedTuple):
    model: str
    element: str
    symbol: str
    file: str
    binding: str
    abstraction: str

    @property
    def where(self) -> str:
        return f"{self.model}: row {self.element!r} ({self.symbol} in {self.file})"

    @property
    def key(self) -> tuple[str, str, str]:
        """(model, element, symbol): how the ledger and [triage] name a row."""
        return self.model, self.element, self.symbol


@dataclasses.dataclass(frozen=True)
class FuncDecl:
    receiver: str
    name: str
    start: int
    end: int

    @property
    def symbol(self) -> str:
        return f"{self.receiver}.{self.name}" if self.receiver else self.name


def model_rows(models: list[formal.Model], root: Path) -> list[Row]:
    return [
        Row(model.name, *cells)
        for model in models
        for cells in formal.correspondence_rows((root / model.file).read_text())
    ]


def function_decls(source: str) -> list[FuncDecl]:
    """Every top-level function declaration with its line range: from the
    `func` line to the next line that is exactly `}` (gofmt'd sources)."""
    lines = source.splitlines()
    decls = []
    for match in FUNC_DECL_RE.finditer(source):
        start = source.count("\n", 0, match.start()) + 1
        end = next((i + 1 for i in range(start - 1, len(lines)) if lines[i] == "}"), None)
        if lines[start - 1].rstrip().endswith("}"):
            end = start  # a one-line function
        if end is None:
            raise PlanError(f"function {match['name']} at line {start} has no closing brace at column 0")
        decls.append(FuncDecl(match["recv"] or "", match["name"], start, end))
    return decls


def split_symbol(symbol: str) -> tuple[str, str]:
    receiver, _, name = symbol.rpartition(".")
    return receiver, name


def find_function(decls: list[FuncDecl], symbol: str) -> FuncDecl | None:
    """The declaration a row's symbol names. `Recv.name` must match the
    receiver; a bare `name` may name a method when it is the file's only
    function of that name."""
    receiver, name = split_symbol(symbol)
    found = [d for d in decls if d.name == name and (not receiver or d.receiver == receiver)]
    if len(found) > 1:
        found = [d for d in found if not d.receiver]
    if len(found) > 1:
        raise PlanError(f"{symbol} names several functions: {', '.join(d.symbol for d in found)}")
    return found[0] if found else None


def test_declarations(root: Path) -> tuple[dict[str, set[str]], dict[str, list[Path]]]:
    """Test name -> directories (relative, POSIX) whose test files declare it,
    and test name -> the declaring files (formal.test_function_files)."""
    files = formal.test_function_files(root)
    return {test: {path.parent.as_posix() for path in paths} for test, paths in files.items()}, files


def go_list_packages(dirs: Iterable[str], root: Path = REPO_ROOT) -> dict[str, str]:
    """Directory (relative) -> import path, through `go list` on each directory."""
    dirs = sorted(set(dirs))
    if not dirs:
        return {}
    go = os.environ.get("GO_CMD", "go")
    patterns = ["./" + d if d != "." else "." for d in dirs]
    result = subprocess.run(
        [go, "list", "-f", "{{.Dir}}\t{{.ImportPath}}", *patterns],
        cwd=root, capture_output=True, text=True, check=False,
    )
    if result.returncode != 0:
        raise PlanError(f"go list failed for {' '.join(patterns)}:\n{result.stderr.strip()}")
    by_dir = {}
    for line in result.stdout.splitlines():
        abs_dir, _, import_path = line.partition("\t")
        by_dir[Path(abs_dir).resolve().relative_to(root.resolve()).as_posix() or "."] = import_path
    missing = [d for d in dirs if d not in by_dir]
    if missing:
        raise PlanError(f"go list did not resolve test directories: {', '.join(missing)}")
    return by_dir


def match_groups(functions: dict[tuple[str, str], dict], decls_by_file: dict[str, list[FuncDecl]]) -> tuple[list[dict], list[str]]:
    """Partition the target files into go-mutesting invocations whose union
    `--match` selects exactly the named functions of every file in them.

    go-mutesting's --match sees only the bare function name, so a leaf such as
    `Validate` named in one file would also select every other `Validate` in
    the files it shares an invocation with. Files are assigned greedily to the
    first group they do not collide with. A collision inside one file (a named
    leaf that also names an unnamed function of the same file) cannot be split
    and fails closed."""
    named: dict[str, set[str]] = {}
    for file, symbol in functions:
        named.setdefault(file, set()).add(symbol)
    leaves = {file: {split_symbol(symbol)[1] for symbol in symbols} for file, symbols in named.items()}

    def selects(leaf_file: str, file: str) -> list[FuncDecl]:
        return [d for d in decls_by_file[file] if d.name in leaves[leaf_file] and d.symbol not in named[file]]

    problems = []
    for file in sorted(named):
        for decl in selects(file, file):
            owners = sorted(f"{r['model']}: row {r['element']!r}" for (f, symbol), e in functions.items()
                            if f == file and split_symbol(symbol)[1] == decl.name for r in e["rows"])
            problems.append(
                f"match collision: {decl.symbol} in {file} (line {decl.start}) shares the leaf {decl.name!r} "
                f"with {'; '.join(owners)}, but no row names it"
            )
    groups: list[list[str]] = []
    for file in sorted(named):
        for group in groups:
            if not any(selects(file, other) or selects(other, file) for other in group):
                group.append(file)
                break
        else:
            groups.append([file])
    return [{"match": formal.replay_pattern(leaf for f in group for leaf in leaves[f]), "files": group} for group in groups], problems


@dataclasses.dataclass
class Classified:
    """The plan's row classification: bound rows grouped by function, the
    unbound rows with their reason, and the fail-closed problems. It reads the
    tables and the target sources only, never `go list`, so `triage --check`
    shares it."""

    functions: dict[tuple[str, str], dict]
    unbound: list[dict]
    problems: list[str]
    decls_by_file: dict[str, list[FuncDecl]]
    # (model, element, symbol) of every bound row.
    bound: set[tuple[str, str, str]] = dataclasses.field(default_factory=set)

    @property
    def reasons(self) -> dict[tuple[str, str, str], str]:
        """(model, element, symbol) -> reason, for every unbound row."""
        return {(u["model"], u["element"], u["symbol"]): u["reason"] for u in self.unbound}


def classify_rows(
    rows: list[Row], root: Path, declared: dict[str, set[str]], read: Callable[[str], str] | None = None
) -> Classified:
    """`read` returns a repository-relative Go file's text (default: from
    `root`); triage --check passes a reader of the base commit."""
    read = read or (lambda file: (root / file).read_text())
    noted = formal.characterisation_tests(rows)
    decls_by_file: dict[str, list[FuncDecl]] = {}
    functions: dict[tuple[str, str], dict] = {}
    unbound = []
    problems = []
    bound: set[tuple[str, str, str]] = set()

    def skip(row: Row, reason: str) -> None:
        unbound.append({"model": row.model, "element": row.element, "symbol": row.symbol, "file": row.file,
                        "reason": reason, "abstraction": row.abstraction})

    for row in rows:
        if row.symbol in {"", "-"}:
            skip(row, REASON_NO_SYMBOL)
            continue
        if row.file not in decls_by_file:
            decls_by_file[row.file] = function_decls(read(row.file))
        try:
            decl = find_function(decls_by_file[row.file], row.symbol)
        except PlanError as err:
            problems.append(f"{row.where}: {err}")
            continue
        if decl is None:
            leaf = split_symbol(row.symbol)[1]
            receivers = formal.receiver_mismatch(row.symbol, (d.receiver for d in decls_by_file[row.file] if d.name == leaf))
            if receivers is not None:
                # The leaf is a function, so this is not a type symbol: the
                # receiver no longer matches (decision 27).
                problems.append(f"{row.where}: {row.symbol} names no function: {row.file} declares "
                                f"{leaf} only with receiver {formal.describe_receivers(receivers)}")
                continue
            # check_correspondence proved the symbol is declared, so it is a
            # type, interface, field, or value rather than a function. That
            # holds bound or not, so a row without a binding is not a binding
            # gap (harden-formal-instruments decision 15).
            skip(row, REASON_TYPE_SYMBOL)
            continue
        tests = formal.binding_tests(row.binding)
        if not tests:
            skip(row, REASON_NO_BINDING)
            continue
        killers = [t for t in tests if not formal.is_trace_harness(t) and not formal.is_characterisation_test(t, noted)]
        if not killers:
            only_harness = all(formal.is_trace_harness(t) for t in tests)
            skip(row, REASON_TRACE_ONLY if only_harness else REASON_CHARACTERISATION_ONLY)
            continue
        for test in killers:
            dirs = declared.get(test, set())
            if len(dirs) != 1:
                where = "no package" if not dirs else "several packages: " + ", ".join(sorted(dirs))
                problems.append(f"{row.where}: killer test {test} is declared in {where}")
        entry = functions.setdefault(
            (row.file, decl.symbol),
            {"file": row.file, "symbol": decl.symbol, "name": decl.name, "start": decl.start, "end": decl.end,
             "rows": [], "killers": set()},
        )
        # Each row keeps its own killers, so the reach gate can tell a row
        # whose binding misses the function from one that shares it.
        entry["rows"].append({"model": row.model, "element": row.element, "killers": sorted(set(killers))})
        entry["killers"].update(killers)
        bound.add(row.key)
    return Classified(functions, unbound, problems, decls_by_file, bound)


def build_plan(
    models: list[formal.Model],
    root: Path = REPO_ROOT,
    suites: list[formal.TraceSuite] | None = None,
    resolve_packages: Callable[[Iterable[str]], dict[str, str]] | None = None,
) -> dict:
    """The formal-bindings plan, or PlanError listing every fail-closed reason."""
    declared, declaring_files = test_declarations(root)
    failures = formal.check_correspondence(models, root=root, suites=suites, files=declaring_files)
    if failures:
        raise PlanError("correspondence check failed:\n  - " + "\n  - ".join(failures))

    classified = classify_rows(model_rows(models, root), root, declared)
    functions, problems = classified.functions, list(classified.problems)
    if not functions and not problems:
        problems.append("the plan has no function targets: no bound row names a Go function")

    groups: list[dict] = []
    if functions:
        groups, collisions = match_groups(functions, classified.decls_by_file)
        problems += collisions
    if problems:
        raise PlanError("formal-bindings plan failed:\n  - " + "\n  - ".join(problems))

    resolve = resolve_packages or (lambda dirs: go_list_packages(dirs, root))
    killer_dirs = {next(iter(declared[t])) for entry in functions.values() for t in entry["killers"]}
    target_dirs = {Path(entry["file"]).parent.as_posix() or "." for entry in functions.values()}
    packages = resolve(killer_dirs | target_dirs)
    inputs = {entry["file"] for entry in functions.values()}
    inputs.update(path.as_posix() for entry in functions.values() for t in entry["killers"] for path in declaring_files[t])
    ordered = []
    for key in sorted(functions):
        entry = functions[key]
        killers = sorted(entry["killers"])
        entry["killers"] = killers
        entry["packages"] = sorted({packages[next(iter(declared[t]))] for t in killers})
        entry["package"] = packages[Path(entry["file"]).parent.as_posix() or "."]
        ordered.append(entry)
    return {
        "version": PLAN_VERSION,
        "root": str(root.resolve()),
        "files": sorted({entry["file"] for entry in ordered}),
        "groups": groups,
        # What decides a mutant's outcome: the target files and the files that
        # declare their killer tests. Its digest identifies rerun evidence.
        "inputs": sorted(inputs),
        "functions": ordered,
        # Killer test -> the import path of the package that declares it.
        "killer_packages": {t: packages[next(iter(declared[t]))] for entry in ordered for t in entry["killers"]},
        "unbound": sorted(classified.unbound, key=lambda u: (u["model"], u["element"], u["symbol"])),
        "preflight": {},
    }


def write_plan(plan: dict, out: Path) -> Path:
    out.mkdir(parents=True, exist_ok=True)
    plan_path = out / PLAN_FILE
    plan_path.write_text(json.dumps(plan, indent=2) + "\n")
    (out / "resolved-targets.txt").write_text("".join(f"{f}\n" for f in plan["files"]))
    (out / "unbound-rows.txt").write_text(
        "".join(f"{u['model']}\t{u['element']}\t{u['symbol']}\t{u['file']}\t{u['reason']}\n" for u in plan["unbound"])
    )
    return plan_path


def load_plan(path: Path) -> dict:
    plan = json.loads(path.read_text())
    if plan.get("version") != PLAN_VERSION:
        raise PlanError(f"{path}: unsupported plan version {plan.get('version')!r}")
    return plan


def inputs_digest(paths: Iterable[str], read: Callable[[str], bytes]) -> str:
    """SHA-256 over the plan inputs, sorted by path, each as path + NUL +
    bytes. Rerun evidence and the baseline carry it, so evidence stays valid
    across squash merges and rebases as long as the inputs are unchanged."""
    digest = hashlib.sha256()
    for path in sorted(paths):
        digest.update(path.encode() + b"\0" + read(path))
    return digest.hexdigest()


def worktree_reader(root: Path = REPO_ROOT) -> Callable[[str], bytes]:
    return lambda path: (root / path).read_bytes()


def git_reader(commit: str) -> Callable[[str], bytes]:
    """Read a path from the tree of `commit` (git show <commit>:<path>)."""
    def read(path: str) -> bytes:
        return subprocess.run(
            ["git", "-C", str(REPO_ROOT), "show", f"{commit}:{path}"], capture_output=True, check=True
        ).stdout
    return read


def plan_digest(plan: dict, read: Callable[[str], bytes] | None = None) -> str:
    return inputs_digest(plan["inputs"], read or worktree_reader(Path(plan["root"])))


def functions_in(plan: dict, file: str) -> list[dict]:
    return [f for f in plan["functions"] if f["file"] == file]


# ---------------------------------------------------------------------------
# Executor lookup


def first_changed_line(original: list[str], changed: list[str]) -> int | None:
    """1-based line of the original where the first difference starts."""
    index = next((i for i, (a, b) in enumerate(zip(original, changed)) if a != b), None)
    if index is None:
        if len(original) == len(changed):
            return None
        index = min(len(original), len(changed))
    return max(1, min(index + 1, len(original)))


def plan_relative(plan: dict, path: str) -> str:
    root = Path(plan["root"])
    resolved = Path(path).resolve()
    try:
        return resolved.relative_to(root).as_posix()
    except ValueError as err:
        raise PlanError(f"{path} is outside the plan root {root}") from err


def timeout_override() -> int | None:
    """MUTATION_FORMAL_EXEC_TIMEOUT in seconds, or None when unset."""
    override = os.environ.get(TIMEOUT_OVERRIDE_ENV, "")
    if not override:
        return None
    if not override.isdigit() or int(override) <= 0:
        raise PlanError(f"{TIMEOUT_OVERRIDE_ENV} must be a positive number of seconds, got {override!r}")
    return int(override)


def exec_timeout(plan: dict, file: str, preflight: bool) -> int:
    override = timeout_override()
    if override is not None:
        return override
    if preflight:
        return PREFLIGHT_TIMEOUT
    record = plan["preflight"].get(file)
    if record is None:
        raise PlanError(f"{file}: no pre-flight record in the plan; run the clean-code pre-flight first")
    return int(record["timeout_seconds"])


def exec_args(plan: dict, original: str, changed: str) -> dict:
    """What the executor runs for one mutant (or, when the files are identical,
    the pre-flight of the whole file)."""
    file = plan_relative(plan, original)
    targets = functions_in(plan, file)
    if not targets:
        raise PlanError(f"{file} is not a formal-bindings target file")
    original_lines = Path(original).read_text().splitlines()
    line = first_changed_line(original_lines, Path(changed).read_text().splitlines())
    if line is None:
        chosen = targets
    else:
        chosen = [f for f in targets if f["start"] <= line <= f["end"]]
        if not chosen:
            raise PlanError(f"{file}:{line}: the mutated line is outside every planned function")
    killers = sorted({t for f in chosen for t in f["killers"]})
    return {
        "file": file,
        "function": ",".join(f["symbol"] for f in chosen),
        "preflight": line is None,
        "timeout": exec_timeout(plan, file, line is None),
        "run": formal.replay_pattern(killers),
        "packages": sorted({p for f in chosen for p in f["packages"]}),
    }


def write_overlay(original: str, changed: str, out: Path) -> None:
    out.write_text(json.dumps({"Replace": {str(Path(original).resolve()): str(Path(changed).resolve())}}))


# ---------------------------------------------------------------------------
# Pre-flight


def test_outcomes(json_lines: str) -> dict[str, str]:
    """Top-level test name -> final action (pass/fail/skip) from `go test -json`."""
    outcomes: dict[str, str] = {}
    for raw in json_lines.splitlines():
        try:
            event = json.loads(raw)
        except json.JSONDecodeError:
            continue  # build output interleaved by go test
        test = event.get("Test", "")
        if test and "/" not in test and event.get("Action") in {"pass", "fail", "skip"}:
            outcomes[test] = event["Action"]
    return outcomes


def derive_timeout(clean_seconds: float) -> int:
    return max(MIN_EXEC_TIMEOUT, math.ceil(TIMEOUT_FACTOR * clean_seconds))


def preflight_record(plan: dict, file: str, json_output: str, status: int, seconds: float) -> dict:
    """Check one file's clean-code run and record its timeout in the plan.

    Every killer of every function in the file must be reported as passed (a
    failure, skip, timeout, or absent report fails the run), and the executor
    must have mapped the run to "escaped" (exit 1)."""
    file = plan_relative(plan, file) if os.path.isabs(file) else file
    targets = functions_in(plan, file)
    if not targets:
        raise PlanError(f"{file} is not a formal-bindings target file")
    outcomes = test_outcomes(json_output)
    problems = []
    for function in targets:
        for test in function["killers"]:
            outcome = outcomes.get(test, "not run")
            if outcome != "pass":
                rows = "; ".join(f"{r['model']}: row {r['element']!r}" for r in function["rows"])
                problems.append(f"{file}: {function['symbol']} ({rows}): killer {test} {outcome} on clean code")
    if status != 1 and not problems:
        problems.append(f"{file}: clean-code run exited {status}, expected 1 (all killer tests pass)")
    if problems:
        raise PlanError("formal-bindings pre-flight failed:\n  - " + "\n  - ".join(problems))
    override = timeout_override()
    record = {
        "clean_seconds": round(seconds, 3),
        "timeout_seconds": derive_timeout(seconds) if override is None else override,
        "overridden": override is not None,
    }
    plan["preflight"][file] = record
    return record


def max_timeout(plan: dict) -> int:
    """The largest recorded per-file exec timeout, for go-mutesting's
    --exec-timeout. The pre-flight fails the run for any file it cannot
    record, and a focused rerun pre-flights only the files it mutates."""
    if not plan["preflight"]:
        raise PlanError("no pre-flight record in the plan; run the clean-code pre-flight first")
    return max(int(r["timeout_seconds"]) for r in plan["preflight"].values())


def rerun_scope(plan: dict, ids: list[str], hints: list[Path]) -> list[tuple[str, str, str]]:
    """(id, group, file) per mutant id. The stable id hashes the file, so a
    file recorded for the id in the baseline or a previous report lets the
    rerun pre-flight and mutate that file alone; an id without a hint (or with
    a file the plan no longer targets) is rerun across every group ("-")."""
    known: dict[str, str] = {}
    for hint in hints:
        if hint.exists():
            for mutant in read_baseline(hint)["mutants"]:
                known.setdefault(mutant["id"], mutant["file"])
    group_of = {file: str(n) for n, group in enumerate(plan["groups"]) for file in group["files"]}
    scope = []
    for mutant in ids:
        file = known.get(mutant, "")
        scope.append((mutant, group_of[file], file) if file in group_of else (mutant, "-", "-"))
    return scope


# ---------------------------------------------------------------------------
# Reruns and baseline


def git(*args: str) -> str:
    return subprocess.run(["git", "-C", str(REPO_ROOT), *args], capture_output=True, text=True, check=True).stdout.strip()


def summary_status(summary: dict) -> str:
    """The status of the mutant id a --run-mutant-id run executed.

    The stable id hashes only the file, the mutator, and the changed text, so
    one id can name the same change at several places. go-mutesting's baseline
    suppresses an id when any of its mutants escapes, so the id counts as
    escaped when at least one did; otherwise all must agree."""
    stats = summary.get("stats", summary)
    counts = {
        "killed": stats.get("killedCount", 0),
        "escaped": stats.get("escapedCount", 0),
        "skipped": stats.get("skippedCount", 0),
        "errored": stats.get("errorCount", 0),
    }
    if counts["escaped"]:
        return "escaped"
    hit = [status for status, count in counts.items() if count]
    if len(hit) != 1:
        raise PlanError(f"expected the rerun to execute the mutant with one status, got {counts}")
    return hit[0]


def append_rerun(
    reruns: Path, mutant_id: str, status: str, digest: str, commit: str = "", now: datetime.datetime | None = None
) -> dict:
    """Append one rerun outcome. `digest` (of the plan inputs) decides
    freshness; `commit` is information only."""
    record = {
        "id": mutant_id,
        "status": status,
        "timestamp": (now or datetime.datetime.now(datetime.UTC)).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "digest": digest,
    }
    if commit:
        record["commit"] = commit
    reruns.parent.mkdir(parents=True, exist_ok=True)
    with reruns.open("a") as handle:
        handle.write(json.dumps(record, sort_keys=True) + "\n")
    return record


def read_baseline(path: Path) -> dict:
    data = json.loads(path.read_text())
    data["mutants"] = data.get("mutants") or []
    return data


def group_dirs(report_dir: Path) -> list[Path]:
    dirs = sorted(report_dir.glob("group-*"), key=lambda d: int(d.name.split("-")[1]))
    if not dirs:
        raise PlanError(f"{report_dir}: no group-* report directories")
    return dirs


def merge_baselines(report_dir: Path, out: Path, digest: str) -> int:
    """Merge the per-group baselines go-mutesting wrote into one file, stamped
    with the digest of the plan inputs it was generated from (rerun evidence
    counts only at the same digest)."""
    mutants = []
    for group in group_dirs(report_dir):
        path = group / "baseline.json"
        if not path.exists():
            raise PlanError(f"{path}: go-mutesting wrote no baseline for this group")
        mutants += read_baseline(path)["mutants"]
    mutants.sort(key=lambda m: (m["file"], m["line"], m["mutator"], m["id"]))
    out.write_text(json.dumps({"version": 1, "digest": digest, "mutants": mutants}, indent=2) + "\n")
    return len(mutants)


STAT_KEYS = ("killedCount", "escapedCount", "errorCount", "skippedCount", "notCoveredCount")
TIMEOUT_KILL_RE = re.compile(r"^formal-mutation: timeout-kill file=(\S+) ", re.M)


def merge_reports(report_dir: Path, log: str) -> dict:
    """One summary, agentic report, full report, and per-file table for all
    groups of a run. MSI is recomputed as go-mutesting does:
    (killed + errored + skipped) / total, as a percentage."""
    stats = dict.fromkeys(STAT_KEYS, 0)
    agentic: list[dict] = []
    lists: dict[str, list[dict]] = {name: [] for name in RERUN_STATUSES}
    for group in group_dirs(report_dir):
        summary = group / "go-mutesting-summary.json"
        if summary.exists():
            data = json.loads(summary.read_text())
            for key in STAT_KEYS:
                stats[key] += data.get(key, 0)
        if (group / "go-mutesting-agentic.json").exists():
            agentic += json.loads((group / "go-mutesting-agentic.json").read_text()).get("mutants") or []
        if (group / "report.json").exists():
            full = json.loads((group / "report.json").read_text())
            for name in RERUN_STATUSES:
                lists[name] += full.get(name) or []
    total = sum(stats.values())
    stats["totalMutantsCount"] = total
    stats["msi"] = round(100 * (stats["killedCount"] + stats["errorCount"] + stats["skippedCount"]) / total, 2) if total else 0.0
    (report_dir / "go-mutesting-summary.json").write_text(json.dumps(stats) + "\n")
    (report_dir / "go-mutesting-agentic.json").write_text(
        json.dumps({"msi": stats["msi"], "escaped_count": len(agentic), "mutants": agentic}, indent=2) + "\n"
    )
    (report_dir / "report.json").write_text(json.dumps({"stats": stats, **lists}) + "\n")

    root = str(REPO_ROOT) + "/"
    columns = [*RERUN_STATUSES, "timeout-kill"]
    per_file: dict[str, dict[str, int]] = {}
    for name, mutants in lists.items():
        for mutant in mutants:
            file = mutant["mutator"]["originalFilePath"].removeprefix(root)
            per_file.setdefault(file, dict.fromkeys(columns, 0))[name] += 1
    timeout_kills = TIMEOUT_KILL_RE.findall(log)
    for file in timeout_kills:
        per_file.setdefault(file, dict.fromkeys(columns, 0))["timeout-kill"] += 1
    with (report_dir / "run-metadata.txt").open("a") as metadata:
        for file in sorted(set(timeout_kills)):
            metadata.write(f"timeout_kills[{file}]={timeout_kills.count(file)}\n")
    lines = ["file\t" + "\t".join(columns)]
    lines += [f"{file}\t" + "\t".join(str(counts[c]) for c in columns) for file, counts in sorted(per_file.items())]
    (report_dir / "per-file.tsv").write_text("\n".join(lines) + "\n")
    return stats


# ---------------------------------------------------------------------------
# Triage ledger


def collapse(text: str) -> str:
    return " ".join(text.split())


def read_reruns(path: Path) -> list[dict]:
    if not path.exists():
        return []
    records = []
    for number, raw in enumerate(path.read_text().splitlines(), 1):
        if not raw.strip():
            continue
        record = json.loads(raw)
        if not RERUN_REQUIRED_KEYS <= set(record) <= RERUN_REQUIRED_KEYS | {"commit"} or record["status"] not in RERUN_STATUSES:
            raise PlanError(f"{path}:{number}: malformed rerun record {raw!r}")
        records.append(record)
    return records


def check_unplanned(
    rows: list[Row], survivors: list[dict], triage: dict[str, list], root: Path
) -> tuple[list[str], list[str]]:
    """The `[triage]` lists (harden-formal-instruments design D13): every
    no-binding row (a binding gap) and every ledger survivor whose row is
    unbound for any reason (an orphan, matched on (model, element, symbol) like
    the ledger check) must be listed, and every listed entry must still apply.
    Rows unbound by design (no-symbol, type-symbol, trace-only,
    characterisation-only) are derived and reported, never listed. Returns the
    failures and the report lines."""
    declared, _files = test_declarations(root)
    classified = classify_rows(rows, root, declared)
    unbound = classified.reasons
    gaps = {key for key, reason in unbound.items() if reason == REASON_NO_BINDING}
    row_keys = {r.key for r in rows}
    listed_gaps = {(r["model"], r["element"], r["symbol"]) for r in triage.get("binding_gaps", [])}
    listed_ids = set(triage.get("orphaned_survivors", []))
    survivor_rows = {s.get("id", ""): (s.get("model"), s.get("element"), s.get("symbol")) for s in survivors}
    orphans = {mutant for mutant, key in survivor_rows.items() if key in unbound}

    # A row the plan cannot classify (a receiver mismatch, an ambiguous
    # symbol) is neither bound nor unbound, so it must fail here too.
    failures = [f"row classification: {problem}" for problem in classified.problems]
    for key in sorted(gaps - listed_gaps):
        failures.append(f"[triage] binding_gaps: new binding gap {key[0]}: {key[1]!r} ({key[2]}) is not listed; "
                        "the list only shrinks: bind the row instead")
    for key in sorted(listed_gaps - gaps):
        if key not in row_keys:
            why = "names no correspondence row"
        elif key in unbound:
            why = f"is unbound by design ({unbound[key]}), which is derived, not listed; remove it"
        else:
            why = "is bound now; remove it"
        failures.append(f"[triage] binding_gaps: stale entry {key[0]}: {key[1]!r} ({key[2]}) {why}")
    for mutant in sorted(orphans - listed_ids):
        key = survivor_rows[mutant]
        failures.append(f"[triage] orphaned_survivors: survivor {mutant} names unbound row {key[0]}: {key[1]!r} ({key[2]}) "
                        "and is not listed")
    for mutant in sorted(listed_ids - orphans):
        why = "is not in the ledger" if mutant not in survivor_rows else "names a bound row now; remove it"
        failures.append(f"[triage] orphaned_survivors: stale entry {mutant} {why}")

    report = [f"binding gap {m}: {e!r} ({sym})" for m, e, sym in sorted(listed_gaps)]
    by_row: dict[tuple, int] = {}
    for mutant in listed_ids:
        by_row[survivor_rows.get(mutant)] = by_row.get(survivor_rows.get(mutant), 0) + 1
    report += [f"orphaned survivors of {k[0]}: {k[1]!r} ({k[2]}): {n}" for k, n in sorted(by_row.items(), key=str) if k]
    report += [f"by design {m}: {e!r} ({sym}, {reason})"
               for (m, e, sym), reason in sorted(unbound.items()) if reason != REASON_NO_BINDING]
    return failures, report


# The lists that may only shrink, by the manifest table that holds them.
GROWTH_LISTS = (("correspondence", "unresolved_elements"), ("triage", "binding_gaps"), ("triage", "orphaned_survivors"))
UNRESOLVED_LIST, GAPS_LIST, ORPHANS_LIST = (f"[{table}] {key}" for table, key in GROWTH_LISTS)

# The variable that names the growth base when --base is not given
# (harden-formal-instruments decision 22); scripts/ci-triage-base.sh resolves
# it in CI. An empty value counts as unset.
TRIAGE_BASE_ENV = "INVOWK_TRIAGE_BASE"
BASE_HINT = f"set {TRIAGE_BASE_ENV}=<sha> or pass --base <sha>"


def shrink_lists(data: dict) -> dict[str, set | None]:
    """The shrink-only lists of a parsed manifest, keyed `[table] key`; None
    for a list the manifest does not have. It reads only these keys, so a base
    whose tables carry other keys still yields its lists; the head manifest is
    validated by load_manifest."""
    out: dict[str, set | None] = {}
    for table, key in GROWTH_LISTS:
        values = data.get(table, {}).get(key)
        if not isinstance(values, list):
            out[f"[{table}] {key}"] = None
            continue
        out[f"[{table}] {key}"] = {
            (v.get("model"), v.get("element"), v.get("symbol")) if isinstance(v, dict) else v for v in values
        }
    return out


def seeds_lists(base: dict) -> bool:
    """Whether a base manifest has none of the shrink-only lists: the seeding
    change (decision 19)."""
    return all(entries is None for entries in shrink_lists(base).values())


def entry_label(entry) -> str:
    return f"{entry[0]}: {entry[1]!r} ({entry[2]})" if isinstance(entry, tuple) else repr(entry)


def check_growth(
    head: dict, base: dict, base_name: str, why: Callable[[str, object], str] | None = None
) -> tuple[list[str], list[str]]:
    """Fail on every shrink-only list entry that the base manifest does not
    list (harden-formal-instruments decision 15). A base with none of the
    lists is the seeding change and passes with a notice per list; a base with
    only some of them fails for each one it lacks, so renaming a key cannot
    reopen seeding (decision 19). An entry that left a list is a notice, with
    the reason `why(list, entry)` gives (decision 18). Returns the failures and
    the notices."""
    head_lists, base_lists = shrink_lists(head), shrink_lists(base)
    failures, notices = [], []
    present = [name for name, entries in base_lists.items() if entries is not None]
    if seeds_lists(base):
        return [], [f"{name}: not in the base {base_name} manifest; this change seeds it" for name in head_lists]
    for name, entries in head_lists.items():
        at_base = base_lists[name]
        if at_base is None:
            failures.append(f"{name}: not in the base {base_name} manifest, which has {', '.join(present)}; "
                            "a list is seeded only from a base with none of the three")
            continue
        for entry in sorted((entries or set()) - at_base, key=str):
            failures.append(f"{name}: {entry_label(entry)} is not listed at the base {base_name}; "
                            "the list only shrinks: fix the row or survivor instead of listing it")
        for entry in sorted(at_base - (entries or set()), key=str):
            reason = why(name, entry) if why else "no longer listed"
            notices.append(f"{name}: {entry_label(entry)} left the list since the base {base_name}: {reason}")
    return failures, notices


def departure_reasons(rows: list[Row], survivors: list[dict], root: Path) -> Callable[[str, object], str]:
    """Why an entry left a shrink-only list (decision 18), for check_growth.
    The row classification is computed on the first call only: entries rarely
    leave, and most runs never call it."""
    state: dict = {}

    def status() -> dict:
        if not state:
            declared, _files = test_declarations(root)
            state["unbound"] = classify_rows(rows, root, declared).reasons
            state["rows"] = {r.key for r in rows}
            state["elements"] = {f"{r.model}: {r.element}" for r in rows}
            state["survivors"] = {s.get("id", ""): (s.get("model"), s.get("element"), s.get("symbol")) for s in survivors}
        return state

    def why(name: str, entry) -> str:
        s = status()
        if name == UNRESOLVED_LIST:
            return "the element resolves now" if entry in s["elements"] else "the row was deleted"
        key = entry if name == GAPS_LIST else s["survivors"].get(entry)
        if key is None:
            return "the survivor left the ledger"
        if key not in s["rows"]:
            return "the row was deleted"
        reason = s["unbound"].get(key)
        if reason is None:
            return "the row is bound now"
        if reason != REASON_NO_BINDING and name == GAPS_LIST:
            return f"the row was reclassified as {reason}"
        return f"the row is still unbound ({reason})"

    return why


def base_rows(base: dict, read: Callable[[str], bytes]) -> tuple[list[Row], Classified]:
    """The correspondence rows of the base manifest's models, read at the base,
    and their classification with the base's Go sources (decision 28). A file
    the base lacks reads as empty: its rows cannot have been bound there."""
    def text(path: str) -> str | None:
        try:
            return read(path).decode()
        except (subprocess.CalledProcessError, OSError, UnicodeDecodeError):
            return None

    rows = []
    for model in base.get("model", []):
        source = text(model.get("file", "")) if isinstance(model, dict) else None
        if source is not None:
            rows += [Row(model.get("name", ""), *cells) for cells in formal.correspondence_rows(source)]
    return rows, classify_rows(rows, REPO_ROOT, {}, read=lambda file: text(file) or "")


def check_base_rows(
    rows: list[Row], head: Classified, at_base: list[Row], base: Classified, unresolved: Iterable[str],
    base_name: str, seeding: bool,
) -> tuple[list[str], list[str]]:
    """Decision 28: a row bound at the base must not become unbound by design
    (no-symbol, type-symbol, trace-only, characterisation-only), matched on the
    row or, when its symbol changed, on its model and element; a new row must
    not join a listed unresolved pair. Deletions and re-points are notices. A
    demotion to no-binding is the binding_gaps growth check's. Under seeding
    (a base without the lists) every failure is a notice. Returns the failures
    and the notices."""
    unbound = head.reasons
    keys = [r.key for r in rows]
    by_pair: dict[tuple[str, str], list[tuple[str, str, str]]] = {}
    for key in keys:
        by_pair.setdefault(key[:2], []).append(key)
    failures, notices = [], []
    for key in sorted(base.bound - head.bound):
        label = entry_label(key)
        same = by_pair.get(key[:2], [])
        if not same:
            notices.append(f"row {label}, bound at the base {base_name}, was deleted")
            continue
        bound_now = [k for k in same if k in head.bound]
        if bound_now:
            notices.append(f"row {label}, bound at the base {base_name}, was re-pointed to {', '.join(k[2] for k in bound_now)}")
            continue
        reasons = sorted({unbound[k] for k in same if k in unbound} - {REASON_NO_BINDING})
        if reasons:
            failures.append(f"row {label} was bound at the base {base_name} and is unbound by design now "
                            f"({', '.join(reasons)}); bind it again or delete the row")
    base_keys = {r.key for r in at_base}
    pairs = set(unresolved)
    for key in sorted(set(keys) - base_keys):
        if f"{key[0]}: {key[1]}" in pairs:
            failures.append(f"{UNRESOLVED_LIST}: row {entry_label(key)} is not in the base {base_name}'s tables and "
                            "joins a listed pair; the list only shrinks: name an element that exists in the model")
    if seeding:
        return [], notices + [f"{failure} (reported only: the base {base_name} seeds the lists)" for failure in failures]
    return failures, notices


def resolve_base(arg: str, environ: dict[str, str] | None = None, run: Callable[..., str] | None = None) -> str:
    """The growth base: --base, else a non-empty INVOWK_TRIAGE_BASE, else the
    merge-base of HEAD and origin/main (decisions 20 and 22)."""
    environ = os.environ if environ is None else environ
    return arg or environ.get(TRIAGE_BASE_ENV, "").strip() or merge_base(run)


def merge_base(run: Callable[..., str] | None = None) -> str:
    """The default growth base: the merge-base of HEAD and origin/main."""
    try:
        return (run or git)("merge-base", "HEAD", "origin/main")
    except (subprocess.CalledProcessError, OSError) as err:
        detail = getattr(err, "stderr", "") or err
        raise PlanError(f"triage --check: cannot determine the base (git merge-base HEAD origin/main failed: "
                        f"{str(detail).strip()}); fetch origin/main, {BASE_HINT}") from err


def base_manifest(base: str, read: Callable[[str], bytes] | None = None) -> dict:
    """The manifest at the growth base, parsed; fails closed when unreadable."""
    path = str(formal.MANIFEST.relative_to(REPO_ROOT))
    try:
        text = (read or git_reader(base))(path)
    except (subprocess.CalledProcessError, OSError) as err:
        detail = getattr(err, "stderr", b"") or err
        if isinstance(detail, bytes):
            detail = detail.decode(errors="replace")
        raise PlanError(f"triage --check: cannot read {path} at the base {base} (git show {base}:{path} failed: "
                        f"{str(detail).strip()}); fetch the base commit, {BASE_HINT}") from err
    try:
        return tomllib.loads(text.decode())
    except (UnicodeDecodeError, tomllib.TOMLDecodeError) as err:
        raise PlanError(f"triage --check: {path} at the base {base} is not valid TOML: {err}") from err


def check_triage(
    baseline: dict,
    ledger: dict,
    reruns: list[dict],
    models: list[formal.Model],
    root: Path = REPO_ROOT,
    triage: dict[str, list] | None = None,
    report: list[str] | None = None,
) -> list[str]:
    """Every fail-closed condition of the triage ledger (design D6) and of the
    `[triage]` lists (design D13 of harden-formal-instruments). `triage`
    defaults to the repository manifest's table for the repository root and to
    empty lists for any other root; the listed entries are appended to
    `report` when given."""
    failures = []
    unknown_tables = set(ledger) - {"survivor", "closed"}
    if unknown_tables:
        failures.append(f"ledger: unknown table(s) {', '.join(sorted(unknown_tables))}")
    survivors = ledger.get("survivor", [])
    closed = ledger.get("closed", [])
    baseline_ids = {m["id"] for m in baseline["mutants"]}
    ledger_ids = [s.get("id", "") for s in survivors]
    if len(ledger_ids) != len(set(ledger_ids)):
        failures.append("ledger: duplicate survivor id")
    for missing in sorted(baseline_ids - set(ledger_ids)):
        failures.append(f"baseline id {missing} has no [[survivor]] entry in the ledger")
    for extra in sorted(set(ledger_ids) - baseline_ids):
        failures.append(f"ledger survivor {extra} is not in the baseline")

    rows = model_rows(models, root)
    if triage is None:
        triage = formal.load_triage() if root == REPO_ROOT else {}
    unplanned_failures, listed = check_unplanned(rows, survivors, triage, root)
    failures += unplanned_failures
    if report is not None:
        report += listed
    findings = {cmd.finding for model in models for cmd in model.commands if cmd.finding}
    by_model = {model.name: model for model in models}
    for survivor in survivors:
        mutant = survivor.get("id", "?")
        keys = set(survivor)
        if keys - SURVIVOR_KEYS or (SURVIVOR_KEYS - {"follow_up"}) - keys:
            failures.append(f"survivor {mutant}: fields must be {sorted(SURVIVOR_KEYS)} (follow_up optional)")
            continue
        cls = survivor["class"]
        follow_up = survivor.get("follow_up", "").strip()
        if cls not in TRIAGE_CLASSES:
            failures.append(f"survivor {mutant}: unknown class {cls!r}")
        if not survivor["reason"].strip():
            failures.append(f"survivor {mutant}: empty reason")
        row = next((r for r in rows if (r.model, r.element, r.symbol) ==
                    (survivor["model"], survivor["element"], survivor["symbol"])), None)
        if row is None:
            failures.append(f"survivor {mutant}: no {survivor['model']} row {survivor['element']!r} names {survivor['symbol']}")
        elif row.file != survivor["file"]:
            failures.append(f"survivor {mutant}: {survivor['symbol']} is tabled in {row.file}, not {survivor['file']}")
        if cls in FOLLOW_UP_CLASSES and not follow_up:
            failures.append(f"survivor {mutant}: class {cls} needs a follow_up")
        if cls == "defect" and follow_up and follow_up not in findings:
            failures.append(f"survivor {mutant}: follow_up {follow_up!r} is not a `finding` command in the manifest")
        if cls == "abstraction" and row is not None and collapse(survivor["reason"]) not in collapse(row.abstraction):
            failures.append(
                f"survivor {mutant}: abstraction reason {survivor['reason']!r} is not stated in the abstraction cell "
                f"of {row.model} row {row.element!r} ({row.abstraction!r})"
            )

    stamp = baseline.get("digest", "")
    if baseline_ids and not stamp:
        failures.append("baseline: no input digest; rerun `make mutation-formal-baseline-update`")
    by_id: dict[str, list[dict]] = {}
    for record in reruns:
        by_id.setdefault(record["id"], []).append(record)
    for mutant in sorted(baseline_ids):
        evidence = [r for r in by_id.get(mutant, []) if stamp and r["digest"] == stamp]
        escaped = sum(1 for r in evidence if r["status"] == "escaped")
        if escaped < REQUIRED_ESCAPED_RERUNS:
            failures.append(
                f"baseline id {mutant}: {escaped} recorded escaped rerun(s) at the baseline's input digest, "
                f"need {REQUIRED_ESCAPED_RERUNS} (make mutation-formal-rerun MUTATION_MUTANT_ID={mutant})"
            )
        flaky = sorted({r["status"] for r in evidence} - {"escaped"})
        if flaky:
            failures.append(f"baseline id {mutant}: inconsistent rerun evidence ({', '.join(flaky)} as well as escaped)")

    for entry in closed:
        mutant = entry.get("id", "?")
        if set(entry) != CLOSED_KEYS:
            failures.append(f"closed {mutant}: fields must be {sorted(CLOSED_KEYS)}")
            continue
        model = by_model.get(entry["model"])
        if model is None:
            failures.append(f"closed {mutant}: unknown model {entry['model']!r}")
        elif mutant not in model.calibration:
            failures.append(f"closed {mutant}: not recorded in {model.name}'s calibration text")
        if mutant in baseline_ids:
            failures.append(f"closed {mutant}: still in the baseline")
    return failures


# ---------------------------------------------------------------------------
# Binding reach gate


def rapid_env(script: Path = MUTATION_SH, environ: dict[str, str] | None = None) -> dict[str, str]:
    """RAPID_SEED, RAPID_NOFAILFILE and RAPID_SHRINKTIME exactly as
    scripts/mutation.sh exports them: its defaults unless the caller set them."""
    environ = os.environ if environ is None else environ
    source = script.read_text()
    pins = {}
    for name in ("RAPID_SEED", "RAPID_SHRINKTIME"):
        match = re.search(rf"^readonly MUTATION_{name}=(\S+)$", source, re.M)
        if match is None:
            raise PlanError(f"{script}: no `readonly MUTATION_{name}=` pin")
        pins[name] = environ.get(name) or match[1]
    pins["RAPID_NOFAILFILE"] = "1"
    return pins


class Block(NamedTuple):
    start: int
    end: int
    statements: int
    count: int


def parse_profile(text: str) -> dict[str, list[Block]]:
    """`go test -coverprofile` blocks per file (import path + "/" + name).
    A block listed several times (one per -coverpkg build) keeps its highest
    count."""
    lines = text.splitlines()
    if not lines or not lines[0].startswith("mode:"):
        raise PlanError("garbled coverage profile: the first line is not `mode: ...`")
    merged: dict[tuple[str, str], Block] = {}
    for number, raw in enumerate(lines[1:], 2):
        if not raw.strip():
            continue
        match = re.fullmatch(r"(?P<file>.+):(?P<a>\d+)\.\d+,(?P<b>\d+)\.\d+ (?P<n>\d+) (?P<c>\d+)", raw)
        if match is None:
            raise PlanError(f"garbled coverage profile: line {number} {raw!r}")
        block = Block(int(match["a"]), int(match["b"]), int(match["n"]), int(match["c"]))
        key = (match["file"], raw.rsplit(" ", 2)[0])
        if key not in merged or block.count > merged[key].count:
            merged[key] = block
    blocks: dict[str, list[Block]] = {}
    for (file, _loc), block in merged.items():
        blocks.setdefault(file, []).append(block)
    return blocks


class KillerRun(NamedTuple):
    status: int
    output: str
    profile: str | None
    seconds: float


def reach_targets(plan: dict) -> dict[str, list[tuple[dict, dict]]]:
    """Killer -> every (function, row) it must reach."""
    targets: dict[str, list[tuple[dict, dict]]] = {}
    for function in plan["functions"]:
        for row in function["rows"]:
            for killer in row["killers"]:
                targets.setdefault(killer, []).append((function, row))
    return targets


def run_killer(plan: dict, killer: str, coverpkg: list[str], out: Path, env: dict[str, str]) -> KillerRun:
    profile = out / f"{killer}.cover"
    profile.unlink(missing_ok=True)
    go = os.environ.get("GO_CMD", "go")
    command = [go, "test", "-count=1", "-json", "-covermode=set", f"-coverpkg={','.join(coverpkg)}",
               f"-coverprofile={profile}", "-run", f"^{killer}$", plan["killer_packages"][killer]]
    started = time.monotonic()
    result = subprocess.run(command, cwd=plan["root"], capture_output=True, text=True, check=False,
                            env=os.environ | env)
    seconds = time.monotonic() - started
    (out / f"{killer}.json").write_text(result.stdout + result.stderr)
    return KillerRun(result.returncode, result.stdout, profile.read_text() if profile.exists() else None, seconds)


def function_profile_file(function: dict) -> str:
    return f"{function['package']}/{Path(function['file']).name}"


def check_reach(plan: dict, runs: dict[str, KillerRun]) -> tuple[list[dict], list[str]]:
    """Each (row, killer, function) triple with its covered/total statements,
    and every failure (design D1, step 3 and 4)."""
    triples = []
    failures = []
    for killer, pairs in sorted(reach_targets(plan).items()):
        rows = "; ".join(sorted({f"{r['model']}: row {r['element']!r}" for _f, r in pairs}))
        run = runs.get(killer)
        problem = None
        blocks: dict[str, list[Block]] = {}
        if run is None:
            problem = "was not run"
        else:
            outcome = test_outcomes(run.output).get(killer, "not run")
            if outcome != "pass":
                problem = f"{outcome} (exit {run.status})"
            elif run.status != 0:
                problem = f"exited {run.status} although the test passed (build or package failure)"
            elif run.profile is None:
                problem = "wrote no coverage profile"
            else:
                try:
                    blocks = parse_profile(run.profile)
                except PlanError as err:
                    problem = str(err)
        if problem is not None:
            failures.append(f"killer {killer} ({rows}): {problem}")
            continue
        for function, row in pairs:
            where = f"{row['model']}: row {row['element']!r}: killer {killer} -> {function['symbol']} ({function['file']})"
            file_blocks = blocks.get(function_profile_file(function))
            triple = {"model": row["model"], "element": row["element"], "killer": killer,
                      "symbol": function["symbol"], "file": function["file"], "covered": 0, "total": 0}
            if file_blocks is None:
                failures.append(f"{where}: not instrumented (the profile has no blocks for the file)")
                triples.append(triple)
                continue
            inside = [b for b in file_blocks if function["start"] <= b.start and b.end <= function["end"]]
            triple["total"] = sum(b.statements for b in inside)
            triple["covered"] = sum(b.statements for b in inside if b.count > 0)
            triples.append(triple)
            if not inside:
                failures.append(f"{where}: no coverage blocks in lines {function['start']}-{function['end']}")
            elif not any(b.count > 0 for b in inside):
                failures.append(f"{where}: executed 0 of {triple['total']} statements")
    return triples, failures


def reach_coverpkg(plan: dict, killer: str) -> list[str]:
    """P_K: the import paths of the target files of the functions K kills for."""
    return sorted({function["package"] for function, _row in reach_targets(plan)[killer]})


def write_reach_report(plan: dict, triples: list[dict], failures: list[str], out: Path) -> None:
    report = {"triples": triples, "failures": failures, "unbound": plan["unbound"]}
    (out / "reach.json").write_text(json.dumps(report, indent=2) + "\n")
    lines = [f"{'UNREACHED' if not t['covered'] else 'reached'}\t{t['model']}: {t['element']}\t{t['killer']}\t"
             f"{t['symbol']} ({t['file']})\t{t['covered']}/{t['total']}" for t in triples]
    lines += [f"unbound\t{u['model']}: {u['element']}\t{u['reason']}\t{u['symbol'] or '-'}\t{u['abstraction']}"
              for u in plan["unbound"]]
    lines += [f"FAILED\t{failure}" for failure in failures]
    (out / "reach.txt").write_text("\n".join(lines) + "\n")


def reach(
    plan: dict,
    out: Path,
    runner: Callable[[dict, str, list[str], Path, dict[str, str]], KillerRun] = run_killer,
    env: dict[str, str] | None = None,
) -> tuple[list[dict], list[str], float]:
    """Run every killer alone with coverage, in parallel, and check reach."""
    out.mkdir(parents=True, exist_ok=True)
    env = rapid_env() if env is None else env
    killers = sorted(reach_targets(plan))
    started = time.monotonic()
    with concurrent.futures.ThreadPoolExecutor(max_workers=formal.parallel_jobs()) as pool:
        futures = {k: pool.submit(runner, plan, k, reach_coverpkg(plan, k), out, env) for k in killers}
        runs = {k: f.result() for k, f in futures.items()}
    seconds = time.monotonic() - started
    triples, failures = check_reach(plan, runs)
    write_reach_report(plan, triples, failures, out)
    return triples, failures, seconds


# ---------------------------------------------------------------------------
# CLI


def reach_main(out: Path) -> int:
    """`reach`: build the plan (every fail-closed check stops the gate before
    any killer runs), run the gate, report, and time it against `[reach]`."""
    if isinstance(sys.stdout, io.TextIOWrapper):
        sys.stdout.reconfigure(line_buffering=True)
    _tools, models = formal.load_manifest()
    budget = formal.load_reach()
    formal.validate_manifest(models, formal.load_trace_suites(), budget)
    plan = build_plan(models)
    write_plan(plan, out)
    triples, failures, seconds = reach(plan, out)
    reached = sum(1 for t in triples if t["covered"])
    print(f"binding reach: {reached} of {len(triples)} (row, killer, function) triples reached, "
          f"{len(reach_targets(plan))} killers, {len(plan['unbound'])} unbound rows -> {out / 'reach.txt'}")
    for unbound in plan["unbound"]:
        if unbound["reason"] == REASON_NO_BINDING:
            print(f"  not gated: {unbound['model']}: row {unbound['element']!r} ({unbound['symbol']}): no-binding")
    if budget is not None:
        formal.report_timings({formal.REACH_LABEL: seconds}, {formal.REACH_LABEL: (budget.budget_seconds, budget.budget_source)})
    else:
        print(f"timing {formal.REACH_LABEL} {seconds:.1f}s (no [reach] budget)")
    if failures:
        print("binding reach FAILED:")
        for failure in failures:
            print(f"  - {failure}")
        return 1
    print("binding reach passed")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="action", required=True)
    p = sub.add_parser("plan")
    p.add_argument("--out", type=Path, required=True)
    p = sub.add_parser("show")
    p.add_argument("--plan", type=Path, required=True)
    p.add_argument("--group", type=int)
    p.add_argument("field", choices=["match", "files", "group-count", "max-timeout"])
    p = sub.add_parser("exec-args")
    p.add_argument("--plan", type=Path, required=True)
    p.add_argument("--original", required=True)
    p.add_argument("--changed", required=True)
    p.add_argument("--overlay-out", type=Path)
    p = sub.add_parser("preflight-record")
    p.add_argument("--plan", type=Path, required=True)
    p.add_argument("--file", required=True)
    p.add_argument("--json", type=Path, required=True)
    p.add_argument("--status", type=int, required=True)
    p.add_argument("--seconds", type=float, required=True)
    p = sub.add_parser("rerun-record")
    p.add_argument("--plan", type=Path, required=True)
    p.add_argument("--summary", type=Path, required=True)
    p.add_argument("--id", required=True)
    p.add_argument("--reruns", type=Path, default=RERUNS)
    p = sub.add_parser("rerun-scope")
    p.add_argument("--plan", type=Path, required=True)
    p.add_argument("--hint", type=Path, action="append", default=[])
    p.add_argument("ids", nargs="+")
    p = sub.add_parser("merge-reports")
    p.add_argument("--report-dir", type=Path, required=True)
    p = sub.add_parser("merge-baselines")
    p.add_argument("--report-dir", type=Path, required=True)
    p.add_argument("--baseline", type=Path, default=BASELINE)
    p = sub.add_parser("triage")
    p.add_argument("--check", action="store_true", required=True)
    p.add_argument("--baseline", type=Path, default=BASELINE)
    p.add_argument("--ledger", type=Path, default=LEDGER)
    p.add_argument("--reruns", type=Path, default=RERUNS)
    p.add_argument("--base", default="",
                   help=f"commit whose manifest the shrink-only lists may not outgrow (default: ${TRIAGE_BASE_ENV}, "
                        "else git merge-base HEAD origin/main)")
    p = sub.add_parser("reach")
    p.add_argument("--out", type=Path, default=REACH_DIR)
    args = parser.parse_args(argv)

    try:
        if args.action == "reach":
            return reach_main(args.out)
        if args.action == "plan":
            _tools, models = formal.load_manifest()
            plan = build_plan(models)
            path = write_plan(plan, args.out)
            print(f"formal-bindings plan: {len(plan['functions'])} functions in {len(plan['files'])} files, "
                  f"{len(plan['unbound'])} unbound rows -> {path}")
        elif args.action == "show":
            plan = load_plan(args.plan)
            if args.field == "group-count":
                print(len(plan["groups"]))
            elif args.field == "max-timeout":
                print(max_timeout(plan))
            elif args.group is None:
                if args.field == "match":
                    raise PlanError("show match needs --group: each group has its own --match")
                print("\n".join(plan["files"]))
            else:
                if not 0 <= args.group < len(plan["groups"]):
                    raise PlanError(f"no group {args.group} in the plan")
                group = plan["groups"][args.group]
                print(group["match"] if args.field == "match" else "\n".join(group["files"]))
        elif args.action == "exec-args":
            plan = load_plan(args.plan)
            result = exec_args(plan, args.original, args.changed)
            if args.overlay_out and not result["preflight"]:
                write_overlay(args.original, args.changed, args.overlay_out)
            print(f"file={result['file']}")
            print(f"function={result['function']}")
            print(f"timeout={result['timeout']}")
            print(f"run={result['run']}")
            print(f"packages={' '.join(result['packages'])}")
        elif args.action == "preflight-record":
            plan = load_plan(args.plan)
            record = preflight_record(plan, args.file, args.json.read_text(), args.status, args.seconds)
            args.plan.write_text(json.dumps(plan, indent=2) + "\n")
            print(f"pre-flight {args.file}: clean {record['clean_seconds']}s, exec timeout {record['timeout_seconds']}s")
        elif args.action == "rerun-record":
            status = summary_status(json.loads(args.summary.read_text()))
            digest = plan_digest(load_plan(args.plan))
            record = append_rerun(args.reruns, args.id, status, digest, git("rev-parse", "HEAD"))
            print(f"rerun evidence: {record['id']} {record['status']} at inputs {digest[:12]}")
        elif args.action == "rerun-scope":
            for mutant, group, file in rerun_scope(load_plan(args.plan), args.ids, args.hint):
                print(f"{mutant}\t{group}\t{file}")
        elif args.action == "merge-reports":
            log = args.report_dir / "go-mutesting.log"
            stats = merge_reports(args.report_dir, log.read_text() if log.exists() else "")
            print(f"formal-bindings: {stats['totalMutantsCount']} mutants: {stats['killedCount']} killed, "
                  f"{stats['escapedCount']} escaped, {stats['skippedCount']} skipped, {stats['errorCount']} errored "
                  f"(MSI {stats['msi']}%); per file: {args.report_dir / 'per-file.tsv'}")
        elif args.action == "merge-baselines":
            count = merge_baselines(args.report_dir, args.baseline, plan_digest(load_plan(args.report_dir / PLAN_FILE)))
            print(f"formal-bindings baseline: {count} surviving mutant(s) -> {args.baseline}")
        elif args.action == "triage":
            _tools, models = formal.load_manifest()
            with args.ledger.open("rb") as handle:
                ledger = tomllib.load(handle)
            listed: list[str] = []
            failures = check_triage(read_baseline(args.baseline), ledger, read_reruns(args.reruns), models, report=listed)
            base = resolve_base(args.base)
            with formal.MANIFEST.open("rb") as handle:
                head = tomllib.load(handle)
            rows = model_rows(models, REPO_ROOT)
            why = departure_reasons(rows, ledger.get("survivor", []), REPO_ROOT)
            at_base = base_manifest(base)
            growth, notices = check_growth(head, at_base, base, why)
            failures += growth
            base_table, base_classified = base_rows(at_base, git_reader(base))
            demoted, departed = check_base_rows(
                rows, classify_rows(rows, REPO_ROOT, {}), base_table, base_classified,
                head.get("correspondence", {}).get("unresolved_elements", []), base, seeds_lists(at_base))
            failures += demoted
            notices += departed
            for notice in notices:
                print(f"formal-bindings triage: {notice}")
            if listed:
                print("formal-bindings triage report: [triage] binding gaps and orphans (shrink-only), "
                      "then the derived by-design rows:")
                for line in listed:
                    print(f"  {line}")
            if failures:
                print("formal-bindings triage check FAILED:")
                for failure in failures:
                    print(f"  - {failure}")
                return 1
            print(f"formal-bindings triage check passed ({len(ledger.get('survivor', []))} survivors, "
                  f"{len(ledger.get('closed', []))} closed)")
        return 0
    except (PlanError, formal.FormalError) as err:
        print(f"formal-bindings: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
