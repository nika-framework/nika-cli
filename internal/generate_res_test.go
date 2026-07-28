package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDatabaseType(t *testing.T) {
	cases := map[string]DatabaseType{
		"mongo":      DatabaseMongo,
		"mongodb":    DatabaseMongo,
		"postgres":   DatabasePostgres,
		"postgresql": DatabasePostgres,
		"postgress":  DatabasePostgres,
		"posgrees":   DatabasePostgres,
		"mysql":      DatabaseMySQL,
		"sqlite":     DatabaseSQLite,
		"sqlite3":    DatabaseSQLite,
	}

	for input, want := range cases {
		if got := ParseDatabaseType(input); got != want {
			t.Errorf("ParseDatabaseType(%q) = %q, want %q", input, got, want)
		}
	}
	if got := ParseDatabaseType("oracle"); got != "" {
		t.Errorf("ParseDatabaseType(oracle) = %q, want empty", got)
	}
}

func TestRunGenerateUsesDatabaseSpecificTemplates(t *testing.T) {
	cases := []struct {
		name               string
		database           string
		modelPkg           string
		modelContains      string
		repositoryContains string
		migrationContains  string
	}{
		{
			name:               "MongoDB",
			database:           "mongodb",
			modelPkg:           "schema",
			modelContains:      "primitive.ObjectID",
			repositoryContains: "common/mongodb/repository",
		},
		{
			name:               "PostgreSQL",
			database:           "postgres",
			modelPkg:           "entity",
			modelContains:      "ID        int64 `db:\"id\" json:\"id\"`",
			repositoryContains: "common/sqldb/repository",
			migrationContains:  "id BIGSERIAL PRIMARY KEY",
		},
		{
			name:               "MySQL",
			database:           "mysql",
			modelPkg:           "entity",
			modelContains:      "ID        int64 `db:\"id\" json:\"id\"`",
			repositoryContains: "common/sqldb/repository",
			migrationContains:  "id BIGINT AUTO_INCREMENT PRIMARY KEY",
		},
		{
			name:               "SQLite",
			database:           "sqlite",
			modelPkg:           "entity",
			modelContains:      "ID        int64 `db:\"id\" json:\"id\"`",
			repositoryContains: "common/sqldb/repository",
			migrationContains:  "id INTEGER PRIMARY KEY AUTOINCREMENT",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			tempDir := t.TempDir()
			t.Chdir(tempDir)
			if err := os.WriteFile("go.mod", []byte("module example.com/testapp\n\ngo 1.23\n"), 0o644); err != nil {
				t.Fatalf("write go.mod: %v", err)
			}

			err := RunGenerate(&GenerateConfig{
				Type:     GenResource,
				Module:   "book",
				Database: test.database,
				Fields: []Field{{
					Name:     "Title",
					BsonName: "title",
					Type:     "string",
					Required: true,
				}},
			})
			if err != nil {
				t.Fatalf("RunGenerate() error = %v", err)
			}

			// SQL modules keep the model under entity/, MongoDB under schema/.
			model := readGeneratedFile(t, filepath.Join("src", "book", test.modelPkg, "book.model.go"))
			if !strings.Contains(model, "package "+test.modelPkg) {
				t.Errorf("model is not in package %q:\n%s", test.modelPkg, model)
			}
			if !strings.Contains(model, test.modelContains) {
				t.Errorf("model does not contain %q:\n%s", test.modelContains, model)
			}

			repository := readGeneratedFile(t, filepath.Join("src", "book", test.modelPkg, "book.repository.go"))
			if !strings.Contains(repository, test.repositoryContains) {
				t.Errorf("repository does not contain %q:\n%s", test.repositoryContains, repository)
			}

			module := readGeneratedFile(t, filepath.Join("src", "book", "book.module.go"))
			if !strings.Contains(module, "example.com/testapp/src/book/"+test.modelPkg) {
				t.Errorf("module does not import the %s package:\n%s", test.modelPkg, module)
			}

			if !strings.Contains(module, "services.NewBookService") || !strings.Contains(module, "Exports() []interface{}") {
				t.Errorf("module does not register the generated service correctly:\n%s", module)
			}

			if test.migrationContains == "" {
				if _, err := os.Stat(filepath.Join("src", "book", "migrations")); !os.IsNotExist(err) {
					t.Errorf("MongoDB resource unexpectedly has a migration directory")
				}
				return
			}

			migration := readGeneratedFile(t, filepath.Join("src", "book", "migrations", "000_create_books.sql"))
			if !strings.Contains(migration, test.migrationContains) {
				t.Errorf("migration does not contain %q:\n%s", test.migrationContains, migration)
			}
			if strings.Contains(model, "go.mongodb.org/mongo-driver") || strings.Contains(repository, "common/mongodb") {
				t.Errorf("SQL resource contains MongoDB dependencies")
			}
		})
	}
}

func readGeneratedFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated %s: %v", path, err)
	}
	return string(content)
}
