"""Embed worker: consumes the Redis `embed` stream (the slow lane).

Jobs:
    kind=embed_batch        document_id, chunk_ids (comma-separated UUIDs),
                            queued by the index worker
    kind=extract_questions  see extract_worker
"""

import logging

from src.db import get_connection, to_vector
from src.extract_worker import handle_extract
from src.llm import embed_texts

log = logging.getLogger("embed")


def handle_embed(fields: dict) -> None:
    if fields.get("kind") == "extract_questions":
        handle_extract(fields)
        return

    document_id = fields.get("document_id")
    chunk_ids = [c for c in fields.get("chunk_ids", "").split(",") if c]
    if not document_id or not chunk_ids:
        log.error("embed job missing document_id or chunk_ids: %r", fields)
        return

    with get_connection() as conn:
        rows = conn.execute(
            """
            SELECT id, content FROM document_chunks
            WHERE id = ANY(%s::uuid[]) AND embedding IS NULL
            ORDER BY chunk_index
            """,
            (chunk_ids,),
        ).fetchall()

    if not rows:
        # Already embedded by an earlier delivery, or the document was
        # re-indexed or deleted since this job was queued.
        return

    try:
        vectors = embed_texts([content for _, content in rows])
    except Exception:
        log.exception("failed to embed a batch of document %s", document_id)
        _set_status(document_id, "failed")
        return

    # `embedding IS NULL` makes a duplicate delivery update nothing, so each
    # batch counts down pending_batches exactly once.
    with get_connection() as conn:
        updated = conn.execute(
            """
            UPDATE document_chunks c SET embedding = v.embedding::vector
            FROM unnest(%s::uuid[], %s::text[]) AS v(id, embedding)
            WHERE c.id = v.id AND c.embedding IS NULL
            RETURNING c.id
            """,
            ([str(chunk_id) for chunk_id, _ in rows], [to_vector(v) for v in vectors]),
        ).fetchall()
        if updated:
            conn.execute(
                """
                UPDATE documents SET
                    pending_batches = pending_batches - 1,
                    status = CASE WHEN pending_batches = 1 AND status = 'embedding'
                                  THEN 'embedded' ELSE status END
                WHERE id = %s
                """,
                (document_id,),
            )

    log.info("embedded %d chunks of document %s", len(updated), document_id)


def _set_status(document_id: str, status: str) -> None:
    with get_connection() as conn:
        conn.execute("UPDATE documents SET status = %s WHERE id = %s", (status, document_id))
