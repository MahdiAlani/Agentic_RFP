"""Offline stand-in for the chat model, used when OPENAI_API_KEY is not set.

It is not a language model. It recognizes the three kinds of prompt the workers
send and answers each with a small deterministic heuristic, so a keyless run (a
fresh clone, CI) still exercises the whole pipeline with plausible output:

- question extraction: the numbered paragraphs of the RFP that ask something
- drafting and chat: quotes the sentences of the context passage that shares
  the most words with the question, citing it, or declines when none does
- quality judge: passes the judged criteria, saying they were not judged; the
  deterministic checks (citations, figures, ...) still apply in full
"""

import json
import re

# Markers from the prompts in extract_worker, generate_worker and quality.
EXTRACT_MARKER = "You are reading a Request for Proposal"
JUDGE_MARKER = "Evaluate the answer against each criterion"
CONTEXT_MARKER = "\nCONTEXT:\n"

NOT_COVERED = (
    "The product documentation does not cover this question, so it needs input "
    "from someone who knows the answer."
)

MIN_OVERLAP = 2  # shared words a passage needs before it is quoted
MAX_SENTENCES = 3


def reply(messages: list[dict], json_mode: bool = False) -> str:
    prompt = "\n".join(m.get("content", "") for m in messages)

    if json_mode and JUDGE_MARKER in prompt:
        return _judge(prompt)
    if EXTRACT_MARKER in prompt:
        return json.dumps(extract_questions(prompt.split("DOCUMENT:", 1)[-1]))
    if json_mode:
        return "{}"

    system = next((m["content"] for m in messages if m.get("role") == "system"), "")
    question = next((m["content"] for m in reversed(messages) if m.get("role") == "user"), "")
    if CONTEXT_MARKER in system:
        return answer(question, system.split(CONTEXT_MARKER, 1)[1])
    return NOT_COVERED


# --- extraction -----------------------------------------------------------------

_NUMBERED = re.compile(r"^\d+(?:\.\d+)+\.?\s+\S")
_ASKS = re.compile(
    r"\?|^\S+\s+(describe|provide|state|explain|list|confirm|detail|outline|"
    r"identify|summarize|submit|include)\b",
    re.IGNORECASE,
)


def extract_questions(document: str) -> list[str]:
    """Numbered paragraphs (e.g. "3.1.2 Does your ...?") that ask or require something."""
    questions = []
    for block in re.split(r"\n\s*\n", document):
        text = " ".join(line.strip() for line in block.strip().splitlines())
        if _NUMBERED.match(text) and _ASKS.search(text):
            questions.append(text)
    if questions:
        return questions
    # No numbering: fall back to every line that is a question.
    return [line.strip() for line in document.splitlines() if line.strip().endswith("?")]


# --- drafting -------------------------------------------------------------------

_PASSAGE = re.compile(r"^\[(\d+)\] .+ \(chunk \d+\)$", re.MULTILINE)
# Common words, plus words that appear all over vendor documentation and so say
# nothing about which passage answers a question.
_STOPWORDS = set(
    """a an and are as at be by can do does for from has have how in is it its of on or
    our over the their them this to was what when where which who will with you your
    any all each into than that then there these they those also must should would
    describe provide state explain list include including not
    customer customers data city vendor product proposed process organization
    current most similar time""".split()
)


def _words(text: str) -> set[str]:
    """Content words, cut to five letters as a crude stem (encrypted ~ encryption)."""
    return {w[:5] for w in re.findall(r"[a-z0-9]+", text.lower()) if len(w) >= 3 and w not in _STOPWORDS}


def _sentences(content: str) -> list[str]:
    """Prose sentences of a passage; headings and tables are skipped."""
    # Rebuild paragraphs: wrapped prose lines join up; a blank line, heading,
    # table or bullet starts a new one.
    paragraphs, current = [], []
    for line in content.splitlines():
        line = line.strip()
        bullet = re.match(r"[-*]\s+", line)
        if not line or line.startswith(("#", "|")) or bullet:
            if current:
                paragraphs.append(" ".join(current))
            current = []
        if line and not line.startswith(("#", "|")):
            current.append(line[bullet.end():] if bullet else line)
    if current:
        paragraphs.append(" ".join(current))

    sentences = [
        s.strip()
        for p in paragraphs
        # Also split inline bullets ("severity: - Priority 1 ...") from a
        # chunker that flattened the lines.
        for s in re.split(r"(?<=[.!?])\s+|\s+[-*]\s+(?=\S)", p.replace("**", ""))
        if s.strip()
    ]
    # A chunk can start mid-sentence (chunks overlap); skip the fragment.
    return [s for s in sentences if re.match(r"[A-Z0-9\"(]", s)]


def answer(question: str, context: str) -> str:
    """Quote the passage that shares the most words with the question.

    Declines when no passage shares at least MIN_OVERLAP words: that is the
    "needs input" path for questions the documentation doesn't cover.
    """
    wanted = _words(question)
    parts = _PASSAGE.split(context)
    # split() gives [preamble, n1, body1, n2, body2, ...]
    passages = [(int(parts[i]), parts[i + 1]) for i in range(1, len(parts) - 1, 2)]
    if not passages:
        return NOT_COVERED

    overlap, _, n, body = max((len(wanted & _words(body)), -n, n, body) for n, body in passages)
    if overlap < MIN_OVERLAP:
        return NOT_COVERED

    scored = [(len(wanted & _words(s)), i, s) for i, s in enumerate(_sentences(body))]
    best = sorted((x for x in scored if x[0]), key=lambda x: (-x[0], x[1]))[:MAX_SENTENCES]
    if not best:
        return NOT_COVERED
    return " ".join(f"{s.rstrip('.:;, ')}. [{n}]" for _, _, s in sorted(best, key=lambda x: x[1]))


# --- quality judge --------------------------------------------------------------


def _judge(prompt: str) -> str:
    criteria = prompt.split("CRITERIA:", 1)[-1].split("QUESTION:", 1)[0]
    names = re.findall(r"^- (\w+):", criteria, re.MULTILINE)
    reason = "Not judged: no OPENAI_API_KEY, so only the deterministic checks apply."
    return json.dumps({name: {"pass": True, "reason": reason} for name in names})
