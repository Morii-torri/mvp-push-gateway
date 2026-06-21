# MVP Push Gateway

MVP Push Gateway is a low-latency message ingress, routing, templating, and delivery gateway.

It gives downstream systems one stable HTTP ingress API, then routes each payload through configurable rules, renders channel-specific templates, resolves recipients, and dispatches the message to upstream push channels such as enterprise IM, email, SMS, webhooks, and open-source push services.

The project is built for operational use: route visualization, message logs, delivery timelines, dead-letter handling, queue monitoring, provider capability metadata, and a full web console are included.

> Status: active development. The current baseline is suitable for local, lab, and controlled private deployments. Review the security notes before exposing it directly to the public internet.

## Highlights

- **Unified ingress API** for downstream systems.
- **Token / HMAC / token+HMAC source authentication** with nonce replay protection.
- **Tenant-ready architecture plan** for future SaaS and private-enterprise modes.
- **Visual route strategy** with source-bound route groups and compiled runtime route cache.
- **Provider-aware templates** using Jinja-like syntax.
- **Recipient model** with organization tree, users, groups, and provider identities.
- **NATS JetStream hot path** for route planning, sending, and result persistence queues.
- **PostgreSQL source of truth** for configuration, logs, audit, metrics, and searchable state.
- **Real-time console notifications** via SSE.
- **Dead-letter queue** with replay / handled / delete operations.
- **Performance test console** for ingress, route, queue, worker, and delivery measurements.
- **Secret field encryption support** for source tokens, HMAC secrets, provider credentials, token cache, and sensitive send configuration.

## Architecture

```text
Downstream system
  -> Ingest API
  -> source authentication / dedupe / quiet hours
  -> JetStream route-plan event
  -> route worker
  -> template render + recipient resolution
  -> JetStream send event
  -> delivery worker
  -> upstream provider
  -> JetStream result event
  -> result writer
  -> PostgreSQL logs / metrics / audit
```

Main components:

- `backend/`: Go API server, workers, provider adapters, migrations, and tests.
- `frontend/`: React + Vite + Ant Design console.
- `docker-compose.yml`: PostgreSQL, NATS JetStream, backend, frontend, and migration service.
- `backend/migrations/`: PostgreSQL schema migrations.
- `docs/`: architecture, API, data model, operations, and implementation notes.

## Supported Channel Families

The provider registry currently includes these channel families:

- Enterprise IM: WeCom, DingTalk, Feishu
- Email: SMTP
- SMS: Aliyun, Tencent Cloud, Baidu Cloud
- Webhook and self-hosted HTTP callbacks
- Open-source / personal push: PushPlus, WxPusher, ServerChan, Bark, PushMe, ntfy, Gotify

Some providers are configuration-dependent and may require real accounts, tokens, or network access before live delivery can be verified.

## Quick Start With Docker Compose

Requirements:

- Docker Engine or Docker Desktop
- Git

Clone the repository:

```bash
git clone https://github.com/<owner>/mvp-push-gateway.git
cd mvp-push-gateway
```

Create local environment file:

```bash
cp .env.example .env
```

Edit `.env` and set at least:

```bash
MGP_POSTGRES_PASSWORD=<use-a-strong-password>
MGP_SECRET_ENCRYPTION_KEY=<base64-32-byte-key>
```

Generate a local encryption key:

```bash
openssl rand -base64 32
```

Start the recommended split deployment:

```bash
docker compose up --build
```

Default addresses:

- Console: `http://127.0.0.1:5173`
- Backend API: `http://127.0.0.1:18080/api/v1`
- PostgreSQL: `127.0.0.1:5432`
- NATS monitor: `http://127.0.0.1:8222`

Stop services:

```bash
docker compose down
```

Remove local PostgreSQL and NATS data as well:

```bash
docker compose down -v
```

Use `-v` only when you intentionally want to remove local data.

## First-Run Setup

After the backend is running, check setup status:

```bash
curl http://127.0.0.1:18080/api/v1/setup/status
```

Create the first administrator:

```bash
curl -X POST http://127.0.0.1:18080/api/v1/setup/admin \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "admin",
    "password": "ChangeMe-Init-123!",
    "confirm_password": "ChangeMe-Init-123!",
    "display_name": "System Admin"
  }'
```

Then open the console and sign in:

```text
http://127.0.0.1:5173
```

