package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nika-framework/nika-cli/common"
	"github.com/nika-framework/nika-cli/templates"
)

// ── Field Definition ────────────────────────────────────────────────

// Field represents a single model field collected from the user.
type Field struct {
	Name       string // Go field name, e.g. "FirstName"
	BsonName   string // MongoDB field name, e.g. "first_name"
	ColumnName string // SQL column name, e.g. "first_name"
	Type       string // Go type, e.g. "string"
	SQLType    string // SQL column type for the generated migration
	Required   bool
	JsonTag    string // tag for response structs
	ModelTag   string // tag for the persistence model
	CreateTag  string // tag for create DTO (json+validate)
	UpdateTag  string // tag for update DTO (json+validate omitempty)
}

// TemplateData is the payload passed to all .tpl files.
type TemplateData struct {
	ModulePath     string // full module path from go.mod, e.g. "github.com/nika-framework/my-app"
	ModuleName     string // e.g. "user"
	TypeName       string // e.g. "User" (exported, PascalCase)
	CollectionName string // e.g. "users"
	TableName      string // e.g. "users"
	Database       DatabaseType
	SQLPrimaryKey  string
	SQLTimestamp   string
	Fields         []Field // user-defined fields (excluding ID/CreatedAt/UpdatedAt)

	// AppName is the microservice this module belongs to ("api"), or "app" in
	// the single-app layout.
	AppName string
	// SrcImport is the import-path fragment of the app's src folder:
	// "src" for a classic project, "apps/api/src" in a workspace. Templates
	// interpolate it instead of hard-coding "src", which is what lets the same
	// templates serve both layouts.
	SrcImport string
}

// target is the app this data was built for.
func (d TemplateData) target() AppTarget {
	return AppTarget{Name: d.AppName, SrcDir: d.SrcImport}
}

// ModelPkg is the folder and package holding the model and repository: SQL
// modules call it "entity", MongoDB modules still call it "schema".
//
// It is a method rather than a field on purpose — the database can be chosen
// interactively after the data is built, and a field would go stale.
func (d TemplateData) ModelPkg() string {
	return modelPkg(d.Database)
}

// modelPkg is ModelPkg for callers that only have a database type.
func modelPkg(database DatabaseType) string {
	if database.IsSQL() {
		return "entity"
	}
	return "schema"
}

