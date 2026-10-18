import pytest

from src import embed_worker
from src.embed_worker import handle_embed

DOC_ROW = [("ws-1", "documents/ws-1/doc-1/spec.txt", "spec.txt")]


@pytest.fixture
def pipeline(monkeypatch):
    """Stub storage and embeddings; returns the list of texts sent to embed."""
    embedded = []
    monkeypatch.setattr(embed_worker, "get_object_bytes", lambda key: b"Para one.\n\nPara two.")

    def fake_embed(texts):
        embedded.extend(texts)
        return [[0.1, 0.2] for _ in texts]

    monkeypatch.setattr(embed_worker, "embed_texts", fake_embed)
    return embedded


def test_embeds_chunks_and_marks_document(pipeline, fake_conn, use_conn):
    conn = use_conn(embed_worker, fake_conn({"FROM documents": DOC_ROW}))
    handle_embed({"kind": "embed_document", "document_id": "doc-1"})

    assert pipeline == ["Para one.\n\nPara two."]
    # Old chunks are replaced so re-embedding the same document is idempotent.
    assert conn.statements("DELETE FROM document_chunks") == [("doc-1",)]
    (sql, rows), = conn.batches
    assert rows == [("doc-1", "ws-1", 0, "Para one.\n\nPara two.", "[0.1,0.2]")]
    assert conn.statements("status = 'embedded'") == [("doc-1",)]


def test_extract_jobs_are_routed_to_the_extractor(monkeypatch):
    seen = []
    monkeypatch.setattr(embed_worker, "handle_extract", seen.append)
    job = {"kind": "extract_questions", "document_id": "d", "project_id": "p", "workspace_id": "w"}
    handle_embed(job)
    assert seen == [job]


def test_missing_document_id_is_a_noop(fake_conn, use_conn):
    conn = use_conn(embed_worker, fake_conn())
    handle_embed({"kind": "embed_document"})
    assert conn.executed == []


def test_deleted_document_is_dropped(pipeline, fake_conn, use_conn):
    conn = use_conn(embed_worker, fake_conn())
    handle_embed({"document_id": "gone"})
    assert pipeline == []
    assert conn.batches == []


@pytest.mark.parametrize("failure", ["download", "empty", "embed"])
def test_failures_mark_document_failed(failure, monkeypatch, fake_conn, use_conn):
    conn = use_conn(embed_worker, fake_conn({"FROM documents": DOC_ROW}))

    def boom(*_):
        raise RuntimeError(failure)

    body = b"" if failure == "empty" else b"text"
    monkeypatch.setattr(embed_worker, "get_object_bytes", boom if failure == "download" else lambda k: body)
    monkeypatch.setattr(embed_worker, "embed_texts", boom if failure == "embed" else lambda t: [[0.0]] * len(t))

    handle_embed({"document_id": "doc-1"})

    assert conn.statements("UPDATE documents SET status = %s") == [("failed", "doc-1")]
    assert conn.batches == []
