#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Pick the domain review lenses for a change, from the paths it touches.

The change-flow workflow template (.agents/workflows/change-flow.js) runs the
`base` and `code-review` lenses on every change, and the domain lenses this
script selects. Path triggers are read live from the files that own them, so
the selector cannot drift from the rules:

    formal        .agents/rules/formal-bindings.md `paths:`, plus formal/manifest.toml
                  `[ci] paths` (models, instruments, triage ledger) minus the
                  repo-wide files in FORMAL_CI_GENERIC
    windows       .agents/rules/windows.md `paths:`
    schema        .agents/rules/cue-patterns.md `paths:`
    ci            .agents/rules/ci-workflows.md and version-pinning.md `paths:`
    perf          the pgo-staleness hook `files:` regex in .pre-commit-config.yaml,
                  plus internal/benchmark/
    security      SECURITY_PATHS (each named in .agents/agents/security-reviewer.md)
    supply-chain  SUPPLY_CHAIN_PATHS (each named in the module-security skill)
    architecture  a Go package directory added, removed or renamed under cmd/,
                  internal/ or pkg/

Output (stdout, JSON): {"lenses": [...], "fast_lane_eligible": bool,
"reasons": {lens: [paths]}}. Diagnostics go to stderr.

Standard library only.

Usage:
    scripts/change-flow-lenses.py --base origin/main --head HEAD
    scripts/change-flow-lenses.py --paths-from FILE   (git diff --name-status lines, or bare paths)
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tomllib
from pathlib import Path
from typing import Callable, Iterable

REPO_ROOT = Path(__file__).resolve().parent.parent

# The fixed lens ids, in output order. The workflow template keeps the same list.
LENS_IDS = ("formal", "windows", "schema", "ci", "security", "supply-chain", "perf", "architecture")

RULE_SOURCES = {
    "formal": (".agents/rules/formal-bindings.md",),
    "windows": (".agents/rules/windows.md",),
    "schema": (".agents/rules/cue-patterns.md",),
    "ci": (".agents/rules/ci-workflows.md", ".agents/rules/version-pinning.md"),
}

FORMAL_MANIFEST = "formal/manifest.toml"
# Repo-wide files in the manifest's [ci] paths. The formal CI check reruns on them,
# but they are not formal instruments; the ci lens covers them.
FORMAL_CI_GENERIC = ("Makefile", "go.mod", "go.sum")

SECURITY_AGENT = ".agents/agents/security-reviewer.md"
# Directories named under "High-Risk Code Areas" in SECURITY_AGENT. The test
# suite checks that each one exists and is still named there.
SECURITY_PATHS = (
    "internal/sshserver/",
    "internal/container/",
    "internal/provision/",
    "internal/config/",
    "internal/tuiserver/",
)

MODULE_SECURITY_SKILL = ".agents/skills/module-security/SKILL.md"
# Module-system surfaces named in MODULE_SECURITY_SKILL. A trailing "*" is a
# prefix match (the internal/app/module* services). The test suite checks that
# each path, without the "*", exists and is named in the skill text.
SUPPLY_CHAIN_PATHS = (
    "internal/audit/",
    "internal/discovery/",
    "internal/app/module*",
    "internal/app/deps/",
    "pkg/invowkmod/",
    "cmd/invowk/audit.go",
)
LOCK_FILE_NAME = "invowkmod.lock.cue"

PRE_COMMIT_CONFIG = ".pre-commit-config.yaml"
PGO_HOOK_ID = "pgo-staleness"
PERF_EXTRA_PREFIXES = ("internal/benchmark/",)

ARCHITECTURE_ROOTS = ("cmd/", "internal/", "pkg/")

# Lenses that keep a change off the fast lane.
FAST_LANE_BLOCKERS = ("formal", "schema", "security", "supply-chain", "architecture")


class LensError(Exception):
    """A trigger source could not be read; the caller falls back to all lenses."""


# Change is (status letter, old path, new path); old == new unless renamed or copied.
Change = tuple[str, str, str]


def glob_to_regex(pattern: str) -> re.Pattern[str]:
    """Translate a rule `paths:` glob ("**/" any dirs, "*" within a segment)."""
    out = []
    i = 0
    while i < len(pattern):
        if pattern.startswith("**/", i):
            out.append("(?:.*/)?")
            i += 3
        elif pattern.startswith("**", i):
            out.append(".*")
            i += 2
        elif pattern[i] == "*":
            out.append("[^/]*")
            i += 1
        elif pattern[i] == "?":
            out.append("[^/]")
            i += 1
        else:
            out.append(re.escape(pattern[i]))
            i += 1
    return re.compile("^" + "".join(out) + "$")


