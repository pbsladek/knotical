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

## Usage

Basic prompt:

```bash
knotical "Summarize the purpose of this repository"
```

Read prompt content from stdin:

```bash
git diff --staged | knotical "Review this patch for bugs"
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

## Common Commands

```bash
knotical --help
knotical models list
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
