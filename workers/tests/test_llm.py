import pytest

from src import llm


def test_chat_uses_mock_when_api_key_is_missing(monkeypatch):
    monkeypatch.setattr(llm, "OPENAI_API_KEY", "")

    assert llm.chat([{"role": "user", "content": "Hello"}]) == (
        "Mock response (OPENAI_API_KEY is not configured)."
    )
    assert llm.chat([], json_mode=True) == "{}"


def test_embeddings_are_deterministic_and_match_openai_dimensions(monkeypatch):
    monkeypatch.setattr(llm, "OPENAI_API_KEY", "")

    first, same, different = llm.embed_texts(["security controls", "security controls", "backup policy"])

    assert len(first) == llm.EMBED_DIMENSIONS
    assert first == same
    assert first != different
    assert sum(value * value for value in first) == pytest.approx(1.0)
