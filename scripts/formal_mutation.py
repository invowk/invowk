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
    scripts/formal_mutation.py show --plan FILE [--group N] [--recorded] {match|files|group-count|max-timeout}
    scripts/formal_mutation.py exec-args --plan FILE --original PATH --changed PATH [--overlay-out FILE]
    scripts/formal_mutation.py preflight-record --plan FILE --file PATH --json FILE --status N --seconds S
    scripts/formal_mutation.py rerun-scope --plan FILE [--hint REPORT ...] ID ...
    scripts/formal_mutation.py rerun-record --summary FILE --id ID [--reruns FILE]
    scripts/formal_mutation.py merge-reports --report-dir DIR
    scripts/formal_mutation.py merge-baselines --report-dir DIR [--baseline FILE]
    scripts/formal_mutation.py triage --check [--baseline FILE] [--ledger FILE] [--reruns FILE]
"""

from __future__ import annotations

import argparse
import dataclasses
import datetime
import difflib
import json
import math
import os
import re
import subprocess
import sys
import tomllib
from collections.abc import Callable, Iterable
from pathlib import Path

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))

import formal  # noqa: E402 - sibling module, imported after the path fix

REPO_ROOT = formal.REPO_ROOT
PLAN_FILE = "formal-plan.json"
PLAN_VERSION = 1
BASELINE = REPO_ROOT / "tools" / "mutation" / "baselines" / "formal-bindings-baseline.json"
LEDGER = REPO_ROOT / "tools" / "mutation" / "triage" / "formal-bindings.toml"
RERUNS = REPO_ROOT / "tools" / "mutation" / "triage" / "formal-bindings-reruns.jsonl"

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
RERUN_STATUSES = ("killed", "escaped", "skipped", "errored")

# A gofmt'd top-level function declaration: optional receiver (named or not,
# pointer or not, generic or not), the name, optional type parameters, then `(`.
FUNC_DECL_RE = re.compile(
    r"^func (?:\((?:\w+\s+)?\*?(?P<recv>\w+)(?:\[[^\]]*\])?\)\s*)?(?P<name>\w+)\s*(?:\[[^\]]*\])?\(",
    re.M,
)


class PlanError(Exception):
    """A fail-closed plan, pre-flight, or triage condition."""


@dataclasses.dataclass(frozen=True)
class Row:
    model: str
    element: str
    symbol: str
    file: str
    binding: str
    abstraction: str

    @property
    def where(self) -> str:
        return f"{self.model}: row {self.element!r} ({self.symbol} in {self.file})"


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


def test_declarations(root: Path) -> dict[str, set[str]]:
    """Test name -> directories (relative, POSIX) whose test files declare it."""
    index: dict[str, set[str]] = {}
    for path in formal.go_test_files(root):
        rel_dir = path.parent.relative_to(root).as_posix()
        for test in formal.TEST_FUNC_RE.findall(path.read_text()):
            index.setdefault(test, set()).add(rel_dir)
    return index


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


def union_match(names: Iterable[str]) -> str:
    return "^(" + "|".join(sorted(set(names))) + ")$"


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
    return [{"match": union_match(leaf for f in group for leaf in leaves[f]), "files": group} for group in groups], problems


def build_plan(
    models: list[formal.Model],
    root: Path = REPO_ROOT,
    suites: list[formal.TraceSuite] | None = None,
    resolve_packages: Callable[[Iterable[str]], dict[str, str]] | None = None,
) -> dict:
    """The formal-bindings plan, or PlanError listing every fail-closed reason."""
    failures = formal.check_correspondence(models, root=root, suites=suites)
    if failures:
        raise PlanError("correspondence check failed:\n  - " + "\n  - ".join(failures))

    rows = model_rows(models, root)
    noted = formal.characterisation_tests([[r.element, r.symbol, r.file, r.binding, r.abstraction] for r in rows])
    declared = test_declarations(root)
    decls_by_file: dict[str, list[FuncDecl]] = {}
    functions: dict[tuple[str, str], dict] = {}
    unbound = []
    problems = []

    def skip(row: Row, reason: str) -> None:
        unbound.append({"model": row.model, "element": row.element, "symbol": row.symbol, "file": row.file, "reason": reason})

    for row in rows:
        if row.symbol in {"", "-"}:
            skip(row, REASON_NO_SYMBOL)
            continue
        tests = formal.binding_tests(row.binding)
        if not tests:
            skip(row, REASON_NO_BINDING)
            continue
        if row.file not in decls_by_file:
            decls_by_file[row.file] = function_decls((root / row.file).read_text())
        try:
            decl = find_function(decls_by_file[row.file], row.symbol)
        except PlanError as err:
            problems.append(f"{row.where}: {err}")
            continue
        if decl is None:
            # check_correspondence proved the symbol is declared, so it is a
            # type, interface, field, or value rather than a function.
            skip(row, REASON_TYPE_SYMBOL)
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
        entry["rows"].append({"model": row.model, "element": row.element})
        entry["killers"].update(killers)

    if not functions and not problems:
        problems.append("the plan has no function targets: no bound row names a Go function")

    groups: list[dict] = []
    if functions:
        groups, collisions = match_groups(functions, decls_by_file)
        problems += collisions
    if problems:
        raise PlanError("formal-bindings plan failed:\n  - " + "\n  - ".join(problems))

    resolve = resolve_packages or (lambda dirs: go_list_packages(dirs, root))
    killer_dirs = {next(iter(declared[t])) for entry in functions.values() for t in entry["killers"]}
    packages = resolve(killer_dirs)
    ordered = []
    for key in sorted(functions):
        entry = functions[key]
        killers = sorted(entry["killers"])
        entry["killers"] = killers
        entry["packages"] = sorted({packages[next(iter(declared[t]))] for t in killers})
        ordered.append(entry)
    return {
        "version": PLAN_VERSION,
        "root": str(root.resolve()),
        "files": sorted({entry["file"] for entry in ordered}),
        "groups": groups,
        "functions": ordered,
        "unbound": sorted(unbound, key=lambda u: (u["model"], u["element"], u["symbol"])),
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


def functions_in(plan: dict, file: str) -> list[dict]:
    return [f for f in plan["functions"] if f["file"] == file]


# ---------------------------------------------------------------------------
# Executor lookup


def first_changed_line(original: list[str], changed: list[str]) -> int | None:
    """1-based line of the original where the first difference starts."""
    for tag, i1, _i2, _j1, _j2 in difflib.SequenceMatcher(None, original, changed, autojunk=False).get_opcodes():
        if tag != "equal":
            return max(1, min(i1 + 1, len(original)))
    return None


def plan_relative(plan: dict, path: str) -> str:
    root = Path(plan["root"])
    resolved = Path(path).resolve()
    try:
        return resolved.relative_to(root).as_posix()
    except ValueError as err:
        raise PlanError(f"{path} is outside the plan root {root}") from err


def exec_timeout(plan: dict, file: str, preflight: bool) -> int:
    override = os.environ.get(TIMEOUT_OVERRIDE_ENV, "")
    if override:
        if not override.isdigit() or int(override) <= 0:
            raise PlanError(f"{TIMEOUT_OVERRIDE_ENV} must be a positive number of seconds, got {override!r}")
        return int(override)
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
        "run": "^(" + "|".join(killers) + ")$",
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
    override = os.environ.get(TIMEOUT_OVERRIDE_ENV, "")
    record = {
        "clean_seconds": round(seconds, 3),
        "timeout_seconds": int(override) if override.isdigit() and int(override) > 0 else derive_timeout(seconds),
        "overridden": bool(override),
    }
    plan["preflight"][file] = record
    return record


def max_timeout(plan: dict, recorded_only: bool = False) -> int:
    """The largest per-file exec timeout, for go-mutesting's --exec-timeout.
    Every file must have a pre-flight record unless recorded_only (a focused
    rerun pre-flights only the files it mutates)."""
    missing = [f for f in plan["files"] if f not in plan["preflight"]]
    if (missing and not recorded_only) or not plan["preflight"]:
        raise PlanError(f"no pre-flight record for: {', '.join(missing) or 'any file'}")
    return max(int(r["timeout_seconds"]) for r in plan["preflight"].values())


def rerun_scope(plan: dict, ids: list[str], hints: list[Path]) -> list[tuple[str, str, str]]:
    """(id, group, file) per mutant id. The stable id hashes the file, so a
    file recorded for the id in the baseline or a previous report lets the
    rerun pre-flight and mutate that file alone; an id without a hint (or with
    a file the plan no longer targets) is rerun across every group ("-")."""
    known: dict[str, str] = {}
    for hint in hints:
        if hint.exists():
            data = json.loads(hint.read_text())
            for mutant in data.get("mutants") or []:
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
    """The status of the mutant a --run-mutant-id run executed. The stable id
    ignores line numbers, so one id can name the same change on several lines;
    they must then agree."""
    stats = summary.get("stats", summary)
    counts = {
        "killed": stats.get("killedCount", 0),
        "escaped": stats.get("escapedCount", 0),
        "skipped": stats.get("skippedCount", 0),
        "errored": stats.get("errorCount", 0),
    }
    hit = [status for status, count in counts.items() if count]
    if len(hit) != 1:
        raise PlanError(f"expected the rerun to execute the mutant with one status, got {counts}")
    return hit[0]


def append_rerun(reruns: Path, mutant_id: str, status: str, commit: str, now: datetime.datetime | None = None) -> dict:
    record = {
        "id": mutant_id,
        "status": status,
        "timestamp": (now or datetime.datetime.now(datetime.UTC)).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "commit": commit,
    }
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


def merge_baselines(report_dir: Path, out: Path, commit: str) -> int:
    """Merge the per-group baselines go-mutesting wrote into one file, stamped
    with the commit it was generated at (rerun evidence must be from that
    commit or a descendant)."""
    mutants = []
    for group in group_dirs(report_dir):
        path = group / "baseline.json"
        if not path.exists():
            raise PlanError(f"{path}: go-mutesting wrote no baseline for this group")
        mutants += read_baseline(path)["mutants"]
    mutants.sort(key=lambda m: (m["file"], m["line"], m["mutator"], m["id"]))
    out.write_text(json.dumps({"version": 1, "commit": commit, "mutants": mutants}, indent=2) + "\n")
    return len(mutants)


STAT_KEYS = ("killedCount", "escapedCount", "errorCount", "skippedCount", "notCoveredCount")
REPORT_LISTS = {"killed": "killed", "escaped": "escaped", "skipped": "skipped", "errored": "errored"}
TIMEOUT_KILL_RE = re.compile(r"^formal-mutation: timeout-kill file=(\S+) ", re.M)


def merge_reports(report_dir: Path, log: str) -> dict:
    """One summary, agentic report, full report, and per-file table for all
    groups of a run. MSI is recomputed as go-mutesting does:
    (killed + errored + skipped) / total, as a percentage."""
    stats = dict.fromkeys(STAT_KEYS, 0)
    agentic: list[dict] = []
    lists: dict[str, list[dict]] = {name: [] for name in REPORT_LISTS}
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
            for name in REPORT_LISTS:
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
    per_file: dict[str, dict[str, int]] = {}
    for name, mutants in lists.items():
        for mutant in mutants:
            file = mutant["mutator"]["originalFilePath"].removeprefix(root)
            per_file.setdefault(file, dict.fromkeys([*REPORT_LISTS, "timeout-kill"], 0))[name] += 1
    for file in TIMEOUT_KILL_RE.findall(log):
        per_file.setdefault(file, dict.fromkeys([*REPORT_LISTS, "timeout-kill"], 0))["timeout-kill"] += 1
    columns = [*REPORT_LISTS, "timeout-kill"]
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
        if set(record) != {"id", "status", "timestamp", "commit"} or record["status"] not in RERUN_STATUSES:
            raise PlanError(f"{path}:{number}: malformed rerun record {raw!r}")
        records.append(record)
    return records


def git_is_ancestor(ancestor: str, descendant: str) -> bool:
    if ancestor == descendant:
        return True
    return subprocess.run(
        ["git", "-C", str(REPO_ROOT), "merge-base", "--is-ancestor", ancestor, descendant],
        capture_output=True, check=False,
    ).returncode == 0


def check_triage(
    baseline: dict,
    ledger: dict,
    reruns: list[dict],
    models: list[formal.Model],
    root: Path = REPO_ROOT,
    is_ancestor: Callable[[str, str], bool] = git_is_ancestor,
) -> list[str]:
    """Every fail-closed condition of the triage ledger (design D6)."""
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

    stamp = baseline.get("commit", "")
    if baseline_ids and not stamp:
        failures.append("baseline: no generation commit; rerun `make mutation-formal-baseline-update`")
    for mutant in sorted(baseline_ids):
        evidence = [r for r in reruns if r["id"] == mutant and stamp and is_ancestor(stamp, r["commit"])]
        escaped = sum(1 for r in evidence if r["status"] == "escaped")
        if escaped < REQUIRED_ESCAPED_RERUNS:
            failures.append(
                f"baseline id {mutant}: {escaped} recorded escaped rerun(s) at or after the baseline commit, "
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
# CLI


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="action", required=True)
    p = sub.add_parser("plan")
    p.add_argument("--out", type=Path, required=True)
    p = sub.add_parser("show")
    p.add_argument("--plan", type=Path, required=True)
    p.add_argument("--group", type=int)
    p.add_argument("--recorded", action="store_true", help="max-timeout: over pre-flighted files only")
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
    args = parser.parse_args(argv)

    try:
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
                print(max_timeout(plan, recorded_only=args.recorded))
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
            record = append_rerun(args.reruns, args.id, status, git("rev-parse", "HEAD"))
            print(f"rerun evidence: {record['id']} {record['status']} at {record['commit'][:12]}")
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
            count = merge_baselines(args.report_dir, args.baseline, git("rev-parse", "HEAD"))
            print(f"formal-bindings baseline: {count} surviving mutant(s) -> {args.baseline}")
        elif args.action == "triage":
            _tools, models = formal.load_manifest()
            with args.ledger.open("rb") as handle:
                ledger = tomllib.load(handle)
            failures = check_triage(read_baseline(args.baseline), ledger, read_reruns(args.reruns), models)
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
