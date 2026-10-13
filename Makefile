.PHONY: generate lint test build run up down

generate:
	buf generate

lint:
	buf lint
	go vet ./...
	golangci-lint run -v --fix

# Integration tests need TEST_DATABASE_URL (e.g. the compose db:
# TEST_DATABASE_URL=postgres://xm:xm@localhost:55432/xm?sslmode=disable) and are skipped without it.
test:
	go test -race ./...

build:
	go build -o bin/app ./cmd/app

run:
	go run ./cmd/app

up:
	docker compose up --build

down:
	docker compose down
