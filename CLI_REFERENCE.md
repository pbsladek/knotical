# knotical CLI Reference

This document is a practical reference for the full `knotical` command surface.
It focuses on how the CLI is actually used, how flags combine, and what each command does.

## Command Shape

The main entrypoint is:

```bash
knotical [PROMPT] [flags]
```

You can also use subcommands:

```bash
knotical [command]
```

Top-level commands:

- `aliases`
- `chats`
- `completion`
- `config`
- `fragments`
- `install-integration`
- `keys`
- `logs`
- `models`
- `roles`
- `templates`

## Core Prompt Flags

These flags apply to the main prompt command:

- `-m, --model <model>`
- `-S, --system <text>`
- `--fragment <name>` repeatable
- `-a, --analyze-logs`
- `-s, --shell`
- `-d, --describe-shell`
- `-c, --code`
- `--no-md`
- `--chat <name>`
- `--repl <name>`
- `-p, --profile <name>`
- `--role <name>`
- `-t, --template <name>`
- `--temperature <float>`
- `--schema <dsl-or-json-file>`
- `--top-p <float>`
- `--cache`
- `--editor`
- `--stdin-mode auto|append|replace`
- `--stdin-label <label>`
- `--transform <name[:arg]>` repeatable
- `--no-pipeline`
- `--clean`
- `--dedupe`
- `--unique`
- `--k8s`
- `--max-input-bytes <n>`
- `--max-input-lines <n>`
- `--max-input-tokens <n>`
- `--input-reduction off|truncate|fail|summarize`
- `--summarize-chunk-tokens <n>`
- `--summarize-intermediate-model <model>`
- `--head-lines <n>`
- `--tail-lines <n>`
- `--tail <n>`
- `--sample-lines <n>`
- `--interaction`
- `--continue`
- `--no-stream`
- `-x, --extract`
- `--save <template-name>`
- `--log`
- `--no-log`

Shell execution flags:

- `--execute host|safe|sandbox`
- `--host`
- `--safe`
- `--sandbox`
- `--force-risky-shell`
- `--sandbox-runtime docker|podman`
- `--docker`
- `--podman`
- `--sandbox-image <image>`
- `--img <image>`
- `--sandbox-network`
- `--net`
- `--sandbox-write`
- `--rw`

Important constraints:

- `--execute` requires `--shell`
- `--force-risky-shell` requires `--execute host`
- sandbox options require `--shell`
- sandbox options require `--execute sandbox` if execution mode is explicitly set
- `--log` and `--no-log` cannot be used together
- `--analyze-logs` cannot be combined with `--shell`, `--code`, or `--describe-shell`
- `--profile` requires `--analyze-logs`

## Prompt Input Modes

### Prompt as an argument

```bash
knotical "Summarize the purpose of this repository"
```

### Prompt from stdin

```bash
git diff --staged | knotical "Review this patch for bugs"
```

Behavior:

- if both prompt text and stdin are present, stdin is appended by default
- if only stdin is present, stdin becomes the prompt body
- `--editor` cannot be combined with piped stdin

Explicit stdin control:

```bash
kubectl logs deploy/api | knotical --stdin-mode append --stdin-label logs "Find the likely root cause"
```

Use stdin as the full prompt body:

```bash
cat incident.txt | knotical --stdin-mode replace "ignored"
```

Use first-class log analysis mode:

```bash
kubectl logs deploy/api | knotical -a -p k8s --tail-lines 400 "Find the likely root cause"
```

Log-analysis behavior:

- uses a log-focused system prompt
- can use `--profile <name>` to select a log profile
- supports shorthand pipeline flags such as `--clean`, `--dedupe`, `--unique`, and `--k8s`
- supports raw ingest transforms with repeatable `--transform`
- defaults the stdin label to `logs`
- disables markdown by default unless enabled in config
- can apply a configured default schema when `--schema` is not provided
- works well with `--input-reduction summarize` for oversized log streams

Log profile and transform examples:

```bash
kubectl logs deploy/api | knotical -a -p k8s --tail 400 "Find the likely root cause"
kubectl logs deploy/api | knotical -a --clean --unique --tail 400 "Summarize the incident"
kubectl logs deploy/api | knotical -a --transform include-regex:'(?i)(error|warn|panic)' --transform dedupe-normalized --max-input-tokens 4000
```

Cheap deterministic reduction for large stdin:

```bash
kubectl logs deploy/api | knotical --stdin-label logs --tail-lines 400 --max-input-lines 200 "Summarize the current failure"
```

Approximate token budgeting:

```bash
journalctl -u nginx -n 2000 | knotical --stdin-label logs --max-input-tokens 1500 --input-reduction truncate "Summarize the current failure"
```

Reduction order:

- byte cap
- head/tail selection
- deterministic sampling
- final line cap

Token budgeting notes:

- token estimation uses a cheap approximate heuristic
- `truncate` is the default oversize mode
- `fail` returns an error instead of sending oversized stdin
- `off` ignores the token budget after deterministic reduction
- `summarize` runs a multi-pass reduction workflow over the reduced stdin payload
- `--summarize-chunk-tokens` controls the approximate chunk size for intermediate summaries
- `--summarize-intermediate-model` lets you use a different model for those intermediate summary passes

Summarize oversized logs:

```bash
journalctl -u nginx -n 20000 | knotical --analyze-logs --max-input-tokens 4000 --input-reduction summarize --summarize-chunk-tokens 800
```

### Prompt from `$EDITOR`

```bash
knotical --editor
```

## Basic Prompting Patterns

### Choose a model explicitly

```bash
knotical --model gpt-4o-mini "Explain Go interfaces"
```

### Add a direct system prompt

```bash
knotical --system "Be concise and technical." "Explain SQLite WAL mode"
```

### Disable markdown rendering

```bash
knotical --no-md "Return plain text only"
```

### Disable streaming

```bash
knotical --no-stream "Give me a short answer"
```

### Extract the first fenced code block

```bash
knotical --extract "Write a shell script that prints disk usage"
```

### Save the current settings as a template

```bash
knotical --model gpt-4o-mini --system "Review code for bugs." --save reviewer "ignored prompt"
```

## Roles, Templates, and Fragments

These features let you reuse prompt context in different ways.

### Roles

Roles are named system prompt presets.

Create:

```bash
knotical roles create reviewer --system "You are a terse code reviewer."
```

Create with `$EDITOR`:

```bash
knotical roles create reviewer
```

List:

```bash
knotical roles list
```

Show:

```bash
knotical roles show reviewer
```

Delete:

```bash
knotical roles delete reviewer
```

Use a role in a prompt:

```bash
knotical --role reviewer "Review this design"
```

### Templates

Templates store a name, system prompt, optional model, optional description, and optional temperature.

Create:

```bash
knotical templates create review --system "Review code for bugs." --model gpt-4o-mini --description "Code review template"
```

List:

```bash
knotical templates list
```

Show:

```bash
knotical templates show review
```

Edit:

```bash
knotical templates edit review
```

Delete:

```bash
knotical templates delete review
```

Use a template:

```bash
knotical --template review "Review this patch"
```

### Fragments

Fragments are named reusable text snippets appended to the prompt.

Set:

```bash
knotical fragments set readme "Project context goes here"
```

Get:

```bash
knotical fragments get readme
```

List:

```bash
knotical fragments list
```

Delete:

```bash
knotical fragments delete readme
```

Use one fragment:

```bash
knotical --fragment readme "Summarize this project"
```

Use multiple fragments:

```bash
knotical --fragment architecture --fragment readme "Prepare onboarding notes"
```

## Structured Output

`--schema` requests structured JSON output.

### DSL form

```bash
knotical --schema "name, age:int, active:bool" "Generate a fake user"
```

### JSON schema file

```bash
knotical --schema ./schema.json "Generate a release summary"
```

Behavior:

- schema-aware providers use native structured output where supported
- other providers fall back to prompt-level JSON instructions
- the final output is parsed, validated, and pretty-printed

## Chat Sessions and REPL

### Start or continue a named chat

```bash
knotical --chat release-notes "Draft release notes from these commits"
```

Follow up in the same session:

```bash
knotical --chat release-notes "Make them shorter"
```

### Continue the last chat

```bash
knotical --continue "Continue from the previous session"
```

### Start a REPL

```bash
knotical --repl scratch
```

Multiline input in REPL:

- type `"""` on its own line to start multiline mode
- type `"""` again to finish

Exit REPL with:

- `exit`
- `quit`
- `/exit`
- `/quit`

### Chat management

List chats:

```bash
knotical chats list
```

Show a chat:

```bash
knotical chats show release-notes
```

Delete a chat:

```bash
knotical chats delete release-notes
```

## Shell Workflows

### Generate a shell command

```bash
knotical --shell "find the 10 largest files in the current directory"
```

### Explain a shell command

```bash
knotical --describe-shell "find . -type f -size +100M"
```

### Shell plus fragments

```bash
knotical --shell --fragment readme "find the scripts related to setup"
```

### Shell plus chat

```bash
knotical --shell --chat shell-notes "show disk usage for the current repo"
```

### Shell plus `--extract`

This is usually more useful for `--code`, but it can still be combined:

```bash
knotical --shell --extract "give me a shell snippet to list hidden files"
```

### Host execution

```bash
knotical --shell --execute host "show git status"
```

Alias form:

```bash
knotical --shell --host "show git status"
```

### Safe execution

```bash
knotical --shell --safe "list tracked files"
```

Safe mode characteristics:

- no shell parsing
- only a constrained allowlist of read-only commands
- rejects high-risk commands

### Sandbox execution

```bash
knotical --shell --sandbox --docker "search for TODO comments"
```

With explicit image:

```bash
knotical --shell --sandbox --podman --img docker.io/library/ubuntu:24.04 "show git status"
```

With workspace write access:

```bash
knotical --shell --sandbox --rw "run go test ./..."
```

With network enabled:

```bash
knotical --shell --sandbox --net "curl https://example.com"
```

Sandbox defaults:

- runtime: resolved from `docker` or `podman`
- image: `docker.io/library/ubuntu:24.04`
- shell target: Linux `sh`
- read-only root filesystem unless write mode is enabled
- mounted workspace is read-only unless `--rw` is set
- network is disabled unless `--net` is set

### Risk override for host execution

```bash
knotical --shell --execute host --force-risky-shell "delete all *.tmp files"
```

Without `--force-risky-shell`, high-risk host commands require confirmation in interactive mode
and are refused in non-interactive mode.

## Code Generation

### Generate code only

```bash
knotical --code "Write a Go function that parses a CSV line"
```

### Code plus extraction

```bash
knotical --code --extract "Write a zsh function that prints the current branch"
```

### Code plus schema is usually not meaningful

`--code` and `--schema` are both supported by the parser, but they pull in different directions.
Prefer `--schema` for structured JSON and `--code` for raw code output.

## Logging

Logging is backed by SQLite.

### Per-invocation logging control

Force logging:

```bash
knotical --log "Explain TCP slow start"
```

Disable logging for one call:

```bash
knotical --no-log "Do not persist this prompt"
```

### List recent logs

```bash
knotical logs
```

### Limit result count

```bash
knotical logs --count 20
```

### Filter by model

```bash
knotical logs --model gpt-4o-mini
```

### Search logs

```bash
knotical logs --search "sqlite"
```

### Search and sort by latest

```bash
knotical logs --search "review" --latest
```

### Show only the response text

```bash
knotical logs --response
```

### Extract code from logged responses

```bash
knotical logs --extract
```

Extract the last fenced block:

```bash
knotical logs --extract-last
```

### JSON output

```bash
knotical logs --json
```

### Short summary view

```bash
knotical logs --short
```

### Filter by conversation

Most recent conversation:

```bash
knotical logs --conversation
```

Specific conversation:

```bash
knotical logs --cid release-notes
```

### Filter by log ID range

```bash
knotical logs --id-gt 20260401
knotical logs --id-gte 20260401
```

### Show one log entry

```bash
knotical logs show <id>
```

### Logging status and storage

```bash
knotical logs status
knotical logs path
```

### Enable or disable logging globally

```bash
knotical logs on
knotical logs off
```

### Clear logs

```bash
knotical logs clear
```

### Back up the log database

Default destination:

```bash
knotical logs backup
```

Explicit destination:

```bash
knotical logs backup /tmp/knotical-logs.db
```

## Model Management

### List models

```bash
knotical models list
```

Filter to one provider:

```bash
knotical models list --provider openai
```

Machine-readable output:

```bash
knotical models list --json
```

Bypass the discovery cache:

```bash
knotical models list --refresh
```

Behavior notes:

- execution accepts arbitrary model strings
- API transports can enumerate models when the provider supports it
- CLI transports generally do not support model enumeration
- `models list` may partially succeed and print warnings for unsupported providers
- `models list` is discovery only, not a validation boundary
- discovery is cached briefly to avoid repeated provider calls

### Set the default model

```bash
knotical models default gpt-4o-mini
```

### Inspect a model

```bash
knotical models info gpt-4o-mini
```

### Explicit provider selection

Use `--provider` when the model name is ambiguous or does not match the built-in heuristics:

```bash
knotical --provider gemini --model custom-model "Summarize this log excerpt"
```

