import json

import pytest

from src import extract_worker
from src.extract_worker import _parse_questions, handle_extract

IDS = {"document_id": "doc-1", "project_id": "proj-1", "workspace_id": "ws-1"}


class TestParseQuestions:
    def test_plain_array(self):
        assert _parse_questions('["1.1 What?", "1.2 Why?"]') == ["1.1 What?", "1.2 Why?"]

    def test_code_fence(self):
        assert _parse_questions('```json\n["Q1"]\n```') == ["Q1"]

    def test_prose_around_array(self):
        assert _parse_questions('Here you go:\n["Q1", "Q2"]\nHope that helps') == ["Q1", "Q2"]

    def test_drops_blank_and_non_string_entries(self):
        assert _parse_questions('["  Q1  ", "", 3, null, {"q": 1}, "Q2"]') == ["Q1", "Q2"]

    def test_array_wrapped_in_object(self):
        assert _parse_questions('{"questions": ["Q1"]}') == ["Q1"]

    @pytest.mark.parametrize("raw", ["", "no json here", "[unclosed", '{"count": 0}'])
    def test_unusable_output_returns_empty(self, raw):
        assert _parse_questions(raw) == []


@pytest.fixture
def doc_conn(fake_conn, use_conn):
    return use_conn(extract_worker, fake_conn({"FROM documents": [("key/rfp.txt", "rfp.txt")]}))


@pytest.fixture
def stub_io(monkeypatch):
    monkeypatch.setattr(extract_worker, "get_object_bytes", lambda key: b"1. What? 2. Why?")

    def stub_chat(reply):
        monkeypatch.setattr(extract_worker, "chat", lambda messages, temperature: reply)

    return stub_chat


def test_creates_one_answer_row_per_question_in_order(doc_conn, stub_io):
    stub_io(json.dumps(["1. What?", "2. Why?"]))
    handle_extract(IDS)

    # Previous untouched extractions are cleared first, so re-running is idempotent.
    assert doc_conn.statements("DELETE FROM answers") == [("doc-1", "proj-1")]
    (sql, rows), = doc_conn.batches
    assert "INSERT INTO answers" in sql
    assert rows == [
        ("ws-1", "proj-1", "doc-1", "1. What?", 0),
        ("ws-1", "proj-1", "doc-1", "2. Why?", 1),
    ]


def test_only_clears_rows_still_marked_new(doc_conn, stub_io):
    stub_io('["Q"]')
    handle_extract(IDS)
    (delete_sql, _), = [(s, p) for s, p in doc_conn.executed if "DELETE" in s]
    assert "status = 'new'" in delete_sql


def test_no_questions_leaves_existing_rows_alone(doc_conn, stub_io):
    stub_io("[]")
    handle_extract(IDS)
    assert doc_conn.statements("DELETE") == []
    assert doc_conn.batches == []


@pytest.mark.parametrize("missing", ["document_id", "project_id", "workspace_id"])
def test_missing_ids_is_a_noop(missing, fake_conn, use_conn):
    conn = use_conn(extract_worker, fake_conn())
    handle_extract({k: v for k, v in IDS.items() if k != missing})
    assert conn.executed == []


def test_deleted_document_is_dropped(fake_conn, use_conn, monkeypatch):
    use_conn(extract_worker, fake_conn())
    monkeypatch.setattr(extract_worker, "get_object_bytes", lambda key: pytest.fail("should not download"))
    handle_extract(IDS)


def test_long_documents_are_truncated(doc_conn, monkeypatch):
    monkeypatch.setattr(extract_worker, "EXTRACT_MAX_CHARS", 50)
    monkeypatch.setattr(extract_worker, "get_object_bytes", lambda key: b"x" * 10_000)
    seen = {}

    def fake_chat(messages, temperature):
        seen["prompt"] = messages[0]["content"]
        return "[]"

    monkeypatch.setattr(extract_worker, "chat", fake_chat)
    handle_extract(IDS)
    assert "x" * 50 in seen["prompt"] and "x" * 51 not in seen["prompt"]
