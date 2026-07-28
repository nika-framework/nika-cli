# Nika Cli
Nika is a modern backend framework for Go, designed for scalability, clean architecture, and developer productivity.
 


## Commands

- [docs](./docs/README.md) - Nika CLI Documentation

### Project layouts

Nika projects come in two shapes, and the CLI detects which from disk:

| Layout | Modules live in | Entry point |
| --- | --- | --- |
| Single application | `src/<module>/` | `main.go` |
| Microservice workspace | `apps/<service>/src/<module>/` | `apps/<service>/main.go` |

In a workspace every command that touches a module asks which service it
belongs to, and generates import paths to match. Answer up front with `--app`
to skip the prompt:

```bash
nika g res user                 # asks: which microservice?
nika g res user -a api          # generates into apps/api/src/user
nika g res order --app micro-grpc
```

Inspect and configure the layout:

```bash
nika app list          # every service, its src folder and run command
nika app use api       # pick the service `nika start` runs by default
nika app sync          # rewrite .nika.toml to match what is on disk
```

`nika start` follows the same rule, so you never have to remember a service's
main.go path:

```bash
nika start                  # the default app
nika start -a micro-grpc    # one specific service
nika start --watch
```

### AI agent

Configure a provider once per project. The API key is never written to
`.nika.toml` — only the name of the environment variable to read it from.

```bash
nika agent init ollama     # local Ollama, no key
nika agent init chatgpt    # OPENAI_API_KEY
nika agent init 9router    # OPENROUTER_API_KEY
nika agent init claude     # ANTHROPIC_API_KEY
```

For Ollama it inspects your installed models and picks the best one for the
agent instead of writing a fixed name:

```bash
nika agent models          # what you have, and which can call tools
```

The agent runs a tool-calling loop: it inspects the project, edits files,
scaffolds modules with Nika's own templates, and runs builds to check its work.
Any instruction works, not only module generation:

```bash
nika agent "add a price field (float64) to the product model and update the DTOs, response and mapper"
nika agent "لطفن ماژول خبر واسم بساز و فیلد تصویر و عنوان و متن و تگ ها رو داشته باشه"
nika agent "run go build ./... and fix what it reports"
nika agent                 # interactive session
```

Models without native function calling — `gemma3`, `llama3`, most older builds
— still work: the agent falls back to a JSON protocol automatically. That makes
small models usable, not equal; for real editing work prefer `qwen2.5-coder:7b`
or larger, or a cloud provider.

A run that ends claiming files were changed when none were is contradicted
rather than reported as success.

#### Browser console

```bash
nika agent start
```

Opens a page that applies everything you type to the directory the command was
started in, streaming each tool call as it happens:

- a **sidebar** of independent chats, each with its own history
- a **Chat** tab, with a single-line scrolling strip of suggestions
- a **Commands** tab — the CLI's generators as forms (resource, controller,
  service, DTOs, response, migration, seed, migrate, workspace, swagger, build,
  test), run directly without the model

The server binds to `127.0.0.1` and requires the token in the URL it prints,
because it holds write access to your source tree.

```bash
nika agent start --port 8080 --open=false
nika agent start --read-only     # inspect without changing anything
```

#### Safety

The agent is confined to the project directory: paths that escape it are
refused, and `run_command` only accepts an allowlist (`go`, `gofmt`, `git
status/diff`, `nika`, and other read-only tools). Extend it per project:

```toml
[agent]
  max_steps = 25
  allow_commands = ["make", "docker compose ps"]
```


---

## Getting Started

```bash
go install github.com/nika-framework/nika-cli@latest
go build -o nika    
```
