.PHONY: proto deps build clean run-master run-agent run-cli test package-master package-agent package-all docker-build docker-up docker-down docker-logs docker-restart docker-clean docker-status docker-restart-master docker-restart-agent docker-logs-master docker-logs-agent docker-shell-master docker-shell-agent

# Variables
PROTO_DIR = proto
PB_DIR = pb
BIN_DIR = bin
DIST_DIR = dist
MASTER_BINARY = $(BIN_DIR)/master
AGENT_BINARY = $(BIN_DIR)/agent
CLI_BINARY = $(BIN_DIR)/lookingglass-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell TZ='Asia/Shanghai' date '+%Y-%m-%d_%H:%M:%S')
LDFLAGS = -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go mod download
	go mod tidy

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@mkdir -p $(PB_DIR)
	protoc --go_out=$(PB_DIR) --go_opt=paths=source_relative --go-grpc_out=$(PB_DIR) --go-grpc_opt=paths=source_relative $(PROTO_DIR)/lookingglass.proto

	mv $(PB_DIR)/proto/* $(PB_DIR) 
	rm -rf $(PB_DIR)/proto

# Build all binaries
build: proto
	@echo "Building Master..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(MASTER_BINARY) ./master

	@echo "Building Agent..."
	go build -ldflags "$(LDFLAGS)" -o $(AGENT_BINARY) ./agent

	@echo "Building CLI..."
	go build -ldflags "$(LDFLAGS)" -o $(CLI_BINARY) ./cli

	@echo "Build complete!"

# Build Master only
build-master: proto
	@echo "Building Master..."
	@mkdir -p $(BIN_DIR)
	go build -o $(MASTER_BINARY) ./master

# Build Agent only
build-agent: proto
	@echo "Building Agent..."
	@mkdir -p $(BIN_DIR)
	go build -o $(AGENT_BINARY) ./agent

# Build CLI only
build-cli: proto
	@echo "Building CLI..."
	@mkdir -p $(BIN_DIR)
	go build -o $(CLI_BINARY) ./cli

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -rf $(PB_DIR)
	rm -rf logs/*.log

# Run Master
run-master:
	@echo "Running Master..."
	$(MASTER_BINARY) -config master/config.yaml

# Run Agent
run-agent:
	@echo "Running Agent..."
	$(AGENT_BINARY) -config agent/config.yaml

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# All: clean, deps, proto, build
all: clean proto deps build

# Package Master for deployment
package-master: build-master
	@echo "Packaging Master..."
	@mkdir -p $(DIST_DIR)/master
	@cp $(MASTER_BINARY) $(DIST_DIR)/master/
	@cp -r master/config.yaml.example $(DIST_DIR)/master/ || cp master/config.yaml $(DIST_DIR)/master/config.yaml.example
	@cp -r web $(DIST_DIR)/master/
	@echo "#!/bin/bash" > $(DIST_DIR)/master/start.sh
	@echo "cd \$$(dirname \$$0)" >> $(DIST_DIR)/master/start.sh
	@echo "./master -config config.yaml" >> $(DIST_DIR)/master/start.sh
	@chmod +x $(DIST_DIR)/master/start.sh
	@cd $(DIST_DIR) && tar czf lookingglass-master-$(VERSION).tar.gz master/
	@echo "Master package created: $(DIST_DIR)/lookingglass-master-$(VERSION).tar.gz"

# Package Agent for deployment
package-agent: build-agent
	@echo "Packaging Agent..."
	@mkdir -p $(DIST_DIR)/agent
	@cp $(AGENT_BINARY) $(DIST_DIR)/agent/
	@cp -r agent/config.yaml.example $(DIST_DIR)/agent/ || cp agent/config.yaml $(DIST_DIR)/agent/config.yaml.example
	@echo "#!/bin/bash" > $(DIST_DIR)/agent/start.sh
	@echo "cd \$$(dirname \$$0)" >> $(DIST_DIR)/agent/start.sh
	@echo "./agent -config config.yaml" >> $(DIST_DIR)/agent/start.sh
	@chmod +x $(DIST_DIR)/agent/start.sh
	@cd $(DIST_DIR) && tar czf lookingglass-agent-$(VERSION).tar.gz agent/
	@echo "Agent package created: $(DIST_DIR)/lookingglass-agent-$(VERSION).tar.gz"

# Package both Master and Agent
package-all: package-master package-agent
	@echo "All packages created in $(DIST_DIR)/"
	@ls -lh $(DIST_DIR)/*.tar.gz

# Docker commands
docker-build:
	@echo "Building Docker images..."
	docker-compose build

docker-up:
	@echo "Starting Docker containers..."
	docker-compose up -d

docker-down:
	@echo "Stopping Docker containers..."
	docker-compose down

docker-logs:
	@echo "Showing Docker logs..."
	docker-compose logs -f

docker-restart:
	@echo "Restarting Docker containers..."
	docker-compose restart

docker-clean:
	@echo "Cleaning Docker resources..."
	docker-compose down -v
	docker system prune -f

# Supervisor management commands
docker-status:
	@echo "=== Master Status ==="
	@docker exec lookingglass-master supervisorctl status || echo "Master container not running"
	@echo ""
	@echo "=== Agent Status ==="
	@docker exec lookingglass-agent supervisorctl status || echo "Agent container not running"

docker-restart-master:
	@echo "Restarting Master process (without restarting container)..."
	@docker exec lookingglass-master supervisorctl restart master

docker-restart-agent:
	@echo "Restarting Agent process (without restarting container)..."
	@docker exec lookingglass-agent supervisorctl restart agent

docker-logs-master:
	@echo "Tailing Master logs..."
	@docker exec -it lookingglass-master supervisorctl tail -f master stdout

docker-logs-agent:
	@echo "Tailing Agent logs..."
	@docker exec -it lookingglass-agent supervisorctl tail -f agent stdout

docker-shell-master:
	@echo "Opening shell in Master container..."
	@docker exec -it lookingglass-master sh

docker-shell-agent:
	@echo "Opening shell in Agent container..."
	@docker exec -it lookingglass-agent sh
