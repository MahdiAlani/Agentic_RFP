import logging

from src.config import RETRIEVAL_TOP_K
from src.db import get_connection, to_vector
from src.llm import embed_text

log = logging.getLogger("retrieval")


def search(
    workspace_id,
    question: str,
    project_id=None,
    limit: int = RETRIEVAL_TOP_K,
) -> list[dict]:
    """Embed the question and return the closest chunks available to it.

    Scope: the workspace knowledge base (documents with no project) plus the
    asking project's own supporting documents. Another project's uploads are
    never visible.

    RFP documents are excluded: an RFP contains the questions themselves, so its
    chunks match a question almost perfectly and would crowd out the product
    documentation that actually answers it.
    """
    query_vector = embed_text(question)

    with get_connection() as conn:
        rows = conn.execute(
            """
            SELECT c.content, c.chunk_index, c.document_id, d.file_name
            FROM document_chunks c
            JOIN documents d ON d.id = c.document_id
            WHERE c.workspace_id = %s
              AND c.embedding IS NOT NULL
              AND d.document_type <> 'rfp'
              AND (d.project_id IS NULL OR d.project_id = %s)
            ORDER BY c.embedding <=> %s::vector
            LIMIT %s
            """,
            (workspace_id, project_id, to_vector(query_vector), limit),
        ).fetchall()

    return [
        {
            "content": content,
            "chunk_index": chunk_index,
            "document_id": document_id,
            "file_name": file_name,
        }
        for content, chunk_index, document_id, file_name in rows
    ]


def format_context(chunks: list[dict]) -> str:
    """Render retrieved chunks as a numbered context block for the prompt."""
    if not chunks:
        return "(No relevant documents were found in the knowledge base.)"
    return "\n\n".join(
        f"[{i + 1}] {c['file_name']} (chunk {c['chunk_index']})\n{c['content']}"
        for i, c in enumerate(chunks)
    )
