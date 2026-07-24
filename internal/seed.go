package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/nika-framework/nika-cli/common"
)

// SeedConfig controls how a new seed file is scaffolded.
type SeedConfig struct {
	Name     string
	Database DatabaseType
	Dir      string
	// Model is a path to a Go file containing the model struct. When set, the
	// seed body inserts a sample row built from the struct's db-tagged fields.
	Model string
	// TypeName selects which struct in Model to use.
	TypeName string
	// Table overrides the derived table/collection name.
	Table string
}

// GenerateSeed writes a new seed .go file into the project and returns the
// path that was created.
func GenerateSeed(cfg *SeedConfig) (string, error) {
	if cfg.Database == "" {
		return "", fmt.Errorf("seed: --database is required (mongodb, postgres, mysql, sqlite)")
	}
	if err := validateMigrationName(cfg.Name); err != nil {
		return "", fmt.Errorf("seed: %w", err)
	}

	dir := cfg.Dir
	if dir == "" {
		dir = filepath.Join("internal", "database", "seeds")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("seed: mkdir %s: %w", dir, err)
	}

	name := toSnake(cfg.Name)
	version, err := nextVersion(dir, time.Now().UTC())
	if err != nil {
		return "", err
	}
	outPath := filepath.Join(dir, fmt.Sprintf("%s_%s.go", version, name))

	body := goSeedBody(cfg.Database, name, version)
	if cfg.Model != "" {
		model, err := ParseModelFile(cfg.Model, cfg.TypeName, cfg.Table, cfg.Database)
		if err != nil {
			return "", fmt.Errorf("seed: %w", err)
		}
		if cfg.Database == DatabaseMongo {
			body = mongoSeedFromModel(model, name, version)
		} else {
			body = sqlSeedFromModel(model, name, version)
		}
	}
	if err := common.WriteFile(outPath, body); err != nil {
		return "", err
	}

	if _, err := ensureSeedRunner(cfg.Database, dir); err != nil {
		return outPath, err
	}
	return outPath, nil
}

// ensureSeedRunner creates ./cmd/seed/main.go if it doesn't exist.
func ensureSeedRunner(db DatabaseType, seedsDir string) (string, error) {
	runnerDir := filepath.Join("cmd", "seed")
	runnerMain := filepath.Join(runnerDir, "main.go")
	if _, err := os.Stat(runnerMain); err == nil {
		return "", nil
	}
	module, err := ResolveModulePath()
	if err != nil {
		return "", nil
	}
	if err := os.MkdirAll(runnerDir, 0o755); err != nil {
		return "", fmt.Errorf("seed runner: mkdir %s: %w", runnerDir, err)
	}
	importPath := module + "/" + filepath.ToSlash(seedsDir)
	if err := common.WriteFile(runnerMain, seedRunnerBody(db, importPath)); err != nil {
		return "", err
	}
	return runnerMain, nil
}

func seedRunnerBody(db DatabaseType, seedsImport string) string {
	if db == DatabaseMongo {
		return fmt.Sprintf(`// cmd/seed is the seed runner. Invoked by "nika seed run|status".
// Reads connection settings from environment variables:
//   MONGO_URI       (required)
//   MONGO_DATABASE  (required)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nika-framework/nika"
	"github.com/nika-framework/nika/common/mongodb"
	"github.com/nika-framework/nika/common/mongodb/seed"

	_ %q
)

func main() {
	if len(os.Args) < 2 {
		exit("usage: seed run|status [names...]")
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

	s := seed.New(db)

	switch sub {
	case "run":
		var ran []string
		if len(os.Args) > 2 {
			ran, err = s.RunOnly(ctx, os.Args[2:]...)
		} else {
			ran, err = s.Run(ctx)
		}
		if err != nil {
			exit(err.Error())
		}
		if len(ran) == 0 {
			fmt.Println("no seeds ran")
			return
		}
		for _, n := range ran {
			fmt.Printf("seeded %%s\n", n)
		}
	case "status":
		applied, err := s.AppliedNames(ctx)
		if err != nil {
			exit(err.Error())
		}
		fmt.Printf("%%d seed(s) applied\n", len(applied))
		for n := range applied {
			fmt.Println(" -", n)
		}
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

func exit(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
`, seedsImport)
	}

	return fmt.Sprintf(`// cmd/seed is the seed runner. Invoked by "nika seed run|status".
// Reads connection settings from environment variables:
//   DATABASE_DRIVER  postgres | mysql | sqlite3 (default: %s)
//   DATABASE_DSN     driver-specific DSN (required)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nika-framework/nika"
	"github.com/nika-framework/nika/common/sqldb"
	"github.com/nika-framework/nika/common/sqldb/seed"

	// SQL driver registration.
	_ %q

	_ %q
)

func main() {
	if len(os.Args) < 2 {
		exit("usage: seed run|status [names...]")
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

	s := seed.New(db)

	switch sub {
	case "run":
		var ran []string
		if len(os.Args) > 2 {
			ran, err = s.RunOnly(ctx, os.Args[2:]...)
		} else {
			ran, err = s.Run(ctx)
		}
		if err != nil {
			exit(err.Error())
		}
		if len(ran) == 0 {
			fmt.Println("no seeds ran")
			return
		}
		for _, n := range ran {
			fmt.Printf("seeded %%s\n", n)
		}
	case "status":
		applied, err := s.AppliedNames(ctx)
		if err != nil {
			exit(err.Error())
		}
		fmt.Printf("%%d seed(s) applied\n", len(applied))
		for n := range applied {
			fmt.Println(" -", n)
		}
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

func exit(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
`, sqlDriverName(db), sqlDriverImport(db), seedsImport, sqlDriverName(db))
}

func goSeedBody(db DatabaseType, name, version string) string {
	if db == DatabaseMongo {
		return fmt.Sprintf(`package seeds

import (
	"context"

	"github.com/nika-framework/nika/common/mongodb/seed"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	seed.Register(&seed.Seed{
		Name:  %q,
		Order: %s,
		Run: func(ctx context.Context, db *mongo.Database) error {
			// TODO: seed data for %s
			_ = db
			return nil
		},
	})
}
`, name, version, name)
	}
	return fmt.Sprintf(`package seeds

import (
	"context"
	"database/sql"

	"github.com/nika-framework/nika/common/sqldb/seed"
)

func init() {
	seed.Register(&seed.Seed{
		Name:  %q,
		Order: %s,
		Run: func(ctx context.Context, tx *sql.Tx) error {
			// TODO: seed data for %s
			_ = tx
			return nil
		},
	})
}
`, name, version, name)
}

// RunSeed shells out to the project's seed binary. Convention: cmd/seed reads
// subcommands (run, name...) from os.Args.
func RunSeed(subcommand string, extraArgs ...string) error {
	if _, err := os.Stat(filepath.Join("cmd", "seed")); err != nil {
		return fmt.Errorf("seed: cmd/seed not found in project — run `nika g seed <name>` first, then implement it")
	}
	args := append([]string{"run", "./cmd/seed", subcommand}, extraArgs...)
	c := exec.Command("go", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}
