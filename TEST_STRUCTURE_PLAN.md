# Test Structure Plan

This note tracks the next cleanup pass for the remaining large test files in `knotical`.

The goal is to make tests line up more closely with the production package splits so failures
localize cleanly and test files are easier to scan.

## Phase 1: Split Config Tests By Concern

Goals:

- Reduce the size of `internal/config/config_test.go`.
- Group tests by defaults/env/runtime/validation/persistence concerns.

Scope:

- Keep test behavior unchanged.
- Split config tests into multiple files that match the production config file layout.

Acceptance:

- Existing config tests continue to pass unchanged.
- No single config test file remains a catch-all.

Status: completed

Result:

- `internal/config/config_test.go` was split into focused files for defaults, env, provider,
  validation, and persistence behavior.
- Config test coverage and assertions were preserved while reducing file-level sprawl.

## Phase 2: Split Large CLI Test Files By Behavior Area

Goals:

- Reduce the size of `internal/cli/root_test.go` and `internal/cli/commands_misc_test.go`.
- Group tests by root validation, prompt flow, config commands, shell integration, and misc
  command surfaces.

Scope:

- Keep existing test helpers where practical.
- Move tests into more focused files without changing assertions.

Acceptance:

- Existing CLI tests continue to pass unchanged.
- The biggest CLI test files are materially smaller.

Status: completed

Result:

- `internal/cli/root_test.go` was split into focused root prompt, validation, and command test
  files.
- `internal/cli/commands_misc_test.go` was split into focused config, storage, integration, and
  roles/templates test files.
- Shared helpers were kept in small anchor files so the package-level test API stayed simple.

## Final Verification

Required checks:

```bash
gofmt -w ...
go test ./...
go build ./...
go vet ./...
```

Status: completed

Result:

- `gofmt -w internal/cli/*test.go internal/config/*test.go`
- `go test ./internal/cli -count=1`
- `go test ./...`
- `go build ./...`
- `go vet ./...`
