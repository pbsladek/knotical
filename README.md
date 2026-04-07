# knotical

`knotical` is a terminal-first CLI for working with large language models.
It combines direct prompting, shell and code generation workflows, persistent chat sessions,
reusable prompt assets, local caching, and SQLite logging in a single binary.

## Features

- Providers: OpenAI, Anthropic, Gemini, and Ollama
- Prompt input from arguments, stdin, or `$EDITOR`
- Streaming and non-streaming responses
- Shell mode, shell explanation mode, code mode, and fenced-code extraction
- Named chats and REPL sessions
- Roles, templates, fragments, aliases, and stored API keys
- Structured JSON output with `--schema`
- SQLite prompt and response logging
- Request caching
- Shell execution modes: `host`, `safe`, and `sandbox`

## Install

Build from source:

```bash
make build
```

This writes the binary to `dist/knotical`.

Run directly from the repository:

```bash
make run CMD='"Explain ownership in one paragraph"'
```

For a full command reference with end-to-end combinations, see
[CLI_REFERENCE.md](/Users/pbsladek/Code/pbsladek/rust/knotical/CLI_REFERENCE.md).

## Configuration

`knotical` stores its config and local state in the standard user config directory under
`knotical/` and supports environment variable overrides with the `KNOTICAL_` prefix.

Typical provider credentials:

```bash
export OPENAI_API_KEY=...
export ANTHROPIC_API_KEY=...
export GEMINI_API_KEY=...
```

Keys can also be stored through the CLI:

```bash
knotical keys set openai
```

The config file is `config.toml` in the user config directory. It can control provider transport
and shell defaults as well as the core model settings.

Useful config commands:

```bash
knotical config generate
knotical config show
knotical config path
knotical config edit
```

Example:

```toml
default_model = "claude-sonnet-4-5"
default_provider = "anthropic"
anthropic_transport = "cli"
openai_transport = "api"
gemini_transport = "cli"
default_log_profile = "k8s"

shell_execute_mode = "sandbox"
shell_sandbox_runtime = "podman"
shell_sandbox_image = "docker.io/library/ubuntu:24.04"
shell_sandbox_network = false
shell_sandbox_write = false
default_log_profile = "k8s"

claude_cli_command = "claude"
claude_cli_args = ["-p", "--output-format", "text"]

codex_cli_command = "codex"
codex_cli_args = ["exec"]

gemini_cli_command = "gemini"
gemini_cli_args = ["-p"]
```

Transport values for OpenAI, Anthropic, and Gemini are:

- `api`
- `cli`

This lets you keep using the same model names while choosing whether a provider is reached through
its API integration or through a local authenticated CLI such as Claude Code, Codex CLI, or Gemini
CLI.

The defaults are intentionally conservative:

- Claude CLI uses native `model`, `system`, and `json-schema` flags by default
- Codex CLI defaults to `codex exec` and falls back to prompt injection for system and schema
- Gemini CLI defaults to `gemini -p` and falls back to prompt injection for system and schema

`knotical models list` is only supported for providers that expose model listing through the active
transport. CLI transports generally do not.

Model support policy:

- execution accepts arbitrary model strings
- `models list` is discovery only, not a validation boundary
- use `--provider <name>` when the model name alone is ambiguous
- `provider/model` syntax is also supported, for example `anthropic/claude-sonnet-4-5`
- `models list --provider <name>` filters discovery to one provider
- `models list --json` emits machine-readable output
- `models list --refresh` bypasses the short discovery cache

## Usage

Basic prompt:

```bash
knotical "Summarize the purpose of this repository"
```

Explicit provider routing:

```bash
knotical --provider anthropic --model claude-sonnet-4-5 "Review this API design"
```

Provider-prefixed model syntax:

```bash
knotical --model openai/gpt-4o-mini "Summarize this diff"
```

Read prompt content from stdin:

```bash
git diff --staged | knotical "Review this patch for bugs"
```

