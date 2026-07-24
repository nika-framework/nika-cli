# nika generate

Generate modules, controllers, and services.

## Usage

```bash
nika generate <type> <name>
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
