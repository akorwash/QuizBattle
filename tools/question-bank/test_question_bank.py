#!/usr/bin/env python3
"""Regression tests for the deterministic QuizBattle question-bank pipeline."""

from __future__ import annotations

import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

from generate import encode_jsonl, generate_all, write_outputs
from source_data import COUNTRIES, ELEMENTS, HISTORY_EVENTS, HTTP_STATUSES, SI_FACTS, SURAHS, UN_ADMISSIONS
from validate import EXPECTED_KEYS, expected_content_hash, load_and_validate, validate_question


class QuestionBankTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.questions = generate_all()

    def test_generator_exceeds_target_and_matches_contract(self) -> None:
        self.assertGreaterEqual(len(self.questions), 1000)
        self.assertEqual(1573, len(self.questions))
        for question in self.questions:
            self.assertEqual(EXPECTED_KEYS, set(question))
            self.assertEqual("ar", question["language"])
            self.assertEqual("active", question["status"])
            self.assertEqual(question["contentHash"], expected_content_hash(question))

    def test_curated_source_snapshots_have_expected_cardinality(self) -> None:
        self.assertEqual(118, len(ELEMENTS))
        self.assertEqual(118, len({name for name, _ in ELEMENTS}))
        self.assertEqual(118, len({symbol for _, symbol in ELEMENTS}))
        self.assertEqual(114, len(SURAHS))
        self.assertEqual(114, len(set(SURAHS)))
        self.assertEqual(len(COUNTRIES), len({country for country, _, _ in COUNTRIES}))
        self.assertEqual(len(COUNTRIES), len({capital for _, capital, _ in COUNTRIES}))
        self.assertEqual(len(UN_ADMISSIONS), len({country for country, _ in UN_ADMISSIONS}))
        self.assertEqual(len(HISTORY_EVENTS), len({event for event, _ in HISTORY_EVENTS}))
        self.assertEqual(len(HTTP_STATUSES), len({code for code, _ in HTTP_STATUSES}))
        self.assertEqual(len(SI_FACTS), len({quantity for quantity, _ in SI_FACTS}))
        self.assertEqual(len(SI_FACTS), len({unit for _, unit in SI_FACTS}))

    def test_correct_answer_positions_are_balanced(self) -> None:
        counts = [sum(question["correctOptionIndex"] == index for question in self.questions) for index in range(4)]
        self.assertLessEqual(max(counts) - min(counts), len(self.questions) // 20)

    def test_generation_is_byte_for_byte_deterministic(self) -> None:
        first = encode_jsonl(self.questions)
        second = encode_jsonl(generate_all())
        self.assertEqual(first, second)
        self.assertEqual(hashlib.sha256(first).hexdigest(), hashlib.sha256(second).hexdigest())

    def test_generated_file_passes_full_validator(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            output = Path(temp_dir) / "questions.ar.jsonl"
            manifest = Path(temp_dir) / "manifest.json"
            write_outputs(output, manifest)
            questions, report = load_and_validate(output)
            self.assertEqual(len(self.questions), len(questions))
            self.assertTrue(report["valid"], report["errors"])
            self.assertEqual(0, report["duplicates"]["ids"])
            self.assertEqual(0, report["duplicates"]["prompts"])

    def test_validator_rejects_invalid_correct_index(self) -> None:
        question = copy.deepcopy(self.questions[0])
        question["correctOptionIndex"] = 4
        question["contentHash"] = expected_content_hash(question)
        errors = validate_question(question, 1)
        self.assertTrue(any("correctOptionIndex" in error for error in errors))

    def test_validator_detects_duplicate_normalized_prompt(self) -> None:
        first = copy.deepcopy(self.questions[0])
        second = copy.deepcopy(self.questions[1])
        second["prompt"] = first["prompt"]
        second["contentHash"] = expected_content_hash(second)
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "duplicate.jsonl"
            path.write_text(
                "\n".join(json.dumps(item, ensure_ascii=False) for item in (first, second)) + "\n",
                encoding="utf-8",
            )
            _, report = load_and_validate(path, minimum=0)
            self.assertEqual(1, report["duplicates"]["prompts"])
            self.assertFalse(report["valid"])


if __name__ == "__main__":
    unittest.main()
