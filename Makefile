.PHONY: help build run test clean docker-up docker-down docker-build migrate lint fmt deps

APP_NAME = ad-delivery-simulator
DOCKER_IMAGE = $(APP_NAME):latest
GO_FILES = $(shell find . -name '*.go' -type f)

help:
	@echo "Available commands:"
	@echo "  make build       - Build the application"
	@echo "  make run         - Run the application locally"
	@echo "  make test        - Run tests"
	@echo "  make bench       - Run benchmarks"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make docker-up   - Start all services with docker-compose"
	@echo "  make docker-down - Stop all services"
	@echo "  make docker-build- Build Docker image"
	@echo "  make migrate     - Run database migrations"
	@echo "  make lint        - Run linter"
	@echo "  make fmt         - Format code"
	@echo "  make deps        - Download dependencies"
	@echo "  make load-test   - Run load testing"

build:
	@echo "Building application..."
	@go build -o bin/$(APP_NAME) cmd/server/main.go
	@echo "Build complete: bin/$(APP_NAME)"

run: build
	@echo "Starting application..."
	@./bin/$(APP_NAME)

test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

clean:
	@echo "Cleaning..."
	@rm -rf bin/ coverage.out
	@go clean -cache

docker-up:
	@echo "Starting Docker services..."
	@docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 10
	@echo "Services are running"

docker-down:
	@echo "Stopping Docker services..."
	@docker-compose down -v

docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .

docker-run: docker-build docker-up
	@echo "Running application in Docker..."
	@docker run --rm \
		--network ad-delivery-simulator_default \
		-p 8080:8080 \
		-e DATABASE_HOST=postgres \
		-e REDIS_HOST=redis \
		-e KAFKA_BROKERS=kafka:29092 \
		$(DOCKER_IMAGE)

migrate:
	@echo "Running database migrations..."
	@go run cmd/migrate/main.go up

lint:
	@echo "Running linter..."
	@golangci-lint run ./... || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && golangci-lint run ./...)

fmt:
	@echo "Formatting code..."
	@gofmt -s -w $(GO_FILES)
	@go fmt ./...

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

load-test:
	@echo "Running load test..."
	@go run scripts/load_test.go

dev: docker-up
	@echo "Starting development environment..."
	@air || (echo "Installing air..." && go install github.com/cosmtrek/air@latest && air)

seed-data:
	@echo "Seeding test data..."
	@go run scripts/seed_data.go

monitoring-up:
	@echo "Setting up monitoring..."
	@docker-compose -f docker-compose.monitoring.yml up -d

monitoring-down:
	@echo "Stopping monitoring..."
	@docker-compose -f docker-compose.monitoring.yml down

health-check:
	@echo "Checking service health..."
	@curl -s http://localhost:8080/health | jq '.' || echo "Service not responding"

logs:
	@docker-compose logs -f $(SERVICE)

ps:
	@docker-compose ps

restart: docker-down docker-up run

install-tools:
	@echo "Installing development tools..."
	@go install github.com/cosmtrek/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest