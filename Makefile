.PHONY: all build build-region build-coordinator test lint clean proto run-region run-coordinator

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
BINARY_REGION=bin/region
BINARY_COORDINATOR=bin/coordinator

# Build flags
LDFLAGS=-ldflags "-s -w"

all: build

build: build-region build-coordinator

build-region:
	@echo "Building region service..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_REGION) ./cmd/region

build-coordinator:
	@echo "Building coordinator service..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_COORDINATOR) ./cmd/coordinator

test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

lint:
	@echo "Running linter..."
	golangci-lint run ./...

clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

proto:
	@echo "Generating protobuf..."
	protoc --go_out=. --go-grpc_out=. pkg/protocol/proto/*.proto

deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

run-region:
	@echo "Starting region service..."
	$(GOCMD) run ./cmd/region --config configs/region.yaml

run-coordinator:
	@echo "Starting coordinator service..."
	$(GOCMD) run ./cmd/coordinator --config configs/coordinator.yaml

# Docker targets
docker-build:
	@echo "Building Docker images..."
	docker build -t jzse-region:latest -f deployments/docker/Dockerfile.region .
	docker build -t jzse-coordinator:latest -f deployments/docker/Dockerfile.coordinator .

# Development helpers
dev-region:
	@echo "Starting region service in development mode..."
	JZSE_LOGGER_DEVELOPMENT=true JZSE_LOGGER_FORMAT=console $(GOCMD) run ./cmd/region

fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

vet:
	@echo "Vetting code..."
	$(GOCMD) vet ./...
