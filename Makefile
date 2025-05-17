# Makefile for Go project with comprehensive testing

.PHONY: test test-unit test-integration test-coverage test-verbose clean build lint

# Default target
all: clean lint test

# Build the application
build:
	go build -v .

# Run all tests
test:
	go test -v ./...

# Run only unit tests (excludes integration tests and skipped tests)
test-unit:
	go test -v -short ./...

# Run integration tests
test-integration:
	go test -v -run Integration ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

# Run tests with coverage and show coverage percentage
test-coverage-check:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@coverage=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $${coverage}%"; \
	if [ "$$(echo "$${coverage} < 70" | bc)" -eq 1 ]; then \
		echo "❌ Coverage is below 70%"; \
		exit 1; \
	else \
		echo "✅ Coverage is above 70%"; \
	fi

# Run tests with verbose output and race detection
test-verbose:
	go test -v -race -coverprofile=coverage.out ./...

# Run benchmarks
benchmark:
	go test -bench=. -benchmem ./...

# Generate test report
test-report:
	go test -v -json ./... > test-report.json
	go tool cover -html=coverage.out -o coverage.html

# Clean generated files
clean:
	rm -f coverage.out coverage.html test-report.json
	go clean ./...

# Lint the code
lint:
	go fmt ./...
	go vet ./...

# Install test dependencies
deps:
	go mod tidy
	go mod download

# Create test certificates for integration testing
create-test-certs:
	mkdir -p certs
	openssl req -x509 -newkey rsa:2048 -keyout certs/key.pem -out certs/cert.pem -days 365 -nodes -subj "/C=US/ST=Test/L=Test/O=Test/CN=localhost"

# Run tests in Docker (optional)
test-docker:
	docker run --rm -v $(PWD):/workspace -w /workspace golang:1.21 make test-coverage

# Help target
help:
	@echo "Available targets:"
	@echo "  build              - Build the application"
	@echo "  test               - Run all tests"
	@echo "  test-unit          - Run unit tests only"
	@echo "  test-integration   - Run integration tests only"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  test-coverage-check - Check if coverage is above 70%"
	@echo "  test-verbose       - Run tests with verbose output and race detection"
	@echo "  benchmark          - Run benchmark tests"
	@echo "  test-report        - Generate comprehensive test report"
	@echo "  clean              - Remove generated files"
	@echo "  lint               - Format and vet code"
	@echo "  deps               - Install dependencies"
	@echo "  create-test-certs  - Create test certificates"
	@echo "  help               - Show this help message"

# Coverage targets for CI/CD
ci-test:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Quick test for development
quick-test:
	go test -short ./...
