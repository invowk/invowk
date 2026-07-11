#!/usr/bin/env python3
# SPDX-License-Identifier: MPL-2.0
"""Offline tests for check-poetic-opening.py."""

from __future__ import annotations

import importlib.util
import sys
import unittest
from pathlib import Path


sys.dont_write_bytecode = True
MODULE_PATH = Path(__file__).with_name("check-poetic-opening.py")
SPEC = importlib.util.spec_from_file_location("check_poetic_opening", MODULE_PATH)
assert SPEC and SPEC.loader
CHECKER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECKER)


def notes(*, order: tuple[str, ...] | None = None, empty: str | None = None) -> str:
    names = order or CHECKER.REQUIRED_SECTIONS
    sections = []
    for name in names:
        if name == "Poetic Opening":
            content = (
                "> Once upon a midnight dreary\n"
                "> While I pondered, weak and weary\n\n"
                '— Edgar Allan Poe, "The Raven"'
            )
        else:
            content = "Nothing required" if name == "Manual Actions Needed" else "None"
        if name == empty:
            content = ""
        sections.append(f"## {name}\n\n{content}")
    return "\n\n".join(sections) + "\n"


class ReleaseNoteStructureTests(unittest.TestCase):
    def test_valid_structure_and_poetry_extraction(self) -> None:
        parsed = CHECKER.required_sections(notes())
        self.assertEqual(set(parsed), set(CHECKER.REQUIRED_SECTIONS))
        self.assertEqual(
            CHECKER.extract_poetic_opening(notes()),
            "Once upon a midnight dreary\nWhile I pondered, weak and weary",
        )

    def test_missing_section_fails(self) -> None:
        order = tuple(name for name in CHECKER.REQUIRED_SECTIONS if name != "Bug Fixes")
        with self.assertRaisesRegex(ValueError, "missing required section"):
            CHECKER.required_sections(notes(order=order))

    def test_duplicate_section_fails(self) -> None:
        with self.assertRaisesRegex(ValueError, "duplicate required section"):
            CHECKER.required_sections(notes() + "\n## Bug Fixes\n\nAgain\n")

    def test_reordered_sections_fail(self) -> None:
        order = list(CHECKER.REQUIRED_SECTIONS)
        order[1], order[2] = order[2], order[1]
        with self.assertRaisesRegex(ValueError, "out of order"):
            CHECKER.required_sections(notes(order=tuple(order)))

    def test_empty_section_fails(self) -> None:
        with self.assertRaisesRegex(ValueError, "empty required section"):
            CHECKER.required_sections(notes(empty="Bug Fixes"))

    def test_duplicate_detection_normalizes_punctuation(self) -> None:
        candidate = "A long-enough poetic opening, with punctuation and accents: été toujours."
        previous = "A long enough poetic opening with punctuation and accents ete toujours"
        self.assertTrue(CHECKER.is_duplicate(candidate, previous))


if __name__ == "__main__":
    unittest.main()
