"""OpenAI access: embeddings + chat completions.
"""

import logging

from openai import OpenAI

from src.config import OPENAI_API_KEY, OPENAI_CHAT_MODEL, OPENAI_EMBED_MODEL

log = logging.getLogger("llm")
EMBED_BATCH = 100

_client: OpenAI | None = None


def client() -> OpenAI:
    global _client
    if _client is None:
        if not OPENAI_API_KEY:
            raise RuntimeError("OPENAI_API_KEY is not set")
        _client = OpenAI(api_key=OPENAI_API_KEY)
    return _client


def embed_texts(texts: list[str]) -> list[list[float]]:
    """Embed a list of strings, preserving input order."""
    vectors: list[list[float]] = []
    for start in range(0, len(texts), EMBED_BATCH):
        batch = texts[start : start + EMBED_BATCH]
        resp = client().embeddings.create(model=OPENAI_EMBED_MODEL, input=batch)
        # The API may return items out of order; `index` maps back to the input.
        vectors.extend(item.embedding for item in sorted(resp.data, key=lambda d: d.index))
    return vectors


def embed_text(text: str) -> list[float]:
    """Embed a single string."""
    return embed_texts([text])[0]


def chat(messages: list[dict], temperature: float = 0.2) -> str:
    """Run a chat completion and return the reply text."""
    resp = client().chat.completions.create(
        model=OPENAI_CHAT_MODEL,
        messages=messages,
        temperature=temperature,
    )
    return (resp.choices[0].message.content or "").strip()
