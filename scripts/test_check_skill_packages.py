#!/usr/bin/env python3
"""Tests for check-skill-packages.py."""

from __future__ import annotations

import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path


sys.dont_write_bytecode = True
SCRIPT = Path(__file__).with_name("check-skill-packages.py")
SPEC = importlib.util.spec_from_file_location("check_skill_packages", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class SkillPackageValidationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary_directory.cleanup)
        self.root = Path(self.temporary_directory.name)
        (self.root / ".agents" / "skills").mkdir(parents=True)

    def write_skill(
        self,
        name: str = "example-skill",
        *,
        frontmatter_extra: str = "",
        with_interface: bool = True,
        body_lines: int = 1,
    ) -> Path:
        skill = self.root / ".agents" / "skills" / name
        skill.mkdir()
        extra = f"{frontmatter_extra}\n" if frontmatter_extra else ""
        body = "\n".join("Instruction." for _ in range(body_lines))
        (skill / "SKILL.md").write_text(
            f"---\nname: {name}\ndescription: Use this skill for example work.\n{extra}---\n\n{body}\n",
            encoding="utf-8",
        )
        if with_interface:
            agents = skill / "agents"
            agents.mkdir()
            (agents / "openai.yaml").write_text(
                "interface:\n"
                '  display_name: "Example Skill"\n'
                '  short_description: "Handle representative example skill tasks"\n'
                f'  default_prompt: "Use ${name} to complete the example task."\n',
                encoding="utf-8",
            )
        return skill

    def test_valid_skill_passes(self) -> None:
        self.write_skill()
        self.assertEqual([], MODULE.validate_root(self.root))

    def test_rejects_unsupported_frontmatter(self) -> None:
        self.write_skill(frontmatter_extra="metadata: unsupported")
        errors = MODULE.validate_root(self.root)
        self.assertTrue(any("unsupported frontmatter keys" in error for error in errors))

    def test_requires_interface_metadata(self) -> None:
        self.write_skill(with_interface=False)
        errors = MODULE.validate_root(self.root)
        self.assertTrue(any("missing agents/openai.yaml" in error for error in errors))

    def test_rejects_oversized_skill_body(self) -> None:
        self.write_skill(body_lines=501)
        errors = MODULE.validate_root(self.root)
        self.assertTrue(any("move detail into references" in error for error in errors))

    def test_excludes_unmanaged_skill_families(self) -> None:
        self.write_skill("speckit.example", with_interface=False)
        self.write_skill("openspec-example", with_interface=False)
        self.assertEqual([], MODULE.validate_root(self.root))


if __name__ == "__main__":
    unittest.main()
