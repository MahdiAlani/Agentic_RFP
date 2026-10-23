"""Central configuration for the Python workers.

Reads the shared repo-root .env so the workers use the same Postgres / Redis /
MinIO settings as the Go backend.
"""

import os
import uuid
from pathlib import Path

from dotenv import load_dotenv

# workers/src/config.py -> repo root is two levels up.
ROOT = Path(__file__).resolve().parents[2]
load_dotenv(ROOT / ".env")

# --- Redis ---
REDIS_ADDR = os.getenv("REDIS_ADDR", "localhost:6379")
REDIS_HOST, _, _port = REDIS_ADDR.partition(":")
REDIS_PORT = int(_port or "6379")

# --- Streams / consumer group ---
STREAM_EMBED = "embed"
STREAM_GENERATE = "generate"
CONSUMER_GROUP = "workers"

# Which pipelines this process runs: "all", "embed", or "generate". Running one
# process per type lets the slow embed work scale separately from chat replies.
WORKER_TYPE = os.getenv("WORKER_TYPE", "all").lower()

# Consumer names must be unique within a group so Redis can track which entries
# each worker holds. Defaults to a random suffix; set WORKER_NAME to something
# stable (e.g. the container name) if you want to inspect pending entries later.
WORKER_NAME = os.getenv("WORKER_NAME", "") or uuid.uuid4().hex[:6]

# --- Postgres ---
def _database_url() -> str:
    url = os.getenv("DATABASE_URL")
    if url:
        return url
    host = os.getenv("POSTGRES_HOST", "localhost")
    port = os.getenv("POSTGRES_PORT", "5432")
    user = os.getenv("POSTGRES_USER", "postgres")
    password = os.getenv("POSTGRES_PASSWORD", "")
    name = os.getenv("POSTGRES_DB", "app")
    return f"postgresql://{user}:{password}@{host}:{port}/{name}"


DATABASE_URL = _database_url()

# --- MinIO / object storage ---
MINIO_ENDPOINT = os.getenv("MINIO_ENDPOINT", "localhost:9000")
MINIO_ACCESS_KEY = os.getenv("MINIO_ACCESS_KEY", "")
MINIO_SECRET_KEY = os.getenv("MINIO_SECRET_KEY", "")
MINIO_BUCKET = os.getenv("MINIO_BUCKET", "rfp-documents")
MINIO_USE_SSL = os.getenv("MINIO_USE_SSL", "false").lower() == "true"

# --- OpenAI ---
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY", "")
OPENAI_CHAT_MODEL = os.getenv("OPENAI_CHAT_MODEL", "gpt-4o-mini")
OPENAI_EMBED_MODEL = os.getenv("OPENAI_EMBED_MODEL", "text-embedding-3-small")
# Grades answers (quality checks, eval). A stronger model than the one that
# drafts, so it catches mistakes rather than sharing them.
OPENAI_JUDGE_MODEL = os.getenv("OPENAI_JUDGE_MODEL", "gpt-4o")

# --- Chunking / retrieval ---
CHUNK_SIZE = int(os.getenv("CHUNK_SIZE", "1200"))
CHUNK_OVERLAP = int(os.getenv("CHUNK_OVERLAP", "150"))

# How many chunks to retrieve as grounding context per question.
RETRIEVAL_TOP_K = int(os.getenv("RETRIEVAL_TOP_K", "10"))

# How many prior chat messages to replay into the model.
MAX_HISTORY_MESSAGES = int(os.getenv("MAX_HISTORY_MESSAGES", "20"))

# Characters of an RFP fed to the question-extraction prompt (~4 chars/token).
EXTRACT_MAX_CHARS = int(os.getenv("EXTRACT_MAX_CHARS", "120000"))
