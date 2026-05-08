# AI Execution Roadmap Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Guide the next AI sessions to turn the approved MVP Push Gateway design into a clean, testable implementation.

**Architecture:** Build the new project under `mvp-push-gateway/` as a Go backend, PostgreSQL data store and queue, and React + Vite + TypeScript + Ant Design frontend. Keep the work incremental: prototype first, backend foundation second, core gateway third, frontend last, with verification after every phase.

**Tech Stack:** Go, PostgreSQL, pgx, sqlc, goose, OpenAPI, React, Vite, TypeScript, Ant Design, React Flow, TanStack Query, Monaco Editor.

---

## How To Use This Plan

每次给 AI 派活时，只发一个阶段或一个任务，不要一次让它全做。每个任务结束时要求 AI 提供：

- 修改了哪些文件。
- 实现了什么行为。
- 运行了什么验证命令。
- 还有什么风险或未完成点。

## Step 0: Lock Scope And Generate UI Prototypes

**Ask AI to do:**

1. Read:
   - `docs/README.md`
   - `docs/architecture/system-design.md`
   - `docs/ui-prototypes/prototype-brief.md`
   - `docs/ui-prototypes/list-field-status-spec.md`
2. Generate UI prototype images for:
   - 总览工作台
   - 来源接入
   - 上级平台
   - 路由画布
   - 路由传统表格
   - 模板中心
   - 组织人员
   - 消息日志详情
   - 队列监控
3. Save accepted prototype notes back to `docs/ui-prototypes/prototype-brief.md`.

**Expected output:**

- `docs/ui-prototypes/generated/` prototype images.
- Updated UI prototype notes.
- User-approved visual direction.

**Acceptance:**

- UI matches light B-end SaaS style.
- Route page includes canvas and table mode.
- Template page shows `{{ payload.title }}` copy format.
- Queue monitoring has its own page.

## Step 1: Create Project Skeleton

**Ask AI to do:**

1. Create backend skeleton:
   - `backend/go.mod`
   - `backend/cmd/server/main.go`
   - `backend/internal/config/`
   - `backend/internal/http/`
2. Create frontend skeleton:
   - `frontend/package.json`
   - `frontend/src/main.tsx`
   - `frontend/src/app/`
3. Add root developer scripts:
   - `scripts/dev-backend.sh`
   - `scripts/dev-frontend.sh`
   - `scripts/test-backend.sh`
   - `scripts/test-frontend.sh`

**Expected output:**

- Backend can start with a health endpoint.
- Frontend can start with an empty admin shell.
- Scripts are documented.

**Acceptance:**

- `go test ./...` passes.
- `npm run build` passes.
- No business logic yet.

## Step 2: Database Migrations And Generated Access Layer

**Ask AI to do:**

1. Convert `docs/data-model/schema-design.md` into migrations.
2. Add PostgreSQL tables for sources, channels, templates, routes, messages, jobs, dedupe, metrics, setup and audit.
3. Add the key constraints:
   - one enabled route flow per source.
   - inbound dedupe unique by `scope + source_id + dedupe_key`.
   - send dedupe unique by `scope + channel_id + dedupe_key`.
4. Add sqlc queries or repository layer.

**Expected output:**

- `backend/migrations/000001_init.sql`
- `backend/sqlc.yaml`
- generated DB models or repository code.

**Acceptance:**

- Migration runs on clean PostgreSQL.
- Duplicate enabled route flow for one source is impossible.
- `route_rule_counters`, `heartbeat_at`, and retention-related indexes exist.

## Step 3: First-Run Admin And Authentication

**Ask AI to do:**

1. Implement first-run setup:
   - `GET /api/v1/setup/status`
   - `POST /api/v1/setup/admin`
2. Implement admin login/logout/me/change-password.
3. Keep first version administrator-only; do not implement RBAC.

**Expected output:**

- One admin account can be created on empty DB.
- Setup endpoint closes after admin exists.
- Admin can log in and call protected APIs.

**Acceptance:**

- No hardcoded password.
- Password hash uses Argon2id or bcrypt-sha256.
- Tests cover setup open/closed state.

## Step 4: Source Ingestion And Auth

**Ask AI to do:**

1. Implement source CRUD.
2. Implement `POST /api/v1/ingest/{source_code}`.
3. Implement Token, HMAC, `token_and_hmac`, `none`, and CIDR allowlist.
4. Implement sync-vs-async boundary:
   - sync errors only for source/auth/IP/JSON/size/rate-limit/dedupe/queue.
   - route/template/send errors recorded asynchronously by `trace_id`.
5. Update `latest_payload_sample` after auth passes and JSON is valid.

**Expected output:**

- Source management API.
- Ingestion API returning `202 Accepted`.
- Latest payload saved even if no route exists.

**Acceptance:**

- Only `Authorization: Bearer <source_token>` is accepted for Token.
- No `X-MGP-Token`.
- No media upload API.
- Tests cover latest payload update and rejection cases.

## Step 5: PostgreSQL Job Queue And Recovery

**Ask AI to do:**

1. Implement job enqueue and claim using `FOR UPDATE SKIP LOCKED`.
2. Keep claim transaction short.
3. Add worker heartbeat updates.
4. Add maintenance recovery for stale `processing` jobs.
5. Add retry and dead-letter behavior.

**Expected output:**

