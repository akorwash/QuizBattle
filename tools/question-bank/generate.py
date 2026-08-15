#!/usr/bin/env python3
"""Generate QuizBattle's Arabic question bank deterministically and offline."""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import random
from collections import Counter
from pathlib import Path
from typing import Iterable, Sequence

from source_data import (
    COUNTRIES,
    ELEMENTS,
    HISTORY_EVENTS,
    HTTP_STATUSES,
    SI_FACTS,
    SOURCES,
    SURAHS,
    UN_ADMISSIONS,
    VERIFIED_AT,
)


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_OUTPUT = ROOT / "data" / "question-bank" / "questions.ar.jsonl"
DEFAULT_MANIFEST = ROOT / "data" / "question-bank" / "manifest.json"


def seeded_rng(stable_id: str) -> random.Random:
    seed = int.from_bytes(hashlib.sha256(stable_id.encode("utf-8")).digest()[:8], "big")
    return random.Random(seed)


def unique(values: Iterable[str]) -> list[str]:
    result: list[str] = []
    seen: set[str] = set()
    for value in values:
        text = str(value).strip()
        if text and text not in seen:
            seen.add(text)
            result.append(text)
    return result


def choose_distractors(stable_id: str, correct: str, candidates: Sequence[str]) -> list[str]:
    pool = [value for value in unique(candidates) if value != correct]
    if len(pool) < 3:
        raise ValueError(f"{stable_id}: fewer than three unique distractors")
    rng = seeded_rng(stable_id + ":distractors")
    rng.shuffle(pool)
    return pool[:3]


