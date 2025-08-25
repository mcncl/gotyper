# GoTyper Makefile

.PHONY: build test lint clean install help

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'

build: ## Build the binary
	go build -o gotyper

test: ## Run tests
	go test ./...

test-coverage: ## Run tests with coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

clean: ## Clean build artifacts
	rm -f gotyper coverage.out coverage.html

install: build ## Install binary
	go install

check: test lint ## Run tests and linting