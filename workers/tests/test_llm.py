import pytest

from src import llm


def test_chat_uses_the_offline_model_when_api_key_is_missing(monkeypatch):
    monkeypatch.setattr(llm, "OPENAI_API_KEY", "")
    monkeypatch.setattr(llm, "client", lambda: pytest.fail("called OpenAI without a key"))

    assert llm.chat([{"role": "user", "content": "Hello"}])
    assert llm.chat([], json_mode=True) == "{}"


def test_embeddings_are_deterministic_and_match_openai_dimensions(monkeypatch):
    monkeypatch.setattr(llm, "OPENAI_API_KEY", "")

    first, same, different = llm.embed_texts(["security controls", "security controls", "backup policy"])

    assert len(first) == llm.EMBED_DIMENSIONS
    assert first == same
    assert first != different
    assert sum(value * value for value in first) == pytest.approx(1.0)