def numeric_distractors(stable_id: str, answer: int, *, positive: bool = True) -> list[str]:
    magnitude = max(1, abs(answer) // 10)
    raw = [
        answer + magnitude,
        answer - magnitude,
        answer + 2 * magnitude,
        answer - 2 * magnitude,
        answer + 1,
        answer - 1,
        answer + 5,
        answer - 5,
        answer + 10,
        answer - 10,
    ]
    if positive:
        raw = [value for value in raw if value >= 0]
    candidates = [str(value) for value in raw if value != answer]
    return choose_distractors(stable_id, str(answer), candidates)


def make_question(
    stable_id: str,
    category: str,
    _subcategory: str,
    difficulty: str,
    prompt: str,
    correct: str,
    distractors: Sequence[str],
    explanation: str,
    source_key: str,
) -> dict:
    options = unique([str(correct), *[str(item) for item in distractors]])
    if len(options) != 4:
        raise ValueError(f"{stable_id}: expected exactly four unique options, got {options!r}")
    seeded_rng(stable_id + ":options").shuffle(options)
    question = {
        "id": stable_id,
        "category": category,
        "difficulty": difficulty,
        "prompt": prompt,
        "options": options,
        "correctOptionIndex": options.index(str(correct)),
        "explanation": explanation,
        "source": copy.deepcopy(SOURCES[source_key]),
        "verifiedAt": VERIFIED_AT,
        "language": "ar",
        "status": "active",
    }
    canonical = json.dumps(question, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    question["contentHash"] = hashlib.sha256(canonical.encode("utf-8")).hexdigest()
    return question


def add_numeric_question(
    questions: list[dict],
    stable_id: str,
    category: str,
    subcategory: str,
    difficulty: str,
    prompt: str,
    answer: int,
    explanation: str,
    source_key: str = "generated_math",
) -> None:
    questions.append(
        make_question(
            stable_id,
            category,
            subcategory,
            difficulty,
            prompt,
            str(answer),
            numeric_distractors(stable_id, answer),
            explanation,
            source_key,
        )
    )


def generate_mathematics() -> list[dict]:
    questions: list[dict] = []

    for i in range(1, 91):
        a = 12 + (i * 17) % 188
        b = 7 + (i * 29) % 173
        answer = a + b
        add_numeric_question(
            questions, f"qb-mathematics-addition-{i:04d}", "mathematics", "addition",
            "easy" if answer < 150 else "medium", f"ما ناتج {a} + {b}؟", answer,
            f"بجمع {a} و{b} يكون الناتج {answer}.",
        )

    for i in range(1, 71):
        b = 8 + (i * 19) % 120
        answer = 15 + (i * 23) % 180
        a = answer + b
        add_numeric_question(
            questions, f"qb-mathematics-subtraction-{i:04d}", "mathematics", "subtraction",
            "easy" if a < 180 else "medium", f"ما ناتج {a} - {b}؟", answer,
            f"بطرح {b} من {a} يتبقى {answer}.",
        )

    for i in range(1, 91):
        a = 2 + (i * 7) % 18
        b = 3 + (i * 11) % 21
        answer = a * b
        add_numeric_question(
            questions, f"qb-mathematics-multiplication-{i:04d}", "mathematics", "multiplication",
            "easy" if max(a, b) <= 12 else "medium", f"ما ناتج {a} × {b}؟", answer,
            f"حاصل ضرب {a} في {b} يساوي {answer}.",
        )

    for i in range(1, 71):
        divisor = 2 + (i * 7) % 18
        answer = 3 + (i * 13) % 31
        dividend = divisor * answer
        add_numeric_question(
            questions, f"qb-mathematics-division-{i:04d}", "mathematics", "division",
            "easy" if dividend <= 144 else "medium", f"ما ناتج {dividend} ÷ {divisor}؟", answer,
            f"لأن {divisor} × {answer} = {dividend}، فإن ناتج القسمة هو {answer}.",
        )

    percentages = [5, 10, 15, 20, 25, 30, 40, 50, 60, 75]
    for i in range(1, 61):
        percent = percentages[(i - 1) % len(percentages)]
        base = 200 * (i + 1)
        answer = base * percent // 100
        add_numeric_question(
            questions, f"qb-mathematics-percentage-{i:04d}", "mathematics", "percentages",
            "medium", f"كم يساوي {percent}% من العدد {base}؟", answer,
            f"نحوّل {percent}% إلى {percent}/100 ثم نضرب في {base}، فيكون الناتج {answer}.",
        )

    for i in range(1, 51):
        coefficient = 2 + (i * 5) % 8
        answer = 2 + (i * 11) % 29
        constant = 1 + (i * 13) % 30
        total = coefficient * answer + constant
        add_numeric_question(
            questions, f"qb-mathematics-linear-{i:04d}", "mathematics", "linear-equations",
            "medium", f"إذا كان {coefficient}س + {constant} = {total}، فما قيمة س؟", answer,
            f"نطرح {constant} من الطرفين فنحصل على {coefficient}س = {total - constant}، ثم نقسم على {coefficient} فتكون س = {answer}.",
        )

    for i in range(1, 41):
        value = i + 10
        answer = value * value
        add_numeric_question(
            questions, f"qb-mathematics-square-{i:04d}", "mathematics", "squares",
            "medium" if value <= 30 else "hard", f"ما مربع العدد {value}؟", answer,
            f"مربع {value} هو {value} × {value} = {answer}.",
        )

    for i in range(1, 31):
        start = 2 + (i * 7) % 31
        step = 2 + (i * 5) % 12
        sequence = [start + step * offset for offset in range(4)]
        answer = sequence[-1] + step
        shown = "، ".join(str(value) for value in sequence)
        add_numeric_question(
            questions, f"qb-mathematics-sequence-{i:04d}", "mathematics", "sequences",
            "medium", f"ما العدد التالي في المتتالية: {shown}، ...؟", answer,
            f"يزداد كل حد بمقدار {step}، لذلك يأتي بعد {sequence[-1]} العدد {answer}.",
        )

    if len(questions) != 500:
        raise AssertionError(f"mathematics generator drifted: {len(questions)}")
    return questions


def nearby_values(values: Sequence[str], index: int, count: int = 8) -> list[str]:
    result: list[str] = []
    radius = 1
    while len(result) < count and radius < len(values):
        result.append(values[(index + radius) % len(values)])
        result.append(values[(index - radius) % len(values)])
        radius += 1
    return unique(result)


def generate_science() -> list[dict]:
    questions: list[dict] = []
    names = [name for name, _ in ELEMENTS]
    symbols = [symbol for _, symbol in ELEMENTS]
    numbers = [str(number) for number in range(1, len(ELEMENTS) + 1)]
    for index, (name, symbol) in enumerate(ELEMENTS):
        number = index + 1
        stable_id = f"qb-science-element-symbol-{number:04d}"
        questions.append(make_question(
            stable_id, "science", "chemistry-elements", "easy" if number <= 36 else "medium",
            f"ما الرمز الكيميائي لعنصر {name}؟", symbol,
            choose_distractors(stable_id, symbol, nearby_values(symbols, index)),
            f"الرمز المعتمد لعنصر {name} هو {symbol}، وعدده الذري {number}.", "iupac",
        ))
        stable_id = f"qb-science-element-number-{number:04d}"
        questions.append(make_question(
            stable_id, "science", "chemistry-elements", "medium",
            f"ما العدد الذري لعنصر {name} ({symbol})؟", str(number),
            choose_distractors(stable_id, str(number), nearby_values(numbers, index)),
            f"يرتب الجدول الدوري عنصر {name} عند العدد الذري {number}.", "iupac",
        ))
    return questions


def generate_geography_and_cities() -> list[dict]:
    questions: list[dict] = []
    country_names = [country for country, _, _ in COUNTRIES]
    capitals = [capital for _, capital, _ in COUNTRIES]
    regions = unique(region for _, _, region in COUNTRIES)
    for index, (country, capital, region) in enumerate(COUNTRIES, start=1):
        idx = index - 1
        regional_capitals = [candidate_capital for _, candidate_capital, candidate_region in COUNTRIES if candidate_region == region]
        regional_countries = [candidate_country for candidate_country, _, candidate_region in COUNTRIES if candidate_region == region]
        capital_candidates = regional_capitals if len(unique(regional_capitals)) >= 4 else unique([*regional_capitals, *nearby_values(capitals, idx, 16)])
        country_candidates = regional_countries if len(unique(regional_countries)) >= 4 else unique([*regional_countries, *nearby_values(country_names, idx, 16)])
        stable_id = f"qb-geography-capital-{index:04d}"
        questions.append(make_question(
            stable_id, "geography", "national-capitals", "easy",
            f"ما عاصمة {country}؟", capital,
            choose_distractors(stable_id, capital, capital_candidates),
            f"عاصمة {country} هي {capital}.", "wikidata",
        ))

        stable_id = f"qb-cities-country-{index:04d}"
        questions.append(make_question(
            stable_id, "cities", "capital-cities", "easy",
            f"مدينة {capital} هي عاصمة أي دولة؟", country,
            choose_distractors(stable_id, country, country_candidates),
            f"{capital} هي عاصمة {country}.", "wikidata",
        ))

        stable_id = f"qb-geography-region-{index:04d}"
        questions.append(make_question(
            stable_id, "geography", "un-m49-regions", "medium",
            f"وفق تصنيف الأمم المتحدة M49، في أي إقليم فرعي تقع {country}؟", region,
            choose_distractors(stable_id, region, regions),
            f"يصنف نظام M49 التابع للأمم المتحدة {country} ضمن إقليم {region}.", "un_m49",
        ))
    return questions


def generate_religion() -> list[dict]:
    questions: list[dict] = []
    chapter_numbers = [str(number) for number in range(1, len(SURAHS) + 1)]
    for index, surah in enumerate(SURAHS):
        number = index + 1
        if number % 2:
            stable_id = f"qb-religion-surah-number-{number:04d}"
            questions.append(make_question(
                stable_id, "religion", "quran-chapter-order", "medium",
                f"ما رقم سورة {surah} في ترتيب سور المصحف؟", str(number),
                choose_distractors(stable_id, str(number), nearby_values(chapter_numbers, index)),
                f"سورة {surah} هي السورة رقم {number} في ترتيب المصحف.", "quran_foundation",
            ))
        else:
            stable_id = f"qb-religion-surah-name-{number:04d}"
            questions.append(make_question(
                stable_id, "religion", "quran-chapter-order", "medium",
                f"ما اسم السورة رقم {number} في ترتيب سور المصحف؟", surah,
                choose_distractors(stable_id, surah, nearby_values(SURAHS, index)),
                f"السورة رقم {number} في ترتيب المصحف هي سورة {surah}.", "quran_foundation",
            ))
    return questions


def generate_civics() -> list[dict]:
    questions: list[dict] = []
    for index, (country, year) in enumerate(UN_ADMISSIONS, start=1):
        stable_id = f"qb-civics-un-admission-{index:04d}"
        plausible_years = [str(year + offset) for offset in (-10, -5, -2, -1, 1, 2, 5, 10)]
        questions.append(make_question(
            stable_id, "civics", "united-nations", "medium",
            f"في أي عام أصبحت {country} عضوًا في الأمم المتحدة؟", str(year),
            choose_distractors(stable_id, str(year), plausible_years),
            f"يسجل دليل الدول الأعضاء بالأمم المتحدة قبول {country} في عام {year}.", "un_members",
        ))
    return questions


def format_historical_year(year: int) -> str:
    return f"{abs(year)} ق.م" if year < 0 else str(year)


def history_distractors(stable_id: str, year: int) -> list[str]:
    span = 10 if abs(year) >= 100 else 5
    candidates = [year - span, year + span, year - 2 * span, year + 2 * span, year - 1, year + 1]
    return choose_distractors(
        stable_id,
        format_historical_year(year),
        [format_historical_year(value) for value in candidates if value != 0],
    )


def generate_history() -> list[dict]:
    questions: list[dict] = []
    for index, (event, year) in enumerate(HISTORY_EVENTS, start=1):
        stable_id = f"qb-history-event-year-{index:04d}"
        display_year = format_historical_year(year)
        questions.append(make_question(
            stable_id, "history", "event-dates", "medium",
            f"في أي عام وقع حدث «{event}»؟", display_year,
            history_distractors(stable_id, year),
            f"التاريخ المسجل لهذا الحدث هو عام {display_year}.", "wikidata",
        ))
    return questions


def generate_technology() -> list[dict]:
    questions: list[dict] = []
    codes = [str(code) for code, _ in HTTP_STATUSES]
    descriptions = [description for _, description in HTTP_STATUSES]
    for index, (code, description) in enumerate(HTTP_STATUSES, start=1):
        idx = index - 1
        stable_id = f"qb-technology-http-code-{index:04d}"
        questions.append(make_question(
            stable_id, "technology", "http-status-codes", "medium",
            f"أي رمز حالة HTTP يعبّر عن «{description}»؟", str(code),
            choose_distractors(stable_id, str(code), nearby_values(codes, idx)),
            f"يسجل IANA الرمز {code} لهذه الحالة في سجل رموز HTTP.", "iana_http",
        ))
        stable_id = f"qb-technology-http-meaning-{index:04d}"
        questions.append(make_question(
            stable_id, "technology", "http-status-codes", "medium",
            f"ما المعنى الأقرب لرمز حالة HTTP {code}؟", description,
            choose_distractors(stable_id, description, nearby_values(descriptions, idx)),
            f"المعنى المسجل للرمز {code} هو «{description}».", "iana_http",
        ))
    return questions


def generate_general_knowledge() -> list[dict]:
    questions: list[dict] = []
    quantities = [quantity for quantity, _ in SI_FACTS]
    units = [unit for _, unit in SI_FACTS]
    for index, (quantity, unit) in enumerate(SI_FACTS, start=1):
        idx = index - 1
        stable_id = f"qb-general-si-unit-{index:04d}"
        questions.append(make_question(
            stable_id, "general-knowledge", "si-units", "easy" if index <= 13 else "medium",
            f"ما وحدة النظام الدولي المستخدمة لقياس {quantity}؟", unit,
            choose_distractors(stable_id, unit, nearby_values(units, idx, 12)),
            f"وحدة قياس {quantity} في النظام الدولي هي {unit}.", "bipm_si",
        ))
        stable_id = f"qb-general-si-quantity-{index:04d}"
        questions.append(make_question(
            stable_id, "general-knowledge", "si-units", "medium",
            f"في النظام الدولي، تُستخدم وحدة {unit} لقياس ماذا؟", quantity,
            choose_distractors(stable_id, quantity, nearby_values(quantities, idx, 12)),
            f"تُستخدم وحدة {unit} لقياس {quantity}.", "bipm_si",
        ))
    return questions


def generate_all() -> list[dict]:
    questions = [
        *generate_mathematics(),
        *generate_science(),
        *generate_geography_and_cities(),
        *generate_religion(),
        *generate_civics(),
        *generate_history(),
        *generate_technology(),
        *generate_general_knowledge(),
    ]
    questions.sort(key=lambda item: item["id"])
    return questions


def encode_jsonl(questions: Sequence[dict]) -> bytes:
    lines = [json.dumps(question, ensure_ascii=False, separators=(",", ":")) for question in questions]
    return ("\n".join(lines) + "\n").encode("utf-8")


def write_outputs(output: Path, manifest_path: Path) -> tuple[int, str]:
    questions = generate_all()
    payload = encode_jsonl(questions)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(payload)
    digest = hashlib.sha256(payload).hexdigest()
    counts = Counter(question["category"] for question in questions)
    difficulties = Counter(question["difficulty"] for question in questions)
    sources = Counter(question["source"]["title"] for question in questions)
    manifest = {
        "schemaVersion": 1,
        "generatedAt": VERIFIED_AT,
        "language": "ar",
        "questionCount": len(questions),
        "sha256": digest,
        "countsByCategory": dict(sorted(counts.items())),
        "countsByDifficulty": dict(sorted(difficulties.items())),
        "countsBySource": dict(sorted(sources.items())),
        "generator": "tools/question-bank/generate.py",
        "output": output.name,
    }
    manifest_path.parent.mkdir(parents=True, exist_ok=True)
    manifest_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return len(questions), digest


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--manifest", type=Path, default=DEFAULT_MANIFEST)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    count, digest = write_outputs(args.output.resolve(), args.manifest.resolve())
    print(f"generated {count} Arabic questions; sha256={digest}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
