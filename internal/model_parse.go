package internal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
)

// ModelField describes one column derived from a Go struct field.
type ModelField struct {
	Name       string // Go field name, e.g. "Username"
	Column     string // SQL column name, e.g. "username"
	GoType     string // e.g. "string", "int64", "time.Time", "*string"
	Nullable   bool   // true when GoType starts with *
	SQLType    string // rendered SQL type for the target dialect
	IsPrimary  bool   // true when column name equals the primary key column
	IsAuto     bool   // true when the primary key is auto-increment
	IsCreated  bool   // true for created_at (managed timestamp)
	IsUpdated  bool   // true for updated_at (managed timestamp)
}

// ParsedModel is the fully-resolved schema of a single struct definition.
type ParsedModel struct {
	TypeName    string       // e.g. "User"
	TableName   string       // e.g. "users"
	PrimaryKey  string       // e.g. "id"
	AutoID      bool         // whether id is INTEGER PK autoincrement
	Fields      []ModelField // includes the primary key
}

// ParseModelFile locates a Go struct inside path and returns its ParsedModel.
//
// If typeName is empty and the file contains exactly one struct, that struct
// is selected. Otherwise the caller must pass an explicit typeName.
// The tableName argument overrides the default (snake_case plural of TypeName).
func ParseModelFile(path, typeName, tableName string, db DatabaseType) (*ParsedModel, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read model: %w", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse model %s: %w", path, err)
	}

	structs := findStructs(file)
	if len(structs) == 0 {
		return nil, fmt.Errorf("no struct types found in %s", path)
	}

	var target *namedStruct
	if typeName != "" {
		for _, s := range structs {
			if s.name == typeName {
				target = s
				break
			}
		}
		if target == nil {
			return nil, fmt.Errorf("struct %q not found in %s", typeName, path)
		}
	} else if len(structs) == 1 {
		target = structs[0]
	} else {
		names := make([]string, len(structs))
		for i, s := range structs {
			names[i] = s.name
		}
		return nil, fmt.Errorf("%s contains multiple structs (%s); pass --type", path, strings.Join(names, ", "))
	}

	pm := &ParsedModel{
		TypeName:   target.name,
		PrimaryKey: "id",
	}
	if tableName != "" {
		pm.TableName = tableName
	} else {
		pm.TableName = pluralize(toSnake(target.name))
	}

	for _, f := range target.st.Fields.List {
		if len(f.Names) == 0 {
			// embedded or anonymous field — skip.
			continue
		}
		tag := ""
		if f.Tag != nil {
			raw, err := strconv.Unquote(f.Tag.Value)
			if err == nil {
				tag = raw
			}
		}
		col := extractStructTag(tag, "db")
		if col == "" || col == "-" {
			continue
		}
		if idx := strings.IndexByte(col, ','); idx >= 0 {
			col = col[:idx]
		}
		goType := renderType(f.Type)
		field := ModelField{
			Name:     f.Names[0].Name,
			Column:   col,
			GoType:   goType,
			Nullable: strings.HasPrefix(goType, "*"),
		}
		switch col {
		case "created_at":
			field.IsCreated = true
		case "updated_at":
			field.IsUpdated = true
		}
		if col == pm.PrimaryKey {
			field.IsPrimary = true
			pm.AutoID = isIntegerType(goType)
			field.IsAuto = pm.AutoID
		}
		field.SQLType = renderSQLType(db, &field)
		pm.Fields = append(pm.Fields, field)
	}

	if len(pm.Fields) == 0 {
		return nil, fmt.Errorf("no db-tagged fields on struct %q", target.name)
	}
	return pm, nil
}

type namedStruct struct {
	name string
	st   *ast.StructType
}

func findStructs(file *ast.File) []*namedStruct {
	var out []*namedStruct
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			out = append(out, &namedStruct{name: ts.Name.Name, st: st})
		}
	}
	return out
}

// extractStructTag mirrors reflect.StructTag.Get without importing reflect on
// an ast tag string.
func extractStructTag(tag, key string) string {
	for tag != "" {
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}
		i = 0
		for i < len(tag) && tag[i] != ':' && tag[i] != ' ' {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := tag[:i]
		tag = tag[i+2:]
		i = 0
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		val, err := strconv.Unquote(`"` + tag[:i] + `"`)
		tag = tag[i+1:]
		if err != nil {
			continue
		}
		if name == key {
			return val
		}
	}
	return ""
}

// renderType turns an ast.Expr into its Go source representation.
func renderType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.StarExpr:
		return "*" + renderType(t.X)
	case *ast.ArrayType:
		return "[]" + renderType(t.Elt)
	}
	return ""
}

func isIntegerType(goType string) bool {
	switch strings.TrimPrefix(goType, "*") {
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		return true
	}
	return false
}

func renderSQLType(db DatabaseType, f *ModelField) string {
	// Primary key gets the dialect's canonical PK form.
	if f.IsPrimary {
		return sqlPrimaryKeyType(db)
	}
	base := strings.TrimPrefix(f.GoType, "*")
	sqlType := sqlColumnType(db, base)
	if sqlType == "" {
		sqlType = "TEXT"
	}
	return sqlType
}

// Table names reuse the existing pluralize helper in generate_res.go so
// migrations and generated resources agree on naming.
