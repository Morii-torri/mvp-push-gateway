# MVP Push Gateway Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a clean new `mvp-push-gateway` implementation as a lightweight but powerful message push gateway.

**Architecture:** Go single-binary backend with PostgreSQL-backed business data and job queue. React + Vite + TypeScript + Ant Design frontend, with React Flow for visual route editing and Monaco Editor for template editing.

**Tech Stack:** Go, PostgreSQL, pgx, sqlc, goose, OpenAPI, React, Vite, TypeScript, Ant Design, React Flow, TanStack Query, Monaco Editor.

---

## Phase 0: Design And Prototype Confirmation

### Task 0.1: Generate UI Prototype Images

**Files:**
- Read: `docs/ui-prototypes/prototype-brief.md`
- Read: `docs/ui-prototypes/list-field-status-spec.md`
- Output: `docs/ui-prototypes/generated/`

**Steps:**

1. Generate prototype images for:
   - 总览工作台
   - 来源接入管理
   - 上级平台配置
   - 路由编排画布模式
   - 路由传统表格模式
   - 模板中心
   - 组织人员
   - 消息日志详情
   - 队列监控和慢规则页面
2. Review with user.
3. Update `docs/ui-prototypes/prototype-brief.md` with accepted visual direction.

### Task 0.2: Finalize API And Schema

**Files:**
- Modify: `docs/data-model/schema-design.md`
- Modify: `docs/api/api-design.md`

**Steps:**

1. Check all user requirements against schema.
2. Confirm provider capabilities include recipient field location.
3. Confirm template parse API returns variable/value columns.
4. Confirm logs join inbound and outbound under one message record.
5. Confirm first version excludes scheduled sending.
6. Confirm queue monitoring, dead letters, platform rate limits and short claim transactions are represented in schema and API.
7. Confirm source auth defaults to `Authorization: Bearer` Token, production supports Token, HMAC is optional, `token_and_hmac` requires both checks, and CIDR IP allowlist is first-version scope.
8. Confirm every list page has Chinese field labels, Chinese status labels, query form, pagination, and create dialog/drawer.
9. Confirm `latest_payload_sample` is updated from the latest auth-passed and valid JSON inbound payload, without requiring route, template or recipient success.
10. Confirm synchronous inbound responses only cover receive-stage validation and queueing; route, template, recipient and send failures are asynchronous and traceable by `trace_id`.
11. Confirm first version has no RBAC, only one administrator account.
12. Confirm first version does not provide media upload API.
13. Confirm dedupe unique indexes are scoped by source for inbound and by channel for send.
14. Confirm each source can have only one enabled route flow, while v1/v2 route versions switch execution inside that flow.
15. Confirm route rules execute by drag-sorted order with `first_match_stop` semantics and cumulative hit count.

### Task 0.3: Finalize Downstream Integration Guide

**Files:**
- Modify: `docs/api/downstream-integration-guide.md`
- Read: `docs/api/api-design.md`

**Steps:**

1. Keep the guide clear enough to send directly to downstream teams.
2. Document `POST /api/v1/ingest/{source_code}` and `Content-Type: application/json`.
3. State that request Body only needs to be valid JSON and has no fixed business field requirement from the gateway.
4. Document Token, HMAC, `token_and_hmac`, no-auth and CIDR allowlist modes.
5. State that first sends may fail later route planning if route, template or recipient settings are missing, but auth-passed valid JSON payloads are still saved as latest samples.
6. Include the exact contact-facing copy: "首次发送如平台日志出现 `MGP-ROUTE-001`、`MGP-TPL-002`、`MGP-REC-001` 等配置类错误码，请联系平台管理员基于最近 payload 配置路由、模板和接收人。`202 Accepted` 只代表请求已被网关接收，不代表已经完成发送。"
7. Maintain a readable error-code table with cause and recommended action.

## Phase 1: Backend Foundation

### Task 1.1: Scaffold Go Backend

