"""Worker entrypoint.

Runs both stream consumers (embed + generate) in one process, each in its own
thread. Each consumer reads its Redis stream via a consumer group, dispatches to
a handler, and XACKs on success. Failed jobs are left un-acked for redelivery.

Run:  cd workers && python main.py
"""

import logging
import signal
import threading
import time

import redis

from src.config import (
    CONSUMER_GROUP,
    REDIS_HOST,
    REDIS_PORT,
    STREAM_EMBED,
    STREAM_GENERATE,
    WORKER_NAME,
    WORKER_TYPE,
)
from src.embed_worker import handle_embed
from src.generate_worker import handle_generate

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s [%(name)s] %(message)s",
)
log = logging.getLogger("workers")

BLOCK_MS = 5000  # how long XREADGROUP blocks waiting for new messages

SOCKET_TIMEOUT = BLOCK_MS / 1000 + 10

RECLAIM_IDLE_MS = 300_000  # entries pending this long are assumed orphaned
RECLAIM_EVERY_S = 60
MAX_DELIVERIES = 3  # after this many attempts a job is dead-lettered

# Time to let in flight jobs finish after SIGTERM.
SHUTDOWN_WAIT_S = 240

shutdown = threading.Event()


def ensure_group(r: redis.Redis, stream: str, group: str) -> None:
    """Create the consumer group (and stream) if it doesn't exist yet."""
    try:
        r.xgroup_create(name=stream, groupname=group, id="0", mkstream=True)
        log.info("created consumer group %r on stream %r", group, stream)
    except redis.ResponseError as e:
        if "BUSYGROUP" in str(e):
            log.info("consumer group %r already exists on %r", group, stream)
        else:
            raise


def process(r: redis.Redis, stream: str, msg_id: str, fields: dict, handler) -> None:
    try:
        handler(fields)
    except Exception:
        log.exception("handler failed for %s on %r", msg_id, stream)
        return
    r.xack(stream, CONSUMER_GROUP, msg_id)


def reclaim(r: redis.Redis, stream: str, consumer_name: str, handler) -> None:
    """Take over entries left pending by workers that died mid job."""
    _cursor, messages, _deleted = r.xautoclaim(
        stream, CONSUMER_GROUP, consumer_name, min_idle_time=RECLAIM_IDLE_MS, count=10
    )
    for msg_id, fields in messages:
        pending = r.xpending_range(stream, CONSUMER_GROUP, msg_id, msg_id, 1)
        delivered = pending[0]["times_delivered"] if pending else 1
        if delivered > MAX_DELIVERIES:
            log.error("dead-lettering %s on %r after %d attempts", msg_id, stream, delivered)
            r.xadd(f"{stream}:dead", fields)
            r.xack(stream, CONSUMER_GROUP, msg_id)
            continue
        log.warning("reclaimed %s on %r (attempt %d)", msg_id, stream, delivered)
        process(r, stream, msg_id, fields, handler)


def consume(r: redis.Redis, stream: str, consumer_name: str, handler) -> None:
    """Blocking consume loop for a single stream."""
    ensure_group(r, stream, CONSUMER_GROUP)
    log.info("consuming %r as %r", stream, consumer_name)
    next_reclaim = 0.0
    while not shutdown.is_set():
        if time.time() >= next_reclaim:
            next_reclaim = time.time() + RECLAIM_EVERY_S
            try:
                reclaim(r, stream, consumer_name, handler)
            except Exception:
                log.exception("reclaim failed on %r", stream)

        try:
            resp = r.xreadgroup(
                CONSUMER_GROUP, consumer_name, {stream: ">"}, count=1, block=BLOCK_MS
            )
        except redis.exceptions.TimeoutError:
            continue
        except redis.exceptions.ConnectionError:
            log.warning("redis connection lost on %r, retrying", stream)
            time.sleep(1)
            continue
        except Exception:
            log.exception("unexpected read error on %r, retrying", stream)
            time.sleep(1)
            continue

        if not resp:
            continue
        for _stream, messages in resp:
            for msg_id, fields in messages:
                process(r, stream, msg_id, fields, handler)

    log.info("consumer %r stopped", consumer_name)


def _on_signal(signum, _frame) -> None:
    log.info("signal %s received, draining", signum)
    shutdown.set()


def main() -> None:
    signal.signal(signal.SIGTERM, _on_signal)
    signal.signal(signal.SIGINT, _on_signal)

    r = redis.Redis(
        host=REDIS_HOST,
        port=REDIS_PORT,
        decode_responses=True,
        socket_timeout=SOCKET_TIMEOUT,
        socket_keepalive=True,

        health_check_interval=30,
    )
    r.ping()
    log.info("connected to redis at %s:%s", REDIS_HOST, REDIS_PORT)

    # WORKER_TYPE selects which pipelines this process serves, so embed and
    # generate can run on separate machines (or scale independently).
    consumers = []
    if WORKER_TYPE in ("all", "embed"):
        consumers.append((STREAM_EMBED, f"embed-{WORKER_NAME}", handle_embed))
    if WORKER_TYPE in ("all", "generate"):
        consumers.append((STREAM_GENERATE, f"generate-{WORKER_NAME}", handle_generate))

    if not consumers:
        log.error(
            "WORKER_TYPE=%r is not one of: all, embed, generate. Exiting.", WORKER_TYPE
        )
        return

    threads = []
    for stream, name, handler in consumers:
        t = threading.Thread(
            target=consume, args=(r, stream, name, handler), name=name, daemon=True
        )
        t.start()
        threads.append(t)

    while not shutdown.is_set():
        for t in threads:
            t.join(timeout=1)
            if not t.is_alive():
                log.error("consumer %r died; exiting", t.name)
                return

    deadline = time.time() + SHUTDOWN_WAIT_S
    for t in threads:
        t.join(timeout=max(0, deadline - time.time()))
    log.info("shutdown complete")


if __name__ == "__main__":
    main()
