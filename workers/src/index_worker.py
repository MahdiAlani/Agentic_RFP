"""Index worker: consumes the `documents.uploaded` Kafka topic.

Event (published by the Go API when an upload is confirmed):
    document_id:  str (UUID)
    workspace_id: str (UUID)

Extracts and chunks the document, stores the chunks without vectors, then
queues one Redis `embed` job per batch so the embedding calls fan out across
embed workers.
"""

import logging
import uuid

from src.db import get_connection
from src.llm import EMBED_BATCH
from src.storage import get_object_bytes
from src.text import chunk_text, extract_text

log = logging.getLogger("index")


def handle_index(event: dict, enqueue) -> None:
    """`enqueue(fields)` adds one job to the Redis embed stream."""
    document_id = event.get("document_id")
    if not document_id:
        log.error("upload event missing document_id: %r", event)
        return

    with get_connection() as conn:
        row = conn.execute(
            "SELECT workspace_id, file_key, file_name FROM documents WHERE id = %s",
            (document_id,),
        ).fetchone()

    if row is None:
        log.warning("document %s no longer exists, dropping event", document_id)
        return

    workspace_id, file_key, file_name = row
    log.info("indexing document %s (%s)", document_id, file_name)

    try:
        chunks = chunk_text(extract_text(file_name, get_object_bytes(file_key)))
        if not chunks:
            raise ValueError("no text could be extracted")
    except Exception:
        log.exception("failed to index document %s", document_id)
        _set_status(document_id, "failed")
        return

    chunk_ids = [str(uuid.uuid4()) for _ in chunks]
    batches = [chunk_ids[i : i + EMBED_BATCH] for i in range(0, len(chunk_ids), EMBED_BATCH)]

    # Replacing the old chunks gives them new ids, so embed jobs still queued
    # from an earlier upload of this document find nothing and do nothing.
    with get_connection() as conn:
        with conn.cursor() as cur:
            cur.execute("DELETE FROM document_chunks WHERE document_id = %s", (document_id,))
            cur.executemany(
                """
                INSERT INTO document_chunks
                    (id, document_id, workspace_id, chunk_index, content)
                VALUES (%s, %s, %s, %s, %s)
                """,
                [
                    (chunk_id, document_id, workspace_id, i, content)
                    for i, (chunk_id, content) in enumerate(zip(chunk_ids, chunks))
                ],
            )
            cur.execute(
                "UPDATE documents SET status = 'embedding', pending_batches = %s WHERE id = %s",
                (len(batches), document_id),
            )

    # A crash before this loop finishes leaves the Kafka event uncommitted, so it
    # is redelivered and the document is indexed again from scratch.
    for batch in batches:
        enqueue({"kind": "embed_batch", "document_id": document_id, "chunk_ids": ",".join(batch)})

    log.info("indexed document %s: %d chunks in %d batches", document_id, len(chunks), len(batches))


def _set_status(document_id: str, status: str) -> None:
    with get_connection() as conn:
        conn.execute("UPDATE documents SET status = %s WHERE id = %s", (status, document_id))
