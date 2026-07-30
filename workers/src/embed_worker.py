"""Embed worker: consumes the `embed` stream.

Message fields (produced by the Go backend on document confirm):
    document_id: str  (UUID)
"""

import logging

from src.db import get_connection, to_vector
from src.extract_worker import handle_extract
from src.llm import embed_texts
from src.storage import get_object_bytes
from src.text import chunk_text, extract_text

log = logging.getLogger("embed")


def handle_embed(fields: dict) -> None:
    """Slow-lane dispatcher: document embedding and RFP question extraction."""
    if fields.get("kind") == "extract_questions":
        handle_extract(fields)
        return

    document_id = fields.get("document_id")
    if not document_id:
        log.error("embed job missing document_id: %r", fields)
        return

    with get_connection() as conn:
        row = conn.execute(
            "SELECT workspace_id, file_key, file_name FROM documents WHERE id = %s",
            (document_id,),
        ).fetchone()

    if row is None:
        # The document was deleted between enqueue and now
        log.warning("document %s no longer exists, dropping job", document_id)
        return

    workspace_id, file_key, file_name = row
    log.info("embedding document %s (%s)", document_id, file_name)

    try:
        raw = get_object_bytes(file_key)
        text = extract_text(file_name, raw)
        chunks = chunk_text(text)
        if not chunks:
            raise ValueError("no text could be extracted")
        vectors = embed_texts(chunks)
    except Exception:
        log.exception("failed to embed document %s", document_id)
        _set_status(document_id, "failed")
        return

    # Chunks and the status flip go in one transaction: either the document is
    # fully embedded or it isn't. Deleting first makes re-embedding idempotent.
    with get_connection() as conn:
        with conn.cursor() as cur:
            cur.execute(
                "DELETE FROM document_chunks WHERE document_id = %s", (document_id,)
            )
            cur.executemany(
                """
                INSERT INTO document_chunks
                    (document_id, workspace_id, chunk_index, content, embedding)
                VALUES (%s, %s, %s, %s, %s::vector)
                """,
                [
                    (document_id, workspace_id, i, content, to_vector(vector))
                    for i, (content, vector) in enumerate(zip(chunks, vectors))
                ],
            )
            cur.execute(
                "UPDATE documents SET status = 'embedded' WHERE id = %s", (document_id,)
            )

    log.info("embedded document %s into %d chunks", document_id, len(chunks))


def _set_status(document_id: str, status: str) -> None:
    with get_connection() as conn:
        conn.execute(
            "UPDATE documents SET status = %s WHERE id = %s", (status, document_id)
        )