// findModuleModel locates a module's model file without knowing its database.
// Modules generated before the SQL rename still keep theirs under schema/, so
// both names are tried rather than assuming one.
func findModuleModel(moduleDir, module string) (string, error) {
	candidates := []string{
		filepath.Join(moduleDir, "entity", module+".model.go"),
		filepath.Join(moduleDir, "schema", module+".model.go"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no model file for module %q — looked for %s", module, strings.Join(candidates, " and "))
}

// ── MongoDB type catalog ────────────────────────────────────────────

// mongoTypes maps the user-facing type label to a Go type.
var mongoTypes = []string{
	"string",
	"int",
	"int64",
	"float64",
	"bool",
	"time.Time",
	"primitive.ObjectID",
	"[]string",
	"map[string]any",
}

var sqlTypes = []string{
	"string",
	"int",
	"int64",
	"float64",
	"bool",
	"time.Time",
}

// renderTemplate renders one embedded template to disk.
func renderTemplate(tpl, out string, data *TemplateData) error {
	content, err := templates.Read(tpl)
	if err != nil {
		return fmt.Errorf("template %s not found in binary: %w", tpl, err)
	}
	rendered, err := common.RenderString(tpl, content, data)
	if err != nil {
		return err
	}
	return common.WriteRendered(out, rendered)
}

// mongoTypeDefaultValidate gives a sensible validate tag per type.
func mongoTypeDefaultValidate(goType string, required bool) string {
	base := "omitempty"
	if required {
		base = "required"
	}
	switch goType {
	case "string":
		if required {
			return base + ",min=1,max=255"
		}
		return base + ",max=255"
	case "int", "int64":
		return base + ",min=0"
	case "float64":
		return base
	case "bool":
		return base
	default:
		return base
	}
}

func supportedFieldTypes(database DatabaseType) []string {
	if database.IsSQL() {
		return sqlTypes
	}
	return mongoTypes
}

// ── Name helpers ────────────────────────────────────────────────────

// toPascalCase converts snake_case to PascalCase.
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
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

// pluralize naively pluralizes a module name for collection & routes.
func pluralize(s string) string {
	if strings.HasSuffix(s, "y") {
		return s[:len(s)-1] + "ies"
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "ch") || strings.HasSuffix(s, "sh") {
		return s + "es"
	}
	return s + "s"
}

// ── Interactive collection ──────────────────────────────────────────

// collectFields runs the interactive prompt loop to gather model fields.
// Stops on EOF or when the user types "done".
func collectFields(sp *common.Spinner, database DatabaseType) ([]Field, error) {
	var fields []Field
	sp.Stop("Ready to collect fields...")

	for {
		fmt.Println()
		raw := common.Prompt("Enter field name (snake_case) or 'done' to finish", "")
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "done" || name == "" {
			break
		}
		// sanitize
		name = strings.ReplaceAll(name, " ", "_")

		// Choose type
		typeChoice := common.SelectOption("Select type for "+name, supportedFieldTypes(database))

		// Required?
		required := common.ConfirmYesNo(fmt.Sprintf("Is %q required?", name))

		f, err := newField(name, typeChoice, required, database)
		if err != nil {
			return nil, err
		}

		fields = append(fields, f)
		fmt.Printf("  ✔ Field %q (%s, required=%v) added\n", name, typeChoice, required)
	}

	return fields, nil
}

// NewField builds a Field with all of its tags derived, validating the name
// and type against what the database supports. Exported for the AI agent,
// which assembles fields from a model's JSON rather than from prompts.
func NewField(name, goType string, required bool, database DatabaseType) (Field, error) {
	return newField(name, goType, required, database)
}

func newField(name, goType string, required bool, database DatabaseType) (Field, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !isValidModule(name) {
		return Field{}, fmt.Errorf("invalid field name: %s", name)
	}
	if !containsString(supportedFieldTypes(database), goType) {
		return Field{}, fmt.Errorf("type %q is not supported by %s", goType, database.DisplayName())
	}

	field := Field{
		Name:       toPascalCase(name),
		BsonName:   name,
		ColumnName: name,
		Type:       goType,
		Required:   required,
		JsonTag:    fmt.Sprintf(`json:"%s"`, name),
		CreateTag:  fmt.Sprintf(`json:"%s" validate:"%s"`, name, mongoTypeDefaultValidate(goType, required)),
		UpdateTag:  fmt.Sprintf(`json:"%s,omitempty" validate:"omitempty"`, name),
	}
	if database.IsSQL() {
		field.ModelTag = fmt.Sprintf(`db:"%s" json:"%s"`, name, name)
		field.SQLType = sqlColumnType(database, goType)
	} else {
		field.ModelTag = fmt.Sprintf(`bson:"%s" json:"%s"`, name, name)
	}
	return field, nil
}

func prepareFields(fields []Field, database DatabaseType) ([]Field, error) {
	prepared := make([]Field, 0, len(fields))
	for _, field := range fields {
		name := field.ColumnName
		if name == "" {
			name = field.BsonName
		}
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		preparedField, err := newField(name, field.Type, field.Required, database)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedField)
	}
	return prepared, nil
}

// ── Main entry point ────────────────────────────────────────────────

// GenerateConfig holds the parameters for a generate run.
type GenerateConfig struct {
	Type     GenerateType
	Module   string  // raw module name from args
	Database string  // optional database name for non-interactive generation
	Fields   []Field // optional fields for non-interactive generation
	// App names the microservice to generate into. Empty means "ask", unless
	// the workspace has only one app.
	App string
	// SkipRegister leaves app.module.go untouched.
	SkipRegister bool
}

// RunGenerate is the top-level entry for `nika g <type> <module>`.
func RunGenerate(cfg *GenerateConfig) error {
	// Step 1: validate environment
	common.Section("Environment Check")
	sp := common.NewSpinner()

	workspace, err := LoadWorkspace()
	if err != nil {
		return err
	}
	modulePath := workspace.ModulePath

	// Step 2: pick the target app. In a microservice workspace this is the
	// question that used to be skipped entirely, which is why every module
	// landed in a top-level src/ that no service imports.
	app, err := workspace.SelectApp(cfg.App)
	if err != nil {
		return err
	}
	if workspace.Microservice {
		fmt.Printf("  📍 Target service: %s (%s)\n", app.Name, app.SrcDir)
	}

	// Step 3: validate module name
	moduleName := strings.ToLower(strings.TrimSpace(cfg.Module))
	if moduleName == "" {
		return fmt.Errorf("module name is required")
	}
	if !isValidModule(moduleName) {
		return fmt.Errorf("invalid module name: %s", moduleName)
	}

	// Build template data
	database := ParseDatabaseType(cfg.Database)
	if cfg.Database != "" && database == "" {
		return fmt.Errorf("unsupported database %q (use mongodb, postgres, mysql, or sqlite)", cfg.Database)
	}

	data := TemplateData{
		ModulePath:     modulePath,
		ModuleName:     moduleName,
		TypeName:       toPascalCase(moduleName),
		CollectionName: pluralize(moduleName),
		TableName:      pluralize(moduleName),
		Database:       database,
		AppName:        app.Name,
		SrcImport:      app.SrcImport(),
	}
	if cfg.Fields != nil {
		data.Fields = cfg.Fields
	}

	// Keep .nika.toml in step with what we just detected, so `nika start` and
	// the next `nika g` agree on the layout. A failure here is not fatal.
	if workspace.Microservice {
		if err := workspace.Sync(); err != nil {
			fmt.Printf("  ⚠ could not update %s: %v\n", nikaConfigPath, err)
		}
	}

	// Step 4: dispatch by type
	switch cfg.Type {
	case GenResource, GenRes:
		if err := runResource(sp, modulePath, &data); err != nil {
			return err
		}
		if !cfg.SkipRegister {
			registerInAppModule(&data)
		}
		return nil
	case GenController, GenC:
		return runControllerOnly(sp, modulePath, &data)
	case GenResponse, GenR:
		return runResponseOnly(sp, modulePath, &data)
	case GenService, GenS:
		return runServiceOnly(sp, modulePath, &data)
	case GenDTO, GenD:
		return runDTOOnly(sp, modulePath, &data)
	default:
		return fmt.Errorf("unknown generate type: %s", cfg.Type)
	}
}

// registerInAppModule wires the new module into the app's app.module.go and
// reports what happened. Registration failing is a warning, not an error: the
// files are already on disk and the user can add the one line by hand.
func registerInAppModule(data *TemplateData) {
	target := data.target()
	added, err := RegisterModule(target, data.ModulePath, data.ModuleName, data.TypeName)
	switch {
	case err != nil:
		fmt.Printf("  ⚠ Could not auto-register the module in %s: %v\n", target.AppModulePath(), err)
		fmt.Printf("     Add it manually: import \"%s/%s/%s\" then %s.New%sModule(),\n",
			data.ModulePath, data.SrcImport, data.ModuleName, data.ModuleName, data.TypeName)
	case added:
		fmt.Printf("  ✔ Registered %sModule in %s\n", data.TypeName, target.AppModulePath())
	default:
		fmt.Printf("  ✔ %sModule was already registered in %s\n", data.TypeName, target.AppModulePath())
	}
}

// isValidModule checks the module name is a valid Go identifier prefix.
func isValidModule(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 && !(r >= 'a' && r <= 'z') {
			return false
		}
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// ── Resource generator (full: schema + dto + controller + service + module) ──

func runResource(sp *common.Spinner, modulePath string, data *TemplateData) error {
	if data.Database == "" {
		// Preserve the previous non-interactive AI behavior when no database is supplied.
		if data.Fields != nil {
			data.Database = DatabaseMongo
		} else {
			common.Section("Database Selection")
			dbChoice := common.SelectOption(
				"Select database for "+data.ModuleName,
				databaseOptions(),
			)
			data.Database = ParseDatabaseType(dbChoice)
			fmt.Printf("Database: %s selected\n", data.Database.DisplayName())
		}
	}
	if data.Database.IsSQL() {
		data.SQLPrimaryKey = sqlPrimaryKeyType(data.Database)
		data.SQLTimestamp = sqlTimestampType(data.Database)
	}

	// Step 5: collect fields
	common.Section("Model Fields")
	if data.Fields == nil {
		fields, err := collectFields(sp, data.Database)
		if err != nil {
			return err
		}
		data.Fields = fields
	} else {
		fields, err := prepareFields(data.Fields, data.Database)
		if err != nil {
			return err
		}
		data.Fields = fields
	}
	fields := data.Fields
	if len(fields) == 0 {
		sp.Stop("No custom fields added — proceeding with system fields only (ID, CreatedAt, UpdatedAt)")
	} else {
		sp.Step(fmt.Sprintf("Collected %d field(s)", len(fields)), "")
	}

	// Step 6: generate schema
	common.Section("Generating Schema")
	if err := generateSchema(sp, data); err != nil {
		return err
	}

	// Step 7: generate DTOs
	common.Section("Generating DTOs")
	if err := generateDTOs(sp, data); err != nil {
		return err
	}

	// Step 8: generate services
	common.Section("Generating Services")
	if err := generateServices(sp, data); err != nil {
		return err
	}

	// Step 9: generate controller
	common.Section("Generating Controller")
	if err := generateController(sp, data); err != nil {
		return err
	}
	// Step 10: generate response
	common.Section("Generating Response")
	if err := generateResponse(sp, data); err != nil {
		return err
	}
	// Step 11: generate module
	common.Section("Generating Module")
	if err := generateModule(sp, data); err != nil {
		return err
	}
	if data.Database.IsSQL() {
		common.Section("Generating Migration")
		if err := generateMigration(sp, data); err != nil {
			return err
		}
	}

	// Step 11: done
	common.Section("Done!")
	fmt.Println()
	fmt.Printf("  📦 Resource %q generated successfully!\n", data.ModuleName)
	fmt.Println()
	fmt.Println("  Files created:")
	printTree(data.target(), data.ModuleName, data.Database)
	fmt.Println()
	if data.Database.IsSQL() {
		fmt.Printf("  ⚠ Configure sqldb.Setup with the %s driver before loading this module.\n", data.Database.DisplayName())
	}
	fmt.Println()
	return nil
}

// ── Schema generator ────────────────────────────────────────────────

func generateSchema(sp *common.Spinner, data *TemplateData) error {
	pkg := data.ModelPkg()
	base := filepath.Join(data.target().ModuleDir(data.ModuleName), pkg)
	tpls := []struct {
		tpl   string
		out   string
		label string
	}{
		{resourceTemplate(data, pkg+"/model.go.tpl"), filepath.Join(base, data.ModuleName+".model.go"), "model"},
		{resourceTemplate(data, pkg+"/repository.go.tpl"), filepath.Join(base, data.ModuleName+".repository.go"), "repository"},
		{resourceTemplate(data, pkg+"/repository.interface.go.tpl"), filepath.Join(base, data.ModuleName+".repository.interface.go"), "repository interface"},
	}

	return generateTpls(sp, tpls, data)
}

// ── DTO generator ───────────────────────────────────────────────────

func generateDTOs(sp *common.Spinner, data *TemplateData) error {
	base := filepath.Join(data.target().ModuleDir(data.ModuleName), "dto")
	tpls := []struct {
		tpl   string
		out   string
		label string
	}{
		{"templates/res/dto/create.dto.go.tpl", filepath.Join(base, "create.dto.go"), "create DTO"},
		{resourceTemplate(data, "dto/update.dto.go.tpl"), filepath.Join(base, "update.dto.go"), "update DTO"},
		{resourceTemplate(data, "dto/findone.dto.go.tpl"), filepath.Join(base, "findone.dto.go"), "findone DTO"},
		{"templates/res/dto/find.dto.go.tpl", filepath.Join(base, "find.dto.go"), "find/list DTO"},
	}

	return generateTpls(sp, tpls, data)
}

// ── Service generator ───────────────────────────────────────────────

func generateServices(sp *common.Spinner, data *TemplateData) error {
	base := filepath.Join(data.target().ModuleDir(data.ModuleName), "services")
	tpls := []struct {
		tpl   string
		out   string
		label string
	}{
		{resourceTemplate(data, "service/service.go.tpl"), filepath.Join(base, data.ModuleName+".service.go"), "service base"},
		{resourceTemplate(data, "service/create.go.tpl"), filepath.Join(base, "create.go"), "create method"},
		{resourceTemplate(data, "service/findone.go.tpl"), filepath.Join(base, "findone.go"), "findone method"},
		{resourceTemplate(data, "service/find.go.tpl"), filepath.Join(base, "find.go"), "find method"},
		{resourceTemplate(data, "service/update.go.tpl"), filepath.Join(base, "update.go"), "update method"},
		{resourceTemplate(data, "service/delete.go.tpl"), filepath.Join(base, "delete.go"), "delete method"},
	}

	return generateTpls(sp, tpls, data)
}

// ── Controller generator ────────────────────────────────────────────

func generateController(sp *common.Spinner, data *TemplateData) error {
	base := filepath.Join(data.target().ModuleDir(data.ModuleName), "controllers")
	tpls := []struct {
		tpl   string
		out   string
		label string
	}{
		{"templates/res/controller/controller.go.tpl", filepath.Join(base, data.ModuleName+".controller.go"), "controller base"},
		{"templates/res/controller/create.go.tpl", filepath.Join(base, "create.go"), "create method"},
		{"templates/res/controller/find-one.go.tpl", filepath.Join(base, "find-one.go"), "findone method"},
		{"templates/res/controller/find.go.tpl", filepath.Join(base, "find.go"), "find method"},
		{"templates/res/controller/delete.go.tpl", filepath.Join(base, "delete.go"), "delete method"},
		{"templates/res/controller/update.go.tpl", filepath.Join(base, "update.go"), "update method"},
	}
	return generateTpls(sp, tpls, data)
}

// ── Module response ────────────────────────────────────────────────

func generateResponse(sp *common.Spinner, data *TemplateData) error {
	base := filepath.Join(data.target().ModuleDir(data.ModuleName), "response")
	tpls := []struct {
		tpl   string
		out   string
		label string
	}{
		{resourceTemplate(data, "response/response.go.tpl"), filepath.Join(base, data.ModuleName+".response.go"), "response base"},
		{resourceTemplate(data, "response/mapper.go.tpl"), filepath.Join(base, data.ModuleName+".mapper.go"), "mapper method"},
	}
	return generateTpls(sp, tpls, data)
}

func generateTpls(sp *common.Spinner, tpls []struct {
	tpl   string
	out   string
	label string
}, data *TemplateData) error {
	for _, t := range tpls {
		sp.Start(fmt.Sprintf("Creating %s...", t.label))
		if err := renderTemplate(t.tpl, t.out, data); err != nil {
			sp.Fail(fmt.Sprintf("Failed to create %s: %v", t.label, err))
			return fmt.Errorf("%s: %w", t.label, err)
		}
		sp.Step(fmt.Sprintf("✔ %s created", t.label), "")
	}
	return nil
}

// ── Module generator ────────────────────────────────────────────────

func generateModule(sp *common.Spinner, data *TemplateData) error {
	moduleDir := data.target().ModuleDir(data.ModuleName)
	out := filepath.Join(moduleDir, data.ModuleName+".module.go")

	sp.Start("Creating module registration...")
	if err := renderTemplate("templates/res/module.go.tpl", out, data); err != nil {
		sp.Fail(fmt.Sprintf("Failed to create module: %v", err))
		return fmt.Errorf("module: %w", err)
	}
	sp.Step("✔ module created", "")
	return nil
}

func generateMigration(sp *common.Spinner, data *TemplateData) error {
	moduleDir := data.target().ModuleDir(data.ModuleName)
	out := filepath.Join(moduleDir, "migrations", "000_create_"+data.TableName+".sql")

	sp.Start("Creating SQL migration...")
	if err := renderTemplate("templates/res/sql/migration.sql.tpl", out, data); err != nil {
		sp.Fail(fmt.Sprintf("Failed to create SQL migration: %v", err))
		return fmt.Errorf("migration: %w", err)
	}
	sp.Step("✔ SQL migration created", "")
	return nil
}

func resourceTemplate(data *TemplateData, path string) string {
	if data.Database.IsSQL() {
		return filepath.Join("templates", "res", "sql", path)
	}
	return filepath.Join("templates", "res", path)
}

// ── Single-purpose generators ───────────────────────────────────────

// but reuses the same template.
func runControllerOnly(sp *common.Spinner, modulePath string, data *TemplateData) error {
	common.Section("Generating Controller")
	return generateController(sp, data)
}
func runResponseOnly(sp *common.Spinner, modulePath string, data *TemplateData) error {
	common.Section("Generating Response")
	return generateResponse(sp, data)
}

func runServiceOnly(sp *common.Spinner, modulePath string, data *TemplateData) error {
	common.Section("Generating Services")
	return generateServices(sp, data)
}

func runDTOOnly(sp *common.Spinner, modulePath string, data *TemplateData) error {
	common.Section("Generating DTOs")
	return generateDTOs(sp, data)
}

// ── Tree printer ────────────────────────────────────────────────────

// printTree prints the generated file structure for the module.
func printTree(app AppTarget, module string, database DatabaseType) {
	base := app.ModuleDir(module)
	pkg := modelPkg(database)
	files := []string{
		filepath.Join(base, module+".module.go"),
		filepath.Join(base, pkg, module+".model.go"),
		filepath.Join(base, pkg, module+".repository.go"),
		filepath.Join(base, pkg, module+".repository.interface.go"),
		filepath.Join(base, "dto", "create.dto.go"),
		filepath.Join(base, "dto", "update.dto.go"),
		filepath.Join(base, "dto", "findone.dto.go"),
		filepath.Join(base, "dto", "find.dto.go"),
		filepath.Join(base, "controllers", module+".controller.go"),
		filepath.Join(base, "controllers", "create.go"),
		filepath.Join(base, "controllers", "find-one.go"),
		filepath.Join(base, "controllers", "find.go"),
		filepath.Join(base, "controllers", "delete.go"),
		filepath.Join(base, "controllers", "update.go"),
		filepath.Join(base, "response", module+".response.go"),
		filepath.Join(base, "response", module+".mapper.go"),

		filepath.Join(base, "services", module+".service.go"),
		filepath.Join(base, "services", "create.go"),
		filepath.Join(base, "services", "findone.go"),
		filepath.Join(base, "services", "find.go"),
		filepath.Join(base, "services", "delete.go"),
		filepath.Join(base, "services", "update.go"),
	}
	for _, f := range files {
		fmt.Printf("    %s\n", f)
	}
	if database.IsSQL() {
		fmt.Printf("    %s\n", filepath.Join(base, "migrations", "000_create_"+pluralize(module)+".sql"))
	}
}
