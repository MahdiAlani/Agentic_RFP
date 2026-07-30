"""Postgres access for the workers (psycopg 3)."""

import psycopg
from psycopg_pool import ConnectionPool

from src.config import DATABASE_URL

# A small shared connection pool. psycopg rows come back as tuples by default;
# pass row_factory at query time if you want dicts.
pool = ConnectionPool(DATABASE_URL, min_size=1, max_size=5, open=True)


def get_connection() -> psycopg.Connection:
    """Borrow a connection from the pool (use as a context manager)."""
    return pool.connection()


def to_vector(embedding: list[float]) -> str:
    """Render an embedding as pgvector's text form, e.g. '[0.1,0.2]'.

    Passed as a normal string parameter and cast with `::vector` in SQL, which
    avoids needing a type adapter registered on every connection.
    """
    return "[" + ",".join(repr(float(x)) for x in embedding) + "]"
