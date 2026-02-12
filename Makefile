.PHONY: help proto deps build run test clean docker-up docker-down migrate

help: ## Display this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

deps: ## Download dependencies
	go mod download
	go mod tidy

proto: ## Generate protobuf code
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/product/v1/product_service.proto

build: ## Build the server binary
	go build -o bin/server cmd/server/main.go

run: ## Run the server
	go run cmd/server/main.go

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Generate test coverage report
	go tool cover -html=coverage.out

docker-up: ## Start Spanner emulator
	docker-compose up -d
	@echo "Waiting for Spanner emulator to be ready..."
	@sleep 5

docker-down: ## Stop Spanner emulator
	docker-compose down

migrate: ## Run database migrations (requires running emulator)
	@echo "Running migrations against Spanner emulator..."
	@gcloud spanner databases create product-catalog \
		--instance=test-instance \
		--project=test-project \
		--ddl-file=migrations/001_initial_schema.sql || true

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out

lint: ## Run linters
	golangci-lint run ./...

fmt: ## Format code
	gofmt -s -w .

# Include E2E test targets
include Makefile.e2e
	go fmt ./...
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

all: deps proto build ## Build everything
