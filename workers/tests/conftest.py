"""Shared fixtures for the worker unit tests.

These tests run without Postgres, Redis, MinIO or OpenAI: the connection pool
is replaced before `src.db` is imported, and each test swaps in fakes for the
database connection and the model calls.
"""

import psycopg_pool
import pytest


class _NoPool:
    """Stands in for ConnectionPool so importing src.db doesn't dial Postgres."""

    def __init__(self, *args, **kwargs):
        pass

    def connection(self):
        raise AssertionError("tests must patch get_connection, not use the real pool")


psycopg_pool.ConnectionPool = _NoPool


class FakeResult:
    def __init__(self, rows):
        self.rows = rows

    def fetchone(self):
        return self.rows[0] if self.rows else None

    def fetchall(self):
        return list(self.rows)


class FakeConn:
    """Records every statement; SELECTs return rows scripted by SQL substring.

    `responses` maps a substring of the SQL to the rows that query returns.
    Statements are recorded as (sql, params) in `executed`, and executemany
    batches as (sql, rows) in `batches`.
    """

    def __init__(self, responses=None):
        self.responses = responses or {}
        self.executed = []
        self.batches = []

    # used as `with get_connection() as conn`
    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def execute(self, sql, params=None):
        self.executed.append((sql, params))
        for needle, rows in self.responses.items():
            if needle in sql:
                return FakeResult(rows)
        return FakeResult([])

    def executemany(self, sql, rows):
        self.batches.append((sql, list(rows)))

    def cursor(self):
        return self

    def statements(self, needle):
        """Params of every executed statement whose SQL contains `needle`."""
        return [p for sql, p in self.executed if needle in sql]


@pytest.fixture
def fake_conn():
    return FakeConn


@pytest.fixture
def use_conn(monkeypatch):
    """Point a module's get_connection at a FakeConn: use_conn(module, conn)."""

    def _use(module, conn):
        monkeypatch.setattr(module, "get_connection", lambda: conn)
        return conn

    return _use
