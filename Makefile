export GOPROXY=https://proxy.golang.org

SHELL= /bin/bash
GO ?= go
BUILD_DIR := ./build
PREFIX := /usr/local
BIN_DIR := $(PREFIX)/bin/
NAME := vgrep
PROJECT := github.com/vrothberg/vgrep
VERSION := $(shell cat ./VERSION)
COMMIT := $(shell git rev-parse HEAD 2> /dev/null || true)
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)

GO_SRC=$(shell find . -name \*.go)

GO_BUILD=$(GO) build
# Go module support: set `-mod=vendor` to use the vendored sources
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
	GO_BUILD=GO111MODULE=on $(GO) build -mod=vendor
endif

COVERAGE_PATH ?= $(shell pwd)/.coverage
COVERAGE_PROFILE ?= $(shell pwd)/coverage.txt
export COVERAGE_PATH
export COVERAGE_PROFILE
$(shell mkdir -p ${COVERAGE_PATH})

all: check build

.PHONY: build
build: $(GO_SRC)
	$(GO_BUILD) -buildmode=pie -o $(BUILD_DIR)/$(NAME) -ldflags "-s -w -X main.version=${VERSION}-$(COMMIT)"

.PHONY: build.coverage
build.coverage: $(GO_SRC)
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
	rm -rf $(BUILD_DIR)

.PHONY: deps
deps:
	$(GO) get -u ./...

.PHONY: check
check: $(GO_SRC)
	${BUILD_DIR}/golangci-lint run

.PHONY: test
test: test-integration

.PHONY: test-integration
test-integration:
	export PATH=./test/bin:$$PATH; bats test/*.bats

.PHONY: test-integration.coverage
test-integration.coverage:
	export PATH=./test/bin:$$PATH; export COVERAGE=1; bats test/*.bats

.PHONY: vendor
vendor:
	GO111MODULE=on go mod tidy
	GO111MODULE=on go mod vendor
	GO111MODULE=on go mod verify

.install.tools:
	export \
		VERSION=v1.26.0 \
		URL=https://raw.githubusercontent.com/golangci/golangci-lint \
		BINDIR=${BUILD_DIR} && \
	curl -sfL $$URL/$$VERSION/install.sh | sh -s $$VERSION
	VERSION=v1.1.0 ./hack/install_bats.sh

	curl -L https://github.com/BurntSushi/ripgrep/releases/download/12.0.1/ripgrep-12.0.1-x86_64-unknown-linux-musl.tar.gz | tar xz
	mkdir -p ./test/bin && mv ripgrep-12.0.1-x86_64-unknown-linux-musl/rg ./test/bin/ && rm -rf ripgrep-12.0.1-x86_64-unknown-linux-musl

.PHONY: install
install:
	cp $(BUILD_DIR)/$(NAME) $(BIN_DIR)
	chmod 755 $(BIN_DIR)/$(NAME)

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