There is no hard-coded default admin password. The setup endpoint can be used only once.

## Basic Usage

The normal setup flow is:

1. **Create a source** in Source Access.
2. **Create a push channel** in Push Channels and test the configuration.
3. **Create a message template** for the source payload and provider message type.
4. **Create recipients** in Organization.
5. **Create and publish a route strategy** that matches payload conditions and sends to the selected channel.
6. **Send a downstream payload** to the source ingress API.
7. **Inspect logs and monitoring** in Log Monitor.

Example downstream ingest call:

```bash
curl -X POST 'http://127.0.0.1:18080/api/v1/ingest/orders' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <source-token>' \
  -d '{
    "title": "Payment latency alert",
    "level": "critical",
    "content": "p99 latency exceeded threshold",
    "biz_id": "order-10001",
    "route_key": "payment",
    "timestamp": "2026-06-21 10:00:00"
  }'
```

Successful ingest returns a trace id:

```json
{
  "trace_id": "9f5d4a78-7f7b-41ad-a6f1-3a2f3e8e0d3b",
  "status": "accepted",
  "message": "accepted"
}
```

Use this trace id in Message Logs to inspect route planning, delivery attempts, upstream response snapshots, and dead-letter state.

## Local Development

Requirements:

- Go `1.22+`
- Node.js `20+`
- npm
- PostgreSQL `16+`
- NATS `2.10+` with JetStream enabled

Copy environment:

```bash
cp .env.example .env
```

Apply migrations:

```bash
./scripts/apply-migrations.sh
```

Start backend:

```bash
./scripts/dev-backend.sh
```

Start frontend:

```bash
./scripts/dev-frontend.sh
```

Run common checks:

```bash
cd backend
go test ./internal/db ./internal/http ./internal/messagelog ./internal/audit
```

```bash
cd frontend
npm test
npm run build
```

## Configuration

Important environment variables:

| Variable | Purpose |
| --- | --- |
| `MGP_PORT` | Backend port, default `18080` locally |
| `MGP_POSTGRES_DSN` | PostgreSQL connection string |
| `MGP_NATS_URL` | NATS JetStream URL |
| `MGP_QUEUE_BACKEND` | Queue backend, default `jetstream` |
| `MGP_SECRET_ENCRYPTION_KEY` | Base64 secret field encryption key |
| `MGP_TRUSTED_PROXIES` | Trusted reverse proxy CIDRs / IPs for real client IP parsing |
| `MGP_PPROF_PORT` | Optional local pprof port, disabled when empty |

Never commit `.env` or real deployment secrets. Use `.env.example` only for safe placeholders.

## Security Notes

Before deploying to an internet-facing server:

- Set a strong `MGP_POSTGRES_PASSWORD`.
- Set `MGP_SECRET_ENCRYPTION_KEY` and run secret backfill for existing plaintext data if needed.
- Put the console behind HTTPS.
- Restrict admin access with firewall, VPN, or a trusted reverse proxy.
- Configure `MGP_TRUSTED_PROXIES` if the service runs behind a load balancer.
- Rotate source tokens, HMAC secrets, provider credentials, and SMTP passwords before public exposure.
- Review message logs before sharing data because payload and request/response snapshots are intentionally not encrypted in the current baseline.

Current baseline:

- Source tokens, HMAC secrets, provider credentials, token cache, and sensitive send config can be encrypted.
- Payload logs and request/response snapshots remain visible to administrators for troubleshooting.
- The repository ignores `.env`, local data directories, logs, private keys, and the local multi-tenant security design note.

## Documentation

- Architecture: `docs/architecture/system-design.md`
- Data model: `docs/data-model/schema-design.md`
- API design: `docs/api/api-design.md`
- Downstream integration guide: `docs/api/downstream-integration-guide.md`
- Operations guide: `docs/operations/operator-guide.md`
- End-to-end smoke test: `docs/operations/end-to-end-smoke.md`
- JetStream architecture: `docs/plans/2026-06-07-nats-jetstream-queue-architecture-plan.md`

## Project Status

This project is evolving quickly. The current codebase focuses on a powerful single-admin operations console and high-throughput gateway runtime. Multi-user SaaS mode, public registration, tenant isolation, and broader field encryption are designed but not yet fully merged into the product baseline.

## License

No license has been declared yet. Add a license before publishing if you want others to use, modify, or redistribute the project.
