SHELL= /bin/bash
GO ?= go
BUILD_DIR := ./build
BIN_DIR := /usr/local/bin
NAME := vgrep
PROJECT := github.com/vrothberg/vgrep
VERSION := $(shell cat ./VERSION)
COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)

GO_SRC=$(shell find . -name \*.go)

all: check build

.PHONY: build
build: $(GO_SRC)
	 $(GO) build -buildmode=pie -o $(BUILD_DIR)/$(NAME) -ldflags "-s -w -X main.version=${VERSION}-$(COMMIT)-dev"

.PHONY: release
release: $(GO_SRC)
	 $(GO) build -buildmode=pie -o $(BUILD_DIR)/$(NAME) -ldflags "-s -w -X main.version=${VERSION}"

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: deps
deps:
	$(GO) get -u ./...

.PHONY: check
check: $(GO_SRC)
	@which gofmt >/dev/null 2>/dev/null || (echo "ERROR: gofmt not found." && false)
	test -z "$$(gofmt -s -l . | grep -vE 'vendor/' | tee /dev/stderr)"
	@which golint >/dev/null 2>/dev/null|| (echo "ERROR: golint not found." && false)
	test -z "$$(golint $(PROJECT)/...  | grep -vE 'vendor/' | tee /dev/stderr)"
	@which golangci-lint >/dev/null 2>/dev/null|| (echo "ERROR: golangci-lint not found." && false)
	test -z "$$(golangci-lint run --disable=errcheck)"
	@go doc cmd/vet >/dev/null 2>/dev/null|| (echo "ERROR: go vet not found." && false)
	test -z "$$($(GO) vet $$($(GO) list $(PROJECT)/...) 2>&1 | tee /dev/stderr)"

.PHONY: test
test: test-integration

.PHONY: test-integration
test-integration:
	bats test/*.bats

.PHONY: vendor
vendor:
	GO111MODULE=on go mod tidy
	GO111MODULE=on go mod vendor
	GO111MODULE=on go mod verify

.PHONY: install
install:
	sudo install -D -m755 $(BUILD_DIR)/$(NAME) $(BIN_DIR)

.PHONY: uninstall
uninstall:
	sudo rm $(BIN_DIR)/$(NAME)

# CONTAINER MAKE TARGETS

CONTAINER_IMAGE := vgrepdev
CONTAINER_RUNCMD := run --rm --privileged -v `pwd`:/go/src/$(PROJECT)

.PHONY: container-image
container-image:
	$(CONTAINER_RUNTIME) build -f Dockerfile -t $(CONTAINER_IMAGE) --build-arg PROJECT=$(PROJECT) .

.PHONY: container-build
container-build: container-image
	$(CONTAINER_RUNTIME) $(CONTAINER_RUNCMD) $(CONTAINER_IMAGE) make build

.PHONY: container-release
container-release: container-image
	$(CONTAINER_RUNTIME) $(CONTAINER_RUNCMD) $(CONTAINER_IMAGE) make release

.PHONY: container-shell
container-shell: container-image
	$(CONTAINER_RUNTIME) $(CONTAINER_RUNCMD) -it $(CONTAINER_IMAGE) sh
