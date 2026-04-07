# knotical Project Context

## Overview

`knotical` is a Go CLI for working with large language models from the terminal.
It combines direct prompting, shell and code generation workflows, persistent chat sessions,
reusable prompt assets, local request caching, and SQLite logging in a single binary.

The project is inspired by:

- [`simonw/llm`](https://github.com/simonw/llm)
- [`TheR1D/shell_gpt`](https://github.com/TheR1D/shell_gpt)

## Current Status

The Go implementation is the only active codebase.
The earlier Rust implementation has been retired.

Implemented features include:

- Prompt input from arguments, stdin, or `$EDITOR`
- Streaming and non-streaming responses
- Shell mode, describe-shell mode, code mode, and fenced-code extraction
- Named chat sessions and REPL mode
- Roles, templates, fragments, aliases, keys, and logs subcommands
- Structured JSON output with `--schema`
- Providers: OpenAI, Anthropic, Gemini, Ollama
- Configurable provider transport for Anthropic, OpenAI, and Gemini: API or local CLI
- Request caching
- SQLite logging and log search
- Shell safety modes: `host`, `safe`, and `sandbox`
- Docker/Podman-backed sandbox shell execution

## Configuration

User state lives under the standard user config directory in `knotical/`.
The environment variable prefix is `KNOTICAL_`.

Important persisted files and directories:

- `config.toml`
- `keys.json`
- `logs.db`
- `chat_cache/`
- `templates/`
- `roles/`
- `fragments/`
- `cache/`

Provider API keys are typically supplied via environment variables such as:

- `OPENAI_API_KEY`
- `ANTHROPIC_API_KEY`
- `GEMINI_API_KEY`

Provider-specific base URL overrides are supported through config and `KNOTICAL_*` env vars.
Anthropic, OpenAI, and Gemini can also be configured to use local CLI transports instead of API
transports.

## Architecture

The codebase is organized around a thin CLI layer and an application service layer.

- `cmd/knotical`: binary entrypoint
- `internal/cli`: Cobra commands and flag parsing
- `internal/app`: orchestration for prompt and REPL execution
- `internal/provider`: provider integrations
- `internal/store`: filesystem and SQLite persistence
- `internal/schema`: schema parsing and validation
- `internal/shell`: shell prompting, safety analysis, and execution
- `internal/output`: terminal rendering
- `internal/config`: config loading and path helpers

The main rule is: keep `internal/cli` thin and push behavior into `internal/app` or lower-level
packages so it stays testable.

## Shell Execution Model

Shell mode generates commands but does not execute them unless explicitly requested.

Supported execution modes:

- `host`: run on the local machine
- `safe`: run only a constrained allowlist of read-only commands without shell parsing
- `sandbox`: run in Docker or Podman inside a Linux container

Sandbox execution defaults to:

- image: `docker.io/library/ubuntu:24.04`
- read-only container filesystem
- read-only mounted workspace
- no network unless explicitly enabled

These defaults can be overridden in `config.toml` with shell-specific settings such as runtime,
image, execute mode, network, and write access.

Interactive shell execution may regenerate commands for sandbox execution so the generated command
matches the Linux `sh` container environment.

## Logging

SQLite logging is enabled by default and powers `knotical logs`.
Logged data includes prompt, response, model, provider, token counts, system prompt, conversation,
schema metadata, and fragment metadata.

Important commands:

- `knotical logs`
- `knotical logs status`
- `knotical logs backup`
- `knotical logs show <id>`

Per-invocation logging overrides:

- `--log`
- `--no-log`

## Verification

Use these commands for normal verification:

```bash
go test ./...
go build ./...
```

When editing Go files, run `gofmt -w` on touched files.

## Contributor Notes

- Prefer small, testable functions and explicit dependencies.
- Add or update tests with behavior changes.
- Keep docs aligned with the actual CLI surface.
- Treat shell execution, logs, and config changes as security-sensitive areas.
