import json

import pytest

from src import quality
from src.quality import BLOCKER, DETERMINISTIC_CHECKS, JUDGED_CHECKS, Draft, blockers, run_checks

PASSAGES = [
    {"position": 1, "content": "Corvane commits to 99.95% monthly uptime for the Enterprise plan."},
    {"position": 2, "content": "Customer data is backed up every hour and retained for 30 days."},
]
GOOD = Draft(
    question="3.2.1 State your guaranteed uptime and backup retention.",
    text="Corvane commits to 99.95% monthly uptime on the Enterprise plan [1]. "
         "Customer data is backed up every hour and backups are retained for 30 days [2].",
    citations=PASSAGES,
)


def results_by_name(d: Draft) -> dict:
    return {r.name: r for r in run_checks(d, use_llm=False)}


def failing(d: Draft) -> set[str]:
    return {name for name, r in results_by_name(d).items() if not r.passed}


def with_text(text: str, **kw) -> Draft:
    return Draft(question=GOOD.question, text=text, citations=kw.get("citations", PASSAGES))


def test_there_are_thirteen_checks():
    assert len(DETERMINISTIC_CHECKS) + len(JUDGED_CHECKS) == 13


def test_well_formed_answer_passes_every_deterministic_check():
    assert failing(GOOD) == set()


def test_empty_answer():
    assert {"answer_present", "length_reasonable"} <= failing(with_text("  "))


def test_uncited_answer_is_a_blocker():
    d = with_text("The documentation does not describe guard staffing at our data centers.", citations=[])
    assert "has_citations" in {r.name for r in blockers(run_checks(d, use_llm=False))}


def test_dangling_and_unused_markers():
    r = results_by_name(with_text(GOOD.text.replace("[2]", "[3]")))["citation_markers_valid"]
    assert not r.passed
    assert "[3]" in r.detail and "[2]" in r.detail


@pytest.mark.parametrize("text", [
    "Uptime is TBD [1] [2].",
    "Contact [insert account manager] for details [1] [2].",
    "Dear {{customer_name}}, uptime is 99.95% [1] [2].",
])
def test_placeholders(text):
    assert "no_placeholders" in failing(with_text(text))


def test_invented_figures_are_caught():
    r = results_by_name(with_text(GOOD.text.replace("30 days", "90 days")))["numbers_grounded"]
    assert not r.passed and "90" in r.detail


def test_figures_from_the_question_and_markers_are_allowed():
    # "3.2.1" comes from the question; [1]/[2] are citation markers, not figures.
    d = with_text("Regarding 3.2.1: Corvane commits to 99.95% uptime [1], with hourly backups kept 30 days [2].")
    assert "numbers_grounded" not in failing(d)


def test_formatted_numbers_match_sources():
    d = Draft(question="Price?", text="The fee is $25,000 [1], about 1,000 hours of work [1].",
              citations=[{"position": 1, "content": "a one-time fee of $25000 covering 1000 hours"}])
    assert "numbers_grounded" not in failing(d)


@pytest.mark.parametrize("check,phrase", [
    ("no_marketing_language", "our industry-leading platform"),
    ("no_ai_artifacts", "Based on the provided context,"),
    ("no_ai_artifacts", "As an AI, I"),
    ("no_hedging", "we believe that"),
])
def test_phrase_checks(check, phrase):
    assert check in failing(with_text(f"{phrase} {GOOD.text}"))


def test_length_bounds():
    assert "length_reasonable" in failing(with_text("Yes [1] [2]."))
    assert "length_reasonable" in failing(with_text(("word " * 400) + "[1] [2]"))


def test_warnings_do_not_block():
    d = with_text("Our industry-leading platform: " + GOOD.text)
    assert blockers(run_checks(d, use_llm=False)) == []


class TestJudge:
    def test_parses_verdicts(self, monkeypatch):
        seen = {}

        def fake_chat(messages, temperature, json_mode, model):
            seen["prompt"] = messages[0]["content"]
            assert json_mode and temperature == 0
            assert model == quality.OPENAI_JUDGE_MODEL
            return json.dumps({
                "grounded": {"pass": True, "reason": "All claims cited."},
                "answers_question": {"pass": False, "reason": "Ignores retention."},
                "professional_tone": {"pass": True, "reason": "Fine."},
                "consistent": {"pass": True, "reason": "No conflicts."},
            })

        monkeypatch.setattr(quality, "chat", fake_chat)
        d = Draft(GOOD.question, GOOD.text, PASSAGES, other_answers=["Support is 24/7."])
        results = {r.name: r for r in run_checks(d)}

        assert results["answers_question"].passed is False
        assert results["answers_question"].detail == "Ignores retention."
        assert results["grounded"].passed is True
        assert "[2] Customer data is backed up" in seen["prompt"]
        assert "Support is 24/7." in seen["prompt"]

    def test_missing_verdict_fails_closed(self, monkeypatch):
        monkeypatch.setattr(quality, "chat", lambda *a, **k: json.dumps({"grounded": {"pass": "yes"}}))
        results = {r.name: r for r in quality.judge(GOOD)}
        assert not results["grounded"].passed  # "yes" is not true
        assert not results["consistent"].passed

    def test_judge_failure_is_reported_not_raised(self, monkeypatch):
        def boom(*a, **k):
            raise RuntimeError("quota exceeded")

        monkeypatch.setattr(quality, "chat", boom)
        results = quality.judge(GOOD)
        assert len(results) == 4 and not any(r.passed for r in results)
        assert all("quota exceeded" in r.detail for r in results)

    def test_skipped_for_empty_answer(self, monkeypatch):
        monkeypatch.setattr(quality, "chat", lambda *a, **k: pytest.fail("judge called"))
        assert len(run_checks(with_text(""))) == len(DETERMINISTIC_CHECKS)

    def test_blocker_severity(self):
        assert JUDGED_CHECKS["grounded"][0] == BLOCKER
