#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Deterministic contract tooling for the Invowk documentation audit."""

from __future__ import annotations

import argparse
import datetime as dt
import glob
import hashlib
import json
import os
import re
import stat
import subprocess
import sys
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Any


SKILL_DIR = Path(__file__).resolve().parent.parent
DEFAULT_REPO_ROOT = SKILL_DIR.parents[2]
REFERENCES_DIR = SKILL_DIR / "references"
OWNERSHIP_PATH = REFERENCES_DIR / "doc-ownership.json"
CHECKLIST_PATH = REFERENCES_DIR / "surface-checklists.md"
SIMPLIFICATIONS_PATH = REFERENCES_DIR / "intentional-simplifications.md"
README_MAP_PATH = REFERENCES_DIR / "readme-sync-map.md"
SURFACE_IDS = tuple(f"S{index}" for index in range(1, 12))
STATUSES = frozenset({"PASS", "FAIL", "SKIP", "BLOCKED"})
SEVERITY_ORDER = {"ERROR": 0, "WARNING": 1, "INFO": 2}
REQUIRED_PROGRAMMATIC_CHECKS = (
    "docs-parity",
    "container-policy",
    "diagram-readability",
    "d2-validate",
    "diagram-renders",
    "check-agent-docs",
    "version-assets",
    "website-typecheck",
    "website-build",
)
CHECK_ID_RE = re.compile(r"^S(?P<surface>\d+)-C(?P<number>\d+)$")
SIMPLIFICATION_ID_RE = re.compile(r"\|\s*(IS-\d{3})\s*\|")


class ContractError(ValueError):
    """Report invalid audit contracts or result artifacts."""


@dataclass(frozen=True)
class Check:
    check_id: str
    text: str
    severity: str
    finding_type: str

    @property
    def surface(self) -> str:
        return self.check_id.split("-", maxsplit=1)[0]


def run_git(repo_root: Path, *args: str, check: bool = True) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=repo_root,
        check=False,
        text=True,
        capture_output=True,
    )
    if check and result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip()
        raise ContractError(f"git {' '.join(args)} failed: {detail}")
    return result.stdout


