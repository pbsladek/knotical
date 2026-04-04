# Feature Comparison

This file tracks the feature set `knotical` intends to keep from the two projects it is
inspired by:

- [`simonw/llm`](https://github.com/simonw/llm)
- [`TheR1D/shell_gpt`](https://github.com/TheR1D/shell_gpt)

It is a checklist for the active Go implementation, not a historical summary.

## Status legend

- `complete`: implemented and present in the Go CLI
- `partial`: present, but behavior is weaker or narrower than the intended feature
- `deferred`: intentionally not in the current target scope

## Target scope

`knotical` is meant to cover the practical terminal workflows we want from both tools:

- direct prompt execution
- shell and code generation workflows
- reusable prompt assets
- persistent chats and REPL use
- local logging and caching
- multiple providers
- structured output

The goal is not full parity with every advanced or provider-specific feature in either upstream
tool. Features marked `deferred` are explicitly outside the current target.

## Core prompt workflows

| Feature | Source inspiration | Status | Notes |
|--------|---------------------|--------|-------|
| Prompt from args | both | `complete` | `knotical "prompt"` |
| Prompt from stdin | both | `complete` | Reads piped input |
| Prompt from `$EDITOR` | `llm` | `complete` | `--editor` |
| System prompt override | both | `complete` | `--system` |
| Model selection | both | `complete` | `--model` plus aliases |
| Streaming output | both | `complete` | Default when provider path supports it |
| Disable streaming | both | `complete` | `--no-stream` |
| Temperature / top-p | `llm` | `complete` | `--temperature`, `--top-p` |
| Extract fenced code block | `shell_gpt` | `complete` | `--extract` |
| Save current prompt settings as template | `llm` | `complete` | `--save <name>` |

## Shell and code workflows

| Feature | Source inspiration | Status | Notes |
|--------|---------------------|--------|-------|
| Shell command generation | `shell_gpt` | `complete` | `--shell` |
| Interactive shell action prompt | `shell_gpt` | `complete` | Execute or describe generated command |
| Dedicated shell explanation mode | `shell_gpt` | `complete` | `--describe-shell` now uses a dedicated prompt path for shell-command explanation |
| Code-only output mode | `shell_gpt` | `complete` | `--code` |
| Shell integration install | `shell_gpt` | `complete` | `install-integration` for `zsh` and `bash` |

## Conversation workflows

| Feature | Source inspiration | Status | Notes |
|--------|---------------------|--------|-------|
| Named chat sessions | `llm` | `complete` | `--chat <name>` |
| Continue previous session | both | `complete` | `--continue` resumes last saved session |
| Interactive REPL | `llm` | `complete` | `--repl <name>` |
| Chat inspection and deletion | `llm` | `complete` | `chats list/show/delete` |
| Conversation-by-ID resume | `llm` | `deferred` | Named sessions are the current model |

## Reusable prompt assets

| Feature | Source inspiration | Status | Notes |
|--------|---------------------|--------|-------|
| Roles | `llm` | `complete` | `roles list/show/create/delete` plus built-ins |
| Templates | `llm` | `complete` | `templates list/show/create/edit/delete` |
| Fragments | `llm` | `complete` | `fragments set/get/list/delete`, `--fragment` |
| Model aliases | `llm` | `complete` | `aliases set/remove/list` |
| Template variable substitution | `llm` | `deferred` | Not currently implemented |
| Templates with schema/tools binding | `llm` | `deferred` | Not currently implemented |

## Structured output and formatting

| Feature | Source inspiration | Status | Notes |
|--------|---------------------|--------|-------|
| Structured JSON output | `llm` | `complete` | `--schema` with DSL or JSON schema file |
| Response validation | `llm` | `complete` | Basic schema validation |
| Pretty-printed JSON output | `llm` | `complete` | Applied after validation |
| Markdown prettification | `llm` | `complete` | Markdown-aware terminal rendering is applied when enabled; `--no-md` disables it |

## Configuration, logging, and persistence

| Feature | Source inspiration | Status | Notes |
|--------|---------------------|--------|-------|
| Stored API keys | both | `complete` | `keys set/get/remove/list/path` |
| Inline key override flag | `llm` | `deferred` | No `--key` flag |
| SQLite logs | `llm` | `complete` | Prompt/response logging in SQLite |
| Log browsing | `llm` | `complete` | `logs`, `logs show`, `logs clear` |
| Logging on/off toggle | `llm` | `complete` | `logs on`, `logs off` |
| Request/response cache | `llm` | `complete` | Disk-backed cache |
| Config file and env overrides | both | `complete` | `KNOTICAL_` env prefix |
| Per-model default options | `llm` | `deferred` | Not currently implemented |

## Providers and model management

| Feature | Source inspiration | Status | Notes |
|--------|---------------------|--------|-------|
| OpenAI provider | both | `complete` | Official Go SDK |
| Anthropic provider | both | `complete` | Official Go SDK |
| Gemini provider | both | `complete` | Official Go SDK |
| Ollama provider | both | `complete` | OpenAI-compatible path |
| Model inspection | `llm` | `complete` | `models info`, `models default` |
| Model listing | `llm` | `complete` | `models list` uses provider-backed discovery for the supported providers |
| Additional providers beyond current set | `llm` | `deferred` | Groq, Mistral, LiteLLM-style expansion not currently targeted |

## Explicitly deferred features

These are known features from one or both inspiration tools that are not part of the current
target:

- multimodal attachments
- function or tool calling
- embeddings and similarity search
- plugin or extension system
- Datasette integration
- Docker packaging
- provider-wide compatibility matrix beyond the current four providers

## Current gap list

No known implementation gaps remain for the currently chosen scope.

If scope expands again, the most likely next additions are the deferred items above rather than
unfinished work in the current feature set.
