"""Generate worker: consumes the `generate` stream.

Jobs carry a `kind` field so this one stream can serve multiple generation types:

  kind = "chat_reply"  (multi-turn chat)
    message_id:      str  (UUID)  -- the pending assistant message to fill
    conversation_id: str  (UUID)

  kind = "answer"  (one-off RFP question)
    answer_id:    str  (UUID)
    workspace_id: str  (UUID)
    question:     str

Both retrieve grounding context from the workspace knowledge base, call the
model, and write the result back to the row the Go backend created.
"""

import logging

from src import citations
from src.config import MAX_HISTORY_MESSAGES
from src.db import get_connection
from src.llm import chat
from src.retrieval import format_context, search

log = logging.getLogger("generate")

SYSTEM_PROMPT = """You are an assistant that helps answer RFP (Request for Proposal) questions about the user's product.

Use the CONTEXT below, which is drawn from the product's documentation, to answer.
Rules:
- Ground every claim in the context. Do not invent product capabilities.
- If the context does not cover the question, say so plainly and state what documentation would be needed.
- Write in a clear, professional tone suitable for a proposal response.
- Be concise; prefer specifics over marketing language.{extra_rules}

CONTEXT:
{context}"""

# Proposal answers are cited inline so reviewers can check every claim.
CITATION_RULES = """
- Cite the numbered context passage that supports each claim, inline, like [1] or [2, 3].
- Only cite passages that actually contain the fact. Never cite a passage you did not use.
- If the context does not cover the question, cite nothing."""


def _system_prompt(chunks: list[dict], extra_rules: str = "") -> str:
    return SYSTEM_PROMPT.format(context=format_context(chunks), extra_rules=extra_rules)


def handle_generate(fields: dict) -> None:
    kind = fields.get("kind", "answer")
    if kind == "chat_reply":
        _chat_reply(fields)
    else:
        _answer(fields)


# --- chat -------------------------------------------------------------------


def _chat_reply(fields: dict) -> None:
    message_id = fields.get("message_id")
    conversation_id = fields.get("conversation_id")
    if not message_id or not conversation_id:
        log.error("chat_reply job missing ids: %r", fields)
        return

    log.info("generating chat reply for message %s", message_id)

    try:
        with get_connection() as conn:
            row = conn.execute(
                """
                SELECT p.workspace_id, c.project_id
                FROM conversations c
                JOIN projects p ON p.id = c.project_id
                WHERE c.id = %s
                """,
                (conversation_id,),
            ).fetchone()
            if row is None:
                log.warning("conversation %s no longer exists", conversation_id)
                return
            workspace_id, project_id = row

            history = conn.execute(
                """
                SELECT role, content FROM messages
                WHERE conversation_id = %s AND status = 'complete' AND content <> ''
                ORDER BY created_at ASC
                """,
                (conversation_id,),
            ).fetchall()

        history = history[-MAX_HISTORY_MESSAGES:]
        question = next(
            (content for role, content in reversed(history) if role == "user"), ""
        )
        if not question:
            raise ValueError("no user message found in conversation")

        chunks = search(workspace_id, question, project_id=project_id)
        messages = [
            {"role": "system", "content": _system_prompt(chunks)},
            *({"role": role, "content": content} for role, content in history),
        ]

        reply = chat(messages)
        _finish_message(message_id, reply, "complete")
        log.info("completed message %s (%d chars)", message_id, len(reply))

    except Exception:
        log.exception("failed to generate reply for message %s", message_id)
        _finish_message(
            message_id,
            "Sorry - I couldn't generate a reply at the moment. Please try again later.",
            "failed",
        )


def _finish_message(message_id: str, content: str, status: str) -> None:
    with get_connection() as conn:
        conn.execute(
            "UPDATE messages SET content = %s, status = %s WHERE id = %s",
            (content, status, message_id),
        )


# --- one-off answers --------------------------------------------------------


def _answer(fields: dict) -> None:
    answer_id = fields.get("answer_id")
    workspace_id = fields.get("workspace_id")
    question = fields.get("question", "")
    if not answer_id or not workspace_id:
        log.error("answer job missing ids: %r", fields)
        return

    log.info("generating answer %s", answer_id)

    try:
        # The answer row knows which project asked, which scopes retrieval to
        # that project's documents plus the workspace knowledge base.
        with get_connection() as conn:
            row = conn.execute(
                "SELECT project_id FROM answers WHERE id = %s", (answer_id,)
            ).fetchone()
        if row is None:
            log.warning("answer %s no longer exists, dropping job", answer_id)
            return
        project_id = row[0]

        chunks = search(workspace_id, question, project_id=project_id)
        messages = [
            {"role": "system", "content": _system_prompt(chunks, CITATION_RULES)},
            {"role": "user", "content": question},
        ]
        text, cited = citations.resolve(chat(messages), chunks)

        with get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    "UPDATE answers SET answer_text = %s, status = 'done' WHERE id = %s",
                    (text, answer_id),
                )
                # Replace, not append: a regenerated answer cites new passages.
                cur.execute("DELETE FROM answer_citations WHERE answer_id = %s", (answer_id,))
                # position is the [n] marker number used in the answer text.
                cur.executemany(
                    """
                    INSERT INTO answer_citations (answer_id, document_id, chunk_ref, position)
                    VALUES (%s, %s, %s, %s)
                    ON CONFLICT DO NOTHING
                    """,
                    [
                        (answer_id, c["document_id"], str(c["chunk_index"]), n)
                        for n, c in enumerate(cited, start=1)
                    ],
                )

        log.info("completed answer %s", answer_id)

    except Exception:
        log.exception("failed to generate answer %s", answer_id)
        with get_connection() as conn:
            conn.execute(
                "UPDATE answers SET status = 'failed' WHERE id = %s", (answer_id,)
            )
