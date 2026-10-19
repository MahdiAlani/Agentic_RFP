"""Quality checks a drafted RFP answer must pass before sign-off.

Shared by the MCP server (which refuses to approve an answer with a failing
blocker) and the eval harness (which reports pass rates across a fixed set of
questions). Nine checks are deterministic; four need judgement and are made
by an LLM in a single call.

Severity: a "blocker" must pass before approval; a "warning" is advisory.
"""

import json
import logging
import re
from dataclasses import asdict, dataclass, field

from src import citations
from src.config import OPENAI_JUDGE_MODEL
from src.llm import chat

log = logging.getLogger("quality")

BLOCKER = "blocker"
WARNING = "warning"

MIN_WORDS = 15
MAX_WORDS = 350


@dataclass
class Draft:
    """An answer under review. `citations` are dicts with position and content."""

    question: str
    text: str
    citations: list[dict] = field(default_factory=list)
    # Other answers in the same RFP, for the consistency check.
    other_answers: list[str] = field(default_factory=list)


@dataclass(frozen=True)
class CheckResult:
    name: str
    passed: bool
    severity: str
    detail: str = ""

    def to_dict(self) -> dict:
        return asdict(self)


# --- deterministic checks -----------------------------------------------------

_PLACEHOLDERS = re.compile(
    r"\b(TBD|TODO|TBC|lorem ipsum|XXX+)\b|\[(insert|company|customer|name)[^\]]*\]|\{\{.*?\}\}",
    re.IGNORECASE,
)
_SUPERLATIVES = re.compile(
    r"\b(best[- ]in[- ]class|world[- ]class|industry[- ]leading|cutting[- ]edge|"
    r"unparalleled|unmatched|revolutionary|state[- ]of[- ]the[- ]art|seamless(ly)?)\b",
    re.IGNORECASE,
)
_AI_ARTIFACTS = re.compile(
    r"\b(as an ai|language model|the (provided )?context|based on the (provided )?(context|documents|passages)|"
    r"i (do not|don't) have access|i cannot|i'm unable)\b",
    re.IGNORECASE,
)
_HEDGES = re.compile(r"\b(we believe|probably|possibly|it is likely|should be able to|to our knowledge)\b", re.IGNORECASE)
# 99.95%, $25,000, 1,000, 24, 2026 - but not the [n] citation markers.
_NUMBER = re.compile(r"(?<!\[)(?<![\d,])\$?\d[\d,]*(?:\.\d+)?%?")


def _normalize_number(token: str) -> str:
    return token.replace("$", "").replace(",", "").rstrip("%")


def _numbers(text: str) -> set[str]:
    text = re.sub(r"\[\d+(?:\s*,\s*\d+)*\]", " ", text)  # drop citation markers
    return {_normalize_number(m.group()) for m in _NUMBER.finditer(text)}


def _present(d: Draft) -> CheckResult:
    ok = bool(d.text.strip())
    return CheckResult("answer_present", ok, BLOCKER, "" if ok else "The answer is empty.")


def _has_citations(d: Draft) -> CheckResult:
    ok = bool(d.citations)
    return CheckResult(
        "has_citations", ok, BLOCKER,
        "" if ok else "No supporting sources: the knowledge base may not cover this question.",
    )


def _markers_valid(d: Draft) -> CheckResult:
    used = citations.markers(d.text)
    available = {c["position"] for c in d.citations}
    dangling, unused = sorted(used - available), sorted(available - used)
    problems = []
    if dangling:
        problems.append(f"markers with no source: {dangling}")
    if unused:
        problems.append(f"sources never referenced: {unused}")
    return CheckResult("citation_markers_valid", not problems, BLOCKER, "; ".join(problems))


def _pattern_check(name: str, severity: str, pattern: re.Pattern, what: str):
    def check(d: Draft) -> CheckResult:
        hits = sorted({m.group(0) for m in pattern.finditer(d.text)}, key=str.lower)
        return CheckResult(name, not hits, severity, f"{what}: {', '.join(hits)}" if hits else "")
    return check


