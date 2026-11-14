import pytest

from src import index_worker
from src.index_worker import handle_index

DOC_ROW = [("ws-1", "documents/ws-1/doc-1/spec.txt", "spec.txt")]
EVENT = {"document_id": "doc-1", "workspace_id": "ws-1"}


@pytest.fixture
def body(monkeypatch):
    """Serve `body.value` as the stored file."""

    class Body:
        value = b"Para one.\n\nPara two."

    monkeypatch.setattr(index_worker, "get_object_bytes", lambda key: Body.value)
    return Body


def test_stores_chunks_and_queues_batches(body, monkeypatch, fake_conn, use_conn):
    monkeypatch.setattr(index_worker, "EMBED_BATCH", 2)
    monkeypatch.setattr(index_worker, "chunk_text", lambda text: ["a", "b", "c"])
    conn = use_conn(index_worker, fake_conn({"FROM documents": DOC_ROW}))
    jobs = []

    handle_index(EVENT, jobs.append)

    # Old chunks are replaced so indexing the same document again is idempotent.
    assert conn.statements("DELETE FROM document_chunks") == [("doc-1",)]
    (_, rows), = conn.batches
    assert [(doc, ws, i, content) for _, doc, ws, i, content in rows] == [
        ("doc-1", "ws-1", 0, "a"), ("doc-1", "ws-1", 1, "b"), ("doc-1", "ws-1", 2, "c"),
    ]
    assert conn.statements("pending_batches = %s") == [(2, "doc-1")]

    ids = [row[0] for row in rows]
    assert jobs == [
        {"kind": "embed_batch", "document_id": "doc-1", "chunk_ids": f"{ids[0]},{ids[1]}"},
        {"kind": "embed_batch", "document_id": "doc-1", "chunk_ids": ids[2]},
    ]


def test_missing_document_id_is_a_noop(fake_conn, use_conn):
    conn = use_conn(index_worker, fake_conn())
    handle_index({}, lambda job: pytest.fail("queued a job"))
    assert conn.executed == []


def test_deleted_document_is_dropped(body, fake_conn, use_conn):
    conn = use_conn(index_worker, fake_conn())
    handle_index(EVENT, lambda job: pytest.fail("queued a job"))
    assert conn.batches == []


@pytest.mark.parametrize("failure", ["download", "empty"])
def test_failures_mark_document_failed(failure, body, monkeypatch, fake_conn, use_conn):
    conn = use_conn(index_worker, fake_conn({"FROM documents": DOC_ROW}))
    if failure == "download":
        def boom(key):
            raise RuntimeError("storage down")

        monkeypatch.setattr(index_worker, "get_object_bytes", boom)
    else:
        body.value = b""

    handle_index(EVENT, lambda job: pytest.fail("queued a job"))

    assert conn.statements("UPDATE documents SET status = %s") == [("failed", "doc-1")]
    assert conn.batches == []
