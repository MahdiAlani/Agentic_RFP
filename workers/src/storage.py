"""MinIO / object-storage access for the workers.

Used by the embed worker to pull an uploaded document's bytes by its file_key.
"""

from minio import Minio

from src.config import (
    MINIO_ACCESS_KEY,
    MINIO_BUCKET,
    MINIO_ENDPOINT,
    MINIO_SECRET_KEY,
    MINIO_USE_SSL,
)

client = Minio(
    MINIO_ENDPOINT,
    access_key=MINIO_ACCESS_KEY,
    secret_key=MINIO_SECRET_KEY,
    secure=MINIO_USE_SSL,
)


def get_object_bytes(file_key: str) -> bytes:
    """Download an object's full contents by key."""
    resp = client.get_object(MINIO_BUCKET, file_key)
    try:
        return resp.read()
    finally:
        resp.close()
        resp.release_conn()
