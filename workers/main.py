"""Worker entrypoint.

Runs one consumer thread per pipeline:
    index     Kafka topic `documents.uploaded` -> chunks a document and queues
              its embedding batches on the Redis `embed` stream
    embed     Redis stream `embed` -> embeds one batch (and extracts RFP questions)
    generate  Redis stream `generate` -> drafts answers and chat replies

Redis consumers XACK on success and leave failed jobs pending for redelivery.
The Kafka consumer commits an event's offset only once it has been handled.

Run:  cd workers && python main.py
"""

import json
import logging
import signal
import threading
import time
from functools import partial

import redis
from confluent_kafka import Consumer, KafkaError, KafkaException, Producer
from confluent_kafka.admin import AdminClient, NewTopic

from src.config import (
    CONSUMER_GROUP,
    KAFKA_BROKERS,
    REDIS_HOST,
    REDIS_PORT,
    STREAM_EMBED,
    STREAM_GENERATE,
    TOPIC_DOCUMENT_UPLOADED,
    TOPIC_PARTITIONS,
    WORKER_NAME,
    WORKER_TYPE,
)
from src.embed_worker import handle_embed
from src.generate_worker import handle_generate
from src.index_worker import handle_index

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


def ensure_topics(topics: list[str]) -> None:
    """Create the topics if they don't exist yet."""
    admin = AdminClient({"bootstrap.servers": KAFKA_BROKERS})
    futures = admin.create_topics([NewTopic(t, num_partitions=TOPIC_PARTITIONS) for t in topics])
    for topic, future in futures.items():
        try:
            future.result()
            log.info("created topic %r", topic)
        except KafkaException as e:
            if e.args[0].code() != KafkaError.TOPIC_ALREADY_EXISTS:
                raise


def handle_event(msg, handler, dead_letter) -> bool:
    """Handle one event; returns whether its offset may be committed.

    A partition is read in order, so a failing event is retried in place rather
    than set aside, then dead-lettered so the events behind it can move on.
    """
    try:
        event = json.loads(msg.value())
    except (TypeError, ValueError):
        log.error("unreadable event at %s[%d]@%d", msg.topic(), msg.partition(), msg.offset())
        dead_letter(msg)
        return True

    for attempt in range(1, MAX_DELIVERIES + 1):
        try:
            handler(event)
            return True
        except Exception:
            log.exception("handler failed for %s[%d]@%d (attempt %d)",
                          msg.topic(), msg.partition(), msg.offset(), attempt)
        if attempt < MAX_DELIVERIES and shutdown.wait(attempt):
            return False

    log.error("dead-lettering %s[%d]@%d after %d attempts",
              msg.topic(), msg.partition(), msg.offset(), MAX_DELIVERIES)
    dead_letter(msg)
    return True


def consume_topic(topic: str, consumer_name: str, handler) -> None:
    """Blocking consume loop for a single Kafka topic."""
    consumer = Consumer({
        "bootstrap.servers": KAFKA_BROKERS,
        "group.id": CONSUMER_GROUP,
        "client.id": consumer_name,
        "enable.auto.commit": False,
        "auto.offset.reset": "earliest",
    })
    producer = Producer({"bootstrap.servers": KAFKA_BROKERS})

    def dead_letter(msg) -> None:
        producer.produce(f"{topic}.dead", key=msg.key(), value=msg.value())
        if producer.flush(10):
            raise RuntimeError(f"timed out dead-lettering to {topic}.dead")

    consumer.subscribe([topic])
    log.info("consuming %r as %r", topic, consumer_name)
    try:
        while not shutdown.is_set():
            msg = consumer.poll(1.0)
            if msg is None:
                continue
            if msg.error():
                log.warning("kafka error on %r: %s", topic, msg.error())
                continue
            if handle_event(msg, handler, dead_letter):
                consumer.commit(message=msg, asynchronous=False)
    finally:
        # Leaves the group straight away so its partitions move to another worker.
        consumer.close()
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

    # WORKER_TYPE selects which pipelines this process serves, so each stage
    # can run on separate machines (or scale independently).
    consumers = []
    if WORKER_TYPE in ("all", "index"):
        ensure_topics([TOPIC_DOCUMENT_UPLOADED, f"{TOPIC_DOCUMENT_UPLOADED}.dead"])
        index = partial(handle_index, enqueue=partial(r.xadd, STREAM_EMBED))
        consumers.append((consume_topic, TOPIC_DOCUMENT_UPLOADED, f"index-{WORKER_NAME}", index))
    if WORKER_TYPE in ("all", "embed"):
        consumers.append((partial(consume, r), STREAM_EMBED, f"embed-{WORKER_NAME}", handle_embed))
    if WORKER_TYPE in ("all", "generate"):
        consumers.append(
            (partial(consume, r), STREAM_GENERATE, f"generate-{WORKER_NAME}", handle_generate)
        )

    if not consumers:
        log.error(
            "WORKER_TYPE=%r is not one of: all, index, embed, generate. Exiting.", WORKER_TYPE
        )
        return

    threads = []
    for loop, source, name, handler in consumers:
        t = threading.Thread(
            target=loop, args=(source, name, handler), name=name, daemon=True
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
