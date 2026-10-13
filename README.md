# xm

gRPC service on Go + PostgreSQL. Migrations are embedded and applied on startup.

## Run

```sh
cp .env.example .env   # adjust if needed
docker compose up --build
```

gRPC listens on `localhost:50051`, PostgreSQL on `localhost:${POSTGRES_PORT}`.

## Environment

Variables are read from `.env` (see `.env.example`). Shell variables override it:

```sh
POSTGRES_PORT=55432 docker compose up
```

Without `.env` the defaults from `docker-compose.yml` are used (`xm`/`xm`, port `5432`).

| Variable            | Default | Used by      |
|---------------------|---------|--------------|
| `POSTGRES_USER`     | `xm`    | db, app      |
| `POSTGRES_PASSWORD` | `xm`    | db, app      |
| `POSTGRES_DB`       | `xm`    | db, app      |
| `POSTGRES_PORT`     | `5432`  | host port of db |
| `GRPC_PORT`         | `50051` | host port of app |
| `DATABASE_URL`      | —       | app, local run only |

## Local run (without Docker for the app)

```sh
docker compose up -d db
export $(grep -v '^#' .env | xargs)
make run
```

## Development

```sh
make generate   # buf generate -> gen/
make lint       # buf lint, go vet, golangci-lint
make build      # bin/app
```

## Call

Server reflection is disabled, so `grpcurl` needs the proto file. Run from repo root.

Create:

```sh
grpcurl -plaintext -import-path proto -proto company/v1/company.proto \
  -d '{"name": "acme", "description": "test", "employees_count": 10, "is_registered": true, "type": "COMPANY_TYPE_COOPERATIVE"}' \
  localhost:50051 company.v1.CompanyService/CreateCompany
```

Get:

```sh
grpcurl -plaintext -import-path proto -proto company/v1/company.proto \
  -d '{"id": "<uuid>"}' \
  localhost:50051 company.v1.CompanyService/GetCompany
```

Update — full replacement, `employees_delta` is added to the current count, omitted `description` clears it:

```sh
grpcurl -plaintext -import-path proto -proto company/v1/company.proto \
  -d '{"id": "<uuid>", "name": "acme2", "employees_delta": -3, "is_registered": false, "type": "COMPANY_TYPE_NON_PROFIT"}' \
  localhost:50051 company.v1.CompanyService/UpdateCompany
```

Delete:

```sh
grpcurl -plaintext -import-path proto -proto company/v1/company.proto \
  -d '{"id": "<uuid>"}' \
  localhost:50051 company.v1.CompanyService/DeleteCompany
```

`grpc.health.v1.Health/Check` is also served (needs the health proto or reflection to call via `grpcurl`).

`type` values: `COMPANY_TYPE_CORPORATIONS`, `COMPANY_TYPE_NON_PROFIT`, `COMPANY_TYPE_COOPERATIVE`, `COMPANY_TYPE_SOLE_PROPRIETORSHIP`.
Errors: `NOT_FOUND` for unknown id, `ALREADY_EXISTS` for duplicate name, `INVALID_ARGUMENT` for missing type.
