# Nika CLI Documentation

Welcome to Nika CLI docs 🚀

A modern CLI tool for building applications with the Nika framework.


## CLI Reference

- [CLI overview](./cli/overview.md) - Complete command and configuration reference
- [Workspaces](./cli/monorepo.md) - Single-app and microservice (`apps/`) layouts
- [Configuration](./cli/configuration.md) - `.nika.toml` reference

## Commands

- [new](./commands/new.md) - Create a new application
- [generate](./commands/generate.md) - Generate modules and components
- [agent](./commands/agent.md) - Run the AI agent, or configure a provider
- [app](./cli/app.md) - Inspect and configure the apps in a workspace
- [start](./cli/start.md) - Run an application, with or without hot reload
- [swagger](./cli/swagger.md) - Swagger/OpenAPI documentation
- [version](./cli/version.md) - Version information
- [ollema](./commands/ollema.md) - Legacy single-prompt Ollama command

---

## Getting Started

```bash
go install github.com/nika-framework/nika-cli@latest
```

```bash
nika new my-app
cd my-app && go mod tidy
nika g res user -d sqlite
nika start --watch
```

In a microservice project, every module command targets one service:

```bash
nika app list
nika g res user -a micro-grpc
nika start -a api --watch
```

To drive the CLI with AI:

```bash
nika agent init ollama
nika agent "add a price field to the product model and update the DTOs"
nika agent start          # chat page in the browser
```
