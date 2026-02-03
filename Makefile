.PHONY: build run test clean docker-build docker-run docker-stop keys help

# Variables
BINARY_NAME=cloudfauxnt
DOCKER_IMAGE=cloudfauxnt:latest
CONFIG_FILE=config.yaml

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the Go binary
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .

run: ## Run the application locally
	@echo "Running $(BINARY_NAME)..."
	go run . --config $(CONFIG_FILE)

test: ## Run unit tests
	@echo "Running tests..."
	go test -v ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	go clean

keys: ## Generate RSA key pair for CloudFront signing
	@echo "Generating RSA key pair..."
	@mkdir -p keys
	openssl genrsa -out keys/private.pem 2048
	openssl rsa -in keys/private.pem -pubout -out keys/public.pem
	@echo "Keys generated in keys/ directory"
	@echo "Public key: keys/public.pem"
	@echo "Private key: keys/private.pem"

config: ## Create config.yaml from example
	@if [ ! -f $(CONFIG_FILE) ]; then \
		cp config.example.yaml $(CONFIG_FILE); \
		echo "Created $(CONFIG_FILE) from example"; \
	else \
		echo "$(CONFIG_FILE) already exists"; \
	fi

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## Run Docker container
	@echo "Starting Docker container..."
	docker compose up -d

docker-stop: ## Stop Docker container
	@echo "Stopping Docker container..."
	docker compose down

docker-logs: ## Show Docker container logs
	docker compose logs -f

docker-shell: ## Open shell in running container
	docker compose exec cloudfauxnt sh

setup: config keys ## Initial setup (create config and generate keys)
	@echo "Setup complete!"
	@echo "1. Edit config.yaml for your environment"
	@echo "2. Run 'make run' or 'make docker-run'"

all: build ## Build everything
