# AGENTS.md

## Purpose

This file gives coding agents the minimum repo-specific guidance needed to work safely and
accurately in `knotical`.

Read [README.md](/Users/pbsladek/Code/pbsladek/rust/knotical/README.md) for user-facing behavior and
[CLAUDE.md](/Users/pbsladek/Code/pbsladek/rust/knotical/CLAUDE.md) for project context before making
substantial changes.

## Ground Truth

- The active implementation is Go.
- The Rust implementation no longer exists in this repo.
- The binary entrypoint is `cmd/knotical`.
- The module path is `github.com/pbsladek/knotical`.

## Working Rules

- Keep Cobra command handlers thin.
- Put orchestration in `internal/app`.
- Put provider-specific logic in `internal/provider`.
- Put persistence in `internal/store`.
- Write or update tests with every behavior change.
- Run `gofmt -w` on touched Go files.
- Verify with:

```bash
go test ./...
go build ./...
```

## Areas That Need Extra Care

### Shell execution

Shell-related changes affect real command execution and are security-sensitive.

Current modes:

- `host`
- `safe`
- `sandbox`

Expectations:

- `safe` must remain meaningfully constrained
- `sandbox` must target Linux `sh`
- host and sandbox execution behavior must match the user-visible prompting model

Relevant package:

- `internal/shell`

### Logging

Logs are stored in SQLite and are user-visible through `knotical logs`.
Changes here affect search, metadata retention, backups, and privacy posture.

Relevant packages:

- `internal/store`
- `internal/cli`
- `internal/app`

### Config and local storage

Config and stored assets are written under the user config directory and must keep secure
permissions and path validation intact.

Relevant packages:

- `internal/config`
- `internal/store`

## Package Guide

- `internal/app`: prompt flow, REPL flow, resolution of model/system/template/role/fragment state
- `internal/cli`: Cobra commands, flag normalization, input helpers
- `internal/provider`: OpenAI, Anthropic, Gemini, and Ollama integrations
- `internal/schema`: schema DSL parsing and JSON validation
- `internal/store`: chats, templates, roles, fragments, cache, keys, logs
- `internal/output`: terminal rendering and sanitization
- `internal/shell`: shell prompt generation, risk analysis, and execution backends

## Testing Expectations

Prefer tests close to the changed behavior:

- CLI wiring: `internal/cli/*_test.go`
- app orchestration: `internal/app/*_test.go`
- provider request/response behavior: `internal/provider/*_test.go`
- persistence and migration behavior: `internal/store/*_test.go`
- shell parsing and execution policy: `internal/shell/*_test.go`

Do not rely only on broad integration coverage when a focused unit test is practical.

## Documentation Expectations

If a change affects command behavior, flags, shell safety, logging, configuration, or provider
support, update the relevant docs in the same change:

- [README.md](/Users/pbsladek/Code/pbsladek/rust/knotical/README.md)
- [CLAUDE.md](/Users/pbsladek/Code/pbsladek/rust/knotical/CLAUDE.md)
- command help text if needed
