#!/usr/bin/env python3
"""Validate QuizBattle's Arabic JSONL question bank and write an audit report."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import unicodedata
from collections import Counter, defaultdict
from datetime import datetime
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_INPUT = ROOT / "data" / "question-bank" / "questions.ar.jsonl"
DEFAULT_REPORT = ROOT / "data" / "question-bank" / "validation-report.json"

EXPECTED_KEYS = {
    "id", "category", "difficulty", "prompt", "options", "correctOptionIndex",
    "explanation", "source", "verifiedAt", "language", "status", "contentHash",
}
EXPECTED_SOURCE_KEYS = {"type", "title", "url", "license"}
ALLOWED_CATEGORIES = {
    "mathematics", "science", "geography", "cities", "history", "civics",
    "religion", "technology", "general-knowledge",
}
MINIMUM_BY_CATEGORY = {
    "mathematics": 400,
    "science": 200,
    "geography": 100,
    "cities": 100,
    "history": 40,
    "civics": 40,
    "religion": 100,
    "technology": 50,
    "general-knowledge": 40,
}
ID_PATTERN = re.compile(r"^qb-[a-z0-9-]+-[0-9]{4}$")
HASH_PATTERN = re.compile(r"^[0-9a-f]{64}$")
ARABIC_PATTERN = re.compile(r"[\u0600-\u06ff]")
VOLATILE_POLITICS_PATTERN = re.compile(
    r"(?:الرئيس|رئيس الوزراء|الملك|الأمير|الوزير|الحاكم)\s+(?:الحالي|الحالية)|"
    r"من\s+(?:هو|هي)\s+(?:رئيس|ملك|أمير|وزير)",
    re.IGNORECASE,
)


def normalize_text(value: str) -> str:
    text = unicodedata.normalize("NFKC", value).lower().strip()
    text = re.sub(r"[\u064b-\u065f\u0670\u06d6-\u06edـ]", "", text)
    text = text.translate(str.maketrans({"أ": "ا", "إ": "ا", "آ": "ا", "ى": "ي", "ؤ": "و", "ئ": "ي"}))
    text = re.sub(r"[^\w\u0600-\u06ff]+", " ", text, flags=re.UNICODE)
    return " ".join(text.split())


def expected_content_hash(question: dict[str, Any]) -> str:
    payload = {key: value for key, value in question.items() if key != "contentHash"}
    canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def is_rfc3339(value: str) -> bool:
    if not isinstance(value, str) or "T" not in value:
        return False
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return False
    return parsed.tzinfo is not None


def validate_question(question: Any, line_number: int) -> list[str]:
    prefix = f"line {line_number}"
    errors: list[str] = []
    if not isinstance(question, dict):
        return [f"{prefix}: question must be an object"]

    keys = set(question)
    missing = sorted(EXPECTED_KEYS - keys)
    extra = sorted(keys - EXPECTED_KEYS)
    if missing:
        errors.append(f"{prefix}: missing fields: {', '.join(missing)}")
    if extra:
        errors.append(f"{prefix}: unknown fields: {', '.join(extra)}")
    if missing:
        return errors

    stable_id = question["id"]
    if not isinstance(stable_id, str) or not ID_PATTERN.fullmatch(stable_id):
        errors.append(f"{prefix}: invalid stable id")

    if question["category"] not in ALLOWED_CATEGORIES:
        errors.append(f"{prefix}: unsupported category {question['category']!r}")
    if question["difficulty"] not in {"easy", "medium", "hard"}:
        errors.append(f"{prefix}: difficulty must be easy, medium, or hard")

    prompt = question["prompt"]
    if not isinstance(prompt, str) or not (10 <= len(prompt.strip()) <= 220):
        errors.append(f"{prefix}: prompt length must be 10..220 characters")
    elif not ARABIC_PATTERN.search(prompt):
        errors.append(f"{prefix}: prompt must contain Arabic text")
    elif VOLATILE_POLITICS_PATTERN.search(prompt):
        errors.append(f"{prefix}: prompt contains a volatile office-holder fact")

    options = question["options"]
    if not isinstance(options, list) or len(options) != 4:
        errors.append(f"{prefix}: options must contain exactly four entries")
    elif any(not isinstance(option, str) or not option.strip() or len(option) > 120 for option in options):
        errors.append(f"{prefix}: every option must be a non-empty string of at most 120 characters")
    elif len({normalize_text(option) for option in options}) != 4:
        errors.append(f"{prefix}: options must be unique after Arabic normalization")

    correct_index = question["correctOptionIndex"]
    if isinstance(correct_index, bool) or not isinstance(correct_index, int) or not 0 <= correct_index <= 3:
        errors.append(f"{prefix}: correctOptionIndex must be an integer from 0 to 3")

    explanation = question["explanation"]
    if not isinstance(explanation, str) or not (10 <= len(explanation.strip()) <= 500):
        errors.append(f"{prefix}: explanation length must be 10..500 characters")
    elif not ARABIC_PATTERN.search(explanation):
        errors.append(f"{prefix}: explanation must contain Arabic text")

    source = question["source"]
    if not isinstance(source, dict):
        errors.append(f"{prefix}: source must be an object")
    else:
        missing_source = sorted(EXPECTED_SOURCE_KEYS - set(source))
        extra_source = sorted(set(source) - EXPECTED_SOURCE_KEYS)
        if missing_source:
            errors.append(f"{prefix}: source missing fields: {', '.join(missing_source)}")
        if extra_source:
            errors.append(f"{prefix}: source has unknown fields: {', '.join(extra_source)}")
        for field in EXPECTED_SOURCE_KEYS:
            if field in source and (not isinstance(source[field], str) or not source[field].strip()):
                errors.append(f"{prefix}: source.{field} must be a non-empty string")
        if isinstance(source.get("url"), str) and not source["url"].startswith(("https://", "urn:")):
            errors.append(f"{prefix}: source.url must use https or urn")
        if source.get("type") == "generated" and question["category"] != "mathematics":
            errors.append(f"{prefix}: generated sources are permitted only for mathematics")

    if not is_rfc3339(question["verifiedAt"]):
        errors.append(f"{prefix}: verifiedAt must be an RFC3339 timestamp with timezone")
    if question["language"] != "ar":
        errors.append(f"{prefix}: language must be ar")
    if question["status"] != "active":
        errors.append(f"{prefix}: status must be active")

    content_hash = question["contentHash"]
    if not isinstance(content_hash, str) or not HASH_PATTERN.fullmatch(content_hash):
        errors.append(f"{prefix}: contentHash must be lowercase SHA-256 hex")
    elif content_hash != expected_content_hash(question):
        errors.append(f"{prefix}: contentHash does not match canonical question content")
    return errors


def load_and_validate(path: Path, minimum: int = 1000) -> tuple[list[dict], dict[str, Any]]:
    errors: list[str] = []
    warnings: list[str] = []
    questions: list[dict] = []
    raw = path.read_bytes()
    if raw.startswith(b"\xef\xbb\xbf"):
        errors.append("file must be UTF-8 without BOM")
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as exc:
        return [], {"valid": False, "errors": [f"file is not valid UTF-8: {exc}"], "warnings": []}
    if text and not text.endswith("\n"):
        warnings.append("file should end with a newline")

    for line_number, line in enumerate(text.splitlines(), start=1):
        if not line.strip():
            errors.append(f"line {line_number}: blank JSONL lines are not allowed")
            continue
        try:
            question = json.loads(line)
        except json.JSONDecodeError as exc:
            errors.append(f"line {line_number}: invalid JSON: {exc.msg}")
            continue
        errors.extend(validate_question(question, line_number))
        if isinstance(question, dict):
            questions.append(question)

    id_lines: dict[str, list[int]] = defaultdict(list)
    prompt_lines: dict[str, list[int]] = defaultdict(list)
    for line_number, question in enumerate(questions, start=1):
        if isinstance(question.get("id"), str):
            id_lines[question["id"]].append(line_number)
        if isinstance(question.get("prompt"), str):
            prompt_lines[normalize_text(question["prompt"])].append(line_number)
    duplicate_ids = {key: value for key, value in id_lines.items() if len(value) > 1}
    duplicate_prompts = {key: value for key, value in prompt_lines.items() if key and len(value) > 1}
    if duplicate_ids:
        errors.append(f"duplicate ids detected: {len(duplicate_ids)}")
    if duplicate_prompts:
        errors.append(f"duplicate normalized prompts detected: {len(duplicate_prompts)}")

    if len(questions) < minimum:
        errors.append(f"question count {len(questions)} is below required minimum {minimum}")

    categories = Counter(q.get("category") for q in questions)
    difficulties = Counter(q.get("difficulty") for q in questions)
    statuses = Counter(q.get("status") for q in questions)
    source_types = Counter(q.get("source", {}).get("type") for q in questions if isinstance(q.get("source"), dict))
    correct_positions = Counter(q.get("correctOptionIndex") for q in questions)
    for category, required in MINIMUM_BY_CATEGORY.items():
        if categories[category] < required:
            errors.append(f"category {category} has {categories[category]} questions; minimum is {required}")

    report: dict[str, Any] = {
        "valid": not errors,
        "input": path.name,
        "questionCount": len(questions),
        "minimumRequired": minimum,
        "sha256": hashlib.sha256(raw).hexdigest(),
        "countsByCategory": dict(sorted(categories.items(), key=lambda item: str(item[0]))),
        "countsByDifficulty": dict(sorted(difficulties.items(), key=lambda item: str(item[0]))),
        "countsByStatus": dict(sorted(statuses.items(), key=lambda item: str(item[0]))),
        "countsBySourceType": dict(sorted(source_types.items(), key=lambda item: str(item[0]))),
        "correctOptionDistribution": {str(key): value for key, value in sorted(correct_positions.items(), key=lambda item: str(item[0]))},
        "duplicates": {
            "ids": len(duplicate_ids),
            "prompts": len(duplicate_prompts),
        },
        "errors": errors,
        "warnings": warnings,
    }
    return questions, report


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, default=DEFAULT_INPUT)
    parser.add_argument("--report", type=Path, default=DEFAULT_REPORT)
    parser.add_argument("--minimum", type=int, default=1000)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    _, report = load_and_validate(args.input.resolve(), args.minimum)
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    if report["valid"]:
        print(
            f"valid: {report['questionCount']} questions; "
            f"duplicate ids={report['duplicates']['ids']}; "
            f"duplicate prompts={report['duplicates']['prompts']}"
        )
        return 0
    print(f"invalid: {len(report['errors'])} error(s); see {args.report}")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