- `route_plan`, `send_message`, `stats_aggregate`, `retention_cleanup`, `dead_letter_replay` job support.
- Stuck processing jobs are requeued or dead-lettered.

**Acceptance:**

- Tests prove HTTP sending/template rendering never happens inside the claim transaction.
- Tests prove stale processing jobs recover correctly.

## Step 6: Providers, Templates, Recipients

**Ask AI to do:**

1. Seed provider capabilities for WeCom, Feishu, DingTalk, email, SMS, gov cloud, self, webhook, custom token.
2. Implement platform instance CRUD and request builder.
3. Implement organization, users, identities and recipient groups.
4. Implement template parse, preview, validate and publish.
5. Ensure copied variables are `{{ payload.path }}`, while internal paths stay `payload.path`.

**Expected output:**

- Provider/channel management APIs.
- Recipient directory APIs.
- Template editor APIs.

**Acceptance:**

- Template validation blocks invalid templates.
- Recipient field placement supports query/header/body/path/none.
- Admin can view Token/secret in clear text in first version.

## Step 7: Route Engine

**Ask AI to do:**

1. Implement route flow CRUD.
2. Enforce one enabled route flow per source; duplicate save returns `MGP-ROUTE-003`.
3. Implement route versions and activate current version.
4. Implement sorted rule execution with `first_match_stop`.
5. Implement rule hit counters capped at 99999.
6. Implement simulation API with final matched rule and stop reason.

**Expected output:**

- Route APIs and compiled execution model.
- Canvas/table share the same backend model.
- Hit count visible in rule list.

**Acceptance:**

- v1/v2 can switch execution via `current_version_id`.
- Reordering rules changes execution order.
- First matched rule stops traversal.
- Hit counter does not reset after reorder/edit/publish.

**Controller review note:**

- Step 7 currently keeps coarse filtering as a placeholder (`coarse_skipped=false`). Real coarse filtering, slow-rule metrics, and route planning cache behavior must be completed when the `route_plan` worker and monitoring steps are implemented.

## Step 8: Delivery Worker

**Ask AI to do:**

1. Implement send-level dedupe.
2. Apply per-channel rate limit and concurrency limit.
3. Resolve token if needed.
4. Build request with token and recipient placement.
5. Send request with timeout, retry and dead-letter policy.
6. Record request and response snapshots in clear text.

**Expected output:**

- Delivery worker.
- Delivery attempts and send logs.
- Platform-level failure isolation.

**Acceptance:**

- Slow platform only backs up its own queue.
- Send dedupe is scoped by channel.
- Retry exhaustion creates dead-letter job.

## Step 9: Frontend Management Console

**Ask AI to do:**

1. Build Ant Design layout and navigation.
2. Build pages:
   - 总览
   - 来源接入
   - 上级平台
   - 路由编排
   - 模板中心
   - 组织人员
   - 匹配组
   - 消息日志
   - 队列监控
   - 操作审计
   - 系统设置
3. Use unified list pattern: query bar, paginated table, create modal/drawer.
4. Use Chinese field/status mappings.
5. Use 5-second polling and manual refresh; no SSE.

**Expected output:**

- Working admin UI.
- Modern light B-end design.
- Route canvas and traditional table both usable.

**Acceptance:**

- No raw English enum displayed.
- `none` auth is a yellow risk tag.
- Rule hit count is visible.
- Template copy button copies `{{ payload.title }}` style text.

## Step 10: Monitoring, Retention And Dashboard

**Ask AI to do:**

1. Implement queue monitoring APIs.
2. Implement overview statistics.
3. Implement `retention_cleanup` small-batch deletion.
4. Show cleanup status in queue monitoring.

**Expected output:**

- Queue monitoring page.
- 24h dashboard.
- Retention cleanup worker.

**Acceptance:**

- Shows route_plan pending, send_message pending, oldest job wait, avg/P95, failures, rate limits and dead letters.
- Cleanup deletes old data in small batches, not one large transaction.

## Step 11: Integration Tests And Packaging

**Ask AI to do:**

1. Add backend integration tests for auth, route, template, queue, delivery and retention.
2. Add frontend tests for critical pages.
3. Add Docker Compose with PostgreSQL.
4. Add README startup instructions.
5. Run full build and test.

**Expected output:**

- Tested backend and frontend.
- Docker Compose local deployment.
- Clear runbook.

**Acceptance:**

- Backend tests pass.
- Frontend build passes.
- Fresh environment can start, create admin, create source, receive sample payload, configure route, and send test message.

## Recommended AI Prompt Order

1. “请按 `docs/plans/2026-05-07-ai-execution-roadmap.md` 执行 Step 0，只做原型图和原型说明更新，不写业务代码。”
2. “请执行 Step 1，只创建项目骨架和健康检查，跑通最小测试。”
3. “请执行 Step 2，只做数据库迁移和访问层，确保约束正确。”
4. “请执行 Step 3 和 Step 4，完成管理员初始化、来源管理和入站接口。”
5. “请执行 Step 5，完成 PostgreSQL job queue 和崩溃恢复。”
6. “请执行 Step 6 和 Step 7，完成模板、接收人、平台能力和路由引擎。”
7. “请执行 Step 8，完成发送 worker。”
8. “请执行 Step 9，完成前端管理台。”
9. “请执行 Step 10 和 Step 11，完成监控、清理、测试和部署。”
