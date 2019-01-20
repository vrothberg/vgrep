SHELL= /bin/bash
GO ?= go
BUILD_DIR := ./build
BIN_DIR := /usr/local/bin
NAME := vgrep
PROJECT := github.com/vrothberg/vgrep
VERSION := $(shell cat ./VERSION)
COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)

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
	@go doc cmd/vet >/dev/null 2>/dev/null|| (echo "ERROR: go vet not found." && false)
	test -z "$$($(GO) vet $$($(GO) list $(PROJECT)/...) 2>&1 | tee /dev/stderr)"

.PHONY: test
test: test-integration

.PHONY: test-integration
test-integration:
	bats test/*.bats

IMAGENAME := vgrepdev
.PHONY:
buildImage:
	docker build -f Dockerfile -t $(IMAGENAME) --build-arg PROJECT=$(PROJECT) .

.PHONY: buildInContainer
buildInContainer: buildImage
	docker run --rm -v `pwd`:/go/src/$(PROJECT) $(IMAGENAME)

.PHONY: vendor
vendor:
	vndr

.PHONY: install
install:
	sudo install -D -m755 $(BUILD_DIR)/$(NAME) $(BIN_DIR)

.PHONY: uninstall
uninstall:
	sudo rm $(BIN_DIR)/$(NAME)
