package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nika-framework/nika-cli/common"
)

// MigrationConfig controls how a new migration file is scaffolded.
type MigrationConfig struct {
	// Name is a short, kebab-or-snake description ("create-users", "add_index").
	Name string
	// Database selects the target backend (mongodb, postgres, mysql, sqlite).
	Database DatabaseType
	// Format is "go" (default) or "sql" (SQL backends only).
	Format string
	// Dir overrides the destination directory (default: "internal/database/migrations").
	Dir string
	// Model is a path to a Go file containing the model struct. When set, the
	// migration body is generated from the struct's db-tagged fields instead
	// of being left as a TODO stub.
	Model string
	// TypeName selects which struct in Model to use (required only when the
	// file declares more than one).
	TypeName string
	// Table overrides the derived table/collection name.
	Table string
}

// migrationNamePattern accepts filesystem-safe identifiers.
var migrationNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_\-]*$`)

// GenerateMigration writes a new migration file (or file pair) into the
// project. Returns the paths that were created.
func GenerateMigration(cfg *MigrationConfig) ([]string, error) {
	if cfg.Database == "" {
		return nil, fmt.Errorf("migration: --database is required (mongodb, postgres, mysql, sqlite)")
	}
	if err := validateMigrationName(cfg.Name); err != nil {
		return nil, err
	}
	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	if format == "" {
		format = "go"
	}
	if format != "go" && format != "sql" {
		return nil, fmt.Errorf("migration: unsupported --format %q (want go or sql)", format)
	}
	if format == "sql" && cfg.Database == DatabaseMongo {
		return nil, fmt.Errorf("migration: --format=sql is not supported for MongoDB")
	}

	dir := cfg.Dir
	if dir == "" {
		dir = filepath.Join("internal", "database", "migrations")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("migration: mkdir %s: %w", dir, err)
	}

	safeName := toSnake(cfg.Name)
	version, err := nextVersion(dir, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	// When --model is given, derive real DDL from the struct's db tags.
	var model *ParsedModel
	if cfg.Model != "" {
		model, err = ParseModelFile(cfg.Model, cfg.TypeName, cfg.Table, cfg.Database)
		if err != nil {
			return nil, fmt.Errorf("migration: %w", err)
		}
	}

	created := []string{}
	switch format {
	case "sql":
		upPath := filepath.Join(dir, fmt.Sprintf("%s_%s.up.sql", version, safeName))
		downPath := filepath.Join(dir, fmt.Sprintf("%s_%s.down.sql", version, safeName))
		upBody := sqlMigrationUpBody(safeName, cfg.Database)
		downBody := sqlMigrationDownBody(safeName, cfg.Database)
		if model != nil {
			upBody = BuildCreateTableSQL(model, cfg.Database) + ";\n"
			downBody = BuildDropTableSQL(model) + ";\n"
		}
		if err := common.WriteFile(upPath, upBody); err != nil {
			return nil, err
		}
		if err := common.WriteFile(downPath, downBody); err != nil {
			return nil, err
		}
		created = append(created, upPath, downPath)
	default:
		outPath := filepath.Join(dir, fmt.Sprintf("%s_%s.go", version, safeName))
		var body string
		switch {
		case model != nil && cfg.Database == DatabaseMongo:
			body = mongoMigrationFromModel(model, version, safeName)
		case model != nil:
			body = goMigrationFromModel(model, cfg.Database, version, safeName)
		default:
			body = goMigrationBody(cfg.Database, version, safeName)
		}
		if err := common.WriteFile(outPath, body); err != nil {
			return nil, err
		}
		created = append(created, outPath)
	}

	// Scaffold cmd/migrate on the first migration so `nika migrate up|down|status`
	// works out of the box.
	runnerPaths, err := ensureMigrateRunner(cfg.Database, dir)
	if err != nil {
		return created, err
	}
	created = append(created, runnerPaths...)
	return created, nil
}

// ensureMigrateRunner creates ./cmd/migrate/main.go if it does not already
// exist. The runner discovers registered migrations by blank-importing the
// migrations package (typically internal/database/migrations).
func ensureMigrateRunner(db DatabaseType, migrationsDir string) ([]string, error) {
	runnerDir := filepath.Join("cmd", "migrate")
	runnerMain := filepath.Join(runnerDir, "main.go")
	if _, err := os.Stat(runnerMain); err == nil {
		return nil, nil
	}

	module, err := ResolveModulePath()
	if err != nil {
		// No go.mod — skip runner scaffolding silently; user is on their own.
		return nil, nil
	}
	if err := os.MkdirAll(runnerDir, 0o755); err != nil {
		return nil, fmt.Errorf("migrate runner: mkdir %s: %w", runnerDir, err)
	}
	importPath := module + "/" + filepath.ToSlash(migrationsDir)
	if err := common.WriteFile(runnerMain, migrateRunnerBody(db, importPath)); err != nil {
		return nil, err
	}
	return []string{runnerMain}, nil
}

// versionPattern matches the leading digits in a migration filename.
var versionPattern = regexp.MustCompile(`^([0-9]{6,})_`)

// nextVersion returns a version string that is guaranteed not to collide
// with any existing file in dir. When two files would land in the same
// second, the second one gets bumped forward by one.
func nextVersion(dir string, now time.Time) (string, error) {
	base, err := strconv.ParseInt(now.Format("20060102150405"), 10, 64)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("migration: read dir %s: %w", dir, err)
	}
	var maxSeen int64
	for _, e := range entries {
		m := versionPattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			continue
		}
		if v > maxSeen {
			maxSeen = v
		}
	}
	if base <= maxSeen {
		base = maxSeen + 1
	}
	return strconv.FormatInt(base, 10), nil
}

func validateMigrationName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("migration: name is required")
	}
	if !migrationNamePattern.MatchString(name) {
		return fmt.Errorf("migration: name %q must start with a letter and contain only letters, digits, _ or -", name)
	}
	return nil
}

// toSnake normalizes a name to lowercase snake_case.
func toSnake(name string) string {
	var b strings.Builder
	for i, r := range name {
		switch {
		case r == '-', r == ' ':
			b.WriteByte('_')
		case r >= 'A' && r <= 'Z':
			if i > 0 && name[i-1] >= 'a' && name[i-1] <= 'z' {
				b.WriteByte('_')
			}
			b.WriteRune(r + 32)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// toPascal produces an exported Go identifier from a name.
func toPascal(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	parts := strings.Split(name, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	return b.String()
}

func sqlMigrationUpBody(name string, db DatabaseType) string {
	return fmt.Sprintf(`-- migration: %s (up)
