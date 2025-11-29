.PHONY: setup start stop test test-parallel clean logs db-shell deps docker-up docker-down venv

# Python virtual environment
VENV_DIR = venv
PYTHON = $(VENV_DIR)/bin/python
PIP = $(VENV_DIR)/bin/pip
PYTEST = $(VENV_DIR)/bin/pytest

# Create virtual environment
venv:
	@echo "Creating Python virtual environment..."
	python3 -m venv $(VENV_DIR)
	@echo "Virtual environment created!"

# Setup project
setup: venv
	@echo "Setting up project..."
	docker-compose up -d mysql
	@echo "Waiting for MySQL to be ready..."
	sleep 15
	docker-compose up -d temporal
	@echo "Waiting for Temporal to be ready..."
	sleep 10
	@echo "Installing Go dependencies..."
	go mod download
	@echo "Installing Python dependencies in virtual environment..."
	$(PIP) install --upgrade pip
	$(PIP) install -r requirements.txt
	@echo "Setup complete!"
	@echo ""
	@echo "Virtual environment created at: $(VENV_DIR)/"
	@echo "To activate manually: source $(VENV_DIR)/bin/activate"

# Install Go dependencies
deps:
	@echo "Installing Go dependencies..."
	go mod download
	go mod tidy

# Start Docker services
docker-up:
	@echo "Starting Docker services..."
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	sleep 15

# Stop Docker services
docker-down:
	@echo "Stopping Docker services..."
	docker-compose down

# Start all services
start: docker-up
	@echo "Starting application services..."
	@echo "Starting Temporal worker..."
	go run cmd/worker/main.go &
	@echo "Starting HTTP server..."
	go run cmd/server/main.go &
	@echo "All services started!"
	@echo "API server: http://localhost:8080"
	@echo "Temporal UI: http://localhost:8088"

# Stop all services
stop:
	@echo "Stopping services..."
	-pkill -f "go run cmd/server/main.go"
	-pkill -f "go run cmd/worker/main.go"
	docker-compose down
	@echo "Services stopped!"

# Run tests
test:
	@echo "Running tests..."
	@if [ ! -d "$(VENV_DIR)" ]; then \
		echo "Virtual environment not found. Run 'make setup' first."; \
		exit 1; \
	fi
	$(PYTEST) tests/ -v --tb=short

# Run tests in parallel
test-parallel:
	@echo "Running tests in parallel..."
	@if [ ! -d "$(VENV_DIR)" ]; then \
		echo "Virtual environment not found. Run 'make setup' first."; \
		exit 1; \
	fi
	$(PYTEST) tests/ -v -n 4

# Clean up everything
clean:
	@echo "Cleaning up..."
	-pkill -f "go run"
	docker-compose down -v
	rm -rf data/
	rm -rf $(VENV_DIR)
	@echo "Cleanup complete!"

# View logs
logs:
	docker-compose logs -f

# MySQL shell
db-shell:
	docker-compose exec mysql mysql -u booking_user -pbooking_pass flight_booking

# Build binaries
build:
	@echo "Building binaries..."
	go build -o bin/server cmd/server/main.go
	go build -o bin/worker cmd/worker/main.go
	@echo "Binaries built in bin/"

# Run server binary
run-server:
	./bin/server

# Run worker binary
run-worker:
	./bin/worker

# Help
help:
	@echo "Available commands:"
	@echo "  make setup          - Initial setup (Docker + venv + dependencies)"
	@echo "  make venv           - Create Python virtual environment only"
	@echo "  make start          - Start all services"
	@echo "  make stop           - Stop all services"
	@echo "  make docker-up      - Start Docker containers only"
	@echo "  make docker-down    - Stop Docker containers"
	@echo "  make test           - Run Python tests (in venv)"
	@echo "  make test-parallel  - Run tests in parallel (in venv)"
	@echo "  make build          - Build Go binaries"
	@echo "  make clean          - Clean up everything (including venv)"
	@echo "  make logs           - View Docker logs"
	@echo "  make db-shell       - Open MySQL shell"
	@echo "  make help           - Show this help message"
