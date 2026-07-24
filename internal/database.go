package internal

import "strings"

// DatabaseType identifies the persistence backend used by a generated resource.
type DatabaseType string

const (
	DatabaseMongo    DatabaseType = "mongodb"
	DatabasePostgres DatabaseType = "postgres"
	DatabaseMySQL    DatabaseType = "mysql"
	DatabaseSQLite   DatabaseType = "sqlite"
)

// ParseDatabaseType normalizes database names accepted by the CLI and AI paths.
func ParseDatabaseType(raw string) DatabaseType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "mongo", "mongodb":
		return DatabaseMongo
	case "postgres", "postgresql", "postgress", "posgrees":
		return DatabasePostgres
	case "mysql":
		return DatabaseMySQL
	case "sqlite", "sqlite3":
		return DatabaseSQLite
	default:
		return ""
	}
}

// IsSQL reports whether the database uses the Nika SQL repository package.
func (d DatabaseType) IsSQL() bool {
	return d == DatabasePostgres || d == DatabaseMySQL || d == DatabaseSQLite
}

// DisplayName returns the label used in interactive prompts and output.
func (d DatabaseType) DisplayName() string {
	switch d {
	case DatabaseMongo:
		return "MongoDB"
	case DatabasePostgres:
		return "PostgreSQL"
	case DatabaseMySQL:
		return "MySQL"
	case DatabaseSQLite:
		return "SQLite"
	default:
		return string(d)
	}
}

func databaseOptions() []string {
	return []string{
		DatabaseMongo.DisplayName(),
		DatabasePostgres.DisplayName(),
		DatabaseMySQL.DisplayName(),
		DatabaseSQLite.DisplayName(),
	}
}

func sqlColumnType(database DatabaseType, goType string) string {
	switch goType {
	case "string":
		return "VARCHAR(255)"
	case "int", "int64":
		return "BIGINT"
	case "float64":
		switch database {
		case DatabasePostgres:
			return "DOUBLE PRECISION"
		case DatabaseSQLite:
			return "REAL"
		default:
			return "DOUBLE"
		}
	case "bool":
		if database == DatabaseSQLite {
			return "INTEGER"
		}
		return "BOOLEAN"
	case "time.Time":
		switch database {
		case DatabasePostgres:
			return "TIMESTAMPTZ"
		case DatabaseMySQL:
			return "DATETIME(6)"
		default:
			return "DATETIME"
		}
	default:
		return ""
	}
}

func sqlPrimaryKeyType(database DatabaseType) string {
	switch database {
	case DatabasePostgres:
		return "BIGSERIAL PRIMARY KEY"
	case DatabaseMySQL:
		return "BIGINT AUTO_INCREMENT PRIMARY KEY"
	default:
		return "INTEGER PRIMARY KEY AUTOINCREMENT"
	}
}

func sqlTimestampType(database DatabaseType) string {
	return sqlColumnType(database, "time.Time")
}