def frontmatter_paths(text: str, source: str) -> list[str]:
    """Return the `paths:` list of a rule's YAML frontmatter."""
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        raise LensError(f"{source}: no frontmatter")
    paths: list[str] = []
    in_paths = False
    for line in lines[1:]:
        if line.strip() == "---":
            break
        if re.match(r"^paths:\s*$", line):
            in_paths = True
            continue
        if in_paths:
            item = re.match(r"^\s+-\s+(.+?)\s*$", line)
            if item:
                paths.append(item.group(1).strip("'\""))
            elif line.strip():
                in_paths = False
    else:
        raise LensError(f"{source}: unterminated frontmatter")
    if not paths:
        raise LensError(f"{source}: frontmatter has no paths")
    return paths


def pgo_files_regex(text: str) -> re.Pattern[str]:
    """Return the `files:` regex of the pgo-staleness pre-commit hook."""
    in_hook = False
    for line in text.splitlines():
        hook = re.match(r"^\s*-\s+id:\s*(\S+)", line)
        if hook:
            in_hook = hook.group(1) == PGO_HOOK_ID
            continue
        if in_hook:
            files = re.match(r"^\s+files:\s*'(.+)'\s*$", line) or re.match(r'^\s+files:\s*"(.+)"\s*$', line)
            if files:
                return re.compile(files.group(1))
    raise LensError(f"{PRE_COMMIT_CONFIG}: hook {PGO_HOOK_ID} has no files: regex")


def manifest_ci_paths(text: str) -> list[str]:
    """Return the formal manifest's `[ci] paths`, without FORMAL_CI_GENERIC."""
    try:
        paths = tomllib.loads(text)["ci"]["paths"]
    except (tomllib.TOMLDecodeError, KeyError, TypeError) as err:
        raise LensError(f"{FORMAL_MANIFEST}: no [ci] paths ({err})") from err
    if not isinstance(paths, list) or not paths or not all(isinstance(p, str) for p in paths):
        raise LensError(f"{FORMAL_MANIFEST}: [ci] paths is not a list of strings")
    return [p for p in paths if p not in FORMAL_CI_GENERIC]


def load_triggers(root: Path = REPO_ROOT) -> dict[str, Callable[[str], bool]]:
    """Path predicates for every lens except architecture, read from their sources."""

    def read(rel: str) -> str:
        try:
            return (root / rel).read_text(encoding="utf-8")
        except OSError as err:
            raise LensError(f"{rel}: {err}") from err

    triggers: dict[str, Callable[[str], bool]] = {}
    for lens, sources in RULE_SOURCES.items():
        regexes = [glob_to_regex(p) for src in sources for p in frontmatter_paths(read(src), src)]
        if lens == "formal":
            regexes += [glob_to_regex(p) for p in manifest_ci_paths(read(FORMAL_MANIFEST))]
        triggers[lens] = lambda path, rx=tuple(regexes): any(r.match(path) for r in rx)

    pgo = pgo_files_regex(read(PRE_COMMIT_CONFIG))
    triggers["perf"] = lambda path: bool(pgo.search(path)) or path.startswith(PERF_EXTRA_PREFIXES)
    triggers["security"] = lambda path: path_in(path, SECURITY_PATHS)
    triggers["supply-chain"] = lambda path: path_in(path, SUPPLY_CHAIN_PATHS) or path.rsplit("/", 1)[-1] == LOCK_FILE_NAME
    return triggers


def path_in(path: str, entries: Iterable[str]) -> bool:
    for entry in entries:
        if entry.endswith("*"):
            if path.startswith(entry[:-1]):
                return True
        elif entry.endswith("/"):
            if path.startswith(entry):
                return True
        elif path == entry:
            return True
    return False


def parse_changes(text: str) -> list[Change]:
    """Parse `git diff --name-status` lines; a bare path counts as modified."""
    changes: list[Change] = []
    for line in text.splitlines():
        if not line.strip():
            continue
        fields = line.split("\t")
        if len(fields) == 1:
            changes.append(("M", fields[0].strip(), fields[0].strip()))
            continue
        status = fields[0].strip()[:1]
        if status in ("R", "C") and len(fields) >= 3:
            changes.append((status, fields[1], fields[2]))
        else:
            changes.append((status, fields[1], fields[1]))
    return changes


def parse_changes_z(text: str) -> list[Change]:
    """Parse `git diff --name-status -z` output: NUL-separated, paths unquoted.

    Without -z, git quotes paths with special or non-ASCII characters
    (core.quotePath), and a quoted path defeats every suffix check and trigger.
    """
    fields = text.split("\0")
    if fields and fields[-1] == "":
        fields.pop()
    changes: list[Change] = []
    i = 0
    while i < len(fields):
        status = fields[i][:1]
        if status in ("R", "C"):
            if i + 2 >= len(fields):
                raise ValueError("truncated rename record in git diff -z output")
            changes.append((status, fields[i + 1], fields[i + 2]))
            i += 3
        else:
            if i + 1 >= len(fields):
                raise ValueError("truncated record in git diff -z output")
            changes.append((status, fields[i + 1], fields[i + 1]))
            i += 2
    return changes


