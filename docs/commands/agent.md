# nika agent

Configure and run the AI provider for the current project. Configuration is
stored in `.nika.toml` under the `[agent]` section.

## Configure a provider

```bash
nika agent init ollama
nika agent init 9router
nika agent init chatgpt
```

`ollama` uses the local Ollama server and defaults to model `gemma3:4b`.
`9router` uses an OpenAI-compatible OpenRouter endpoint and reads
`OPENROUTER_API_KEY`. `chatgpt` uses the OpenAI endpoint and reads
`OPENAI_API_KEY`.

The initializer preserves the existing build configuration in `.nika.toml` and
does not write API keys to disk.

## Run a prompt

```bash
nika agent "Add a mock data creation route to the news module"
```

The command automatically detects module-generation and route-generation
requests and uses the existing Nika generators. Other prompts are sent to the
configured provider and printed as text.