def load_json(path: Path) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise ContractError(f"read {path}: {exc}") from exc
    except json.JSONDecodeError as exc:
        raise ContractError(f"parse {path}: {exc}") from exc


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.tmp")
    temporary.write_text(
        json.dumps(value, indent=2, sort_keys=True, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )
    temporary.replace(path)


def canonical_sha256(value: Any) -> str:
    payload = json.dumps(
        value,
        sort_keys=True,
        ensure_ascii=False,
        separators=(",", ":"),
    ).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def parse_checks(path: Path = CHECKLIST_PATH) -> dict[str, Check]:
    checks: dict[str, Check] = {}
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        stripped = line.strip()
        if not stripped.startswith("| S"):
            continue
        cells = [cell.strip() for cell in stripped.strip("|").split("|")]
        if not cells or not CHECK_ID_RE.fullmatch(cells[0]):
            continue
        if len(cells) < 4:
            raise ContractError(f"{path}:{line_number}: malformed checklist row")
        check_id = cells[0]
        if check_id in checks:
            raise ContractError(f"{path}:{line_number}: duplicate check ID {check_id}")
        severity = cells[-2]
        finding_type = cells[-1]
        if severity not in SEVERITY_ORDER:
            raise ContractError(f"{path}:{line_number}: invalid severity {severity}")
        if not re.fullmatch(r"[a-z][a-z0-9-]+", finding_type):
            raise ContractError(f"{path}:{line_number}: invalid finding type {finding_type}")
        checks[check_id] = Check(check_id, cells[1], severity, finding_type)
    if not checks:
        raise ContractError(f"no checklist rows found in {path}")
    for surface in SURFACE_IDS:
        numbers = sorted(
            int(CHECK_ID_RE.fullmatch(check_id).group("number"))
            for check_id in checks
            if check_id.startswith(f"{surface}-")
        )
        expected = list(range(1, len(numbers) + 1))
        if numbers != expected:
            raise ContractError(f"{surface} check IDs are not contiguous: {numbers}")
    return checks


def parse_simplifications(
    path: Path = SIMPLIFICATIONS_PATH,
) -> tuple[set[str], dict[str, set[str]]]:
    simplification_ids: set[str] = set()
    whole_check_ids: dict[str, set[str]] = {}
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not re.match(r"^\|\s*IS-\d{3}\s*\|", line):
            continue
        cells = [cell.strip() for cell in line.strip("|").split("|")]
        if len(cells) != 5:
            raise ContractError(f"{path}:{line_number}: malformed simplification row")
        simplification_id = cells[0]
        if simplification_id in simplification_ids:
            raise ContractError(f"{path}:{line_number}: duplicate {simplification_id}")
        simplification_ids.add(simplification_id)
        ids = set(re.findall(r"S\d+-C\d+", cells[4]))
        whole_check_ids[simplification_id] = ids
    return simplification_ids, whole_check_ids


def parse_simplification_ids(path: Path = SIMPLIFICATIONS_PATH) -> set[str]:
    return parse_simplifications(path)[0]


def validate_readme_locators(repo_root: Path) -> int:
    headings = set((repo_root / "README.md").read_text(encoding="utf-8").splitlines())
    count = 0
    for line_number, line in enumerate(README_MAP_PATH.read_text(encoding="utf-8").splitlines(), 1):
        if not line.startswith("|"):
            continue
        cells = [cell.strip() for cell in line.strip("|").split("|")]
        if len(cells) != 4 or cells[0] in {"README Section", "---"}:
            continue
        locator = cells[1]
        match = re.fullmatch(r"`(#{2,3} .+)`", locator)
        if not match:
            raise ContractError(f"{README_MAP_PATH}:{line_number}: locator must be an exact heading")
        if match.group(1) not in headings:
            raise ContractError(
                f"{README_MAP_PATH}:{line_number}: README heading does not exist: {match.group(1)}"
            )
        count += 1
    if not count:
        raise ContractError("README sync map has no stable heading locators")
    return count


def resolve_source(repo_root: Path, source: str) -> list[Path]:
    if Path(source).is_absolute() or ".." in Path(source).parts:
        raise ContractError(f"source path escapes repository: {source}")
    if glob.has_magic(source):
        return [Path(match) for match in glob.glob(str(repo_root / source), recursive=True)]
    path = repo_root / source
    return [path] if path.exists() else []


def validate_ownership(repo_root: Path, checks: dict[str, Check]) -> dict[str, Any]:
    contract = load_json(OWNERSHIP_PATH)
    if set(contract) != {"schema_version", "inventory_root", "documents"}:
        raise ContractError("doc-ownership.json has unknown or missing top-level keys")
    if contract["schema_version"] != 1:
        raise ContractError("unsupported doc ownership schema version")
    inventory_root = contract["inventory_root"]
    if inventory_root != "website/docs":
        raise ContractError(f"unexpected inventory root: {inventory_root}")
    documents = contract["documents"]
    if not isinstance(documents, list):
        raise ContractError("documents must be a list")

    live_paths = sorted(
        path.relative_to(repo_root).as_posix()
        for path in (repo_root / inventory_root).rglob("*.mdx")
    )
    owned_paths: list[str] = []
    for index, document in enumerate(documents):
        location = f"doc-ownership.json documents[{index}]"
        if set(document) != {"path", "surface", "check_id", "sources"}:
            raise ContractError(f"{location} has unknown or missing keys")
        path = document["path"]
        surface = document["surface"]
        check_id = document["check_id"]
        sources = document["sources"]
        if not isinstance(path, str) or glob.has_magic(path):
            raise ContractError(f"{location} path must be a literal string")
        if not path.startswith(f"{inventory_root}/") or "/versioned_docs/" in path:
            raise ContractError(f"{location} owns an excluded path: {path}")
        if surface not in SURFACE_IDS:
            raise ContractError(f"{location} has invalid surface {surface}")
        if check_id not in checks:
            raise ContractError(f"{location} references unknown check {check_id}")
        if checks[check_id].surface != surface:
            raise ContractError(f"{location} surface does not match {check_id}")
        if check_id in {"S2-C13", "S2-C14"} or surface == "S11":
            raise ContractError(f"{location} uses a mechanical check as semantic owner")
        if not isinstance(sources, list) or not sources:
            raise ContractError(f"{location} has no sources of truth")
        for source in sources:
            if not isinstance(source, str) or not resolve_source(repo_root, source):
                raise ContractError(f"{location} source resolves to nothing: {source}")
        owned_paths.append(path)

    duplicates = sorted(path for path, count in Counter(owned_paths).items() if count > 1)
    if duplicates:
        raise ContractError(f"duplicate ownership paths: {', '.join(duplicates)}")
    casefolded = [path.casefold() for path in owned_paths]
    collisions = sorted(path for path, count in Counter(casefolded).items() if count > 1)
    if collisions:
        raise ContractError(f"case-fold ownership collisions: {', '.join(collisions)}")
    missing = sorted(set(live_paths) - set(owned_paths))
    stale = sorted(set(owned_paths) - set(live_paths))
    if missing or stale:
        raise ContractError(
            f"ownership mismatch; missing={missing or 'none'}, stale={stale or 'none'}"
        )
    return {"status": "PASS", "document_count": len(live_paths), "paths": live_paths}


def ownership_targets_by_check() -> dict[str, set[str]]:
    contract = load_json(OWNERSHIP_PATH)
    targets: dict[str, set[str]] = {}
    for document in contract["documents"]:
        targets.setdefault(document["check_id"], set()).add(document["path"])
    return targets


def inventories(repo_root: Path) -> dict[str, list[str]]:
    def relative(paths: list[Path]) -> list[str]:
        return sorted(path.relative_to(repo_root).as_posix() for path in paths)

    d2_paths = [
        path
        for path in (repo_root / "docs/diagrams").rglob("*.d2")
        if "experiments" not in path.parts
    ]
    return {
        "website_docs": relative(list((repo_root / "website/docs").rglob("*.mdx"))),
        "snippets": relative(list((repo_root / "website/src/components/Snippet/data").glob("*.ts"))),
        "d2": relative(d2_paths),
        "architecture": relative(list((repo_root / "docs/architecture").glob("*.md"))),
        "agent_docs": [
            "AGENTS.md",
            ".agents/commands/review-docs.md",
            ".agents/skills/docs/SKILL.md",
            ".agents/skills/review-docs/SKILL.md",
            ".agents/skills/review-docs/references/doc-ownership.json",
            ".agents/skills/review-docs/references/surface-checklists.md",
            ".agents/skills/review-docs/references/structured-output-format.md",
            ".agents/skills/review-docs/references/verification-commands.md",
        ],
    }


def workspace_sha256(repo_root: Path) -> str:
    digest = hashlib.sha256()
    output = subprocess.run(
        ["git", "ls-files", "-co", "--exclude-standard", "-z"],
        cwd=repo_root,
        check=True,
        capture_output=True,
    ).stdout
    paths = sorted(filter(None, output.split(b"\0")))
    for raw_path in paths:
        relative = os.fsdecode(raw_path)
        path = repo_root / relative
        digest.update(raw_path)
        digest.update(b"\0")
        if not path.exists() and not path.is_symlink():
            digest.update(b"MISSING")
        else:
            digest.update(oct(stat.S_IMODE(path.lstat().st_mode)).encode("ascii"))
        if path.is_symlink():
            digest.update(b"L")
            digest.update(os.fsencode(os.readlink(path)))
        elif path.is_file():
            digest.update(b"F")
            with path.open("rb") as handle:
                for chunk in iter(lambda: handle.read(1024 * 1024), b""):
                    digest.update(chunk)
        digest.update(b"\0")
    return digest.hexdigest()


def is_dirty(repo_root: Path, path: str) -> bool:
    return bool(run_git(repo_root, "status", "--porcelain=v1", "--", path).strip())


def last_commit_epoch(repo_root: Path, path: str) -> int | None:
    output = run_git(repo_root, "log", "-1", "--format=%ct", "--follow", "--", path)
    stripped = output.strip()
    return int(stripped) if stripped else None


def i18n_lag(repo_root: Path, english_paths: list[str], days: int, limit: int) -> dict[str, Any]:
    threshold = days * 86400
    locale_root = "website/i18n/pt-BR/docusaurus-plugin-content-docs/current"
    candidates: list[dict[str, Any]] = []
    blocked: list[dict[str, str]] = []
    if run_git(repo_root, "rev-parse", "--is-shallow-repository").strip() == "true":
        return {
            "status": "BLOCKED",
            "threshold_days": days,
            "limit": limit,
            "candidates": [],
            "blocked": [{"english_path": "*", "reason": "shallow git history"}],
        }
    for english_path in english_paths:
        suffix = english_path.removeprefix("website/docs/")
        locale_path = f"{locale_root}/{suffix}"
        if not (repo_root / locale_path).exists():
            continue
        if is_dirty(repo_root, english_path) or is_dirty(repo_root, locale_path):
            blocked.append({"english_path": english_path, "reason": "uncommitted locale pair"})
            continue
        english_epoch = last_commit_epoch(repo_root, english_path)
        locale_epoch = last_commit_epoch(repo_root, locale_path)
        if english_epoch is None or locale_epoch is None:
            blocked.append({"english_path": english_path, "reason": "missing git history"})
            continue
        lag_seconds = english_epoch - locale_epoch
        if lag_seconds > threshold:
            candidates.append(
                {
                    "english_path": english_path,
                    "locale_path": locale_path,
                    "english_epoch": english_epoch,
                    "locale_epoch": locale_epoch,
                    "lag_days": lag_seconds // 86400,
                }
            )
    candidates.sort(key=lambda item: (-item["lag_days"], item["english_path"]))
    return {
        "status": "BLOCKED" if blocked else "PASS",
        "threshold_days": days,
        "limit": limit,
        "candidates": candidates[:limit],
        "blocked": blocked,
    }


def validate_programmatic_checks(value: Any) -> dict[str, Any]:
    if not isinstance(value, dict) or set(value) != set(REQUIRED_PROGRAMMATIC_CHECKS):
        raise ContractError(
            "programmatic checks must contain exactly: "
            + ", ".join(REQUIRED_PROGRAMMATIC_CHECKS)
        )
    normalized: dict[str, Any] = {}
    for name in REQUIRED_PROGRAMMATIC_CHECKS:
        check = value[name]
        if not isinstance(check, dict) or set(check) - {"status", "detail", "exit_code"}:
            raise ContractError(f"programmatic check {name} has invalid fields")
        status = check.get("status")
        if status not in {"PASS", "FAIL", "BLOCKED"}:
            raise ContractError(f"programmatic check {name} has invalid status {status}")
        detail = check.get("detail", "")
        if not isinstance(detail, str):
            raise ContractError(f"programmatic check {name} detail must be a string")
        exit_code = check.get("exit_code")
        if exit_code is not None and not isinstance(exit_code, int):
            raise ContractError(f"programmatic check {name} exit_code must be an integer or null")
        normalized[name] = {"status": status, "detail": detail, "exit_code": exit_code}
    return normalized


def validate_contract(repo_root: Path) -> dict[str, Any]:
    checks = parse_checks()
    ownership = validate_ownership(repo_root, checks)
    simplifications, whole_check_skips = parse_simplifications()
    if not simplifications:
        raise ContractError("intentional simplifications must have stable IS-* IDs")
    unknown_skip_checks = sorted(
        check_id
        for check_ids in whole_check_skips.values()
        for check_id in check_ids
        if check_id not in checks
    )
    if unknown_skip_checks:
        raise ContractError(f"simplifications reference unknown whole-check IDs: {unknown_skip_checks}")
    readme_locator_count = validate_readme_locators(repo_root)
    return {
        "check_count": len(checks),
        "surface_counts": dict(sorted(Counter(check.surface for check in checks.values()).items())),
        "document_count": ownership["document_count"],
        "simplification_count": len(simplifications),
        "readme_locator_count": readme_locator_count,
    }


def ensure_external_output(repo_root: Path, output: Path) -> None:
    resolved_repo = repo_root.resolve()
    resolved_output = output.resolve()
    if resolved_output == resolved_repo or resolved_repo in resolved_output.parents:
        raise ContractError("audit artifacts must be written outside the repository")


def prepare_context(args: argparse.Namespace) -> int:
    repo_root = args.repo_root.resolve()
    ensure_external_output(repo_root, args.output)
    summary = validate_contract(repo_root)
    checks = validate_programmatic_checks(load_json(args.checks))
    live_inventories = inventories(repo_root)
    context = {
        "schema_version": 1,
        "audit_date": dt.date.today().isoformat(),
        "snapshot": {
            "git_head": run_git(repo_root, "rev-parse", "HEAD").strip(),
            "workspace_sha256": workspace_sha256(repo_root),
        },
        "contract": summary,
        "inventories": live_inventories,
        "ownership": {"status": "PASS", "document_count": summary["document_count"]},
        "i18n_lag": i18n_lag(
            repo_root,
            live_inventories["website_docs"],
            args.lag_days,
            args.lag_limit,
        ),
        "programmatic_checks": checks,
    }
    context["context_id"] = canonical_sha256(context)
    write_json(args.output, context)
    print(args.output)
    return 0


def validate_context(repo_root: Path, context: Any, verify_snapshot: bool) -> dict[str, Any]:
    required = {
        "schema_version",
        "audit_date",
        "snapshot",
        "contract",
        "inventories",
        "ownership",
        "i18n_lag",
        "programmatic_checks",
        "context_id",
    }
    if not isinstance(context, dict) or set(context) != required or context["schema_version"] != 1:
        raise ContractError("invalid context artifact")
    validate_programmatic_checks(context["programmatic_checks"])
    unsigned_context = {key: value for key, value in context.items() if key != "context_id"}
    if context["context_id"] != canonical_sha256(unsigned_context):
        raise ContractError("context_id does not match the canonical context payload")
    snapshot = context["snapshot"]
    if set(snapshot) != {"git_head", "workspace_sha256"}:
        raise ContractError("invalid context snapshot")
    if verify_snapshot:
        current_head = run_git(repo_root, "rev-parse", "HEAD").strip()
        current_hash = workspace_sha256(repo_root)
        if snapshot["git_head"] != current_head or snapshot["workspace_sha256"] != current_hash:
            raise ContractError("repository snapshot changed after context preparation")
        current_contract = validate_contract(repo_root)
        if context["contract"] != current_contract:
            raise ContractError("context contract summary differs from the repository")
        current_inventories = inventories(repo_root)
        if context["inventories"] != current_inventories:
            raise ContractError("context inventories differ from the repository")
        expected_ownership = {
            "status": "PASS",
            "document_count": current_contract["document_count"],
        }
        if context["ownership"] != expected_ownership:
            raise ContractError("context ownership summary differs from the repository")
        lag = context["i18n_lag"]
        current_lag = i18n_lag(
            repo_root,
            current_inventories["website_docs"],
            lag["threshold_days"],
            lag["limit"],
        )
        if lag != current_lag:
            raise ContractError("context i18n lag data differs from the repository")
    return context


def expected_ids(checks: dict[str, Check], surface: str) -> list[str]:
    def number(check_id: str) -> int:
        return int(CHECK_ID_RE.fullmatch(check_id).group("number"))

    return sorted((check_id for check_id in checks if check_id.startswith(f"{surface}-")), key=number)


def validate_relative_existing(repo_root: Path, value: str, label: str) -> None:
    path = Path(value)
    if path.is_absolute() or ".." in path.parts or not (repo_root / path).exists():
        raise ContractError(f"{label} must be an existing repository-relative path: {value}")


def validate_finding(repo_root: Path, finding: Any, location: str) -> None:
    required = {"target", "source", "current", "expected", "rationale"}
    if not isinstance(finding, dict) or set(finding) != required:
        raise ContractError(f"{location} has unknown or missing fields")
    target = finding["target"]
    source = finding["source"]
    if not isinstance(target, dict) or set(target) != {"path", "locator"}:
        raise ContractError(f"{location}.target is invalid")
    if not isinstance(source, dict) or set(source) != {"path", "symbol"}:
        raise ContractError(f"{location}.source is invalid")
    validate_relative_existing(repo_root, target["path"], f"{location}.target.path")
    validate_relative_existing(repo_root, source["path"], f"{location}.source.path")
    for field in ("locator",):
        if not isinstance(target[field], str) or not target[field].strip():
            raise ContractError(f"{location}.target.{field} must be non-empty")
    if not isinstance(source["symbol"], str) or not source["symbol"].strip():
        raise ContractError(f"{location}.source.symbol must be non-empty")
    for field in ("current", "expected", "rationale"):
        if not isinstance(finding[field], str) or not finding[field].strip():
            raise ContractError(f"{location}.{field} must be non-empty")


def validate_result(
    repo_root: Path,
    context: dict[str, Any],
    result: Any,
    checks: dict[str, Check],
) -> dict[str, Any]:
    required = {"schema_version", "surface", "context_id", "items", "candidates"}
    if not isinstance(result, dict) or set(result) != required or result["schema_version"] != 1:
        raise ContractError("invalid result artifact")
    surface = result["surface"]
    if surface not in SURFACE_IDS:
        raise ContractError(f"invalid result surface {surface}")
    if result["context_id"] != context["context_id"]:
        raise ContractError(f"{surface} result uses a stale or different context")
    if not isinstance(result["candidates"], list):
        raise ContractError(f"{surface} candidates must be a list")
    items = result["items"]
    if not isinstance(items, list):
        raise ContractError(f"{surface} items must be a list")
    by_id: dict[str, dict[str, Any]] = {}
    simplification_ids, whole_check_skips = parse_simplifications()
    ownership_targets = ownership_targets_by_check()
    allowed_item_keys = {
        "check_id",
        "status",
        "targets",
        "evidence",
        "findings",
        "reason",
        "simplification_id",
    }
    for index, item in enumerate(items):
        location = f"{surface} items[{index}]"
        if not isinstance(item, dict) or set(item) - allowed_item_keys:
            raise ContractError(f"{location} has unknown fields")
        if not {"check_id", "status", "targets", "evidence", "findings"}.issubset(item):
            raise ContractError(f"{location} is missing required fields")
        check_id = item["check_id"]
        if check_id in by_id:
            raise ContractError(f"{surface} duplicates {check_id}")
        if check_id not in checks or checks[check_id].surface != surface:
            raise ContractError(f"{surface} contains foreign check {check_id}")
        status = item["status"]
        targets = item["targets"]
        evidence = item["evidence"]
        findings = item["findings"]
        if status not in STATUSES:
            raise ContractError(f"{check_id} has invalid status {status}")
        if not isinstance(targets, list) or not targets or len(targets) != len(set(targets)):
            raise ContractError(f"{check_id} targets must be a non-empty unique path list")
        for target in targets:
            if not isinstance(target, str):
                raise ContractError(f"{check_id} targets must contain strings")
            validate_relative_existing(repo_root, target, f"{check_id} target")
        required_targets = ownership_targets.get(check_id, set())
        missing_targets = sorted(required_targets - set(targets))
        if missing_targets:
            raise ContractError(f"{check_id} omits owned targets: {missing_targets}")
        if not isinstance(evidence, list) or any(
            not isinstance(value, str) or not value.strip() for value in evidence
        ):
            raise ContractError(f"{check_id} evidence must contain non-empty strings")
        if not isinstance(findings, list):
            raise ContractError(f"{check_id} findings must be a list")
        if status == "PASS" and (not evidence or findings):
            raise ContractError(f"{check_id} PASS requires evidence and forbids findings")
        if status == "FAIL" and (not evidence or not findings):
            raise ContractError(f"{check_id} FAIL requires evidence and at least one finding")
        if status == "SKIP":
            simplification_id = item.get("simplification_id")
            if (
                findings
                or simplification_id not in simplification_ids
                or check_id not in whole_check_skips.get(simplification_id, set())
            ):
                raise ContractError(
                    f"{check_id} SKIP requires an explicit whole-check exemption and no findings"
                )
        if status == "BLOCKED":
            if findings or not isinstance(item.get("reason"), str) or not item["reason"].strip():
                raise ContractError(f"{check_id} BLOCKED requires a reason and no findings")
        if status != "BLOCKED" and "reason" in item:
            raise ContractError(f"{check_id} reason is only valid for BLOCKED")
        if status != "SKIP" and "simplification_id" in item:
            raise ContractError(f"{check_id} simplification_id is only valid for SKIP")
        for finding_index, finding in enumerate(findings):
            validate_finding(repo_root, finding, f"{check_id} findings[{finding_index}]")
            if finding["target"]["path"] not in targets:
                raise ContractError(
                    f"{check_id} finding target is absent from item targets: "
                    f"{finding['target']['path']}"
                )
        by_id[check_id] = item
    expected = expected_ids(checks, surface)
    if set(by_id) != set(expected):
        raise ContractError(
            f"{surface} checklist mismatch; missing={sorted(set(expected) - set(by_id))}, "
            f"extra={sorted(set(by_id) - set(expected))}"
        )
    result["items"] = [by_id[check_id] for check_id in expected]
    return result


def escape_markdown(value: Any) -> str:
    return str(value).replace("\\", "\\\\").replace("|", "\\|").replace("\n", "<br>")


def merge_results(
    repo_root: Path,
    context: dict[str, Any],
    results: list[dict[str, Any]],
    checks: dict[str, Check],
) -> dict[str, Any]:
    surfaces = [result["surface"] for result in results]
    if sorted(surfaces, key=lambda value: int(value[1:])) != list(SURFACE_IDS):
        raise ContractError(f"expected one result for each surface, got {surfaces}")
    counts: dict[str, Counter[str]] = {}
    merged_by_fingerprint: dict[tuple[str, ...], dict[str, Any]] = {}
    blockers: list[dict[str, Any]] = []
    blocked = False
    for result in results:
        surface = result["surface"]
        counts[surface] = Counter(item["status"] for item in result["items"])
        blocked = blocked or bool(counts[surface]["BLOCKED"])
        for item in result["items"]:
            check = checks[item["check_id"]]
            if item["status"] == "BLOCKED":
                blockers.append(
                    {
                        "kind": "checklist",
                        "surface": surface,
                        "check_id": item["check_id"],
                        "reason": item["reason"],
                        "targets": item["targets"],
                    }
                )
            for finding in item["findings"]:
                fingerprint = (
                    finding["target"]["path"],
                    finding["target"]["locator"],
                    finding["current"].strip(),
                    finding["expected"].strip(),
                )
                candidate = {
                    **finding,
                    "check_id": check.check_id,
                    "surface": check.surface,
                    "severity": check.severity,
                    "finding_type": check.finding_type,
                    "provenance": [check.check_id],
                }
                existing = merged_by_fingerprint.get(fingerprint)
                if existing is None:
                    merged_by_fingerprint[fingerprint] = candidate
                else:
                    existing["provenance"] = sorted(set(existing["provenance"] + [check.check_id]))
                    current_key = (SEVERITY_ORDER[candidate["severity"]], candidate["check_id"])
                    existing_key = (SEVERITY_ORDER[existing["severity"]], existing["check_id"])
                    if current_key < existing_key:
                        provenance = existing["provenance"]
                        merged_by_fingerprint[fingerprint] = candidate
                        merged_by_fingerprint[fingerprint]["provenance"] = provenance
    findings = list(merged_by_fingerprint.values())
    findings.sort(
        key=lambda finding: (
            SEVERITY_ORDER[finding["severity"]],
            int(finding["surface"][1:]),
            int(CHECK_ID_RE.fullmatch(finding["check_id"]).group("number")),
            finding["target"]["path"],
            finding["target"]["locator"],
        )
    )
    for index, finding in enumerate(findings, 1):
        finding["id"] = f"RD-{index:03d}"
    programmatic_blocked = any(
        check["status"] == "BLOCKED" for check in context["programmatic_checks"].values()
    )
    for name, check in context["programmatic_checks"].items():
        if check["status"] == "BLOCKED":
            blockers.append(
                {
                    "kind": "programmatic",
                    "check": name,
                    "reason": check["detail"],
                }
            )
    for item in context["i18n_lag"]["blocked"]:
        blockers.append({"kind": "i18n-lag", **item})
    blockers.sort(
        key=lambda item: (
            item["kind"],
            item.get("surface", ""),
            item.get("check_id", ""),
            item.get("check", ""),
            item.get("english_path", ""),
        )
    )
    blocked = blocked or programmatic_blocked or context["i18n_lag"]["status"] == "BLOCKED"
    return {
        "schema_version": 1,
        "verdict": "INCOMPLETE" if blocked else "COMPLETE",
        "audit_date": context["audit_date"],
        "snapshot": context["snapshot"],
        "programmatic_checks": context["programmatic_checks"],
        "blockers": blockers,
        "findings": findings,
        "checklist_completion": {
            surface: {
                status: counts[surface][status]
                for status in ("PASS", "FAIL", "SKIP", "BLOCKED")
            }
            for surface in SURFACE_IDS
        },
    }


def report_markdown(report: dict[str, Any]) -> str:
    lines = [
        "# Documentation Review Report",
        "",
        f"- Date: {report['audit_date']}",
        f"- Verdict: **{report['verdict']}**",
        f"- Git HEAD: `{report['snapshot']['git_head']}`",
        f"- Workspace snapshot: `{report['snapshot']['workspace_sha256']}`",
        "",
        "## Programmatic Checks",
        "",
        "| Check | Status | Detail |",
        "|---|---|---|",
    ]
    for name in REQUIRED_PROGRAMMATIC_CHECKS:
        check = report["programmatic_checks"][name]
        lines.append(
            f"| {name} | {check['status']} | {escape_markdown(check['detail'])} |"
        )
    lines.extend(
        [
            "",
            "## Blockers",
            "",
        ]
    )
    if report["blockers"]:
        for blocker in report["blockers"]:
            identity = blocker.get("check_id", blocker.get("check", blocker.get("english_path", "unknown")))
            lines.append(
                f"- `{escape_markdown(blocker['kind'])}` `{escape_markdown(identity)}`: "
                f"{escape_markdown(blocker['reason'])}"
            )
    else:
        lines.append("None.")
    lines.extend(
        [
            "",
            "## Findings",
            "",
        ]
    )
    if not report["findings"]:
        lines.append("No admitted findings.")
    for finding in report["findings"]:
        lines.extend(
            [
                f"### {finding['id']} [{finding['severity']}] {finding['check_id']}",
                "",
                f"- Type: `{finding['finding_type']}`",
                f"- Target: `{finding['target']['path']}` — {escape_markdown(finding['target']['locator'])}",
                f"- Source: `{finding['source']['path']}` — {escape_markdown(finding['source']['symbol'])}",
                f"- Current: {escape_markdown(finding['current'])}",
                f"- Expected: {escape_markdown(finding['expected'])}",
                f"- Rationale: {escape_markdown(finding['rationale'])}",
                f"- Provenance: {', '.join(finding['provenance'])}",
                "",
            ]
        )
    lines.extend(
        [
            "## Checklist Completion",
            "",
            "| Surface | PASS | FAIL | SKIP | BLOCKED |",
            "|---|---:|---:|---:|---:|",
        ]
    )
    for surface in SURFACE_IDS:
        row = report["checklist_completion"][surface]
        lines.append(
            f"| {surface} | {row['PASS']} | {row['FAIL']} | {row['SKIP']} | {row['BLOCKED']} |"
        )
    return "\n".join(lines) + "\n"


def command_validate(args: argparse.Namespace) -> int:
    summary = validate_contract(args.repo_root.resolve())
    print(json.dumps(summary, sort_keys=True))
    return 0


def command_snapshot_verify(args: argparse.Namespace) -> int:
    validate_context(args.repo_root.resolve(), load_json(args.context), verify_snapshot=True)
    print("snapshot: PASS")
    return 0


def command_i18n(args: argparse.Namespace) -> int:
    repo_root = args.repo_root.resolve()
    docs = inventories(repo_root)["website_docs"]
    print(json.dumps(i18n_lag(repo_root, docs, args.days, args.limit), indent=2, sort_keys=True))
    return 0


def command_result_validate(args: argparse.Namespace) -> int:
    repo_root = args.repo_root.resolve()
    context = validate_context(
        repo_root,
        load_json(args.context),
        verify_snapshot=args.verify_snapshot,
    )
    result = validate_result(repo_root, context, load_json(args.input), parse_checks())
    print(f"{result['surface']} result: PASS")
    return 0


def command_merge(args: argparse.Namespace) -> int:
    repo_root = args.repo_root.resolve()
    ensure_external_output(repo_root, args.output)
    if args.output_json:
        ensure_external_output(repo_root, args.output_json)
    context = validate_context(repo_root, load_json(args.context), verify_snapshot=True)
    checks = parse_checks()
    results = []
    for surface in SURFACE_IDS:
        path = args.results / f"{surface}.json"
        results.append(validate_result(repo_root, context, load_json(path), checks))
    report = merge_results(repo_root, context, results, checks)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report_markdown(report), encoding="utf-8")
    if args.output_json:
        write_json(args.output_json, report)
    print(args.output)
    return 3 if report["verdict"] == "INCOMPLETE" else 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=DEFAULT_REPO_ROOT)
    subparsers = parser.add_subparsers(dest="command", required=True)

    validate_parser = subparsers.add_parser("validate", help="validate the skill contract")
    validate_parser.set_defaults(handler=command_validate)

    prepare_parser = subparsers.add_parser("prepare", help="create a deterministic context artifact")
    prepare_parser.add_argument("--checks", type=Path, required=True)
    prepare_parser.add_argument("--output", type=Path, required=True)
    prepare_parser.add_argument("--lag-days", type=int, default=60)
    prepare_parser.add_argument("--lag-limit", type=int, default=3)
    prepare_parser.set_defaults(handler=prepare_context)

    snapshot_parser = subparsers.add_parser("snapshot-verify", help="verify a prepared snapshot")
    snapshot_parser.add_argument("--context", type=Path, required=True)
    snapshot_parser.set_defaults(handler=command_snapshot_verify)

    i18n_parser = subparsers.add_parser("i18n-candidates", help="select deterministic stale locale candidates")
    i18n_parser.add_argument("--days", type=int, default=60)
    i18n_parser.add_argument("--limit", type=int, default=3)
    i18n_parser.set_defaults(handler=command_i18n)

    result_parser = subparsers.add_parser("validate-result", help="validate one subagent JSON result")
    result_parser.add_argument("--context", type=Path, required=True)
    result_parser.add_argument("--input", type=Path, required=True)
    result_parser.add_argument("--verify-snapshot", action="store_true")
    result_parser.set_defaults(handler=command_result_validate)

    merge_parser = subparsers.add_parser("merge", help="validate and merge all surface results")
    merge_parser.add_argument("--context", type=Path, required=True)
    merge_parser.add_argument("--results", type=Path, required=True)
    merge_parser.add_argument("--output", type=Path, required=True)
    merge_parser.add_argument("--output-json", type=Path)
    merge_parser.set_defaults(handler=command_merge)
    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    try:
        return args.handler(args)
    except ContractError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
