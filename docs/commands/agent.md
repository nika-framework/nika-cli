# nika agent

Run the AI agent against the current project. Provider configuration is stored
in `.nika.toml` under the `[agent]` section.

Full reference: [cli/agent.md](../cli/agent.md).

## Configure a provider

```bash
nika agent init ollama
nika agent init chatgpt
nika agent init 9router
nika agent init claude
```

For `ollama` the initializer looks at the models you actually have installed
and picks the best one for the agent, rather than writing a fixed name that may
not exist locally. `chatgpt` reads `OPENAI_API_KEY`, `9router` reads
`OPENROUTER_API_KEY`, and `claude` reads `ANTHROPIC_API_KEY`.

The initializer preserves every other section of `.nika.toml` and does not
write API keys to disk — only the environment variable name is stored.

## Models

```bash
nika agent models
```

Lists your installed Ollama models and whether each supports native tool
calling. Models that do not — `gemma3`, `llama3`, most older builds — still
work: the agent drives them with a JSON protocol instead. That is chosen
automatically; nothing to configure.

Emulation makes small models usable, not equal. For real editing work prefer
`qwen2.5-coder:7b` or larger, or a cloud provider.

## Run an instruction

```bash
nika agent "add a price field (float64) to the product model and update the DTOs and mapper"
nika agent "generate a category module with name and slug for sqlite"
nika agent "run go build ./... and fix what it reports"
```

The agent runs a tool-calling loop: it inspects the project, edits files,
scaffolds modules with Nika's templates, and runs builds to verify its work,
repeating until the task is done. Any instruction works — it is not limited to
module or route generation.

## Interactive session

```bash
nika agent
```

Starts a REPL that keeps the conversation across turns. `/reset` clears it,
`/changed` lists the files changed, `/exit` quits.

## Browser chat

```bash
nika agent start
```

Opens a page that applies everything you type to the directory the command was
started in, streaming each tool call as it happens. It has a sidebar of
independent chats and two tabs: **Chat**, and **Commands** — the CLI's
generators as forms, run directly without the model. Binds to `127.0.0.1` and
requires the token in the URL it prints.

## Editor AI files

```bash
nika agent files
```

Installs Nika instructions, prompts, and skills under `.github` for Copilot and
Cursor. This used to be `nika agent` with no arguments, which now starts an
interactive session instead.

## Flags

| Flag | Purpose |
|------|---------|
| `-q`, `--quiet` | Print only the final answer, not the tool trace |
| `--read-only` | Inspect the project but change nothing |
| `--allow-any-command` | Remove the `run_command` allowlist |
| `-C`, `--dir` | Work in another directory |
