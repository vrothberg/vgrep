GO ?= go
BUILD_DIR := ./build
BIN_DIR := /usr/local/bin
NAME := vgrep
PROJECT := github.com/vrothberg/vgrep

GO_SRC=$(shell find . -name \*.go)

all: check build

build: $(GO_SRC)
	 $(GO) build -o $(BUILD_DIR)/$(NAME)

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

IMAGENAME := vgrepdev
.PHONY:
buildImage:
	docker build -f Dockerfile -t $(IMAGENAME) --build-arg PROJECT=$(PROJECT) .

.PHONY: buildInContainer
buildInContainer: buildImage
	docker run --rm -v `pwd`:/go/src/$(PROJECT) $(IMAGENAME)

.PHONY: install
install: deps build
	sudo install -D -m755 $(BUILD_DIR)/$(NAME) $(BIN_DIR)

.PHONY: uninstall
uninstall:
	sudo rm $(BIN_DIR)/$(NAME)
