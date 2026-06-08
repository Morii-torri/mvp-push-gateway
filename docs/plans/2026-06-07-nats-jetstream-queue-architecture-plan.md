# NATS JetStream Queue Architecture Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the PostgreSQL table-queue hot path with NATS JetStream for route planning and outbound delivery, while keeping PostgreSQL as the source of truth for business records, audit, logs, and final searchable state.

**Architecture:** Inbound requests persist the message record and publish a durable route-plan event to JetStream. Planning workers consume route-plan events, create delivery attempts in PostgreSQL, and publish send-message events to JetStream. Delivery workers consume send-message events, dispatch HTTP requests, publish durable result events to `MGP_RESULT`, and then acknowledge the send-message event. Result writer workers consume `MGP_RESULT` and batch-write final delivery state to PostgreSQL. PostgreSQL no longer acts as the high-frequency queue claim/ack engine.

**Tech Stack:** Go, PostgreSQL, NATS JetStream, durable pull consumers, explicit ack, async JetStream publish, batched PostgreSQL result writer.

---

## Current Bottleneck Summary

The current implementation already batches delivery completion, but it is still synchronous with the delivery worker:

- The delivery worker sends HTTP requests.
- It collects completion results for the batch.
- It synchronously waits for PostgreSQL updates on `delivery_attempts` and `jobs`.
- Only after the batch write returns can the worker fully release that batch.

Recent pressure tests show the queue hot path is database-bound:

- `send_message` job claiming has high P95 at large concurrency.
- `delivery_attempts/jobs` completion writes grow sharply.
- DB connection wait time grows with concurrency.
- The fake upstream itself is about 1 ms, so outbound network is not the bottleneck.

JetStream should be used to remove PostgreSQL from the high-frequency queue claim/ack loop. PostgreSQL should still store durable business state and queryable logs.

---

## Target Semantics

### Durable Mode

Use this as the only production mode in the JetStream migration. Do not implement a lossy acknowledgement mode in this milestone.

- Inbound returns success only after:
  - message record is persisted
  - route-plan event is published to JetStream and acknowledged by NATS
- Planning event is acked only after:
  - route selection finishes
  - delivery attempts are persisted
  - send-message events are published to JetStream
- Send-message event is acked only after:
  - HTTP request finishes
  - a result event is published to `MGP_RESULT`
  - JetStream confirms the result event was accepted
- Result-writer consumers ack `MGP_RESULT` only after PostgreSQL final-state write succeeds.
- If the process crashes after `MGP_RESULT` publish and before PostgreSQL result persistence, the result event is redelivered to result writers.
- If the process crashes after HTTP send but before `MGP_RESULT` publish, the send-message event is redelivered. This can duplicate the upstream HTTP side effect, so delivery idempotency must rely on `delivery_attempt_id`, send dedupe keys, provider-side idempotency where available, and result event message ID dedupe.

Out of scope:

- No lossy acknowledgement mode.
- No memory-only result mode.
- No “ack before durable result event” mode.

---

## JetStream Model

### Streams

Create three streams:

1. `MGP_ROUTE_PLAN`
   - Subjects: `mgp.route_plan.*`
   - Retention: work queue
   - Storage: file
   - Replicas: configurable, default `1` for local dev, `3` for HA
   - Max age: match operational retention for pending work, for example 7 days

2. `MGP_SEND`
   - Subjects: `mgp.send.*`
   - Retention: work queue
   - Storage: file
   - Replicas: configurable
   - Max age: based on retry/dead-letter window

3. `MGP_RESULT`
   - Subjects: `mgp.result.*`
   - Retention: work queue
   - Storage: file
   - Replicas: configurable
   - Purpose: durable result-write queue. It decouples outbound dispatch from PostgreSQL result persistence without accepting result loss.

### Consumers

Use durable pull consumers:

- `route-plan-workers`
  - stream: `MGP_ROUTE_PLAN`
  - filter: `mgp.route_plan.*`
  - ack policy: explicit
  - max ack pending: configurable
  - ack wait: configurable, based on route planning timeout

- `send-workers`
  - stream: `MGP_SEND`
  - filter: `mgp.send.*` or `mgp.send.<channel_id_hash>`
  - ack policy: explicit
  - max ack pending: configurable
  - ack wait: configurable, based on channel timeout + retry buffer

