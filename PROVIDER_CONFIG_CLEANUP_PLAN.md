# Provider And Config Cleanup Plan

This note tracks the next structural cleanup pass for `internal/provider` and
`internal/config`.

The goal is to reduce coordination pressure in provider wiring, make the CLI transport path
more local, and keep runtime/config helpers aligned with those same boundaries.

## Phase 1: Split Provider Core And CLI Transport

Goals:

- Reduce the size and mixed responsibilities of `internal/provider/types.go`.
- Keep core provider types and model resolution separate from CLI transport behavior.

Scope:

- Keep shared provider request/interface types in one file.
- Move provider resolution/build helpers into a focused file.
- Move CLI transport implementation into its own file.
- Preserve behavior for API and CLI transports.

Acceptance:

- Existing provider and CLI tests continue to pass.
- `internal/provider/types.go` becomes a small shared-types file.

## Phase 2: Split Config Runtime Helpers By Concern

Goals:

- Keep config runtime/view helpers aligned with actual domains.
- Reduce the amount of unrelated runtime logic in one file.

Scope:

- Separate provider runtime helpers from shell/ingest/log-analysis helpers.
- Preserve the public `config.Config` API used by callers.

Acceptance:

- Existing config, app, and CLI tests continue to pass.
- Runtime helper files line up with provider and shell concerns.

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
  - split provider core types, model resolution/build logic, and CLI transport adapter into focused files
- Phase 2: completed
  - split config runtime helpers into provider runtime and general runtime files
