# Backend

Go HTTP skeleton for MVP Push Gateway.

```bash
../scripts/dev-backend.sh
../scripts/test-backend.sh
```

The initial health endpoint is `GET /api/v1/health`. PostgreSQL configuration is represented in code but no database connection is opened in Step 1.

## Database

Step 2 adds the PostgreSQL baseline schema in `migrations/000001_init.sql`.

```bash
../scripts/test-migrations.sh
```

`test-migrations.sh` requires `MGP_TEST_DATABASE_URL`. The test creates a temporary schema, applies only the goose `Up` section, verifies the key unique constraints, and drops the schema after the run.
