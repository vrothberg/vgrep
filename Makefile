export GOPROXY=https://proxy.golang.org

SHELL= /bin/bash
.DELETE_ON_ERROR:
.DEFAULT_GOAL := all

GO ?= go
GOPATH := $(shell $(GO) env GOPATH)
GOBIN := $(shell $(GO) env GOBIN)
BUILD_DIR := ./build
PREFIX := $(if $(prefix),$(prefix),/usr/local)
BIN_DIR := $(DESTDIR)$(PREFIX)/bin/
MAN_DIR := $(DESTDIR)$(PREFIX)/share/man
NAME := vgrep
PROJECT := github.com/vrothberg/vgrep
VERSION := $(shell cat ./VERSION)
COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)
PATH := $(CURDIR)/test/bin:$(PATH)

GO_SRC=$(shell find . -name \*.go)
GO_BUILD=$(GO) build -mod=vendor

COVERAGE_PATH ?= $(shell pwd)/.coverage
COVERAGE_PROFILE ?= $(shell pwd)/coverage.txt
export COVERAGE_PATH
export COVERAGE_PROFILE

ifeq ($(GOBIN),)
	GOBIN := $(GOPATH)/bin
endif

GOMD2MAN ?= $(shell command -v go-md2man || echo '$(GOBIN)/go-md2man')

MANPAGES_MD = $(wildcard docs/*.md)
MANPAGES ?= $(MANPAGES_MD:%.md=%)

all: check build

.PHONY: build
build: $(GO_SRC)
	$(GO_BUILD) -o $(BUILD_DIR)/$(NAME) -ldflags "-s -w -X main.version=${VERSION}-$(COMMIT)"

.PHONY: build.coverage
build.coverage: $(GO_SRC)
	@mkdir -p $(COVERAGE_PATH)
	$(GO) test \
		-covermode=count \
		-coverpkg=./... \
		-mod=vendor \
		-tags coverage \
		-buildmode=pie -c -o $(BUILD_DIR)/$(NAME) \
		-ldflags "-s -w -X main.version=${VERSION}-$(COMMIT)"

.PHONY: codecov
codecov:
	bash <(curl -s https://codecov.io/bash) -v -s $(COVERAGE_PATH) -f "coverprofile.integration.*"

.PHONY: release
release: $(GO_SRC)
	$(GO_BUILD) -buildmode=pie -o $(BUILD_DIR)/$(NAME) -ldflags "-s -w -X main.version=${VERSION}"

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR) docs/*.1

.PHONY: deps
deps:
	$(GO) get -u ./...

.PHONY: check
check: $(GO_SRC)
	$(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint run

.PHONY: test
test: test-integration

.PHONY: test-integration
test-integration:
	bats test/*.bats

.PHONY: test-integration.coverage
test-integration.coverage:
	export COVERAGE=1; bats test/*.bats

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor
	go mod verify

.PHONY: .install.tools .install.go-md2man

.install.tools:
	@echo "golangci-lint is now managed via go.mod and tools.go"
	@echo "No installation needed - it will be automatically downloaded when running 'make check'"

.install.go-md2man:
	@echo "go-md2man is now managed via go.mod and tools.go"
	@echo "No installation needed - it will be automatically downloaded when generating docs"

.PHONY: install
install: install-docs
	install -d -m 755 $(BIN_DIR)
	install -m 755 $(BUILD_DIR)/$(NAME) $(BIN_DIR)

.PHONY: install-docs
install-docs: docs
	install -d -m 755 ${MAN_DIR}/man1
	install -m 644 docs/*.1 ${MAN_DIR}/man1/

.PHONY: uninstall
uninstall:
	rm $(BIN_DIR)/$(NAME)

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

$(MANPAGES): %:%.md
	sed -e 's/\((vgrep.*\.md)\)//' -e 's/\[\(vgrep.*\)\]/\1/' $<  | $(GO) run github.com/cpuguy83/go-md2man -in /dev/stdin -out $@

.PHONY: docs
docs: $(MANPAGES)
