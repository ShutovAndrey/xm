# xm

gRPC service on Go + PostgreSQL that publishes company events to a [go-kafka](https://gitlab.com/shutovAndrey/go-kafka) broker via a transactional outbox. Migrations are embedded and applied on startup.

## Run

```sh
cp .env.example .env   # adjust if needed
docker compose up --build
```

gRPC listens on `localhost:50051`, PostgreSQL on `localhost:${POSTGRES_PORT}`, the broker on `localhost:${BROKER_PORT}`. The `broker` image is built from the go-kafka git repository on first `up`.

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
| `JWT_SECRET`        | `dev-secret-change-me` | app, HS256 signing key |
| `JWT_TTL`           | `1h`    | app, token lifetime |
| `BROKER_PORT`       | `9090`  | host port of broker |
| `BROKER_TOPIC`      | `companies` | app, topic for company events |
| `BROKER_TIMEOUT`    | `5s`    | app, timeout per publish call |
| `OUTBOX_POLL_INTERVAL` | `1s` | app, how often the relay publishes a batch |
| `OUTBOX_BATCH_SIZE` | `100`   | app, max events per relay tick |
| `DATABASE_URL`      | —       | app, local run only |

## Test users

The `seed` service inserts them on `docker compose up` (idempotent, see `seeds/users.sql`). Dev only.

| Login   | Password   |
|---------|------------|
| `alice` | `alice123` |
| `bob`   | `bob123`   |
| `carol` | `carol123` |

Passwords are stored as bcrypt via pgcrypto (`crypt(password, salt)`).

## Events

Every `CreateCompany`, `UpdateCompany`, `DeleteCompany` publishes an event to the `go-kafka` broker (service `broker`, built from [go-kafka](https://gitlab.com/shutovAndrey/go-kafka)) through a transactional outbox:

1. The change and the event row are written to Postgres in one transaction (`outbox` table).
2. The relay goroutine in `app` polls `outbox` every `OUTBOX_POLL_INTERVAL`, publishes up to 100 rows in `id` order and deletes the published ones. On a failure it records `attempts` / `last_error` on the row and retries next tick; a broker outage only grows the backlog.

Delivery is at-least-once, consumers dedup by the `event-id` header. Message key is the company id, so events of one company stay ordered on one partition.

| Header       | Value |
|--------------|-------|
| `event-type` | `company.created`, `company.updated`, `company.deleted` |
| `event-id`   | UUID, unique per event |

Payload (JSON): full company state for `created` / `updated` (`employees_count` is the value after applying the delta), `{"id": "<uuid>"}` for `deleted`.

Consume with the [go-kafka-client](https://gitlab.com/shutovAndrey/go-kafka-client) `Reader` on `localhost:${BROKER_PORT}`, topic `companies`.

## Local run (without Docker for the app)

```sh
docker compose up -d db broker
export $(grep -v '^#' .env | xargs)
make run   # BROKER_HOST defaults to localhost, BROKER_PORT comes from .env
```

## Development

```sh
make generate   # buf generate -> gen/
make lint       # buf lint, go vet, golangci-lint
make build      # bin/app
```

## Call

Server reflection is disabled, so `grpcurl` needs the proto file. Run from repo root.

`CreateCompany`, `UpdateCompany`, `DeleteCompany` require `authorization: Bearer <token>` metadata. `GetCompany` and `Login` are public.

Login:

```sh
grpcurl -plaintext -import-path proto -proto auth/v1/auth.proto \
  -d '{"login": "alice", "password": "alice123"}' \
  localhost:50051 auth.v1.AuthService/Login
```

Create:

```sh
grpcurl -plaintext -import-path proto -proto company/v1/company.proto \
  -H 'authorization: Bearer <token>' \
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
  -H 'authorization: Bearer <token>' \
  -d '{"id": "<uuid>", "name": "acme2", "employees_delta": -3, "is_registered": false, "type": "COMPANY_TYPE_NON_PROFIT"}' \
  localhost:50051 company.v1.CompanyService/UpdateCompany
```

Delete:

```sh
grpcurl -plaintext -import-path proto -proto company/v1/company.proto \
  -H 'authorization: Bearer <token>' \
  -d '{"id": "<uuid>"}' \
  localhost:50051 company.v1.CompanyService/DeleteCompany
```

`grpc.health.v1.Health/Check` is also served (needs the health proto or reflection to call via `grpcurl`).

`type` values: `COMPANY_TYPE_CORPORATIONS`, `COMPANY_TYPE_NON_PROFIT`, `COMPANY_TYPE_COOPERATIVE`, `COMPANY_TYPE_SOLE_PROPRIETORSHIP`.
Errors: `NOT_FOUND` for unknown id, `ALREADY_EXISTS` for duplicate name, `INVALID_ARGUMENT` for missing type or malformed id, `UNAUTHENTICATED` for missing/invalid token or wrong credentials.
