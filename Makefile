# Bitcoin Indexer Makefile

.PHONY: build run test clean docker-build docker-run setup help

# Default target
all: build

# Build the application
build:
	@echo "🔨 Building Bitcoin Indexer..."
	go build -o bitcoin-indexer ./cmd/indexer
	@echo "✅ Build complete!"

# Run the application
run: build
	@echo "🚀 Starting Bitcoin Indexer..."
	./bitcoin-indexer

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "🧪 Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning up..."
	rm -f bitcoin-indexer
	rm -f coverage.out coverage.html
	@echo "✅ Cleanup complete!"

# Setup development environment
setup:
	@echo "🚀 Setting up development environment..."
	@chmod +x scripts/setup.sh
	@./scripts/setup.sh

# Docker build
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t bitcoin-indexer .

# Docker run
docker-run: docker-build
	@echo "🐳 Running with Docker..."
	docker run -p 8080:8080 bitcoin-indexer

# Docker compose up
docker-up:
	@echo "🐳 Starting with Docker Compose..."
	docker-compose up -d

# Docker compose down
docker-down:
	@echo "🐳 Stopping Docker Compose..."
	docker-compose down

# Format code
fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "🔍 Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Install dependencies
deps:
	@echo "📥 Installing dependencies..."
	go mod download
	go mod tidy

# Generate mocks (if using mockgen)
mocks:
	@echo "🎭 Generating mocks..."
	@if command -v mockgen >/dev/null 2>&1; then \
		mockgen -source=internal/bitcoin/rpc.go -destination=internal/bitcoin/mocks/rpc_mock.go; \
	else \
		echo "⚠️  mockgen not installed. Install with: go install github.com/golang/mock/mockgen@latest"; \
	fi

# Run development server with hot reload (requires air)
dev:
	@echo "🔥 Starting development server with hot reload..."
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "⚠️  air not installed. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "📝 Or run: make run"; \
	fi

# Show help
help:
	@echo "Bitcoin Indexer - Available Commands:"
	@echo ""
	@echo "  build         - Build the application"
	@echo "  run           - Build and run the application"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  setup         - Setup development environment"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run with Docker"
	@echo "  docker-up     - Start with Docker Compose"
	@echo "  docker-down   - Stop Docker Compose"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  deps          - Install dependencies"
	@echo "  mocks         - Generate mocks"
	@echo "  dev           - Run with hot reload (requires air)"
	@echo "  help          - Show this help"
	@echo ""
	@echo "Quick start:"
	@echo "  make setup    # Setup development environment"
	@echo "  make run      # Build and run the indexer"
	@echo "  make docker-up # Run with Docker Compose"