- `result-writers`
  - stream: `MGP_RESULT`
  - durable consumer name: `result-writers`
  - filter: `mgp.result.*`
  - ack policy: explicit
  - max ack pending: configurable
  - ack wait: based on PostgreSQL result-write timeout
  - ack after PostgreSQL result write succeeds

Use subject partitioning only if needed:

- Basic: `mgp.send.default`
- Better: `mgp.send.<provider_type>.<channel_id_hash>`
- If per-channel ordering or rate limit isolation becomes important, use channel hash subjects and consumer filters.

---

## PostgreSQL Model After JetStream

Keep these tables:

- `message_records`
- `delivery_attempts`
- `dead_letter_jobs`
- audit/log tables
- provider/source/template/route configuration tables

Change the role of `jobs`:

- Keep it during migration for compatibility and UI fallback.
- Stop using it as the primary high-QPS queue.
- Later either:
  - retain it only as a low-rate maintenance/dead-letter table, or
  - replace queue monitoring with JetStream consumer/stream metrics and remove hot-path writes.

Add mapping columns/tables:

- `message_records.route_plan_msg_id`
- `delivery_attempts.send_msg_id`
- `delivery_attempts.dispatch_status`
  - `pending`
  - `published`
  - `dispatching`
  - `dispatched`
  - `sent`
  - `failed`
  - `dead`
- `delivery_attempts.dispatched_at`
- `delivery_attempts.result_persisted_at`

Add idempotency keys:

- route-plan message ID: `trace_id` or `message_id`
- send-message message ID: `delivery_attempt_id`
- result message ID: `delivery_attempt_id:attempt_no`

---

## End-to-End Flow

### Inbound

1. Authenticate source.
2. Validate payload.
3. Insert `message_records`.
4. Publish route-plan event to JetStream using async publish with message ID.
5. Record `route_plan_msg_id`.
6. Return 202.

If JetStream publish fails:

- return failure and mark message `queue_publish_failed`
- do not silently claim accepted if the route-plan event was not durably queued

### Planning Worker

1. Pull batch from `MGP_ROUTE_PLAN`.
2. Load route execution model from cache.
3. Evaluate routing.
4. Render templates.
5. Insert `delivery_attempts` in batch.
6. Publish send-message events to `MGP_SEND` with `delivery_attempt_id` as message ID.
7. Ack route-plan message after DB insert and JetStream publish succeed.

If planning fails:

- retry with `NakWithDelay` for transient errors
- write dead-letter record for permanent route/template errors
- ack only after dead-letter state is persisted

### Delivery Worker

1. Pull batch from `MGP_SEND`.
2. Group by channel.
3. Apply per-channel concurrency and rate limit.
4. Build request.
5. Dispatch HTTP.
6. Build a result event containing final status, request snapshot reference, response snapshot reference, duration, retry/dead-letter decision, and attempt metadata.
7. Publish the result event to `MGP_RESULT` using `delivery_attempt_id:attempt_no` as the JetStream message ID.
8. Ack the send-message event only after JetStream confirms the result event publish.
9. If result publish fails, do not ack the send-message event; allow redelivery and rely on idempotency/dedupe.

### Result Writer

- Pull batches from `MGP_RESULT`.
- Batch PostgreSQL updates by size and interval, for example 500 rows or 50 ms.
- Persist `delivery_attempts` final status, response snapshots, dead-letter records, retry state, metrics, and searchable message status.
- Ack each result event only after its PostgreSQL write succeeds.
- On partial batch failure, ack successful rows and `NakWithDelay` failed rows where the NATS client supports per-message acknowledgement.
- On shutdown, drain outstanding writes with timeout.

---

## Performance Metrics After Migration

Split metrics into these counters:

- `inbound_accepted_qps`: HTTP ingress accepted
- `route_plan_publish_qps`: inbound to JetStream route-plan publish
- `route_plan_processed_qps`: planning consumer ack rate
- `send_publish_qps`: send-message event publish rate
- `dispatch_qps`: HTTP request written to upstream
- `upstream_response_qps`: upstream response classified
- `result_persist_qps`: delivery result written to PostgreSQL

