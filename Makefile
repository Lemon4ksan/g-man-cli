# Project variables
PKG=$(shell go list ./... | grep -v /vendor/)
COVER_OUT?=coverage.out
COVER_PKG?=$(PKG)

# Colors for console output
CYAN  := \033[0;36m
RESET := \033[0m

.PHONY: all proto build test format clean help

all: build # Default target

proto: # Generate protobuf and gRPC files from daemon.proto
	cd proto && \
	protoc --go_out=./daemon --go_opt=paths=source_relative \
		--go-grpc_out=./daemon --go-grpc_opt=paths=source_relative \
		daemon.proto && \
	protoc --go_out=./paymaster --go_opt=paths=source_relative \
		--go-grpc_out=./paymaster --go-grpc_opt=paths=source_relative \
		paymaster.proto
		
build: # Build both daemon and CLI client
	go build -o bin/g-mand ./cmd/g-mand/
	go build -o bin/gmanctl ./cmd/gmanctl/

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
	addlicense -c "Lemon4ksan" -l bsd -ignore "pkg/protobuf/**" -ignore "**/*.yml" .
	golangci-lint run --fix

help: ## Show this message
	@printf "Usage: make [target]\n"
	@printf "\n"
	@printf "Targets:\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
