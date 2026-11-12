EMULATOR ?= localhost:9010
DATABASE ?= projects/test-project/instances/test-instance/databases/product-catalog

.PHONY: help test e2e up down migrate run proto fmt vet

help: ## List targets
	@grep -E '^[a-z0-9-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-10s %s\n", $$1, $$2}'

test: ## Unit and transport tests (no Docker needed)
	go test -race -count=1 ./...

up: ## Start the Spanner emulator
	docker compose up -d

down: ## Stop the Spanner emulator
	docker compose down

e2e: up ## End-to-end tests against the emulator
	SPANNER_EMULATOR_HOST=$(EMULATOR) go test -count=1 -v ./test/e2e/...

migrate: up ## Create the instance and database on the emulator
	SPANNER_EMULATOR_HOST=$(EMULATOR) SPANNER_DATABASE=$(DATABASE) go run ./cmd/migrate

run: migrate ## Run the server against the emulator
	SPANNER_EMULATOR_HOST=$(EMULATOR) SPANNER_DATABASE=$(DATABASE) go run ./cmd/server

proto: ## Regenerate gRPC code (needs protoc, protoc-gen-go, protoc-gen-go-grpc)
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/product/v1/product_service.proto

fmt: ## Format
	gofmt -w .

vet: ## Vet
	go vet ./...