Split P95:

- `inbound_write_p95`
- `route_plan_queue_wait_p95`
- `planning_p95`
- `send_queue_wait_p95`
- `dispatch_p95`
- `upstream_roundtrip_p95`
- `result_persist_p95`

Expose JetStream metrics:

- stream pending messages
- consumer pending
- redelivered
- ack pending
- num waiting pull requests
- consumer ack floor
- publish ack latency

---

## UI Changes

### System Settings

Add queue mode settings:

- Queue backend:
  - PostgreSQL
  - NATS JetStream
- Result persistence:
  - JetStream result queue
- NATS URL
- NATS credentials path or token
- stream replicas
- route consumer workers
- send consumer workers
- result writer workers
- result writer batch size
- result writer flush interval

### Performance Test

Replace the current single “完成” view with a pipeline report:

- 入站接收
- 路由发布
- 路由处理
- 发送发布
- 请求发出
- 上级响应
- 结果落库

The headline QPS should default to `dispatch_qps`, because the product goal is “从入站到请求发出”. Keep `result_persist_qps` as a separate durability metric.

### Queue Monitoring

Add JetStream tab:

- Streams
- Consumers
- Pending
- Ack pending
- Redeliveries
- Oldest pending age
- Publish/ack latency

Keep PostgreSQL queue monitoring only when queue backend is PostgreSQL or for maintenance jobs.

---

## Implementation Tasks

### Task 1: Add Queue Abstraction

**Files:**

- Modify: `backend/internal/queue/types.go`
- Create: `backend/internal/queue/broker.go`
- Modify: `backend/internal/runtime/harness.go`

Create interfaces:

```go
type Broker interface {
    PublishRoutePlan(ctx context.Context, payload RoutePlanEvent) (PublishResult, error)
    PublishSend(ctx context.Context, payload SendMessageEvent) (PublishResult, error)
    PublishResult(ctx context.Context, payload DeliveryResultEvent) (PublishResult, error)
    SubscribeRoutePlan(ctx context.Context, handler RoutePlanHandler) error
    SubscribeSend(ctx context.Context, handler SendHandler) error
    SubscribeResult(ctx context.Context, handler ResultHandler) error
}
```

Keep PostgreSQL adapter behind the same interface for rollback.

### Task 2: Add NATS Configuration

**Files:**

- Modify: `backend/internal/config/config.go`
- Modify: `scripts/dev-backend.sh`
- Modify: `.env.example`

Add:

- `MGP_QUEUE_BACKEND=postgres|jetstream`
- `MGP_NATS_URL=nats://127.0.0.1:4222`
- `MGP_NATS_CREDS=`
- `MGP_NATS_STREAM_REPLICAS=1`
- `MGP_NATS_ROUTE_CONSUMERS=20`
- `MGP_NATS_SEND_CONSUMERS=20`
- `MGP_NATS_RESULT_CONSUMERS=10`
- `MGP_RESULT_WRITER_BATCH_SIZE=500`
- `MGP_RESULT_WRITER_FLUSH_INTERVAL_MS=50`

### Task 3: Add JetStream Client Adapter

**Files:**

- Create: `backend/internal/queue/jetstream.go`
- Create: `backend/internal/queue/jetstream_test.go`

Use:

- async publish for throughput
- message ID headers for dedupe
- durable pull consumers
- explicit ack
- `NakWithDelay` for retryable failures

### Task 4: Move Route Plan Queue To Broker

**Files:**

- Modify: `backend/internal/source/service.go`
- Modify: `backend/internal/db/source.go`
- Modify: `backend/internal/planning/worker.go`

Change inbound enqueue:

- keep `message_records` insert in PostgreSQL
- publish route-plan event through broker
- do not insert hot-path `jobs(route_plan)` when JetStream is enabled

### Task 5: Move Send Queue To Broker

**Files:**

- Modify: `backend/internal/planning/worker.go`
- Modify: `backend/internal/db/planning.go`
- Modify: `backend/internal/delivery/service.go`

Change planning completion:

- insert `delivery_attempts`
- publish send-message events through broker
- do not insert `jobs(send_message)` when JetStream is enabled