Read stdin without an explicit instruction:

```bash
journalctl -u nginx -n 200 | knotical
```

Control stdin composition explicitly:

```bash
kubectl logs deploy/api | knotical --stdin-mode append --stdin-label logs "Find the likely root cause"
```

Apply cheap deterministic reduction before sending large stdin:

```bash
kubectl logs deploy/api | knotical --stdin-label logs --tail-lines 400 --max-input-lines 200 "Summarize the current failure"
```

Apply an approximate token budget:

```bash
journalctl -u nginx -n 2000 | knotical --stdin-label logs --max-input-tokens 1500 --input-reduction truncate "Summarize the current failure"
```

Use multi-pass summarization when the reduced input is still too large:

```bash
journalctl -u nginx -n 20000 | knotical --analyze-logs --max-input-tokens 4000 --input-reduction summarize --summarize-chunk-tokens 800
```

Use the dedicated log-analysis mode:

```bash
kubectl logs deploy/api | knotical -a -p k8s --tail-lines 400 "Find the likely root cause"
```

Use built-in log cleanup shorthands:

```bash
kubectl logs deploy/api | knotical -a --clean --unique --tail 400 "Summarize the incident"
```

Use raw ingest transforms for advanced filtering:

```bash
kubectl logs deploy/api | knotical -a --transform include-regex:'(?i)(error|warn|panic)' --transform dedupe-normalized --max-input-tokens 4000
```

Code-only output:

```bash
knotical --code "Write a Go function that parses a CSV line"
```

Structured JSON output:

```bash
knotical --schema "name, age:int, active:bool" "Generate a fake user"
```

Named chat session:

```bash
knotical --chat release-notes "Draft release notes from these commits"
```

Start a REPL session:

```bash
knotical --repl scratch
```

Reusable fragment injection:

```bash
knotical fragments set readme "$(cat README.md)"
knotical --fragment readme "Summarize this project"
```

## Shell Workflows

Generate a shell command:

```bash
knotical --shell "find the 10 largest files in the current directory"
```

Explain a shell command:

```bash
knotical --describe-shell "find . -type f -size +100M"
```

Execute a generated command on the host:

```bash
knotical --shell --execute host "show git status"
```

Execute in constrained safe mode:

```bash
knotical --shell --safe "list tracked files"
```

Execute in a Docker or Podman sandbox:

```bash
knotical --shell --sandbox --docker "search for TODO comments"
```

Sandbox defaults:

- image: `docker.io/library/ubuntu:24.04`
- Linux `sh` execution environment
- read-only container filesystem
- read-only mounted workspace
- network disabled unless `--sandbox-network` is set

Shell defaults can also be set in `config.toml` using:

- `shell_execute_mode`
- `shell_sandbox_runtime`
- `shell_sandbox_image`
- `shell_sandbox_network`
- `shell_sandbox_write`

Input reduction defaults can be set in `config.toml` using:

- `default_log_profile`
- `max_input_bytes`
- `max_input_lines`
- `max_input_tokens`
- `input_reduction_mode`
- `summarize_chunk_tokens`
- `summarize_chunk_overlap_lines`
- `summarize_intermediate_model`
- `default_head_lines`
- `default_tail_lines`
- `default_sample_lines`

Log-analysis defaults can be set in `config.toml` using:

- `default_log_profile`
- `log_analysis_markdown`
- `log_analysis_schema`
- `log_analysis_system_prompt`

## Common Commands

```bash
knotical --help
knotical models list
knotical models list --provider openai --json
knotical logs --help
knotical roles list
knotical templates list
knotical fragments list
```

## Development

Format and verify:

```bash
make verify
```

Other useful targets:

```bash
make fmt
make test
make test-cover
```

## License

MIT

## Acknowledgements

`knotical` is inspired by:

- [`simonw/llm`](https://github.com/simonw/llm)
- [`TheR1D/shell_gpt`](https://github.com/TheR1D/shell_gpt)