def _length(d: Draft) -> CheckResult:
    words = len(d.text.split())
    ok = MIN_WORDS <= words <= MAX_WORDS
    return CheckResult(
        "length_reasonable", ok, WARNING,
        "" if ok else f"{words} words; expected {MIN_WORDS}-{MAX_WORDS}.",
    )


def _numbers_grounded(d: Draft) -> CheckResult:
    """Every figure in the answer should appear in a cited passage (or the question)."""
    sources = _numbers(" ".join(c.get("content", "") for c in d.citations) + " " + d.question)
    invented = sorted(_numbers(d.text) - sources)
    return CheckResult(
        "numbers_grounded", not invented, BLOCKER,
        f"figures not found in any cited source: {', '.join(invented)}" if invented else "",
    )


DETERMINISTIC_CHECKS = [
    _present,
    _has_citations,
    _markers_valid,
    _pattern_check("no_placeholders", BLOCKER, _PLACEHOLDERS, "placeholder text"),
    _length,
    _numbers_grounded,
    _pattern_check("no_marketing_language", WARNING, _SUPERLATIVES, "unsupported superlatives"),
    _pattern_check("no_ai_artifacts", BLOCKER, _AI_ARTIFACTS, "reads as AI output"),
    _pattern_check("no_hedging", WARNING, _HEDGES, "hedging language"),
]


# --- LLM-judged checks --------------------------------------------------------

# name -> (severity, criterion shown to the judge)
JUDGED_CHECKS = {
    "grounded": (BLOCKER, "Every factual claim in the answer is supported by the cited passages."),
    "answers_question": (BLOCKER, "The answer directly addresses every part of the question."),
    "professional_tone": (WARNING, "The tone is clear, specific and professional, suitable for a formal proposal."),
    "consistent": (BLOCKER, "The answer does not contradict any of the other answers in this RFP."),
}

JUDGE_PROMPT = """You are reviewing a drafted answer to an RFP question before it is sent to a customer.

Evaluate the answer against each criterion and reply with a JSON object of the form:
{{"<criterion name>": {{"pass": true or false, "reason": "<one sentence>"}}, ...}}

CRITERIA:
{criteria}

QUESTION:
{question}

ANSWER:
{answer}

CITED PASSAGES:
{passages}

OTHER ANSWERS IN THIS RFP:
{others}"""


def judge(d: Draft) -> list[CheckResult]:
    """Ask the model to grade the answer on every judged criterion in one call."""
    criteria = "\n".join(f"- {name}: {text}" for name, (_, text) in JUDGED_CHECKS.items())
    passages = "\n\n".join(f"[{c['position']}] {c.get('content', '')}" for c in d.citations) or "(none)"
    others = "\n\n".join(f"- {a}" for a in d.other_answers) or "(none)"
    prompt = JUDGE_PROMPT.format(criteria=criteria, question=d.question, answer=d.text,
                                 passages=passages, others=others)

    try:
        verdicts = json.loads(chat([{"role": "user", "content": prompt}], temperature=0, json_mode=True,
                                   model=OPENAI_JUDGE_MODEL))
    except Exception as e:  # network, quota or malformed JSON: report rather than crash the review
        log.exception("quality judge failed")
        return [CheckResult(name, False, sev, f"judge unavailable: {e}") for name, (sev, _) in JUDGED_CHECKS.items()]

    results = []
    for name, (severity, _) in JUDGED_CHECKS.items():
        verdict = verdicts.get(name) or {}
        results.append(CheckResult(name, verdict.get("pass") is True, severity,
                                   str(verdict.get("reason", "no verdict returned"))))
    return results


# --- entry point ----------------------------------------------------------------

def run_checks(d: Draft, use_llm: bool = True) -> list[CheckResult]:
    """Run all checks. Without an answer there's nothing for the judge to read."""
    results = [check(d) for check in DETERMINISTIC_CHECKS]
    if use_llm and d.text.strip():
        results += judge(d)
    return results


def blockers(results: list[CheckResult]) -> list[CheckResult]:
    return [r for r in results if not r.passed and r.severity == BLOCKER]
