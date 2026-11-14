"""Consumer behaviour. Redis: ack on success, redeliver on failure, reclaim
orphaned jobs, dead-letter jobs that keep failing. Kafka: commit once handled,
retry in place, then dead-letter."""

import pytest

import main


class FakeRedis:
    def __init__(self, claimed=(), deliveries=None):
        self.claimed = list(claimed)
        self.deliveries = deliveries or {}
        self.acked = []
        self.added = []

    def xack(self, stream, group, msg_id):
        self.acked.append((stream, msg_id))

    def xadd(self, stream, fields):
        self.added.append((stream, fields))

    def xautoclaim(self, stream, group, consumer, min_idle_time, count):
        return "0-0", self.claimed, []

    def xpending_range(self, stream, group, start, end, count):
        n = self.deliveries.get(start)
        return [{"message_id": start, "times_delivered": n}] if n else []


def test_process_acks_after_handler_succeeds():
    r, seen = FakeRedis(), []
    main.process(r, "embed", "1-0", {"document_id": "d"}, seen.append)
    assert seen == [{"document_id": "d"}]
    assert r.acked == [("embed", "1-0")]


def test_process_leaves_failed_job_pending_for_redelivery():
    r = FakeRedis()

    def boom(fields):
        raise RuntimeError("model timeout")

    main.process(r, "generate", "2-0", {}, boom)
    assert r.acked == []


def test_reclaim_retries_orphaned_jobs():
    r, seen = FakeRedis(claimed=[("3-0", {"answer_id": "a"})], deliveries={"3-0": 2}), []
    main.reclaim(r, "generate", "generate-x", seen.append)
    assert seen == [{"answer_id": "a"}]
    assert r.acked == [("generate", "3-0")]
    assert r.added == []


def test_reclaim_dead_letters_after_max_deliveries():
    fields = {"answer_id": "poison"}
    r = FakeRedis(claimed=[("4-0", fields)], deliveries={"4-0": main.MAX_DELIVERIES + 1})

    def must_not_run(_):
        raise AssertionError("dead-lettered job was processed again")

    main.reclaim(r, "generate", "generate-x", must_not_run)
    assert r.added == [("generate:dead", fields)]
    # Acked on the source stream so it stops being reclaimed.
    assert r.acked == [("generate", "4-0")]


def test_reclaim_at_exactly_max_deliveries_still_retries():
    r, seen = FakeRedis(claimed=[("5-0", {})], deliveries={"5-0": main.MAX_DELIVERIES}), []
    main.reclaim(r, "embed", "embed-x", seen.append)
    assert seen == [{}] and r.added == []


class FakeMessage:
    def __init__(self, value):
        self._value = value

    def value(self):
        return self._value

    def topic(self):
        return "documents.uploaded"

    def partition(self):
        return 0

    def offset(self):
        return 7


@pytest.fixture
def no_backoff(monkeypatch):
    monkeypatch.setattr(main.shutdown, "wait", lambda seconds: False)


def test_event_is_committed_after_handler_succeeds():
    seen, dead = [], []
    assert main.handle_event(FakeMessage(b'{"document_id": "d"}'), seen.append, dead.append)
    assert seen == [{"document_id": "d"}]
    assert dead == []


def test_failing_event_is_retried_then_dead_lettered(no_backoff):
    calls, dead = [], []
    msg = FakeMessage(b'{"document_id": "poison"}')

    def boom(event):
        calls.append(event)
        raise RuntimeError("storage down")

    assert main.handle_event(msg, boom, dead.append)
    assert len(calls) == main.MAX_DELIVERIES
    assert dead == [msg]


def test_event_succeeding_on_retry_is_not_dead_lettered(no_backoff):
    attempts, dead = [], []

    def flaky(event):
        attempts.append(event)
        if len(attempts) == 1:
            raise RuntimeError("blip")

    assert main.handle_event(FakeMessage(b"{}"), flaky, dead.append)
    assert len(attempts) == 2 and dead == []


def test_unreadable_event_is_dead_lettered_without_running_handler():
    dead = []
    msg = FakeMessage(b"not json")
    assert main.handle_event(msg, lambda e: pytest.fail("handler ran"), dead.append)
    assert dead == [msg]


def test_shutdown_during_backoff_leaves_event_uncommitted(monkeypatch):
    monkeypatch.setattr(main.shutdown, "wait", lambda seconds: True)
    dead = []

    def boom(event):
        raise RuntimeError("storage down")

    # Not committed, so another worker picks the event up after the rebalance.
    assert not main.handle_event(FakeMessage(b"{}"), boom, dead.append)
    assert dead == []
