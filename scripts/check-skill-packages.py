#!/usr/bin/env python3
"""Validate repository skill package structure and interface metadata."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


SKILL_NAME_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
TOP_LEVEL_KEY_RE = re.compile(r"^([A-Za-z0-9_-]+):(?:\s|$)")
INTERFACE_FIELD_RE = re.compile(r"^  ([A-Za-z0-9_-]+):\s*(.*)$")
EXCLUDED_PREFIXES = ("openspec-", "speckit.")
MAX_SKILL_LINES = 500


def unquote(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
        return value[1:-1]
    return value


def parse_frontmatter(skill_md: Path) -> tuple[dict[str, str], list[str]]:
    errors: list[str] = []
    lines = skill_md.read_text(encoding="utf-8").splitlines()
    if not lines or lines[0] != "---":
        return {}, ["SKILL.md must start with YAML frontmatter"]

    try:
        end = lines.index("---", 1)
    except ValueError:
        return {}, ["SKILL.md frontmatter is not terminated"]

    frontmatter = lines[1:end]
    keys: dict[str, str] = {}
    current_key: str | None = None
    for line in frontmatter:
        match = TOP_LEVEL_KEY_RE.match(line)
        if match:
            current_key = match.group(1)
            keys[current_key] = line.split(":", 1)[1].strip()
            continue
        if current_key and line.startswith((" ", "\t")):
            keys[current_key] = f"{keys[current_key]} {line.strip()}".strip()
            continue
        if line.strip():
            errors.append(f"invalid frontmatter line: {line}")

    unexpected = sorted(set(keys) - {"name", "description"})
    if unexpected:
        errors.append(f"unsupported frontmatter keys: {', '.join(unexpected)}")
    for required in ("name", "description"):
        if required not in keys:
            errors.append(f"missing frontmatter key: {required}")

    description = keys.get("description", "")
    if description in {"", ">", ">-", "|", "|-"}:
        errors.append("description must not be empty")
    return keys, errors


def parse_interface(openai_yaml: Path) -> tuple[dict[str, str], list[str]]:
    if not openai_yaml.is_file():
        return {}, ["missing agents/openai.yaml"]

    fields: dict[str, str] = {}
    errors: list[str] = []
    in_interface = False
    for line in openai_yaml.read_text(encoding="utf-8").splitlines():
        if line == "interface:":
            in_interface = True
            continue
        if in_interface and line and not line.startswith(" "):
            break
        if not in_interface:
            continue
        match = INTERFACE_FIELD_RE.match(line)
        if match:
            fields[match.group(1)] = unquote(match.group(2))

    for required in ("display_name", "short_description", "default_prompt"):
        if not fields.get(required):
            errors.append(f"missing interface field: {required}")
    description = fields.get("short_description", "")
    if description and not 25 <= len(description) <= 64:
        errors.append("short_description must contain 25-64 characters")
    return fields, errors


def validate_skill(skill_dir: Path) -> list[str]:
    errors: list[str] = []
    skill_md = skill_dir / "SKILL.md"
    if not skill_md.is_file():
        return ["missing SKILL.md"]

    lines = skill_md.read_text(encoding="utf-8").splitlines()
    if len(lines) > MAX_SKILL_LINES:
        errors.append(
            f"SKILL.md has {len(lines)} lines; move detail into references to stay at or below {MAX_SKILL_LINES}"
        )

    frontmatter, frontmatter_errors = parse_frontmatter(skill_md)
    errors.extend(frontmatter_errors)
    name = unquote(frontmatter.get("name", ""))
    if name and name != skill_dir.name:
        errors.append(f"frontmatter name {name!r} does not match directory {skill_dir.name!r}")
    if name and not SKILL_NAME_RE.fullmatch(name):
        errors.append(f"invalid skill name: {name!r}")

    interface, interface_errors = parse_interface(skill_dir / "agents" / "openai.yaml")
    errors.extend(interface_errors)
    prompt = interface.get("default_prompt", "")
    if name and prompt and f"${name}" not in prompt:
        errors.append(f"default_prompt must explicitly mention ${name}")
    return errors


def validate_root(root: Path) -> list[str]:
    skills_root = root / ".agents" / "skills"
    errors: list[str] = []
    for skill_dir in sorted(path for path in skills_root.iterdir() if path.is_dir()):
        if skill_dir.name.startswith(EXCLUDED_PREFIXES):
            continue
        for error in validate_skill(skill_dir):
            errors.append(f"{skill_dir.relative_to(root)}: {error}")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--root",
        type=Path,
        default=Path(__file__).resolve().parent.parent,
        help="repository root (defaults to the parent of scripts/)",
    )
    args = parser.parse_args()

    errors = validate_root(args.root.resolve())
    if errors:
        for error in errors:
            print(f"ERROR: {error}")
        return 1
    print("Skill package validation passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
