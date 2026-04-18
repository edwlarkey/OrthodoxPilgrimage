# =========================================================================== #
# VARIABLES
# =========================================================================== #

# APP_NAME is the name of the application, determined by the directory name
APP_NAME ?= $(shell basename $(shell pwd))
# Architecture, defaults to amd64 if not set by the environment
GOARCH ?= amd64
# Output binary path
BIN_DIR = ./bin
BIN_PATH = $(BIN_DIR)/$(APP_NAME)
DOCKER_BIN_PATH = $(BIN_DIR)/$(APP_NAME)-$(GOARCH)
# Build target path, defaults to all packages
BUILD_TARGET ?= ./cmd/server

# =========================================================================== #
# HELPERS
# =========================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

## install/tools: installs tooling needed in some other commands
.PHONY: install/tools
install/tools:
	@echo 'Installing golangci-lint...'
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo 'Installing sqlc...'
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# =========================================================================== #
# DEVELOPMENT
# =========================================================================== #

## deps: download dependencies
.PHONY: deps
deps:
	@echo 'Downloading dependencies...'
	go mod download

## tidy: tidy and verify dependencies
.PHONY: tidy
tidy:
	@echo 'Tidying dependencies...'
	go mod tidy

## audit: format code, vet code, run linters, and test
.PHONY: audit
audit:
	@echo 'Formatting code...'
	go fmt ./...
	@echo 'Vetting code...'
	go vet ./...
	golangci-lint run ./...
	@echo 'Running tests...'
	# go test includes some go vet checks by default. turn them off because
	# we just ran go vet above
	go test -race -vet=off ./...

## test: run tests and provide coverage information
.PHONY: test
test:
	@echo 'Running tests...'
	go test -fullpath -race -v -cover ./...

# =========================================================================== #
# GENERATE
# =========================================================================== #

## generate: run sqlc generate
.PHONY: generate
generate:
	@echo 'Generating sqlc code...'
	sqlc generate

# =========================================================================== #
# BUILD
# =========================================================================== #

## build: build the application to $(BIN_PATH)
.PHONY: build
build: generate
	@echo 'Building $(APP_NAME)'
	go build -ldflags="-s" -o=$(BIN_PATH) $(BUILD_TARGET)

## build.arch: build the application for docker to $(BIN_PATH)
.PHONY: build.arch
build.arch: generate
	@echo 'Building $(APP_NAME) for docker ($(GOARCH))...'
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -tags lambda.norpc -o=$(DOCKER_BIN_PATH) $(BUILD_TARGET)

## build.docker: build the application for all supported architectures
.PHONY: build.docker
build.docker:
	@echo 'Building $(APP_NAME) for all supported architectures...'
	$(MAKE) build.arch GOARCH=amd64
	$(MAKE) build.arch GOARCH=arm64
