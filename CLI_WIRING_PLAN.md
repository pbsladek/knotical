# CLI Wiring Cleanup Plan

This note tracks a focused cleanup pass for the remaining CLI/app wiring pressure points.

The goal is to keep Cobra handlers thin, reduce large coordination files, and make option
validation easier to maintain without changing user-visible behavior.

## Phase 1: Split Root Command Wiring

Goals:

- Reduce the size and coordination burden of `internal/cli/root.go`.
- Keep root command behavior unchanged while separating responsibilities.

Scope:

- Keep root option types and root command creation.
- Move flag registration into a dedicated file.
- Move run-path helpers into a dedicated file.
- Move validation and normalization into a dedicated file.

Acceptance:

- Root command behavior and flags remain unchanged.
- `internal/cli/root.go` becomes a small entrypoint file.
- Existing root tests continue to pass without behavior changes.

## Phase 2: Split Models Command Surface

Goals:

- Keep `models` Cobra wiring thinner and move list/default/info paths into smaller focused
  files.

Scope:

- Separate `models list` command construction and rendering helpers.
- Separate `models default` and `models info`.
- Keep `internal/modelcatalog` as the discovery/cache layer.

Acceptance:

- `models list`, `models default`, and `models info` behavior remain unchanged.
- `internal/cli/commands_models.go` becomes a small command entrypoint.

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
  - split root command wiring into `root.go`, `root_flags.go`, `root_request.go`, and `root_validate.go`
- Phase 2: completed
  - split `models` command wiring into entrypoint, list, and config/info files
