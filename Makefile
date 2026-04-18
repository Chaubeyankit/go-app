.PHONY: run build test test-cover migrate-up migrate-down migrate-create migrate-status docker-up docker-down lint tidy help

run:
	go run ./cmd/api

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server ./cmd/api

test:
	go test ./... -v -race -timeout 60s

test-cover:
	go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out

migrate-up:
	migrate -path ./migrations -database "postgres://ankit:Root@123@localhost:5432/myapp?sslmode=disable" up

migrate-down:
	migrate -path ./migrations -database "postgres://ankit:Root@123@localhost:5432/myapp?sslmode=disable" down 1

# Create a new migration
migrate-create:
	@read -p "Enter migration name: " name; \
	./scripts/migrate.sh create $$name

# Show migration status
migrate-status:
	./scripts/migrate.sh version

docker-up:
	docker compose -f docker/docker-compose.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml down

lint:
	golangci-lint run ./...

tidy:
	go mod tidy && go mod verify

help:
	@echo "Available commands:"
	@echo "  make run              - Start the server"
	@echo "  make build            - Build the server binary"
	@echo "  make test             - Run tests"
	@echo "  make test-cover       - Run tests with coverage report"
	@echo "  make lint             - Run linter"
	@echo "  make tidy             - Tidy and verify go.mod"
	@echo ""
	@echo "Database:"
	@echo "  make docker-up        - Start Redis in Docker"
	@echo "  make docker-down      - Stop Docker services"
	@echo "  make migrate-create   - Create a new migration"
	@echo "  make migrate-up       - Run pending migrations"
	@echo "  make migrate-down     - Rollback last migration"
	@echo "  make migrate-status   - Show migration version"
	@echo ""
	@echo "Or use the migration script directly:"
	@echo "  ./scripts/migrate.sh create <name>"
	@echo "  ./scripts/migrate.sh up"
	@echo "  ./scripts/migrate.sh down"
	@echo "  ./scripts/migrate.sh version"