def is_go_package_file(path: str) -> bool:
    return path.endswith(".go") and not path.endswith("_test.go") and path.startswith(ARCHITECTURE_ROOTS) and "/testdata/" not in path


def package_dir(path: str) -> str:
    return path.rsplit("/", 1)[0] if "/" in path else ""


def architecture_paths(changes: list[Change], dir_has_go: Callable[[str, str], bool] | None) -> list[str]:
    """Paths that add, remove or rename a Go package directory.

    dir_has_go(side, dir) says whether dir holds a non-test .go file on side
    "base" or "head". Without it (paths-from mode), any added, deleted or
    cross-directory renamed package file counts: conservative by design.
    """
    hits: list[str] = []
    for status, old, new in changes:
        if status == "A" and is_go_package_file(new):
            if dir_has_go is None or not dir_has_go("base", package_dir(new)):
                hits.append(new)
        elif status == "D" and is_go_package_file(old):
            if dir_has_go is None or not dir_has_go("head", package_dir(old)):
                hits.append(old)
        elif status == "R" and (is_go_package_file(old) or is_go_package_file(new)):
            if package_dir(old) != package_dir(new):
                if dir_has_go is None or not dir_has_go("base", package_dir(new)) or not dir_has_go("head", package_dir(old)):
                    hits.append(new)
    return hits


def outside_testdata(path: str) -> bool:
    return not (path.startswith("testdata/") or "/testdata/" in path)


def select_lenses(
    changes: list[Change],
    triggers: dict[str, Callable[[str], bool]],
    dir_has_go: Callable[[str, str], bool] | None = None,
) -> dict:
    """Pure selection: lenses, fast-lane eligibility and the paths behind each lens."""
    paths: list[str] = []
    for _, old, new in changes:
        for path in (old, new):
            if path and path not in paths:
                paths.append(path)
    reasons: dict[str, list[str]] = {}
    for lens in LENS_IDS:
        if lens == "architecture":
            hits = architecture_paths(changes, dir_has_go)
        else:
            hits = [p for p in paths if triggers[lens](p)]
        if hits:
            reasons[lens] = hits
    lenses = [lens for lens in LENS_IDS if lens in reasons]
    has_go = any(p.endswith(".go") for p in paths)
    has_cue = any(p.endswith(".cue") and outside_testdata(p) for p in paths)
    blocked = any(lens in reasons for lens in FAST_LANE_BLOCKERS)
    return {
        "lenses": lenses,
        "fast_lane_eligible": bool(paths) and not (has_go or has_cue or blocked),
        "reasons": reasons,
    }


def all_lenses(reason: str) -> dict:
    """The fallback: every lens, never fast lane."""
    return {"lenses": list(LENS_IDS), "fast_lane_eligible": False, "reasons": {lens: [reason] for lens in LENS_IDS}}


def git(*argv: str, root: Path = REPO_ROOT) -> str:
    completed = subprocess.run(["git", "-C", str(root), *argv], capture_output=True, text=True, check=True)
    return completed.stdout


def git_dir_has_go(revs: dict[str, str], root: Path = REPO_ROOT) -> Callable[[str, str], bool]:
    cache: dict[tuple[str, str], bool] = {}

    def check(side: str, directory: str) -> bool:
        key = (side, directory)
        if key not in cache:
            try:
                listing = git("ls-tree", "--name-only", f"{revs[side]}:{directory}" if directory else revs[side], root=root)
            except subprocess.CalledProcessError:
                listing = ""  # the directory does not exist on that side
            cache[key] = any(n.endswith(".go") and not n.endswith("_test.go") for n in listing.splitlines())
        return cache[key]

    return check


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--base", default="", help="base revision (git diff <base>...<head>)")
    parser.add_argument("--head", default="HEAD", help="head revision (default HEAD)")
    parser.add_argument("--paths-from", default="", help="read name-status lines or bare paths from FILE instead of git")
    args = parser.parse_args(argv)

    try:
        triggers = load_triggers()
    except LensError as err:
        print(f"change-flow-lenses: {err}; selecting all lenses", file=sys.stderr)
        print(json.dumps(all_lenses(f"trigger source unreadable: {err}"), sort_keys=True))
        return 0

    dir_has_go = None
    if args.paths_from:
        changes = parse_changes(Path(args.paths_from).read_text(encoding="utf-8"))
    elif args.base:
        try:
            changes = parse_changes_z(git("diff", "--name-status", "-z", "-M", f"{args.base}...{args.head}"))
            merge_base = git("merge-base", args.base, args.head).strip()
            dir_has_go = git_dir_has_go({"base": merge_base, "head": args.head})
        except subprocess.CalledProcessError as err:
            print(f"change-flow-lenses: git failed: {err.stderr.strip()}", file=sys.stderr)
            return 2
        except ValueError as err:
            print(f"change-flow-lenses: {err}", file=sys.stderr)
            return 2
    else:
        parser.error("pass --base (with optional --head) or --paths-from")

    print(json.dumps(select_lenses(changes, triggers, dir_has_go), sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
