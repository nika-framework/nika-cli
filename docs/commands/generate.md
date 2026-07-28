# nika generate

Generate modules, controllers, and services.

Full reference: [cli/generate.md](../cli/generate.md).

## Usage

```bash
nika generate <type> <name>
nika g <type> <name>
```

## Database-backed resources

When generating a resource, Nika prompts for the persistence backend:

- MongoDB
- PostgreSQL
- MySQL
- SQLite

Pass `--database` (or `-d`) to skip the prompt:

```bash
nika generate res user --database postgres
nika generate res product -d mysql
nika generate res note -d sqlite
nika generate res event -d mongodb
```

PostgreSQL, MySQL, and SQLite resources use `common/sqldb`, an `int64` primary
key, SQL `db` struct tags, and a driver-specific migration under the module's
`migrations` directory. Configure `sqldb.Setup` once in the application before
loading a SQL-backed module.

## Microservice workspaces

In a project with an `apps/` directory, the command asks which service the
module belongs to and generates into that service's `src` folder, with import
paths and module registration to match. Pass `--app` (or `-a`) to skip the
prompt:

```bash
nika generate res user -a api
nika generate res order --app micro-grpc
```

Partial names work when unambiguous (`-a grpc` → `micro-grpc`), and `NIKA_APP`
sets the target for a whole shell session. See
[Workspaces](../cli/monorepo.md).

## Module registration

The new module is registered automatically in the target app's
`app.module.go` — its import added and its constructor appended to
`Imports()`. Re-running the same command does not duplicate either. If the file
cannot be parsed, the CLI leaves it alone and prints the line to add by hand.