**Files:**
- Create: `backend/go.mod`
- Create: `backend/cmd/server/main.go`
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/http/router.go`

**Steps:**

1. Initialize Go module.
2. Add config loader with safe defaults.
3. Configure separate PostgreSQL pools for API, planning, sending and maintenance.
4. Add health endpoint.
5. Run `go test ./...`.

### Task 1.2: Add PostgreSQL Migrations

**Files:**
- Create: `backend/migrations/000001_init.sql`
- Create: `backend/sqlc.yaml`
- Create: `backend/internal/db/`

**Steps:**

1. Create base tables from `docs/data-model/schema-design.md`.
2. Add key indexes.
3. Generate sqlc models.
4. Run migration against local PostgreSQL.

### Task 1.3: First-Run Setup

**Files:**
- Create: `backend/internal/setup/`
- Modify: `backend/internal/http/router.go`

**Steps:**

1. Implement setup state detection.
2. Implement create-admin endpoint.
3. Hash password with Argon2id or bcrypt-sha256.
4. Ensure setup endpoint closes after admin exists.
5. Do not implement RBAC in first version; all management operations are administrator-only.

## Phase 2: Core Gateway

### Task 2.1: Sources And Authentication

**Files:**
- Create: `backend/internal/sources/`
- Create: `backend/internal/security/hmac.go`

**Steps:**

1. CRUD sources.
2. Default new sources to Token auth.
3. Implement Token auth from `Authorization: Bearer <source_token>` only.
4. Implement optional HMAC with generated source secret.
5. Implement `token_and_hmac` requiring both Token and HMAC checks.
6. Implement CIDR IP allowlist.
7. Add tests for accepted and rejected inbound requests.
8. Add tests that source latest payload samples only update after auth passes and JSON parsing succeeds.

### Task 2.2: Provider Capabilities And Channels

**Files:**
- Create: `backend/internal/providers/`

**Steps:**

1. Seed built-in provider capabilities.
2. Implement channel CRUD.
3. Implement custom token platform config.
4. Implement request builder for token and recipient locations.
5. Implement editable platform rate limit, concurrency, timeout, retry and dead-letter settings.

### Task 2.3: Inbound Message Records

**Files:**
- Create: `backend/internal/messages/`
- Create: `backend/internal/jobs/`

**Steps:**

1. Implement `POST /api/v1/ingest/{source_code}`.
2. Write `message_records`.
3. Apply inbound dedupe.
4. Enqueue `route_plan` job.
5. Update source latest payload sample immediately after auth passes and JSON parsing succeeds; do not wait for route planning, template rendering or recipient resolution.

### Task 2.4: PostgreSQL Job Queue

**Files:**
- Create: `backend/internal/jobs/queue.go`
- Create: `backend/internal/worker/`

**Steps:**

1. Implement job enqueue.
2. Implement claim with `FOR UPDATE SKIP LOCKED`.
3. Implement retry and dead-letter states.
4. Add worker health snapshot.
5. Ensure claim transactions only flip job state and commit before route evaluation or HTTP sending.
6. Implement `heartbeat_at` updates for processing jobs.
7. Implement maintenance recovery for stale `processing` jobs: requeue before `max_attempts`, dead-letter after attempts are exhausted.

## Phase 3: Routing, Templates, Recipients

### Task 3.1: Organization And User Identities

**Files:**
- Create: `backend/internal/recipients/`

**Steps:**

1. CRUD org units and users.
2. Implement move org.
3. Implement user identities by provider.
4. Implement recipient resolution.

### Task 3.2: Match Groups

**Files:**
- Create: `backend/internal/matchgroups/`

**Steps:**

1. CRUD groups and items.
2. Implement membership checks for exact, CIDR/IP, regex and text values.

### Task 3.3: Template Engine

**Files:**
- Create: `backend/internal/templates/`

**Steps:**

1. Implement payload parser returning variable/value columns.
2. Implement preview.
3. Implement schema validation.
4. Block publish when validation fails.
5. Ensure copied variables use Jinja-like text such as `{{ payload.title }}`, while internal paths remain `payload.title`.

### Task 3.4: Route Engine

**Files:**
- Create: `backend/internal/routes/`

**Steps:**

1. Store route flows and versions.
2. Enforce one enabled route flow per source; reject duplicates with “路由组已存在”.
3. Support v1/v2 version publishing and activating via `current_version_id`.
4. Compile canvas/table rules into executable model.
5. Cache compiled model by `source_id + route_version_id`.
6. Add coarse filtering before full condition evaluation without changing rule order.
7. Evaluate condition tree by `sort_order` and stop at the first matched enabled rule.
8. Preserve stable `rule_key` across reorder/edit/publish.
9. Increment cumulative route hit count up to 99999.
10. Record planning duration and slow-rule metrics.
11. Produce delivery attempts.

### Task 3.5: Delivery Worker

**Files:**
- Create: `backend/internal/delivery/`

**Steps:**

1. Apply send-level dedupe.
2. Apply per-channel concurrency and active rate limiting.
3. Resolve token.
4. Build request with token and recipient placement.
5. Apply channel timeout, retry and dead-letter policy.
6. Send and record delivery attempts.

## Phase 4: Frontend

### Task 4.1: Scaffold React App

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/app/App.tsx`

