import pytest

from src import generate_worker
from src.generate_worker import handle_generate

CHUNKS = [
    {"content": "AES-256 at rest.", "chunk_index": 0, "document_id": "doc-a", "file_name": "sec.pdf"},
    {"content": "TLS 1.3 in transit.", "chunk_index": 4, "document_id": "doc-b", "file_name": "net.pdf"},
]


@pytest.fixture
def model(monkeypatch):
    """Stub retrieval and the chat model; records what they were called with."""
    calls = {"search": [], "chat": []}

    def fake_search(workspace_id, question, project_id=None):
        calls["search"].append((workspace_id, question, project_id))
        return CHUNKS

    def fake_chat(messages):
        calls["chat"].append(messages)
        return "Grounded reply."

    monkeypatch.setattr(generate_worker, "search", fake_search)
    monkeypatch.setattr(generate_worker, "chat", fake_chat)
    return calls


class TestAnswer:
    JOB = {"kind": "answer", "answer_id": "ans-1", "workspace_id": "ws-1", "question": "How is data encrypted?"}

    def test_stores_only_cited_passages_numbered_like_the_text(self, model, monkeypatch, fake_conn, use_conn):
        conn = use_conn(generate_worker, fake_conn({"FROM answers": [("proj-1",)]}))
        monkeypatch.setattr(generate_worker, "chat", lambda m: model["chat"].append(m) or "TLS 1.3 [2]. AES [2].")
        handle_generate(self.JOB)

        assert model["search"] == [("ws-1", "How is data encrypted?", "proj-1")]
        system = model["chat"][0][0]["content"]
        assert "[1] sec.pdf (chunk 0)" in system and "AES-256 at rest." in system
        assert "Cite the numbered context passage" in system

        # Passage [2] was the only one used, so it becomes [1].
        assert conn.statements("SET answer_text") == [("TLS 1.3 [1]. AES [1].", "ans-1")]
        (_, rows), = conn.batches
        assert rows == [("ans-1", "doc-b", "4", 1)]

    def test_regenerating_replaces_old_citations(self, model, fake_conn, use_conn):
        conn = use_conn(generate_worker, fake_conn({"FROM answers": [("proj-1",)]}))
        handle_generate(self.JOB)
        assert conn.statements("DELETE FROM answer_citations") == [("ans-1",)]

    def test_uncited_answer_stores_no_citations(self, model, fake_conn, use_conn):
        conn = use_conn(generate_worker, fake_conn({"FROM answers": [("proj-1",)]}))
        handle_generate(self.JOB)  # stub reply "Grounded reply." has no markers
        (_, rows), = conn.batches
        assert rows == []

    def test_kind_defaults_to_answer(self, model, fake_conn, use_conn):
        conn = use_conn(generate_worker, fake_conn({"FROM answers": [("proj-1",)]}))
        handle_generate({k: v for k, v in self.JOB.items() if k != "kind"})
        assert conn.statements("SET answer_text")

    def test_deleted_answer_is_dropped(self, model, fake_conn, use_conn):
        use_conn(generate_worker, fake_conn())
        handle_generate(self.JOB)
        assert model["chat"] == []

    def test_model_failure_marks_answer_failed(self, model, monkeypatch, fake_conn, use_conn):
        conn = use_conn(generate_worker, fake_conn({"FROM answers": [("proj-1",)]}))
        monkeypatch.setattr(generate_worker, "chat", lambda m: (_ for _ in ()).throw(RuntimeError("429")))
        handle_generate(self.JOB)
        assert conn.statements("status = 'failed'") == [("ans-1",)]
        assert conn.batches == []

    def test_missing_ids_is_a_noop(self, model, fake_conn, use_conn):
        conn = use_conn(generate_worker, fake_conn())
        handle_generate({"kind": "answer", "question": "Q"})
        assert conn.executed == [] and model["chat"] == []


class TestChatReply:
    JOB = {"kind": "chat_reply", "message_id": "msg-9", "conversation_id": "conv-1"}

    def _conn(self, fake_conn, history):
        return fake_conn({"FROM conversations": [("ws-1", "proj-1")], "FROM messages": history})

    def test_replays_history_and_retrieves_on_latest_user_message(self, model, fake_conn, use_conn):
        history = [("user", "First question"), ("assistant", "First answer"), ("user", "Follow-up?")]
        conn = use_conn(generate_worker, self._conn(fake_conn, history))
        handle_generate(self.JOB)

        assert model["search"] == [("ws-1", "Follow-up?", "proj-1")]
        sent = model["chat"][0]
        assert sent[0]["role"] == "system"
        assert [(m["role"], m["content"]) for m in sent[1:]] == history
        assert conn.statements("UPDATE messages") == [("Grounded reply.", "complete", "msg-9")]

    def test_history_is_capped(self, model, monkeypatch, fake_conn, use_conn):
        monkeypatch.setattr(generate_worker, "MAX_HISTORY_MESSAGES", 2)
        history = [("user", f"m{i}") for i in range(5)]
        use_conn(generate_worker, self._conn(fake_conn, history))
        handle_generate(self.JOB)
        assert [m["content"] for m in model["chat"][0][1:]] == ["m3", "m4"]

    def test_no_user_message_fails_gracefully(self, model, fake_conn, use_conn):
        conn = use_conn(generate_worker, self._conn(fake_conn, [("assistant", "hi")]))
        handle_generate(self.JOB)
        (content, status, msg_id), = conn.statements("UPDATE messages")
        assert status == "failed" and msg_id == "msg-9" and content
        assert model["chat"] == []

    def test_deleted_conversation_is_dropped(self, model, fake_conn, use_conn):
        conn = use_conn(generate_worker, fake_conn())
        handle_generate(self.JOB)
        assert conn.statements("UPDATE messages") == []
