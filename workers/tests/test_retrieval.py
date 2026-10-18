from src import retrieval
from src.db import to_vector
from src.retrieval import format_context, search


def test_to_vector_matches_pgvector_text_format():
    assert to_vector([0.5, -1, 2.25]) == "[0.5,-1.0,2.25]"
    assert to_vector([]) == "[]"


def test_search_scopes_query_and_maps_rows(monkeypatch, fake_conn, use_conn):
    monkeypatch.setattr(retrieval, "embed_text", lambda q: [0.1, 0.2])
    conn = use_conn(retrieval, fake_conn({"FROM document_chunks": [("text", 3, "doc-1", "a.pdf")]}))

    result = search("ws-1", "question", project_id="proj-1", limit=5)

    assert result == [{"content": "text", "chunk_index": 3, "document_id": "doc-1", "file_name": "a.pdf"}]
    (sql, params), = conn.executed
    assert params == ("ws-1", "proj-1", "[0.1,0.2]", 5)
    # RFPs are excluded (they would match their own questions), and another
    # project's documents are never visible.
    assert "document_type <> 'rfp'" in sql
    assert "d.project_id IS NULL OR d.project_id = %s" in sql
    assert "c.workspace_id = %s" in sql


def test_format_context_numbers_chunks_with_sources():
    ctx = format_context([
        {"file_name": "a.pdf", "chunk_index": 0, "content": "Alpha"},
        {"file_name": "b.docx", "chunk_index": 7, "content": "Beta"},
    ])
    assert ctx == "[1] a.pdf (chunk 0)\nAlpha\n\n[2] b.docx (chunk 7)\nBeta"


def test_format_context_empty_tells_model_nothing_was_found():
    assert "No relevant documents" in format_context([])
