"""Central configuration for the Python workers.

Reads the shared repo-root .env so the workers use the same Postgres / Redis /
MinIO settings as the Go backend.
"""

import os
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

# --- Postgres ---
def _database_url() -> str:
    url = os.getenv("DATABASE_URL")
    if url:
        return url
    host = os.getenv("POSTGRES_HOST", "localhost")
    port = os.getenv("POSTGRES_PORT", "5432")
    user = os.getenv("POSTGRES_USER", "postgres")
    password = os.getenv("POSTGRES_PASSWORD", "postgres")
    name = os.getenv("POSTGRES_DB", "app")
    return f"postgresql://{user}:{password}@{host}:{port}/{name}"


DATABASE_URL = _database_url()

# --- MinIO / object storage ---
MINIO_ENDPOINT = os.getenv("MINIO_ENDPOINT", "localhost:9000")
MINIO_ACCESS_KEY = os.getenv("MINIO_ACCESS_KEY", "minioadmin")
MINIO_SECRET_KEY = os.getenv("MINIO_SECRET_KEY", "minioadmin")
MINIO_BUCKET = os.getenv("MINIO_BUCKET", "rfp-documents")
MINIO_USE_SSL = os.getenv("MINIO_USE_SSL", "false").lower() in ("1", "true", "yes")
