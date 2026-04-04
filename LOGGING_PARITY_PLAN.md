# Logging Parity Plan

This file tracks incremental work to close the highest-value logging gaps between
`knotical` and `llm`.

## Scope

The goal is not to clone `llm` exactly. The goal is to close the most useful gaps
in phases while keeping the code testable and the CLI stable.

## Baseline

Already implemented:

- SQLite-backed prompt logs in `logs.db`
- `logs`, `logs show`, `logs clear`, `logs on`, `logs off`, `logs path`
- Prompt/response/model/system prompt/token usage storage
- Basic model filter and substring search

Known gaps from the review:

- No per-invocation `--log` / `--no-log`
- No `logs status`
- No JSON / short / response-only / extract views
- No conversation-oriented log browsing
- No incremental ID filters
- No FTS-backed search or relevance ordering
- No richer log schema for schemas, fragments, attachments, tools
- No backup command

## Phases

### Phase 1: Logging Controls And Status

Goal:

- Add per-invocation `--log` and `--no-log`
- Apply the log decision consistently in prompt execution and REPL turns
- Add `logs status`
- Add tests for log override behavior and status reporting

Acceptance:

- `knotical --no-log "prompt"` skips logging even if logging is enabled globally
- `knotical --log "prompt"` logs even if logging is disabled globally
- `knotical logs status` shows on/off state, DB path, response count, conversation count, and file size

### Phase 2: Output Modes

Goal:

- Add `logs --json`
- Add `logs --response`
- Add `logs --extract` and `logs --extract-last`
- Add `logs --short`
- Add tests for each rendering mode

### Phase 3: Query And Filter Improvements

Goal:

- Add conversation filters for latest and specific conversation IDs
- Add latest-vs-search sorting controls
- Add ID range filters
- Add tests for query semantics

### Phase 4: Search And Storage Improvements

Goal:

- Add SQLite FTS for prompt/response search
- Improve the schema to support richer future log features
- Add migration tests and query tests

### Phase 5: Backup And Follow-On Integrations

Goal:

- Add `logs backup`
- Reassess follow-on parity items such as schema-aware and fragment-aware log browsing

## Status

- [x] Plan written
- [x] Phase 1 complete
- [x] Phase 2 complete
- [x] Phase 3 complete
- [x] Phase 4 complete
- [x] Phase 5 complete

## Verification

Run after each phase:

```bash
go test ./...
go build ./...
```
