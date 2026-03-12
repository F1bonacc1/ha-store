BINARY_NAME=ha-store

.PHONY: all build run test coverage docker-up docker-down clean help

all: build

build: ## Build the application
	go build -o ${BINARY_NAME} main.go

run: ## Run the application
	go run main.go

test: ## Run tests
	go test -v ./...

coverage: ## Run tests with coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-integration: ## Run integration tests
	go test -v -tags=integration ./tests/integration/...

docker-up: ## Start docker dependencies
	docker compose up -d

docker-down: ## Stop docker dependencies
	docker compose down

clean: ## Clean build artifacts
	go clean
	rm -f ${BINARY_NAME}
	rm -f coverage.out

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
