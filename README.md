# MVP Push Gateway

Step 11 delivery packaging for the new `mvp-push-gateway/` implementation. This repository currently provides:

- PostgreSQL-backed Go API server
- React + Vite + Ant Design frontend build output
- migrations, integration tests, queue/monitoring endpoints, and local startup scripts

This README only documents the current repository state. It does not assume hidden defaults, built-in admin passwords, or extra worker processes that are not exposed by this tree.

## Quick Start With Docker Compose

1. Copy environment variables:

```bash
cp .env.example .env
```

2. Edit `.env` and set at least:

- `MGP_POSTGRES_PASSWORD`
- optional host ports if `5432`, `18080`, or `5173` are already occupied

3. Start the stack:

```bash
docker compose up --build
```

Services:

- frontend: `http://127.0.0.1:5173`
- backend health: `http://127.0.0.1:18080/api/v1/health`
- PostgreSQL: `127.0.0.1:5432`

Compose behavior:

- PostgreSQL creates the main dev database and a separate test database on first boot.
- backend waits for PostgreSQL, applies `backend/migrations/*.sql`, then starts `./cmd/server`.
- frontend is built once and served by Nginx with `/api/v1/*` proxied to the backend container.

Stop and clean up:

```bash
docker compose down
docker compose down -v
```

Use `-v` only when you want to remove the PostgreSQL data volume.

## Local Development

### Dependencies

- Go `1.22+`
- Node.js `20+` and npm
- PostgreSQL `16+`
- `psql`
- Docker Desktop or Docker Engine if you want the Compose path

### PostgreSQL Initialization

If you are not using Compose, create a dedicated role and two databases first:

```sql
CREATE ROLE mvp_push_gateway LOGIN PASSWORD 'change-me-dev-password';
CREATE DATABASE mvp_push_gateway_dev OWNER mvp_push_gateway;
CREATE DATABASE mvp_push_gateway_test OWNER mvp_push_gateway;
```

Then copy `.env.example` to `.env` and make sure these values point to that role and those databases:

- `MGP_POSTGRES_DSN`
- `MGP_TEST_DATABASE_URL`

### Apply Migrations

```bash
./scripts/apply-migrations.sh
```

### Start Backend

```bash
./scripts/dev-backend.sh
```

Default backend address:

```text
http://127.0.0.1:18080/api/v1
```

### Start Frontend

```bash
./scripts/dev-frontend.sh
```

Default frontend address:

```text
http://127.0.0.1:5173
```

The current frontend repository state is still a console shell/demo-oriented UI. First-run setup, admin creation, and API verification are most reliable through the backend HTTP API below.

## First-Run Admin Initialization

Check setup state:

```bash
curl http://127.0.0.1:18080/api/v1/setup/status
```

Create the first admin:

```bash
curl -X POST http://127.0.0.1:18080/api/v1/setup/admin \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "admin",
    "password": "ChangeMe-Init-123!",
    "display_name": "System Admin"
  }'
```

Login and save the bearer token:

```bash
curl -X POST http://127.0.0.1:18080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "admin",
    "password": "ChangeMe-Init-123!"
  }'
```

Export the returned `token` for later calls:

```bash
export ADMIN_TOKEN='replace-with-login-token'
```

## Fresh Environment Verification Path

This path verifies that a clean environment can boot, initialize, ingest a payload, configure core objects, and read monitoring data.

### 1. Create a Source

```bash
curl -X POST http://127.0.0.1:18080/api/v1/sources \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "code": "orders",
    "name": "订单中心",
    "auth_mode": "token",
    "auth_token": "src-orders-dev-token",
    "enabled": true
  }'
```

Save both:

- source `id`
- source `auth_token`

### 2. Send a Sample Payload

```bash
curl -X POST http://127.0.0.1:18080/api/v1/ingest/orders \
  -H 'Authorization: Bearer src-orders-dev-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "订单已支付",
    "content": "订单 2026-0001 已完成支付",
    "severity": "info"
  }'
```

Expected result:

- HTTP `202`
- response contains `trace_id`
- source detail later shows `latest_payload_sample`

### 3. Create and Publish a Template

Create the template shell:

```bash
curl -X POST http://127.0.0.1:18080/api/v1/templates \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "订单通知模板",
    "description": "Webhook 验证模板",
    "source_id": "replace-with-source-id",
    "enabled": true
  }'
```

Publish a version:

```bash
curl -X POST http://127.0.0.1:18080/api/v1/templates/replace-with-template-id/publish \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "message_type": "json",
    "target_provider_type": "webhook",
    "template_body": "{{ payload.title }}",
    "message_body_schema": {"type": "object"},
    "sample_payload": {
      "title": "订单已支付",
      "content": "订单 2026-0001 已完成支付",
      "severity": "info"
    }
  }'
```

