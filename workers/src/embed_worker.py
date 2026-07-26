"""Embed worker: consumes the `embed` stream.

Message fields (produced by the Go backend on document confirm):
    document_id: str  (UUID)

TODO (real implementation):
  1. Look up the document row by id -> get file_key, workspace_id, document_type.
  2. Download the file bytes from MinIO (src.storage.get_object_bytes).
  3. Extract text + chunk it.
  4. Embed each chunk (embedding model of choice).
  5. INSERT rows into document_chunks (content, embedding vector, metadata).
  6. UPDATE documents SET status = 'embedded' WHERE id = document_id.
"""

import logging

log = logging.getLogger("embed")


def handle_embed(fields: dict) -> None:
    document_id = fields.get("document_id")
    log.info("embed job received: document_id=%s", document_id)
    # TODO: implement the embedding pipeline described above.
