# AGENTS

Repository rules for coding agents working in `minos`:

- Never push commits, branches, or tags to any remote. Local commits are allowed only when explicitly requested.
- Do not make or leave code changes unless repository-wide test coverage remains at or above `80%`.
- Do not create commits unless `gofmt -l .` returns no files.
- Before committing, ensure `go test ./...` passes.
- Before committing, ensure `golangci-lint run ./...` passes.