-- database: %s
-- Write your forward SQL below.

`, name, db)
}

func sqlMigrationDownBody(name string, db DatabaseType) string {
	return fmt.Sprintf(`-- migration: %s (down)
-- database: %s
-- Write the rollback SQL below.

`, name, db)
}

func goMigrationBody(db DatabaseType, version, name string) string {
	fnName := "Migration_" + version + "_" + toPascal(name)
	if db == DatabaseMongo {
		return fmt.Sprintf(`package migrations

import (
	"context"

	"github.com/nika-framework/nika/common/mongodb/migration"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	migration.Register(&migration.Migration{
		Version: %s,
		Name:    %q,
		Up: func(ctx context.Context, db *mongo.Database) error {
			// TODO: apply schema changes for %s
			_ = db
			return nil
		},
		Down: func(ctx context.Context, db *mongo.Database) error {
			// TODO: rollback %s
			_ = db
			return nil
		},
	})
}

var _ = %q // silence unused warnings if the migration is empty
`, version, name, name, name, fnName)
	}
	return fmt.Sprintf(`package migrations

import (
	"context"
	"database/sql"

	"github.com/nika-framework/nika/common/sqldb/migration"
)

func init() {
	migration.Register(&migration.Migration{
		Version: %s,
		Name:    %q,
		Up: func(ctx context.Context, tx *sql.Tx) error {
			// TODO: apply schema changes for %s
			_ = tx
			return nil
		},
		Down: func(ctx context.Context, tx *sql.Tx) error {
			// TODO: rollback %s
			_ = tx
			return nil
		},
	})
}

var _ = %q // silence unused warnings if the migration is empty
`, version, name, name, name, fnName)
}

func migrateRunnerBody(db DatabaseType, migrationsImport string) string {
	if db == DatabaseMongo {
		return fmt.Sprintf(`// cmd/migrate is the migration runner. Invoked by "nika migrate up|down|status".
// Reads connection settings from environment variables:
//   MONGO_URI       (required)
//   MONGO_DATABASE  (required)
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/nika-framework/nika"
	"github.com/nika-framework/nika/common/mongodb"
	"github.com/nika-framework/nika/common/mongodb/migration"

	// Blank import so every migration's init() registers itself.
	_ %q
)

