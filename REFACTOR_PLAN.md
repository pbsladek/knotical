# Refactor Plan

This file tracks the Go architecture refactor for `knotical`.

## Goals

- Reduce orchestration complexity in the CLI.
- Improve unit-testability by introducing explicit seams.
- Remove provider drift between streaming and non-streaming paths.
- Make storage and output layers easier to reason about and evolve.
- Keep the CLI surface and observable behavior stable while refactoring.

## Baseline

- Largest files:
  - `internal/cli/root.go`: 1157 lines
  - `internal/provider/provider.go`: 637 lines
  - `internal/store/files.go`: 374 lines
- Cyclomatic complexity hotspots:
  - `runPrompt`: 58
  - `runRepl`: 30
  - `resolveModelAndSystem`: 18
- Coverage:
  - `internal/cli`: 23.5%
  - `internal/provider`: 69.0%
  - `internal/store`: 47.5%
  - `internal/schema`: 56.4%
  - `internal/output`: 50.0%
  - `internal/shell`: 14.8%
  - `internal/config`: 0.0%

## Phases

### Phase 1: Application Layer Extraction

Create `internal/app` and move prompt/REPL orchestration behind a service layer.

Target:
- `runPrompt` complexity < 12
- `runRepl` complexity < 10
- `internal/cli` becomes a thin Cobra adapter

### Phase 2: Explicit Dependencies

Introduce small interfaces for provider creation, storage, config loading, and rendering.

Target:
- core prompt flow unit-testable without filesystem + SQLite + HTTP in one test
- `internal/cli` or `internal/app` coverage > 50%

### Phase 3: Provider Refactor

Split provider code into per-provider files and centralize request/response mapping for
streaming and non-streaming paths.

Target:
- remove duplicated request construction
- no provider file > 250 lines

### Phase 4: Store Refactor

Split `internal/store/files.go` by concern and improve SQLite log store lifecycle.

Target:
- `LogStore` reuses a database handle
- shared row mapping for `Get` and `Query`
- `internal/store` coverage > 65%

### Phase 5: Injectable Output

Replace package-global printing with an `io.Writer`-backed renderer abstraction.

Target:
- deterministic output tests
- core orchestration not coupled to global stdout

### Phase 6: Focused Test Expansion

Add tests for config, application orchestration, provider request builders, and store seams.

Target:
- `internal/config` coverage > 70%
- no new behavior regressions

## Status

- [x] Plan written
- [x] Phase 1 complete
- [x] Phase 2 complete
- [x] Phase 3 complete
- [x] Phase 4 complete
- [x] Phase 5 complete
- [x] Phase 6 complete

## Results

- CLI orchestration now lives in `internal/app`, and `internal/cli/root.go` is down to 127 lines.
- The old monolithic provider implementation was split into per-provider files, each under 250 lines.
- The old multi-purpose store file was split by concern, and `LogStore` now reuses a database handle with shared row scanning.
- Output is injectable through `output.Printer`, which allows deterministic tests without global stdout coupling.
- Focused tests were added for config, app orchestration, request resolution, and store behavior.

Measured results:

- File sizes:
  - `internal/cli/root.go`: 127 lines
  - `internal/app/service.go`: 166 lines
  - `internal/app/prompt.go`: 313 lines
  - `internal/app/resolve.go`: 234 lines
- Coverage:
  - `internal/app`: 61.7%
  - `internal/config`: 73.5%
  - `internal/store`: 77.6%
  - `internal/provider`: 72.7%
- Cyclomatic hotspots still over 10:
  - `app.(*Service).executePromptFlow`: 11
  - Provider/schema/test helpers remain above 10, but the former CLI orchestration hotspots no longer do.

## Verification

Run after each meaningful phase:

```bash
go test ./...
go build ./...
```

Final verification:

```bash
go test ./...
go build ./...
go test -cover ./...
gocyclo -over 10 cmd internal
```
