import pytest

from src import embed_worker
from src.embed_worker import handle_embed

JOB = {"kind": "embed_batch", "document_id": "doc-1", "chunk_ids": "c-1,c-2"}
CHUNKS = [("c-1", "Para one."), ("c-2", "Para two.")]


@pytest.fixture
def embedded(monkeypatch):
    """Stub embeddings; returns the list of texts sent to embed."""
    texts = []

    def fake_embed(batch):
        texts.extend(batch)
        return [[0.1, 0.2] for _ in batch]

    monkeypatch.setattr(embed_worker, "embed_texts", fake_embed)
    return texts


def test_embeds_batch_and_counts_it_down(embedded, fake_conn, use_conn):
    conn = use_conn(embed_worker, fake_conn({
        "FROM document_chunks": CHUNKS,
        "UPDATE document_chunks": [("c-1",), ("c-2",)],
    }))
    handle_embed(JOB)

    assert embedded == ["Para one.", "Para two."]
    assert conn.statements("SELECT id, content") == [(["c-1", "c-2"],)]
    assert conn.statements("UPDATE document_chunks") == [(["c-1", "c-2"], ["[0.1,0.2]", "[0.1,0.2]"])]
    assert conn.statements("pending_batches - 1") == [("doc-1",)]


def test_already_embedded_batch_is_skipped(embedded, fake_conn, use_conn):
    conn = use_conn(embed_worker, fake_conn())
    handle_embed(JOB)
    assert embedded == []
    assert conn.statements("pending_batches") == []


def test_losing_a_duplicate_delivery_race_does_not_count_twice(embedded, fake_conn, use_conn):
    # Another worker stored these vectors first, so the update matches no rows.
    conn = use_conn(embed_worker, fake_conn({"FROM document_chunks": CHUNKS}))
    handle_embed(JOB)
    assert conn.statements("pending_batches") == []


def test_extract_jobs_are_routed_to_the_extractor(monkeypatch):
    seen = []
    monkeypatch.setattr(embed_worker, "handle_extract", seen.append)
    job = {"kind": "extract_questions", "document_id": "d", "project_id": "p", "workspace_id": "w"}
    handle_embed(job)
    assert seen == [job]


@pytest.mark.parametrize("job", [{"kind": "embed_batch"}, {"kind": "embed_batch", "document_id": "d", "chunk_ids": ""}])
def test_incomplete_job_is_a_noop(job, fake_conn, use_conn):
    conn = use_conn(embed_worker, fake_conn())
    handle_embed(job)
    assert conn.executed == []


def test_embedding_failure_marks_document_failed(monkeypatch, fake_conn, use_conn):
    conn = use_conn(embed_worker, fake_conn({"FROM document_chunks": CHUNKS}))

    def boom(_):
        raise RuntimeError("model timeout")

    monkeypatch.setattr(embed_worker, "embed_texts", boom)
    handle_embed(JOB)

    assert conn.statements("UPDATE documents SET status = %s") == [("failed", "doc-1")]
    assert conn.statements("UPDATE document_chunks") == []
