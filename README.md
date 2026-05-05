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
| Infrastructure | Terraform on AWS (Account A / prod) |
| CI/CD | GitHub Actions + OIDC |

## Project structure

```
cmd/api/main.go          — Lambda entrypoint; mode-switched via LAMBDA_HANDLER_MODE
internal/config/         — Secrets Manager reads, DSN building
internal/handler/        — chi router, middleware, HTTP handlers
internal/migrate/        — DB provisioning (provision.go) + golang-migrate runner
internal/seed/           — Seed SQL runner (staging PR envs)
migrations/              — SQL migration files (bundled into Lambda zip)
terraform/modules/prod/  — Lambda, API Gateway, RDS access, IAM, secrets, DNS
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

PRs run lint, tests, and `terraform plan` only — no AWS writes.

## Infrastructure

Prod infrastructure lives in AWS Account A (`eu-central-1`). Key resources:

- **Lambda** — `provided.al2023` runtime, ARM64, VPC-attached
- **API Gateway** — HTTP API v2 (`aws_apigatewayv2_api`)
- **RDS** — Aurora PostgreSQL (shared, managed by `urbanpetr-platform`)
- **Secrets Manager** — separate secrets for master / migrator / app / readonly DB credentials
- **S3** — `urbanpetr-artifacts` bucket for Lambda zip artifacts

Platform outputs (VPC, RDS endpoint, security groups) are consumed via `terraform_remote_state` from the `urbanpetr-platform` repo.

## Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Returns `{"status":"ok"}` |
