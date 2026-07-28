package internal

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// mustParse fails the test if the edited source is not valid Go — the property
// that matters most, since this code rewrites a file the project cannot build
// without.
func mustParse(t *testing.T, source string) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), "app.module.go", source, parser.ParseComments); err != nil {
		t.Fatalf("edited source does not parse: %v\n%s", err, source)
	}
}

func TestAddModuleToGroupedImports(t *testing.T) {
	source := `package src

import (
	"nikaapp/apps/api/src/user"

	"github.com/nika-framework/nika"
)

type AppModule struct{}

func (m *AppModule) Imports() []nika.Module {
	return []nika.Module{
		user.NewUserModule(),
	}
}
`
	updated, changed, err := addModuleToSource(source, "nikaapp/apps/api/src/product", "product", "product.NewProductModule()")
	if err != nil {
		t.Fatalf("addModuleToSource() error = %v", err)
	}
	if !changed {
		t.Fatal("no change reported")
	}
	mustParse(t, updated)
	if !strings.Contains(updated, `"nikaapp/apps/api/src/product"`) {
		t.Errorf("import missing:\n%s", updated)
	}
	if !strings.Contains(updated, "product.NewProductModule(),") {
		t.Errorf("constructor missing:\n%s", updated)
	}
	if !strings.Contains(updated, "user.NewUserModule(),") {
		t.Errorf("existing entry was dropped:\n%s", updated)
	}
}

func TestAddModuleToEmptySlice(t *testing.T) {
	source := `package src

import "github.com/nika-framework/nika"

type AppModule struct{}

func (m *AppModule) Imports() []nika.Module { return []nika.Module{} }
`
	updated, changed, err := addModuleToSource(source, "app/src/book", "book", "book.NewBookModule()")
	if err != nil {
		t.Fatalf("addModuleToSource() error = %v", err)
	}
	if !changed {
		t.Fatal("no change reported")
	}
	mustParse(t, updated)
	// No trailing comma asserted: gofmt drops it on a one-line literal, and
	// the result is run through gofmt when the original was already clean.
	if !strings.Contains(updated, "book.NewBookModule()") {
		t.Errorf("constructor missing:\n%s", updated)
	}
	// The lone import had to be converted into a group.
	if !strings.Contains(updated, `"app/src/book"`) {
		t.Errorf("import missing:\n%s", updated)
	}
}

// TestAddModuleFormatsOnlyCleanFiles: tidying an already-gofmt'd file is a
// courtesy; reformatting one the user keeps unformatted is not our call.
func TestAddModuleFormatsOnlyCleanFiles(t *testing.T) {
	messy := `package src

import (
"github.com/nika-framework/nika"
)

type AppModule struct{}

func (m *AppModule) Imports() []nika.Module {
return []nika.Module{
}
}
`
	updated, _, err := addModuleToSource(messy, "app/src/x", "x", "x.NewXModule()")
	if err != nil {
		t.Fatalf("addModuleToSource() error = %v", err)
	}
	mustParse(t, updated)
	if strings.Contains(updated, "\timport") || strings.Contains(updated, "\treturn []nika.Module") {
		t.Errorf("an unformatted file was reformatted:\n%s", updated)
	}
	if !strings.Contains(updated, "x.NewXModule()") {
		t.Errorf("constructor missing:\n%s", updated)
	}
}

// TestAddModuleIsIdempotent: running `nika g res user` twice must not produce
// a duplicate import or a duplicate slice entry.
func TestAddModuleIsIdempotent(t *testing.T) {
	source := `package src

import (
	"app/src/user"

	"github.com/nika-framework/nika"
)

type AppModule struct{}

func (m *AppModule) Imports() []nika.Module {
	return []nika.Module{
		user.NewUserModule(),
	}
}
`
	updated, changed, err := addModuleToSource(source, "app/src/user", "user", "user.NewUserModule()")
	if err != nil {
		t.Fatalf("addModuleToSource() error = %v", err)
	}
	if changed {
		t.Errorf("already-registered module was re-added:\n%s", updated)
	}
}

// TestAddModuleAliasesClashingPackage covers the workspace gateway case, where
// user-grpc, user-tcp and user-redis are all package "user".
func TestAddModuleAliasesClashingPackage(t *testing.T) {
	source := `package src

import (
	grpcuser "nikaapp/apps/api/src/user-grpc"

	"github.com/nika-framework/nika"
)

type AppModule struct{}

func (m *AppModule) Imports() []nika.Module {
	return []nika.Module{
		grpcuser.NewUserModule(),
	}
}
`
	updated, _, err := addModuleToSource(source, "nikaapp/apps/api/src/user-tcp", "grpcuser", "grpcuser.NewUserModule2()")
	if err != nil {
		t.Fatalf("addModuleToSource() error = %v", err)
	}
	mustParse(t, updated)
	if strings.Count(updated, `"nikaapp/apps/api/src/user-tcp"`) != 1 {
		t.Errorf("new import missing or duplicated:\n%s", updated)
	}
	// The clashing identifier must have been aliased, not emitted bare.
	if !strings.Contains(updated, `grpcusermodule "nikaapp/apps/api/src/user-tcp"`) {
		t.Errorf("clashing package was not aliased:\n%s", updated)
	}
}

// TestAddModuleRefusesUnparseableFile: better to print a manual instruction
// than to add a second syntax error to a file that is already broken.
func TestAddModuleRefusesUnparseableFile(t *testing.T) {
	_, _, err := addModuleToSource("package src\nfunc broken( {", "app/src/x", "x", "x.NewXModule()")
	if err == nil {
		t.Fatal("invalid Go source was accepted")
	}
	if !strings.Contains(err.Error(), "does not parse") {
		t.Errorf("unhelpful error: %v", err)
	}
}

// TestAddModuleRefusesMissingImportsMethod.
func TestAddModuleRefusesMissingImportsMethod(t *testing.T) {
	source := `package src

import "github.com/nika-framework/nika"

type AppModule struct{}

func (m *AppModule) Controllers() []interface{} { return nil }

var _ = nika.Module(nil)
`
	_, _, err := addModuleToSource(source, "app/src/x", "x", "x.NewXModule()")
	if err == nil || !strings.Contains(err.Error(), "Imports()") {
		t.Errorf("err = %v, want a message about the missing Imports() method", err)
	}
}
