.PHONY: up down run sqlc migrate lint test test/unit arch vuln

up:
	docker compose up -d --wait

down:
	docker compose down -v

run:
	go run ./cmd/api

sqlc:
	sqlc generate

migrate:
	psql "$(POSTGRES_DSN)" -f sql/migrations/001_init.sql

lint:
	golangci-lint run ./...

test:
	go test -race ./...

test/unit:
	go test -race -short ./...

arch:
	go test ./internal/arch/...

vuln:
	govulncheck ./...