Save the returned template version `id`.

### 4. Create a Webhook Channel

```bash
curl -X POST http://127.0.0.1:18080/api/v1/channels \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "provider_type": "webhook",
    "name": "HTTPBin Webhook",
    "enabled": true,
    "send_config": {
      "method": "POST",
      "url": "https://httpbin.org/post",
      "headers": {
        "Content-Type": "application/json"
      },
      "recipient": {
        "location": "none"
      }
    }
  }'
```

### 5. Create, Publish, and Activate a Route Flow

Create the flow:

```bash
curl -X POST http://127.0.0.1:18080/api/v1/route-flows \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "source_id": "replace-with-source-id",
    "name": "订单路由",
    "enabled": true,
    "mode": "table"
  }'
```

Save one rule:

```bash
curl -X PUT http://127.0.0.1:18080/api/v1/route-flows/replace-with-flow-id/rules \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "rules": [
      {
        "sort_order": 10,
        "name": "默认订单通知",
        "enabled": true,
        "condition_tree": {
          "operator": "always"
        },
        "action": {
          "template_version_id": "replace-with-template-version-id",
          "channel_ids": ["replace-with-channel-id"],
          "recipient_strategy": {},
          "send_dedupe_config": {},
          "failure_policy": {}
        }
      }
    ]
}'
```

If `rule_key` is omitted, the backend generates a stable UUID rule key in the save response.

Publish the draft:

```bash
curl -X POST http://127.0.0.1:18080/api/v1/route-flows/replace-with-flow-id/publish \
  -H "Authorization: Bearer ${ADMIN_TOKEN}"
```

Activate the published version:

```bash
curl -X POST http://127.0.0.1:18080/api/v1/route-flows/replace-with-flow-id/versions/replace-with-version-id/activate \
  -H "Authorization: Bearer ${ADMIN_TOKEN}"
```

Optional rule simulation:

```bash
curl -X POST http://127.0.0.1:18080/api/v1/route-flows/replace-with-flow-id/simulate \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "payload": {
      "title": "订单已支付",
      "content": "订单 2026-0001 已完成支付",
      "severity": "info"
    }
  }'
```

### 6. Check Monitoring

Queue metrics:

```bash
curl http://127.0.0.1:18080/api/v1/monitor/queues \
  -H "Authorization: Bearer ${ADMIN_TOKEN}"
```

Overview metrics:

```bash
curl http://127.0.0.1:18080/api/v1/stats/overview \
  -H "Authorization: Bearer ${ADMIN_TOKEN}"
```

### 7. Validate Outbound Request Shape

The current repository exposes request-building and delivery internals, but the default runtime entrypoint in this tree starts only the HTTP API service. A practical smoke test is to validate the channel request shape directly:

```bash
curl -X POST http://127.0.0.1:18080/api/v1/channels/replace-with-channel-id/build-request \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "body": {
      "title": "订单已支付",
      "content": "订单 2026-0001 已完成支付"
    }
  }'
```

This confirms the provider/channel configuration can build a concrete outbound HTTP request from your stored channel config.

## Test Commands

Backend:

```bash
./scripts/test-backend.sh
```

Frontend:

```bash
./scripts/test-frontend.sh
```

Migration constraints:

```bash
./scripts/test-migrations.sh
```

Shell script syntax check:

```bash
./scripts/check-shell-scripts.sh
```

Compose file expansion:

```bash
docker compose config
```

`./scripts/test-migrations.sh` and backend integration tests require a writable PostgreSQL database referenced by `MGP_TEST_DATABASE_URL`.

## Common Issues

### `database connection failed`

Usually one of these is wrong:

- PostgreSQL is not running
- `MGP_POSTGRES_DSN` points to the wrong host, port, user, or password
- migrations were not applied and the backend cannot seed/query expected tables

Check:

```bash
psql "$MGP_POSTGRES_DSN" -c 'SELECT 1'
./scripts/apply-migrations.sh
```

### `setup/admin` returns conflict

`POST /api/v1/setup/admin` is one-time initialization. Check the current state:

```bash
curl http://127.0.0.1:18080/api/v1/setup/status
```

If `setup_open` is `false`, log in with the existing admin instead of trying to initialize again.

### Frontend opens but does not behave like a full production console

That is expected for the current repository state. The frontend build is useful for packaging smoke tests and API-adjacent console work, but setup/auth/config verification should still be driven by the backend API examples above.

### Queue metrics show pending jobs after ingest

That is also expected with the current public entrypoints in this repository. Ingest enqueues async jobs, but the shipped `backend/cmd/server` binary only starts the HTTP API. Queue growth, route simulation, template publish, and channel request building can still be verified end-to-end from a packaging perspective.
