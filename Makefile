# Loads .env if present and exports it to the commands.
-include .env
export

DB_DSN ?= postgres://hamy:hamy@localhost:5432/hamy?sslmode=disable
SQLC_VERSION ?= v1.31.1

.PHONY: run build test coverage test-integration seed smoke generate db-up storage-up infra-up up down ps logs db-logs migrate-up migrate-down migrate-create

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test -race ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Requires a migrated database. Defaults to the local DB_DSN.
test-integration:
	DB_TEST_DSN="$${DB_TEST_DSN:-$(DB_DSN)}" go test -race -tags=integration ./internal/repository/postgres/...

# Idempotent demo accounts (credentials reset) and album.
seed:
	go run ./cmd/seed

# End-to-end flow against a running stack (make up).
smoke:
	./scripts/smoke.sh

# Regenerates internal/repository/postgres/sqlcgen from queries/*.sql
generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

# Postgres only
db-up:
	docker compose up -d db

# MinIO only
storage-up:
	docker compose up -d minio

# Postgres + MinIO (native development with `make run`)
infra-up:
	docker compose up -d db minio

# Full stack: db + minio + migrations + api in containers
up:
	docker compose up -d --build

down:
	docker compose down

ps:
	docker compose ps

logs:
	docker compose logs -f

db-logs:
	docker compose logs -f db

migrate-up:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(DB_DSN)" up

migrate-down:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(DB_DSN)" down

# make migrate-create name=add_phone
migrate-create:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations create $(name) sql
