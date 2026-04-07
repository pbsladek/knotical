# Logs And App Cleanup Plan

This note tracks the next structural cleanup pass in `knotical`.

The goal is to keep command wiring thin and further reduce orchestration density in
`internal/app` without changing user-visible behavior.

## Phase 1: Split Logs Command Query Wiring

Goals:

- Keep `knotical logs` command construction small.
- Separate store wiring and query execution from Cobra setup.

Scope:

- Move log-store construction and query execution into focused helpers.
- Keep flag registration and subcommand wiring unchanged.
- Preserve current rendering behavior and query semantics.

Acceptance:

- `internal/cli/commands_logs.go` becomes a small entrypoint.
- Existing log command tests continue to pass unchanged.

## Phase 2: Split Prompt Flow

Goals:

- Reduce the coordination burden in `internal/app/prompt_flow.go`.
- Separate prompt preparation, provider request execution, and run-context assembly.

Scope:

- Move prompt preparation steps into dedicated helpers/files.
- Move execution request construction/cache lookup helpers into a focused file.
- Preserve prompt behavior, schema handling, ingest behavior, and summarization flow.

Acceptance:

- `internal/app/prompt_flow.go` becomes materially smaller.
- Existing app and CLI prompt tests continue to pass unchanged.

## Final Verification

Required checks:

```bash
gofmt -w ...
go test ./...
go build ./...
go vet ./...
```

## Status

- Phase 1: completed
  - split `logs` command query execution and store wiring into focused files
- Phase 2: completed
  - split prompt flow into preparation and request execution helpers
