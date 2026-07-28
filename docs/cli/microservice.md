# `nika microservice`

Grow a project into several processes, and wire each one to a message
transport.

```bash
nika microservice init
nika microservice <transport> [name]
nika microservice list
```

Aliases: `nika micro`, `nika ms`.

A new Nika project is one application with its modules in `src/`. Two commands
take it from there: `init` turns that into a workspace, and one command per
transport adds services alongside it.

## `nika microservice init`

Convert a single-application project into a microservice workspace.

```bash
nika microservice init            # src/ → apps/api/src/
nika microservice init -n gateway # src/ → apps/gateway/src/
```

It performs four steps:

1. moves `src/` to `apps/<name>/src/` and `main.go` to `apps/<name>/main.go`
2. moves `docs/` to `apps/<name>/docs/` when it holds generated Swagger output,
   since `main.go` imports it with a blank identifier
3. rewrites every `"<module>/src/…"` and `"<module>/docs"` import across the
   repository
4. writes `mode = "microservice"` and `default_app` into `.nika.toml`

Shared code stays at the project root: `go.mod`, `.env`, `internal/`,
`cmd/migrate` and `cmd/seed` are untouched, so migrations and seeds keep
working. Because every service is started from the root, a `.env` at the root
is still the one they all read.

```text
─────────────────
  Restructuring
─────────────────
  ✔ apps/api created
  ✔ src → apps/api/src
  ✔ main.go → apps/api/main.go
  ✔ docs → apps/api/docs

─────────────────────
  Rewriting Imports
─────────────────────
  ✔ Updated 44 file(s)
```

The import rewrite is the part worth automating. A project with twenty modules
has hundreds of `<module>/src/...` imports, and the rewrite matches only
`…/src"` and `…/src/`, so a package named `srcutil` and another module's `src`
are both left alone.

The command refuses to run twice: a project that already has apps under `apps/`
is reported as a workspace rather than converted again.

## Adding a service

```bash
nika microservice grpc                 # apps/grpc-micro
nika microservice kafka orders-worker  # apps/orders-worker
```

| Transport | Aliases | Default app | Trade-off |
|-----------|---------|-------------|-----------|
| `kafka` | | `apps/kafka-micro` | Ordered, replayable event log. At-least-once; a consumer group is required. |
| `nats` | | `apps/nats-micro` | Native request/reply. At-most-once on core NATS. |
| `rabbit` | `rabbitmq`, `amqp` | `apps/rabbit-micro` | Topic exchange, durable queues, at-least-once with publisher confirms. |
| `redis` | `redismq` | `apps/redis-micro` | At-most-once and stores nothing — right for invalidation and presence, wrong for orders. |
| `grpc` | `grpcmq` | `apps/grpc-micro` | Synchronous RPC, no broker and no store-and-forward. No protoc step. |
| `tcp` | `tcpmq` | `apps/tcp-micro` | No broker at all. Nothing to run in tests; nothing to absorb a restart either. |

`nika microservice list` prints the same table with the environment variables
each one adds.

Each command creates three files:

```text
apps/<name>/
  main.go                  transport built from .env, then app.RunWorker()
  src/app.module.go        the root module — `nika g res` appends to its Imports()
  src/app.controller.go    two sample handlers
```

and registers the app in `.nika.toml`:

```toml
[workspace]
  apps = ["api", "grpc-micro"]

[apps.grpc-micro]
  cmd = "go run ./apps/grpc-micro/main.go"
```

### Environment

The transport's variables are appended to both `.env` and `.env.example`, each
with a one-line comment. A key either file already assigns is left alone, so
re-running the command — or adding a second service on the same broker — never
resets a URL you have pointed at your own infrastructure.

```bash
# ── grpc-micro (gRPC microservice) ──
# Listen address for this service
GRPC_ADDR=:50051
# Address this service calls out to; leave empty if it only serves
GRPC_TARGET=
# Plaintext gRPC. Set false and supply TLS before this leaves localhost
GRPC_INSECURE=true
```

| Transport | Variables |
|-----------|-----------|
| `kafka` | `KAFKA_BROKERS`, `KAFKA_TOPIC`, `KAFKA_GROUP_ID`, `KAFKA_CONCURRENCY` |
| `nats` | `NATS_URL`, `NATS_PREFIX`, `NATS_NAME`, `NATS_QUEUE_GROUP` |
| `rabbit` | `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`, `RABBITMQ_QUEUE`, `RABBITMQ_PREFETCH` |
| `redis` | `REDIS_MQ_URL`, `REDIS_MQ_PREFIX` |
| `grpc` | `GRPC_ADDR`, `GRPC_TARGET`, `GRPC_INSECURE` |
| `tcp` | `TCP_ADDR`, `TCP_DIAL_ADDR` |

`REDIS_MQ_URL` is deliberately not `REDIS_URL`: the cache and the message
transport are usually different instances, and sharing one variable makes that
impossible to express.

### Message handlers

A handler is a controller field carrying a `transport` and a `pattern` tag:

```go
type AppController struct {
	Ping func(*gin.Context) `transport:"grpc" pattern:"grpc_micro_ping"`
	Echo func(*gin.Context) `transport:"grpc" pattern:"grpc_micro_echo"`
}
```

A field may carry a `route` tag as well, in which case the same handler serves
HTTP and messages — `microservice.IsMessage(c)` tells them apart.

Patterns use `_` as their separator, never `.`. A dot is AMQP's topic word
separator, so RabbitMQ rejects a pattern containing one, and `_` is the single
separator all six transports accept.

### Generating modules into a service

Every service is an ordinary Nika app, so the generator targets it with `-a`:

```bash
nika g res order -d postgres -a orders-worker
```

The module lands in `apps/orders-worker/src/order/` with import paths to match,
and is registered in that service's `app.module.go`.

## Running them

```bash
nika start --watch -a api          # one service
nika start --watch -a grpc-micro   # another
nika start --watch -a              # all of them, one process each
```

See [`nika start`](start.md#running-every-service) for what the multi-service
mode does about output and restarts.

## See also

- [Workspaces](monorepo.md) - the `apps/` layout in detail
- [`nika app`](app.md) - inspecting and configuring the app list
- [`nika start`](start.md) - running one service or all of them
- [Configuration](configuration.md) - the `[workspace]` and `[apps.*]` sections
