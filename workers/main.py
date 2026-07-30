"""Worker entrypoint.

Runs both stream consumers (embed + generate) in one process, each in its own
thread. Each consumer reads its Redis stream via a consumer group, dispatches to
a handler, and XACKs on success. Failed jobs are left un-acked for redelivery.

Run:  cd workers && python main.py
"""

import logging
import threading

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


def consume(r: redis.Redis, stream: str, consumer_name: str, handler) -> None:
    """Blocking consume loop for a single stream."""
    ensure_group(r, stream, CONSUMER_GROUP)
    log.info("consuming %r as %r", stream, consumer_name)
    while True:
        resp = r.xreadgroup(
            CONSUMER_GROUP, consumer_name, {stream: ">"}, count=1, block=BLOCK_MS
        )
        if not resp:
            continue
        for _stream, messages in resp:
            for msg_id, fields in messages:
                try:
                    handler(fields)
                    r.xack(stream, CONSUMER_GROUP, msg_id)
                except Exception:
                    # Left un-acked -> visible via XPENDING for retry/claim.
                    log.exception("handler failed for %s on %r", msg_id, stream)


def main() -> None:
    r = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, decode_responses=True)
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

    for t in threads:
        t.join()


if __name__ == "__main__":
    main()
