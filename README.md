# urbanpetr-api

Go REST API running on AWS Lambda (ARM64) with PostgreSQL (RDS Aurora), deployed via GitHub Actions to `api.urbanpetr.com`.

## Stack

| Layer | Choice |
|---|---|
| Language | Go 1.25 |
| Router | [chi v5](https://github.com/go-chi/chi) |
| Lambda adapter | [aws-lambda-go-api-proxy](https://github.com/awslabs/aws-lambda-go-api-proxy) — HTTP API v2 |
| Database | PostgreSQL via [pgx v5](https://github.com/jackc/pgx) |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) — SQL files |
| Infrastructure | Terraform on AWS (Account A / prod, Account B / staging) |
| CI/CD | GitHub Actions + OIDC |

## Project structure

```
cmd/api/main.go          — Lambda entrypoint; mode-switched via LAMBDA_HANDLER_MODE
internal/config/         — Secrets Manager reads, DSN building
internal/handler/        — chi router, middleware, HTTP handlers
internal/migrate/        — DB provisioning (provision.go) + golang-migrate runner
internal/seed/           — Seed SQL runner (staging PR envs)
migrations/              — SQL migration files (bundled into Lambda zip)
terraform/modules/prod/  — Lambda, API Gateway, CloudFront, WAF, RDS access, IAM, secrets, DNS
terraform/envs/prod/     — Prod environment wiring
```

## Single binary, multiple modes

The same `bootstrap` binary is deployed to three Lambda functions. `LAMBDA_HANDLER_MODE` selects the behaviour:

| Value | Function | What it does |
|---|---|---|
| `api` (default) | `urbanpetr-api-prod` | Serves HTTP via chi + API Gateway HTTP API v2 |
| `migrate` | `urbanpetr-api-prod-migrations` | Provisions DB users + runs golang-migrate |
| `seed` | *(staging only)* | Loads seed SQL into the PR database |

## Local development

```bash
make build    # cross-compile for linux/arm64 → bootstrap + lambda.zip
make test     # go test ./... -v -count=1
make lint     # golangci-lint run ./...
make clean    # remove bootstrap and lambda.zip
```

Tests run against a local Go binary — no Docker required.

## Deploy

Push to `main` triggers the full pipeline:

1. **Build** — cross-compile `GOOS=linux GOARCH=arm64`, zip with `migrations/`
2. **Upload** — `s3://urbanpetr-artifacts/urbanpetr-api/<sha>.zip`
3. **Terraform apply** — idempotent infra update (prod, Account A)
4. **Update Lambda code** — `aws lambda update-function-code` × 2
5. **Migrate** — invoke migrations Lambda synchronously
6. **Smoke test** — `curl -f https://api.urbanpetr.com/health`

PRs run lint, tests, and `terraform plan` only — no AWS writes unless the `stage` label is applied.

## PR staging environments

Add the `stage` label to a PR to spin up an ephemeral environment in AWS Account B (`eu-central-1`):

1. **Build** — same binary as prod, zipped with `migrations/`
2. **Upload** — `s3://urbanpetr-artifacts-staging/urbanpetr-api/pr-<N>/<sha>.zip`
3. **Terraform apply** — isolated state at `urbanpetr_api/pr-<N>/terraform.tfstate`
4. **Update Lambda code** — `aws lambda update-function-code` × 3 (api, migrations, seed)
5. **Migrate + seed** — invoke migrations then seed Lambda synchronously
6. **DNS** — Route53 ALIAS record `api-stage<N>.urbanpetr.com` → API Gateway (Account A)
7. **PR comment** — posts the URL when ready

Remove the `stage` label or close the PR to tear everything down. The shared Lambda security group (`urbanpetr-api-staging-lambda`) and wildcard cert (`*.urbanpetr.com`) are created once via `terraform/envs/staging-base` and persist across all PR envs.

## Infrastructure

Prod infrastructure lives in AWS Account A (`eu-central-1`), staging in Account B. Key resources:

- **CloudFront** — `api.urbanpetr.com` terminates at a CloudFront distribution (`PriceClass_100`); origin is the API Gateway raw invoke URL
- **WAF** — shared `CLOUDFRONT`-scope WebACL (us-east-1) attached to the API CloudFront distribution: IP reputation list, common rule set, 1000 req/5 min per-IP rate limit
- **Origin secret** — CloudFront injects `X-Origin-Secret` on every origin request; the Lambda middleware rejects any request that arrives without the correct value, preventing API Gateway bypass
- **API Gateway** — HTTP API v2, no custom domain (CloudFront handles TLS termination for `api.urbanpetr.com`); throttle burst 200 / rate 100 rps at stage level
- **Lambda** — `provided.al2023` runtime, ARM64, VPC-attached
- **CORS** — handled by chi middleware; allows `https://*.urbanpetr.com` (covers prod, admin, and all stage environments)
- **RDS** — Aurora PostgreSQL (shared, managed by `urbanpetr-platform`)
- **Secrets Manager** — separate secrets for master / migrator / app / readonly DB credentials
- **S3** — `urbanpetr-artifacts` (prod) / `urbanpetr-artifacts-staging` (staging) for Lambda zip artifacts

Platform outputs (VPC, RDS endpoint, security groups) are consumed via `terraform_remote_state` from the `urbanpetr-platform` repo.

## Database conventions

| Convention | Rule | Example |
|---|---|---|
| Table names | Singular | `youtube_video`, not `youtube_videos` |
| Primary key | `id BIGSERIAL` | `id BIGSERIAL PRIMARY KEY` |
| External identifier | `uuid UUID` with unique index | `uuid UUID NOT NULL DEFAULT gen_random_uuid()` |
| Foreign keys | `id_<table>` or `id_<table>_<role>` | `id_youtube_video`, `id_user_created_by` |
| Units in column names | Suffix with unit | `duration_seconds`, `length_mm`, `weight_kg`, `price_cents` |
| API exposure | Expose `uuid`, never `id` | `id` is internal only; FK columns are also internal |
| Timestamps in API responses | Object with `timestamp` (Unix) and `formatted` (human) | `"watched_at": { "timestamp": 1623027945, "formatted": "6 Jun 2021, 13:05" }` |

Every table follows the same skeleton:

```sql
CREATE TABLE thing (
    id    BIGSERIAL PRIMARY KEY,
    uuid  UUID NOT NULL DEFAULT gen_random_uuid()
);
CREATE UNIQUE INDEX thing_uuid_idx ON thing (uuid);
```

## YouTube watch history

### Ingestion

Upload a Google Takeout zip via `POST /v1/history/youtube/ingest`. The handler:

1. Extracts the `.json` file from the zip
2. Parses entries and classifies each as `video` or `short` (see below)
3. Upserts into `youtube_video` and inserts new rows into `youtube_history`
4. Auto-enriches the latest 500 un-enriched videos (see below)

The response includes `total` (parsed entries), `created` (new history rows), and `enriched` (videos fetched from the YouTube API in this run).

### Short detection

Three rules are applied in order during parsing. Once a video is classified as `short` by any rule it stays that way.

**1. URL prefix** — `/shorts/<id>` URLs are definitively shorts.

**2. Hashtag in title** — if the title contains `#short` or `#shorts` (case-insensitive) the video is always a short.

**3. Timing heuristic** — after all entries are sorted by `watched_at` ascending, consecutive pairs are examined:
- If the gap since the previous watch is **< 80 seconds**, the video is a short candidate.
- But if the *next* video started **< 90 seconds** later, the candidate is dropped — this filters false positives where the user quickly abandoned a regular video and moved on.

The net effect: a timing-based short must have started rapidly *and* the user must have spent at least 90 seconds on it before switching away.

**Post-enrich correction** — when details are fetched from the YouTube API, any `short` with `duration_seconds > 80` is reverted to `video`. This catches videos that were misclassified by the timing heuristic (e.g. the user paused between two long videos).

### Auto-enrichment

After every ingest the API fetches YouTube metadata for the **500 most recently watched un-enriched videos** (`enriched_at IS NULL`), in batches of 50. The YouTube Data API v3 allows up to 50 IDs per `videos.list` call (1 quota unit per call regardless of batch size), so 500 videos costs 10 units.

Videos already enriched (`enriched_at IS NOT NULL`) are skipped. Individual videos can also be enriched on demand via `POST /v1/history/youtube/{uuid}/enrich`.

The auto-enrich limit (`autoEnrichLimit = 500`) and batch size (50, dictated by the YouTube API) will be tuned as needed.

## Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | — | Returns `{"status":"ok"}` |
| GET | `/v1/history/youtube` | — | List active (public) YouTube watch history |
| GET | `/v1/history/youtube/all` | admin | List all YouTube watch history entries |
| GET | `/v1/history/youtube/{uuid}` | admin | Get a single YouTube history entry |
| POST | `/v1/history/youtube/ingest` | admin | Ingest a Google Takeout zip |
| POST | `/v1/history/youtube/{uuid}/activate` | admin | Mark an entry as active (public) |
| POST | `/v1/history/youtube/{uuid}/deactivate` | admin | Mark an entry as inactive (hidden) |
| PATCH | `/v1/history/youtube/{uuid}` | admin | Update entry details |
| POST | `/v1/history/youtube/{uuid}/enrich` | admin | Fetch YouTube metadata for a single video |

Admin endpoints require a Cognito-issued JWT with the `urbanpetr_admin` scope (`Authorization: Bearer <token>`).
