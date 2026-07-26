"""Generate worker: consumes the `generate` stream.

Message fields (produced by the Go backend when an answer is requested):
    answer_id:    str  (UUID)
    workspace_id: str  (UUID)
    question:     str

TODO (real implementation):
  1. Embed the question.
  2. Vector-search document_chunks within the workspace for relevant context.
  3. Call the LLM with the question + retrieved context.
  4. UPDATE answers SET answer_text = ..., status = 'done' WHERE id = answer_id.
  5. INSERT the cited chunks into answer_citations (answer_id, document_id, chunk_ref).
"""

import logging

log = logging.getLogger("generate")


def handle_generate(fields: dict) -> None:
    answer_id = fields.get("answer_id")
    question = fields.get("question")
    log.info("generate job received: answer_id=%s question=%r", answer_id, question)
    # TODO: implement the RAG + LLM pipeline described above.
