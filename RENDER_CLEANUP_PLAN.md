# Render Cleanup Plan

This note tracks the next structural cleanup pass for the remaining dense rendering files in
`knotical`.

The goal is to reduce coordination pressure in user-facing rendering code without changing
behavior.

## Phase 1: Split Logs Rendering

Goals:

- Reduce the size of `internal/cli/commands_logs_render.go`.
- Separate output-mode selection from detailed rendering helpers.

Scope:

- Keep log render behavior unchanged.
- Split:
  - output mode dispatch
  - default/short rendering
  - code block extraction
  - reduction metadata formatting

Acceptance:

- Existing log rendering tests continue to pass.
- `commands_logs_render.go` becomes a small dispatch file.

## Phase 2: Split Prompt Rendering

Goals:

- Reduce the size and responsibility concentration of `internal/app/prompt_render.go`.
- Separate prompt rendering, shell interaction flow, and shell execution policy helpers.

Scope:

- Keep prompt and shell behavior unchanged.
- Split:
  - general prompt response rendering
  - shell interaction flow
  - shell execution request/policy helpers

Acceptance:

- Existing app and shell-related tests continue to pass.
- `prompt_render.go` becomes a smaller entrypoint.

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
  - split logs rendering into dispatch, entry rendering, extraction, and reduction formatting files
- Phase 2: completed
  - split prompt rendering into prompt dispatch, shell flow, and shell execution policy files
