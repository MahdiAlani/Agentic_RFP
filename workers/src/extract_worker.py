"""Question extraction: reads an uploaded RFP and creates one answer row per question.

Job fields (kind = "extract_questions" on the slow stream):
    document_id:  str (UUID)  -- the RFP document to read
    project_id:   str (UUID)  -- the project the questions belong to
    workspace_id: str (UUID)  -- knowledge base the answers will be grounded in

The model is asked for a JSON array of question strings so the output maps
cleanly onto rows. Existing extracted questions for the document are replaced,
which makes re-running the extraction idempotent; answers already edited or
generated are kept (only untouched 'new' rows are cleared).
"""

import json
import logging

from src.config import EXTRACT_MAX_CHARS
from src.db import get_connection
from src.llm import chat
from src.storage import get_object_bytes
from src.text import extract_text

log = logging.getLogger("extract")

EXTRACT_PROMPT = """You are reading a Request for Proposal (RFP) document.

Extract every question or requirement that the responding vendor must answer.

Rules:
- Return ONLY a JSON array of strings. No prose, no markdown, no code fences.
- One array entry per question, in the order they appear in the document.
- Keep the original wording, including any section number (e.g. "3.1 Describe...").
- Include requirement statements the vendor must respond to, even if not phrased
  as a question (e.g. "Describe your incident response process").
- Skip headings, instructions to bidders, boilerplate, and table-of-contents lines.
- If the document contains no questions, return [].

DOCUMENT:
{document}"""


def handle_extract(fields: dict) -> None:
    document_id = fields.get("document_id")
    project_id = fields.get("project_id")
    workspace_id = fields.get("workspace_id")
    if not document_id or not project_id or not workspace_id:
        log.error("extract job missing ids: %r", fields)
        return

    with get_connection() as conn:
        row = conn.execute(
            "SELECT file_key, file_name FROM documents WHERE id = %s", (document_id,)
        ).fetchone()

    if row is None:
        log.warning("document %s no longer exists, dropping job", document_id)
        return

    file_key, file_name = row
    log.info("extracting questions from %s (%s)", document_id, file_name)

    try:
        text = extract_text(file_name, get_object_bytes(file_key))
        if not text.strip():
            raise ValueError("no text could be extracted")

        questions = _extract_questions(text)
        log.info("found %d questions in %s", len(questions), file_name)
        if not questions:
            return

        with get_connection() as conn:
            with conn.cursor() as cur:
                # Clear previous untouched extractions so re-running doesn't
                # duplicate; anything answered or in flight is preserved.
                cur.execute(
                    """
                    DELETE FROM answers
                    WHERE document_id = %s AND project_id = %s AND status = 'new'
                    """,
                    (document_id, project_id),
                )
                cur.executemany(
                    """
                    INSERT INTO answers
                        (workspace_id, project_id, document_id, question, status, position)
                    VALUES (%s, %s, %s, %s, 'new', %s)
                    """,
                    [
                        (workspace_id, project_id, document_id, q, i)
                        for i, q in enumerate(questions)
                    ],
                )

    except Exception:
        log.exception("failed to extract questions from document %s", document_id)


def _extract_questions(text: str) -> list[str]:
    """Ask the model for the document's questions as a JSON array of strings."""
    # Very long RFPs are truncated: the tail of a proposal document is usually
    # appendices rather than questions.
    document = text[:EXTRACT_MAX_CHARS]
    raw = chat(
        [{"role": "user", "content": EXTRACT_PROMPT.format(document=document)}],
        temperature=0,
    )
    return _parse_questions(raw)


def _parse_questions(raw: str) -> list[str]:
    """Parse the model's reply, tolerating code fences around the JSON."""
    cleaned = raw.strip()
    if cleaned.startswith("```"):
        cleaned = cleaned.split("\n", 1)[-1].rsplit("```", 1)[0]

    # Fall back to the outermost array if the model added stray prose.
    start, end = cleaned.find("["), cleaned.rfind("]")
    if start != -1 and end > start:
        cleaned = cleaned[start : end + 1]

    try:
        parsed = json.loads(cleaned)
    except json.JSONDecodeError:
        log.error("could not parse questions as JSON: %r", raw[:500])
        return []

    if not isinstance(parsed, list):
        return []
    return [q.strip() for q in parsed if isinstance(q, str) and q.strip()]