**Steps:**

1. Create Vite React TypeScript app.
2. Add Ant Design theme matching accepted prototype.
3. Add auth shell and layout.

### Task 4.2: Management Pages

**Files:**
- Create: `frontend/src/features/sources/`
- Create: `frontend/src/features/channels/`
- Create: `frontend/src/features/templates/`
- Create: `frontend/src/features/routes/`
- Create: `frontend/src/features/recipients/`
- Create: `frontend/src/features/messages/`
- Create: `frontend/src/features/monitoring/`

**Steps:**

1. Build list pages with query forms and pagination.
2. Build detail drawers.
3. Use create buttons that open dialogs or drawers for new records.
4. Use `docs/ui-prototypes/list-field-status-spec.md` for Chinese field labels and Chinese enum/status labels everywhere; no raw English enum values in tables.
5. Render `none` source auth as a yellow risk tag.
6. Add 5-second polling where needed.
7. Include queue monitoring page for backlog, P95, platform limits, dead letters and slow rules.

### Task 4.3: Route Canvas

**Files:**
- Create: `frontend/src/features/routes/canvas/`

**Steps:**

1. Build node palette.
2. Build React Flow canvas.
3. Build right-side property panel.
4. Add rule drag sorting, move up and move down.
5. Show current route version and cumulative hit count.
6. Add validate, simulate, publish and activate-version actions.

### Task 4.4: Template Editor

**Files:**
- Create: `frontend/src/features/templates/editor/`

**Steps:**

1. Add source and payload selector.
2. Add auto parsed field table with variable/value columns.
3. Add copy variable action that copies `{{ payload.path }}`.
4. Add Monaco editor, preview and validation panel.

## Phase 5: Operations And QA

### Task 5.1: Docker Compose

**Files:**
- Create: `docker-compose.yml`
- Create: `backend/Dockerfile`
- Create: `frontend/Dockerfile`

**Steps:**

1. Add PostgreSQL.
2. Add backend.
3. Add frontend or static serving path.
4. Verify first-run setup.
5. Verify `retention_cleanup` runs batch small-step deletion for data older than 30 days.

### Task 5.2: End-To-End Tests

**Files:**
- Create: `backend/tests/`
- Create: `frontend/src/**/*.test.tsx`

**Steps:**

1. Test source auth.
2. Test template validation failure.
3. Test route match and multi-provider delivery.
4. Test inbound and send dedupe.
5. Test message log detail.
6. Test short job claim transaction behavior.
7. Test per-channel rate limiting and dead-letter transition.
8. Test route compiled model cache and slow-rule metrics.
9. Test stale processing job recovery.
10. Test first-match-stop route execution and cumulative hit count cap at 99999.
11. Test duplicate route flow creation for the same enabled source is rejected.
12. Test retention cleanup deletes expired records in small batches.
13. Test media upload API is not exposed in first version.

## Execution Notes

- Keep the new project isolated under `mvp-push-gateway/`.
- Do not import old backend code directly unless a function is intentionally ported.
- Prefer writing tests before implementation for routing, templates, auth, dedupe and request building.
- If this directory is not a Git repository, initialize version control before implementation or record changes in `docs/plans/`.