### Task 6: Add Durable Result Queue

**Files:**

- Modify: `backend/internal/delivery/service.go`
- Create: `backend/internal/delivery/result_event.go`
- Create: `backend/internal/delivery/result_writer.go`
- Create: `backend/internal/delivery/result_writer_test.go`
- Modify: `backend/internal/db/delivery.go`
- Modify: `backend/internal/runtime/harness.go`

Introduce:

```go
type ResultWriter interface {
    Process(ctx context.Context, events []DeliveryResultEvent) error
    Flush(ctx context.Context) error
}
```

Rules:

- delivery worker publishes `DeliveryResultEvent` to `MGP_RESULT`
- delivery worker acks send-message event after result publish ack
- result writer acks result event after PostgreSQL write succeeds
- no in-memory best-effort result mode

### Task 7: Monitoring

**Files:**

- Modify: `backend/internal/monitoring/service.go`
- Modify: `backend/internal/http/monitoring_handlers.go`
- Modify: `frontend/src/pages/ConsolePages.tsx`

Add JetStream stream/consumer metrics and separate:

- dispatch QPS
- upstream response QPS
- result persist QPS

### Task 8: Migration And Rollback

**Files:**

- Modify: `docs/operations/operator-guide.md`
- Modify: `docker-compose.yml`

Migration path:

1. Add NATS service.
2. Keep PostgreSQL queue backend as default.
3. Enable JetStream in staging.
4. Dual-write queue events for one test window if needed.
5. Switch consumers to JetStream.
6. Keep PostgreSQL jobs fallback for one release.
7. Remove hot-path PostgreSQL jobs after metrics prove stable.

Rollback:

- Set `MGP_QUEUE_BACKEND=postgres`.
- Stop JetStream consumers.
- PostgreSQL business records remain compatible.

---

## Test Plan

### Unit Tests

- JetStream publish uses message ID dedupe.
- Pull consumer acks only after handler succeeds.
- Retryable handler failure sends NAK with delay.
- Permanent handler failure writes dead-letter and ACKs.
- Result writer batches by size and time.
- Send-message ACK waits for `MGP_RESULT` publish acknowledgement.
- Result event ACK waits for PostgreSQL result persistence.

### Integration Tests

- Inbound creates message and route-plan stream message.
- Planning consumes route-plan and publishes send-message.
- Delivery consumes send-message and dispatches fake upstream.
- Crash simulation before ACK causes redelivery.
- Duplicate send-message event does not create duplicate attempt.
- Result writer drains on shutdown.

### Performance Tests

Run same matrix:

- 1000
- 5000
- 10000
- 20000 if local machine allows

Compare:

- `dispatch_qps`
- `result_persist_qps`
- DB connection wait
- JetStream pending
- redeliveries
- stream storage growth
- result stream publish latency
- result writer persist QPS

---

## Expected Impact

Expected improvements:

- `send_message` claim P95 should drop sharply because claiming moves from PostgreSQL row locks to JetStream pull consumer delivery.
- `dispatch_qps` should rise because workers stop competing on `jobs` table updates before dispatch.
- DB wait should fall because PostgreSQL no longer handles every queue claim/heartbeat/status transition.

Still remaining bottlenecks:

- Inbound still writes `message_records`.
- Planning still writes `delivery_attempts`.
- Result persistence still writes `delivery_attempts` final status.

With `MGP_RESULT`, result persistence is decoupled from dispatch QPS. If `result_persist_qps` is lower than `dispatch_qps`, `MGP_RESULT` pending will grow, but send workers should not block on PostgreSQL result writes.

---

## Recommended Milestones

1. Add broker abstraction and JetStream adapter without changing behavior.
2. Move `send_message` queue to JetStream first. This targets the current largest outbound P95 pain.
3. Add `MGP_RESULT` stream and durable result writer.
4. Move `route_plan` queue to JetStream.
5. Add JetStream monitoring and performance report.
6. Remove PostgreSQL hot-path `jobs` writes after JetStream metrics prove stable.

Do not implement lossy acknowledgement or memory-only result modes in this milestone. The target reliability model is durable JetStream queues for route planning, sending, and result persistence.
