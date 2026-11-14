"""The offline stand-in model must carry the demo end to end without a key."""

from pathlib import Path

import pytest

from src import citations, llm, quality
from src.extract_worker import _extract_questions
from src.generate_worker import CITATION_RULES, _system_prompt
from src.text import chunk_text

DEMO = Path(__file__).resolve().parents[2] / "demo"


@pytest.fixture(autouse=True)
def keyless(monkeypatch):
    monkeypatch.setattr(llm, "OPENAI_API_KEY", "")


def knowledge_base() -> list[dict]:
    chunks = []
    for path in sorted((DEMO / "knowledge-base").glob("*.md")):
        for i, content in enumerate(chunk_text(path.read_text(encoding="utf-8"))):
            chunks.append({"document_id": path.stem, "file_name": path.name, "chunk_index": i, "content": content})
    return chunks


def draft(question: str) -> quality.Draft:
    """Draft an answer the way generate_worker does, with every passage as context."""
    chunks = knowledge_base()
    raw = llm.chat([
        {"role": "system", "content": _system_prompt(chunks, CITATION_RULES)},
        {"role": "user", "content": question},
    ])
    text, cited = citations.resolve(raw, chunks)
    return quality.Draft(
        question=question,
        text=text,
        citations=[{"position": n, "content": c["content"]} for n, c in enumerate(cited, start=1)],
    )


def test_extracts_every_demo_question_in_order():
    rfp = (DEMO / "rfp" / "city-of-ashford-rfp.md").read_text(encoding="utf-8")
    questions = _extract_questions(rfp)

    assert len(questions) == 13
    assert questions[0].startswith("3.1.1 Describe how City data is encrypted at rest and in transit")
    assert questions[0].endswith("including how encryption keys are managed.")  # wrapped line re-joined
    assert questions[-1].startswith("3.5.1 Provide annual subscription pricing")


def test_covered_question_gets_a_cited_answer_that_passes_the_checks():
    d = draft("3.2.1 State your guaranteed uptime and the remedies available if it is not met.")

    assert "99.95%" in d.text
    assert d.citations
    failed = [r for r in quality.run_checks(d) if not r.passed and r.severity == quality.BLOCKER]
    assert not failed, failed


def test_uncovered_question_is_declined_without_sources():
    d = draft("3.4.2 Provide three references from municipal government customers of similar size.")

    assert d.text == "The product documentation does not cover this question, so it needs input from someone who knows the answer."
    assert not d.citations


def test_judge_defers_to_the_deterministic_checks():
    d = draft("3.2.1 State your guaranteed uptime and the remedies available if it is not met.")
    verdicts = quality.judge(d)

    assert {v.name for v in verdicts} == set(quality.JUDGED_CHECKS)
    assert all(v.passed and "Not judged" in v.detail for v in verdicts)


def test_chat_without_context_declines():
    assert llm.chat([{"role": "user", "content": "Hello"}]).startswith("The product documentation does not cover")
    assert llm.chat([{"role": "user", "content": "{}"}], json_mode=True) == "{}"
