.PHONY: help up down logs backend frontend migrate-up migrate-down test lint tidy build-backend build-frontend fmt

help:
	@echo "Pterodactyl Billing — make targets:"
	@awk 'BEGIN{FS=":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

up: ## docker compose up (dev stack)
	docker compose up -d --build

down: ## docker compose down
	docker compose down -v

logs: ## follow all logs
	docker compose logs -f --tail=200

backend: ## run backend locally (requires local postgres/redis)
	cd backend && go run ./cmd/server

frontend: ## run frontend dev server
	cd frontend && npm run dev

migrate-up: ## apply DB migrations
	cd backend && go run ./cmd/migrate up

migrate-down: ## rollback last migration
	cd backend && go run ./cmd/migrate down

test: ## run backend tests
	cd backend && go test ./... -race -count=1

lint: ## run linters
	cd backend && go vet ./...
	cd frontend && npm run lint

tidy: ## go mod tidy
	cd backend && go mod tidy

fmt: ## format code
	cd backend && gofmt -s -w .
	cd frontend && npm run format

build-backend: ## build backend binary
	cd backend && CGO_ENABLED=0 go build -o bin/server ./cmd/server

build-frontend: ## build frontend static
	cd frontend && npm run build
