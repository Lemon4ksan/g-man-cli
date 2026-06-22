# Project variables
PKG=$(shell go list ./... | grep -v /vendor/)
COVER_OUT?=coverage.out
COVER_PKG?=$(PKG)

# pprof variables
PPROF_PORT?=6060
PPROF_UI_PORT?=8080

# Colors for console output
CYAN  := \033[0;36m
RESET := \033[0m

.PHONY: all proto build test format clean help pprof-cpu pprof-heap pprof-allocs pprof-goroutine

all: build # Default target

proto: # Generate protobuf and gRPC files from daemon.proto and paymaster.proto
	cd proto && \
	protoc --go_out=./daemon --go_opt=paths=source_relative \
		--go-grpc_out=./daemon --go-grpc_opt=paths=source_relative \
		daemon.proto

build: # Build both daemon and CLI client
	go build -o bin/g-mand.exe ./cmd/g-mand
	go build -o bin/gmanctl.exe ./cmd/gmanctl

test: ## Run normal quick tests
	@printf "$(CYAN)Running unit tests...$(RESET)\n"
	go test -v $(PKG)

race: ## Run tests with race detector
	@printf "$(CYAN)Running tests with race detector...$(RESET)\n"
	go test -v -race -timeout 30s $(PKG)

cover: ## Run tests and open the coverage report in a browser
	@printf "$(CYAN)Generating coverage report...$(RESET)\n"
	go test -coverprofile=$(COVER_OUT) $(COVER_PKG)
	go tool cover -html=$(COVER_OUT)

cover-clean: ## Display the clean coverage report in the terminal
	@printf "$(CYAN)Generating clean coverage report...$(RESET)\n"
	go test -coverprofile=$(COVER_OUT) $(COVER_PKG)
	go run cmd/coverage/main.go --file=$(COVER_OUT) --sort=percent

lint: ## Check the code with a linter (requires golangci-lint)
	@printf "$(CYAN)Running linter...$(RESET)\n"
	golangci-lint run ./...

clean: ## Delete temporary files and binaries
	@printf "$(CYAN)Cleaning up...$(RESET)\n"
	rm -rf bin/
	rm -f coverage.out

format: ## Run go code formatting
	addlicense -c "Lemon4ksan" -l bsd -ignore "proto/**" -ignore "**/*.yml" -ignore "**/Dockerfile" .
	golangci-lint run --fix

pprof-cpu: ## Collect CPU profile (30s) and open web UI
	@printf "$(CYAN)Collecting CPU profile for 30s and starting web UI on port $(PPROF_UI_PORT)...$(RESET)\n"
	go tool pprof -http=:$(PPROF_UI_PORT) http://localhost:$(PPROF_PORT)/debug/pprof/profile

pprof-heap: ## Collect Heap profile and open web UI
	@printf "$(CYAN)Collecting Heap profile and starting web UI on port $(PPROF_UI_PORT)...$(RESET)\n"
	go tool pprof -sample_index=alloc_space -http=:$(PPROF_UI_PORT) http://localhost:$(PPROF_PORT)/debug/pprof/heap

pprof-allocs: ## Collect Allocs profile and open web UI
	@printf "$(CYAN)Collecting Allocs profile and starting web UI on port $(PPROF_UI_PORT)...$(RESET)\n"
	go tool pprof -http=:$(PPROF_UI_PORT) http://localhost:$(PPROF_PORT)/debug/pprof/allocs

pprof-goroutine: ## Collect Goroutine profile and open web UI
	@printf "$(CYAN)Collecting Goroutine profile and starting web UI on port $(PPROF_UI_PORT)...$(RESET)\n"
	go tool pprof -http=:$(PPROF_UI_PORT) http://localhost:$(PPROF_PORT)/debug/pprof/goroutine

help: ## Show this message
	@printf "Usage: make [target]\n"
	@printf "\n"
	@printf "Targets:\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
