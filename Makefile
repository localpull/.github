.PHONY: up down run sqlc migrate lint test test/unit

up:
	docker compose up -d --wait

down:
	docker compose down -v

run:
	go run ./cmd/api

sqlc:
	sqlc generate

migrate:
	psql "postgresql://orders:orders@localhost:5432/orders" \
		-f sql/migrations/001_init.sql

lint:
	golangci-lint run ./...

test:
	go test -race ./...

test/unit:
	go test -race -short ./...