func main() {
	if len(os.Args) < 2 {
		exit("usage: migrate up|down|status [n]")
	}
	sub := os.Args[1]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	app := nika.NewApp()
	db, err := mongodb.Setup(app, mongodb.Config{
		URI:      mustEnv("MONGO_URI"),
		Database: mustEnv("MONGO_DATABASE"),
	})
	if err != nil {
		exit(err.Error())
	}
	defer db.Close(context.Background())

	m := migration.New(db)

	switch sub {
	case "up":
		n := parseN(os.Args, 2)
		versions, err := m.UpN(ctx, n)
		if err != nil {
			exit(err.Error())
		}
		if len(versions) == 0 {
			fmt.Println("nothing to apply")
			return
		}
		for _, v := range versions {
			fmt.Printf("applied %%d\n", v)
		}
	case "down":
		n := parseN(os.Args, 2)
		if n <= 0 {
			n = 1
		}
		versions, err := m.DownN(ctx, n)
		if err != nil {
			exit(err.Error())
		}
		if len(versions) == 0 {
			fmt.Println("nothing to roll back")
			return
		}
		for _, v := range versions {
			fmt.Printf("rolled back %%d\n", v)
		}
	case "status":
		s, err := m.Status(ctx)
		if err != nil {
			exit(err.Error())
		}
		fmt.Print(s)
	default:
		exit("unknown subcommand: " + sub)
	}
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		exit(k + " environment variable is required")
	}
	return v
}

func parseN(args []string, idx int) int {
	if len(args) <= idx {
		return 0
	}
	n, err := strconv.Atoi(args[idx])
	if err != nil {
		exit("invalid number: " + args[idx])
	}
	return n
}

func exit(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
`, migrationsImport)
	}

	return fmt.Sprintf(`// cmd/migrate is the migration runner. Invoked by "nika migrate up|down|status".
// Reads connection settings from environment variables:
//   DATABASE_DRIVER  postgres | mysql | sqlite3 (default: %s)
//   DATABASE_DSN     driver-specific DSN (required)
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/nika-framework/nika"
	"github.com/nika-framework/nika/common/sqldb"
	"github.com/nika-framework/nika/common/sqldb/migration"

	// SQL driver registration.
	_ %q

	// Blank import so every migration's init() registers itself.
	_ %q
)

func main() {
	if len(os.Args) < 2 {
		exit("usage: migrate up|down|status [n]")
	}
	sub := os.Args[1]

	driver := os.Getenv("DATABASE_DRIVER")
	if driver == "" {
		driver = %q
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	app := nika.NewApp()
	db, err := sqldb.Setup(app, sqldb.Config{
		Driver: sqldb.Driver(driver),
		DSN:    mustEnv("DATABASE_DSN"),
	})
	if err != nil {
		exit(err.Error())
	}
	defer db.Close()

	m := migration.New(db)

	switch sub {
	case "up":
		n := parseN(os.Args, 2)
		versions, err := m.UpN(ctx, n)
		if err != nil {
			exit(err.Error())
		}
		if len(versions) == 0 {
			fmt.Println("nothing to apply")
			return
		}
		for _, v := range versions {
			fmt.Printf("applied %%d\n", v)
		}
	case "down":
		n := parseN(os.Args, 2)
		if n <= 0 {
			n = 1
		}
		versions, err := m.DownN(ctx, n)
		if err != nil {
			exit(err.Error())
		}
		if len(versions) == 0 {
			fmt.Println("nothing to roll back")
			return
		}
		for _, v := range versions {
			fmt.Printf("rolled back %%d\n", v)
		}
	case "status":
		s, err := m.Status(ctx)
		if err != nil {
			exit(err.Error())
		}
		fmt.Print(s)
	default:
		exit("unknown subcommand: " + sub)
	}
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		exit(k + " environment variable is required")
	}
	return v
}

func parseN(args []string, idx int) int {
	if len(args) <= idx {
		return 0
	}
	n, err := strconv.Atoi(args[idx])
	if err != nil {
		exit("invalid number: " + args[idx])
	}
	return n
}

func exit(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
`, sqlDriverName(db), sqlDriverImport(db), migrationsImport, sqlDriverName(db))
}

// sqlDriverName is the database/sql driver name for the backend.
func sqlDriverName(db DatabaseType) string {
	switch db {
	case DatabaseMySQL:
		return "mysql"
	case DatabaseSQLite:
		return "sqlite3"
	default:
		return "postgres"
	}
}

// sqlDriverImport is the Go import path that registers the driver.
func sqlDriverImport(db DatabaseType) string {
	switch db {
	case DatabaseMySQL:
		return "github.com/go-sql-driver/mysql"
	case DatabaseSQLite:
		return "github.com/mattn/go-sqlite3"
	default:
		return "github.com/lib/pq"
	}
}

// RunMigrate shells out to the project's migration binary. The convention is
// that "nika g migration" scaffolds a "cmd/migrate" main package that reads
// its subcommand (up, down, status) from os.Args[1].
func RunMigrate(subcommand string, extraArgs ...string) error {
	if _, err := os.Stat(filepath.Join("cmd", "migrate")); err != nil {
		return fmt.Errorf("migrate: cmd/migrate not found in project — run `nika g migration <name>` first, then implement it")
	}
	args := append([]string{"run", "./cmd/migrate", subcommand}, extraArgs...)
	c := exec.Command("go", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