You can also use `provider/model` syntax:

```bash
knotical --model anthropic/claude-sonnet-4-5 "Review this change"
```

## Aliases

Aliases let you create short names for models.

Set:

```bash
knotical aliases set fast gpt-4o-mini
```

List:

```bash
knotical aliases list
```

Remove:

```bash
knotical aliases remove fast
```

Use an alias:

```bash
knotical --model fast "Summarize this file"
```

## Keys

Manage stored provider API keys.

Set:

```bash
knotical keys set openai
```

Get masked:

```bash
knotical keys get openai
```

Get full value:

```bash
knotical keys get openai --reveal
```

List:

```bash
knotical keys list
```

Remove:

```bash
knotical keys remove openai
```

Show storage path:

```bash
knotical keys path
```

## Config Command

The `config` command is for generating, inspecting, and editing `config.toml`.

Generate a default config:

```bash
knotical config generate
```

Generate to an explicit path:

```bash
knotical config generate ./config.toml
```

Overwrite an existing config:

```bash
knotical config generate --force ./config.toml
```

Show the effective merged config:

```bash
knotical config show
```

Show config path:

```bash
knotical config path
```

Edit config in `$EDITOR`:

```bash
knotical config edit
```

### Important Config Fields

Provider routing:

- `default_model`
- `default_provider`
- `openai_transport`
- `anthropic_transport`
- `gemini_transport`

Provider endpoints:

- `openai_base_url`
- `anthropic_base_url`
- `gemini_base_url`
- `ollama_base_url`
- `api_base_url` for legacy Ollama fallback

Core generation defaults:

- `request_timeout`
- `stream`
- `prettify_markdown`
- `temperature`
- `top_p`
- `log_to_db`
- `max_input_bytes`
- `max_input_lines`
- `max_input_tokens`
- `input_reduction_mode`
- `default_head_lines`
- `default_tail_lines`
- `default_sample_lines`

Shell defaults:

- `shell_execute_mode`
- `shell_sandbox_runtime`
- `shell_sandbox_image`
- `shell_sandbox_network`
- `shell_sandbox_write`

Log-analysis defaults:

- `log_analysis_markdown`
- `log_analysis_schema`
- `log_analysis_system_prompt`

External CLI transport settings:

- `claude_cli_command`
- `claude_cli_args`
- `claude_cli_model_flag`
- `claude_cli_system_flag`
- `claude_cli_schema_flag`
- `codex_cli_command`
- `codex_cli_args`
- `codex_cli_model_flag`
- `codex_cli_system_flag`
- `codex_cli_schema_flag`
- `gemini_cli_command`
- `gemini_cli_args`
- `gemini_cli_model_flag`
- `gemini_cli_system_flag`
- `gemini_cli_schema_flag`

## Shell Integration

Install shell integration:

```bash
knotical install-integration
```

Current behavior:

- installs integration for `zsh` or `bash`
- binds `Ctrl+L`
- uses the current `knotical` binary path

## Completion

`completion` is the standard Cobra completion command.

Examples:

```bash
knotical completion bash
knotical completion zsh
knotical completion fish
knotical completion powershell
```

## Practical Combinations

### Template plus chat

```bash
knotical --template review --chat review-session "Review this patch"
```

### Role plus fragment plus model alias

```bash
knotical --role reviewer --fragment readme --model fast "Summarize the architecture"
```

### Schema plus no streaming

```bash
knotical --schema "title, summary, risk_level" --no-stream "Summarize this incident"
```

### Shell generation with configured sandbox defaults

```bash
knotical --shell "show git status"
```

If `shell_execute_mode = "sandbox"` is set in config, this can automatically use sandbox execution
without repeating the runtime/image flags on every call.

### Chat with structured JSON output

```bash
knotical --chat release --schema "title, bullets" "Draft release notes"
```

### CLI transport with local Claude login

```bash
knotical --model claude-sonnet-4-5 "Summarize this repository"
```

If `anthropic_transport = "cli"` is configured, this uses the local Claude CLI instead of an
Anthropic API key.

## Notes and Caveats

- CLI transports are best-effort wrappers around external tools.
- Claude CLI currently has the strongest native non-interactive flag support.
- Codex CLI and Gemini CLI may rely on prompt injection for system and schema behavior depending on
  your configured flags.
- `models list` is not guaranteed for CLI transports.
- Shell execution is intentionally safety-sensitive. Prefer `safe` or `sandbox` over `host` unless
  you really need host execution.
