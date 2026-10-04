# AGENTS

## Build and test

- `make build` compiles `./build/vgrep`
- `make check` runs golangci-lint
- `go test ./...` runs Go unit tests
- `make container-test` builds and runs all tests (Go + bats integration) inside a container; always use this on macOS as bats tests are very slow natively
- `make test` runs bats integration tests locally (requires `bats`, `less`, `ripgrep`)

## Project structure

- `vgrep.go` is the single main source file containing CLI args, grep dispatch, command parsing and all interactive shell commands
- `internal/ansi/` handles ANSI color formatting with a global disable switch
- `internal/colwriter/` prints columnar output using the ansi package
- Dependencies are vendored in `vendor/`; run `make vendor` after changing `go.mod`

## Conventions

- Use `go build -mod=vendor` (the Makefile handles this)
- Do not modify vendored code directly
- Integration tests are bats scripts in `test/`; follow existing patterns in `test/simple.bats`
- CLI flags are defined as struct tags on `cliArgs` in `vgrep.go` using `github.com/jessevdk/go-flags`

## Commits

- Keep commits small and incremental; one logical change per commit
- Always use the `-s` flag to sign off commits (`git commit -s`)
