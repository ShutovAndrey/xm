.PHONY: generate lint build run up down

generate:
	buf generate

lint:
	buf lint
	go vet ./...

build:
	go build -o bin/app ./cmd/app

run:
	go run ./cmd/app

up:
	docker compose up --build

down:
	docker compose down
