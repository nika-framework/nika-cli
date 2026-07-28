# `nika start`

Run a Nika application in normal or watch mode.

```bash
nika start [file-or-dir]
nika start -a <app>
nika start -a
nika start --watch
```

## Choosing what runs

With no arguments the command runs the `[build] cmd` from `.nika.toml`. In a
microservice workspace that is the default app's `main.go`, so you never have
to remember each service's path.

Precedence, highest first:

1. an explicit path argument
2. the `-a` / `--app` flag
3. `workspace.default_app` in `.nika.toml`
4. an interactive prompt, when the workspace has several apps and no default

```bash
nika start                        # the default app
nika start -a micro-grpc          # one specific service
nika start ./apps/api/main.go     # an explicit path
```

```text
▶️  Starting micro-grpc: go run ./apps/micro-grpc/main.go
✅ SQL Database connected (sqlite3)
```

Partial app names are accepted when unambiguous, so `-a grpc` resolves to
`micro-grpc`. Set the default once with:

```bash
nika app use api
```

See [Workspaces](monorepo.md) and [`nika app`](app.md).

## Normal mode

Normal mode runs the resolved command once in `root` and returns when the
process exits:

```bash
nika start
nika start -a api
```

## Watch mode

Watch mode runs the same command and restarts it when an included file
changes:

```bash
nika start --watch
nika start -a micro-grpc --watch
```

The default watch settings are:

- Root: `.`
- Command: `go run .` in a single-app project, the app's `main.go` in a workspace
- Delay: `1000` milliseconds
- Included extension: `.go`
- Excluded directories: `docs`, `tmp`, `vendor`, `testdata`, `.git`, `cache`
- Excluded file pattern: `^\\.`

Watch mode supports `pre_cmd`, `post_cmd`, environment variables, included
extensions, excluded files, excluded directories, and exclude regular
expressions — taken from the root `[build]` section merged with the selected
app's `[apps.<name>]` override. See [Configuration](configuration.md).

Press Ctrl+C to stop; the running process is terminated before the CLI exits.

## Running every service

`-a` takes an optional value. With no name it starts every app in the
workspace, each in its own process:

```bash
nika start -a
nika start --watch -a
```

This is the answer to the four-terminal problem: an API, a gRPC service and two
consumers only make sense running together, and running them together is the
common case in development.

Output is tagged with the service that produced it, one colour per service, and
buffered by line so a process that prints in several writes does not get a tag
in the middle of a line:

```text
▶️  Starting 3 services:
   api           go run ./apps/api/main.go
   grpc-micro    go run ./apps/grpc-micro/main.go
   orders-worker go run ./apps/orders-worker/main.go
🔄 Watch mode enabled – watching for changes...
api           │ ✅ SQL Database connected (sqlite3)
api           │ ***🚀 Nika is running on http://localhost:3007 *****
grpc-micro    │ INFO grpc server started addr=[::]:50051 patterns="[grpc_micro_ping]"
```

In watch mode, a change under `apps/<name>/` restarts only that service; a
change to anything shared — `internal/`, root-level code, `go.mod` — restarts
all of them, because there is no cheap way to know which services import it:

```text
📁 Change detected: apps/grpc-micro/src/app.controller.go → restarting grpc-micro
📁 Change detected: internal/database/migrations/001_users.go → restarting api, grpc-micro, orders-worker
```

Ctrl+C stops every service and waits for each to be reaped, so a restart never
leaves two copies bound to the same port.

Each service uses its own `[apps.<name>]` build overrides, including `pre_cmd`,
`post_cmd` and `env`.

> **`-a name` still works.** The flag's value is optional, which means
> `--app=api` needs the `=`. The short form `-a api` is handled: the CLI
> recognises a trailing argument that names a real app and treats it as the
> selection rather than as a path.
