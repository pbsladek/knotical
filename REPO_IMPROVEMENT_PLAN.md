# Repo Improvement Plan

This note tracks the next cleanup pass for `knotical`. The goal is to improve what already
exists, with an emphasis on simplicity, correctness, and maintainability rather than broad new
feature work.

## Phase 1: Shell Hardening

Goals:

- Tighten `internal/shell` parsing behavior around tricky argument cases.
- Add focused tests for safe execution parsing and policy edges.

Scope:

- Preserve current user-visible shell modes: `host`, `safe`, `sandbox`
- Improve handling of quoted empty args and escaped whitespace
- Add tests for parser edge cases and safe-command validation

Acceptance:

- `internal/shell` coverage increases
- No shell behavior regression in existing tests

## Phase 2: Split Config And Logs Command Files

Goals:

- Reduce file-level maintenance hotspots without changing behavior.

Scope:

- Split `internal/config/config.go` into smaller files by concern:
  - core/load/save/paths
  - env overrides
  - validation
  - runtime/view helpers
- Split `internal/cli/commands_logs.go` into:
  - root/query command
  - subcommands
  - status/backup helpers

Acceptance:

- External config shape and command behavior remain unchanged
- File sizes are materially reduced

## Phase 3: Extract Model Discovery From Cobra

Goals:

- Keep Cobra handlers thin and move `models list` discovery/cache behavior into a reusable
  package-level service.

Scope:

- Extract provider iteration, filtering, warnings, and cache handling out of
  `internal/cli/commands_models.go`
- Keep CLI-only rendering concerns in the command layer

Acceptance:

- `models list` behavior stays the same
- Core discovery logic is testable without Cobra

## Phase 4: Split Resolve Logic

Goals:

- Break `internal/app/resolve.go` into smaller focused files.

Scope:

- Session helpers
- request/model/provider resolution
- defaults
- provider construction
- shared request-to-provider helpers

Acceptance:

- No behavior change
- Smaller files and narrower responsibility per file

## Phase 5: Split Oversized Test Files

Goals:

- Improve readability and maintenance of the largest test files.

Scope:

- Split `internal/app/service_test.go` by feature area
- Split `internal/store/files_test.go` by storage area
- Reuse existing helpers where practical

Acceptance:

- Test behavior unchanged
- Failures are easier to localize by file

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
  - hardened shell parsing around quoted empty args and escape handling
  - added focused shell parser tests
- Phase 2: completed
  - split config into core/env/runtime/validate files
  - split logs command into query/subcommand/helper files
- Phase 3: completed
  - extracted `models list` discovery and cache logic into `internal/modelcatalog`
- Phase 4: completed
  - replaced `internal/app/resolve.go` with focused resolve files by concern
- Phase 5: completed
  - split `internal/app/service_test.go` and `internal/store/files_test.go` by feature area
