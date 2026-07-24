package internal

import (
	"fmt"
	"strconv"
	"strings"
)

// BuildCreateTableSQL renders a CREATE TABLE statement for the parsed model.
func BuildCreateTableSQL(pm *ParsedModel, db DatabaseType) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE IF NOT EXISTS %s (\n", pm.TableName)

	parts := make([]string, 0, len(pm.Fields))
	for _, f := range pm.Fields {
		line := fmt.Sprintf("    %s %s", f.Column, f.SQLType)
		// Primary key type strings already carry PRIMARY KEY / AUTOINCREMENT.
		if !f.IsPrimary && !f.Nullable {
			line += " NOT NULL"
		}
		parts = append(parts, line)
	}
	b.WriteString(strings.Join(parts, ",\n"))
	b.WriteString("\n)")
	return b.String()
}

// BuildDropTableSQL renders the rollback statement.
func BuildDropTableSQL(pm *ParsedModel) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s", pm.TableName)
}

// goMigrationFromModel renders a Go migration whose Up creates the table for
// the model and whose Down drops it.
func goMigrationFromModel(pm *ParsedModel, db DatabaseType, version, name string) string {
	createSQL := BuildCreateTableSQL(pm, db)
	dropSQL := BuildDropTableSQL(pm)

	return fmt.Sprintf(`package migrations

import (
	"context"
	"database/sql"

	"github.com/nika-framework/nika/common/sqldb/migration"
)

// Generated from model %s (table %q).
func init() {
	migration.Register(&migration.Migration{
		Version: %s,
		Name:    %q,
		Up: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, %s)
			return err
		},
		Down: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, %s)
			return err
		},
	})
}
`, pm.TypeName, pm.TableName, version, name, backquote(createSQL), backquote(dropSQL))
}

// mongoMigrationFromModel renders a Mongo migration that creates the
// collection plus an index on each unique-ish field.
func mongoMigrationFromModel(pm *ParsedModel, version, name string) string {
	return fmt.Sprintf(`package migrations

import (
	"context"

	"github.com/nika-framework/nika/common/mongodb/migration"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Generated from model %s (collection %q).
func init() {
	migration.Register(&migration.Migration{
		Version: %s,
		Name:    %q,
		Up: func(ctx context.Context, db *mongo.Database) error {
			if err := db.CreateCollection(ctx, %q); err != nil {
				// Ignore "collection already exists".
				if !mongo.IsDuplicateKeyError(err) && !isNamespaceExists(err) {
					return err
				}
			}
			_, err := db.Collection(%q).Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "created_at", Value: -1}},
				Options: options.Index().SetName("created_at_desc"),
			})
			return err
		},
		Down: func(ctx context.Context, db *mongo.Database) error {
			return db.Collection(%q).Drop(ctx)
		},
	})
}

func isNamespaceExists(err error) bool {
	ce, ok := err.(mongo.CommandError)
	return ok && ce.Code == 48
}
`, pm.TypeName, pm.TableName, version, name, pm.TableName, pm.TableName, pm.TableName)
}

// sqlSeedFromModel renders a seed that inserts sample rows for the model.
func sqlSeedFromModel(pm *ParsedModel, name, order string) string {
	cols := make([]string, 0, len(pm.Fields))
	placeholders := make([]string, 0, len(pm.Fields))
	values := make([]string, 0, len(pm.Fields))

	i := 0
	for _, f := range pm.Fields {
		if f.IsPrimary && f.IsAuto {
			continue // let the database assign it
		}
		i++
		cols = append(cols, f.Column)
		placeholders = append(placeholders, "?")
		values = append(values, sampleValueExpr(&f))
	}

	insert := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		pm.TableName,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)

	return fmt.Sprintf(`package seeds

import (
	"context"
	"database/sql"
	"time"

	"github.com/nika-framework/nika/common/sqldb/seed"
)

// Generated from model %s (table %q).
func init() {
	seed.Register(&seed.Seed{
		Name:  %q,
		Order: %s,
		Run: func(ctx context.Context, tx *sql.Tx) error {
			now := time.Now().UTC()
			_ = now

			rows := [][]any{
				{%s},
			}

			for _, row := range rows {
				if _, err := tx.ExecContext(ctx, %s, row...); err != nil {
					return err
				}
			}
			return nil
		},
	})
}
`, pm.TypeName, pm.TableName, name, order, strings.Join(values, ", "), backquote(insert))
}

// mongoSeedFromModel renders a seed that inserts a sample document.
func mongoSeedFromModel(pm *ParsedModel, name, order string) string {
	fields := make([]string, 0, len(pm.Fields))
	for _, f := range pm.Fields {
		if f.Column == "_id" || (f.IsPrimary && f.IsAuto) {
			continue
		}
		fields = append(fields, fmt.Sprintf("\t\t\t\t\t%q: %s,", f.Column, sampleValueExpr(&f)))
	}

	return fmt.Sprintf(`package seeds

import (
	"context"
	"time"

	"github.com/nika-framework/nika/common/mongodb/seed"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Generated from model %s (collection %q).
func init() {
	seed.Register(&seed.Seed{
		Name:  %q,
		Order: %s,
		Run: func(ctx context.Context, db *mongo.Database) error {
			now := time.Now().UTC()
			_ = now

			docs := []any{
				bson.M{
%s
				},
			}

			_, err := db.Collection(%q).InsertMany(ctx, docs)
			return err
		},
	})
}
`, pm.TypeName, pm.TableName, name, order, strings.Join(fields, "\n"), pm.TableName)
}

// sampleValueExpr produces a Go literal suitable as seed data for the field.
func sampleValueExpr(f *ModelField) string {
	if f.IsCreated || f.IsUpdated {
		return "now"
	}
	base := strings.TrimPrefix(f.GoType, "*")
	switch base {
	case "string":
		return fmt.Sprintf("%q", "sample_"+f.Column)
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		return "1"
	case "float32", "float64":
		return "1.0"
	case "bool":
		return "true"
	case "time.Time":
		return "now"
	}
	return "nil"
}

// backquote wraps a SQL string in a Go raw string literal. Any embedded
// backquote is escaped by splitting the literal.
func backquote(s string) string {
	if !strings.Contains(s, "`") {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}
