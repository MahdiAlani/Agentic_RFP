"""Redis stream consumer behaviour: ack on success, redeliver on failure,
reclaim orphaned jobs, and dead-letter jobs that keep failing."""

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
