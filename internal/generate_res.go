package internal

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sajadweb/nika-cli/common"
)

// ── Field Definition ────────────────────────────────────────────────
var templateFS embed.FS

// Field represents a single model field collected from the user.
type Field struct {
	Name      string // Go field name, e.g. "FirstName"
	BsonName  string // bson tag, e.g. "first_name"
	Type      string // Go type, e.g. "string"
	Required  bool
	ModelTag  string // tag for model struct (bson+json)
	CreateTag string // tag for create DTO (json+validate)
	UpdateTag string // tag for update DTO (json+validate omitempty)
}

// TemplateData is the payload passed to all .tpl files.
type TemplateData struct {
	ModulePath     string  // full module path from go.mod, e.g. "github.com/sajadweb/my-app"
	ModuleName     string  // e.g. "user"
	TypeName       string  // e.g. "User" (exported, PascalCase)
	CollectionName string  // e.g. "users"
	Fields         []Field // user-defined fields (excluding ID/CreatedAt/UpdatedAt)
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

func assets(tpl string) string {
	 _, filename, _, ok := runtime.Caller(0)
	 if !ok { 
        return "templates"
    }
	fmt.Printf("Assets in filename %s \n",filename)
	 dir := filepath.Dir(filename)
	 dir =strings.ReplaceAll(dir,"/internal","/")
	 fmt.Printf("Assets Dir in  %s \n",dir)
	 fmt.Printf("Assets File in filepath %s \n",filepath.Join(dir, tpl))
	 
	return filepath.Join(dir, tpl)
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
func collectFields(sp *common.Spinner) []Field {
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
		typeChoice := common.SelectOption("Select type for "+name, mongoTypes)

		// Required?
		required := common.ConfirmYesNo(fmt.Sprintf("Is %q required?", name))

		bsonName := name
		pascalName := toPascalCase(name)

		f := Field{
			Name:      pascalName,
			BsonName:  bsonName,
			Type:      typeChoice,
			Required:  required,
			ModelTag:  fmt.Sprintf(`bson:"%s" json:"%s"`, bsonName, bsonName),
			CreateTag: fmt.Sprintf(`json:"%s" validate:"%s"`, bsonName, mongoTypeDefaultValidate(typeChoice, true)),
			UpdateTag: fmt.Sprintf(`json:"%s,omitempty" validate:"omitempty"`, bsonName),
		}

		fields = append(fields, f)
		fmt.Printf("  ✔ Field %q (%s, required=%v) added\n", name, typeChoice, required)
	}

	return fields
}

// ── Main entry point ────────────────────────────────────────────────

// GenerateConfig holds the parameters for a generate run.
type GenerateConfig struct {
	Type   GenerateType
	Module string // raw module name from args
}

// RunGenerate is the top-level entry for `nika g <type> <module>`.
func RunGenerate(cfg *GenerateConfig) error {
	// Step 1: validate environment
	common.Section("Environment Check")
	sp := common.NewSpinner()
	// sp.Stop("")
	// sp.Start("Checking for go.mod...")
	if _, err := os.Stat("go.mod"); err != nil {
		// sp.Fail("go.mod not found — run this command inside a Nika project root")
		return fmt.Errorf("not in a Go project (no go.mod)")
	}
	modulePath, err := ResolveModulePath()
	if err != nil {
		// sp.Fail(fmt.Sprintf("Failed to read module path: %v", err))
		return fmt.Errorf("module path: %w", err)
	}
	// sp.Step(fmt.Sprintf("Module: %s", modulePath), "Validating module name...")

	// Step 2: validate module name
	moduleName := strings.ToLower(strings.TrimSpace(cfg.Module))
	if moduleName == "" {
		// sp.Fail("module name is required")
		return fmt.Errorf("module name is required")
	}
	if !isValidModule(moduleName) {
		// sp.Fail(fmt.Sprintf("invalid module name %q — use lowercase letters, digits, underscores", moduleName))
		return fmt.Errorf("invalid module name: %s", moduleName)
	}
	// sp.Step(fmt.Sprintf("Module name %q is valid", moduleName), "")

	// Build template data
	data := TemplateData{
		ModulePath:     modulePath,
		ModuleName:     moduleName,
		TypeName:       toPascalCase(moduleName),
		CollectionName: pluralize(moduleName),
	}

	fmt.Println(data.CollectionName, cfg.Type)
	// Step 3: dispatch by type
	switch cfg.Type {
	case GenResource, GenRes:
		return runResource(sp, modulePath, &data)
	case GenController, GenC:
		return runControllerOnly(sp, modulePath, &data)
	case GenService, GenS:
		return runServiceOnly(sp, modulePath, &data)
	case GenDTO, GenD:
		return runDTOOnly(sp, modulePath, &data)
	default:
		return fmt.Errorf("unknown generate type: %s", cfg.Type)
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
	// Step 4: choose database
	common.Section("Database Selection")
	dbChoice := common.SelectOption(
		"Select database for "+data.ModuleName,
		[]string{"MongoDB"},
	)
	fmt.Printf("Database: %s selected", dbChoice)
	// sp.Start(fmt.Sprintf("Database: %s selected", dbChoice))
	// sp.Step("Database configured", "Collecting model fields...")

	// Step 5: collect fields
	common.Section("Model Fields")
	fields := collectFields(sp)
	data.Fields = fields
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

	// Step 10: generate module
	common.Section("Generating Module")
	if err := generateModule(sp, data); err != nil {
		return err
	}

	// Step 11: done
	common.Section("Done!")
	fmt.Println()
	fmt.Printf("  📦 Resource %q generated successfully!\n", data.ModuleName)
	fmt.Println()
	fmt.Println("  Files created:")
	printTree(data.ModuleName)
	fmt.Println()
	fmt.Printf("  ⚠ Don't forget to import %sModule in your app.module.go Imports()!\n", data.TypeName)
	fmt.Printf("     import \"%s/src/%s\"\n", modulePath, data.ModuleName)
	fmt.Printf("     // then add: %s.New%sModule(),\n", data.ModuleName, data.TypeName)
	fmt.Println()
	return nil
}

// ── Schema generator ────────────────────────────────────────────────

func generateSchema(sp *common.Spinner, data *TemplateData) error {
	base := filepath.Join("src", data.ModuleName, "schema")
	tpls := []struct {
		tpl   string
		out   string
		label string
	}{
		{"templates/res/schema/model.go.tpl", filepath.Join(base, data.ModuleName+".model.go"), "model"},
		{"templates/res/schema/repository.go.tpl", filepath.Join(base, data.ModuleName+".repository.go"), "repository"},
		{"templates/res/schema/repository.interface.go.tpl", filepath.Join(base, data.ModuleName+".repository.interface.go"), "repository interface"},
	}

	for _, t := range tpls {
		sp.Start(fmt.Sprintf("Creating %s (%s)...", t.label, filepath.Base(t.out)))
		if err := common.RenderToFile(assets(t.tpl), t.out, data); err != nil {
			sp.Fail(fmt.Sprintf("Failed to create %s: %v", t.label, err))
			return fmt.Errorf("schema %s: %w", t.label, err)
		}
		sp.Step(fmt.Sprintf("✔ %s created", t.label), "")
	}
	return nil
}

// ── DTO generator ───────────────────────────────────────────────────

func generateDTOs(sp *common.Spinner, data *TemplateData) error {
	base := filepath.Join("src", data.ModuleName, "dto")
	tpls := []struct {
		tpl   string
		out   string
		label string
	}{
		{"templates/res/dto/create.dto.go.tpl", filepath.Join(base, "create.dto.go"), "create DTO"},
		{"templates/res/dto/update.dto.go.tpl", filepath.Join(base, "update.dto.go"), "update DTO"},
		{"templates/res/dto/findone.dto.go.tpl", filepath.Join(base, "findone.dto.go"), "findone DTO"},
		{"templates/res/dto/find.dto.go.tpl", filepath.Join(base, "find.dto.go"), "find/list DTO"},
	}

	for _, t := range tpls {
		sp.Start(fmt.Sprintf("Creating %s...", t.label))
		if err := common.RenderToFile(assets(t.tpl), t.out, data); err != nil {
			sp.Fail(fmt.Sprintf("Failed to create %s: %v", t.label, err))
			return fmt.Errorf("dto %s: %w", t.label, err)
		}
		sp.Step(fmt.Sprintf("✔ %s created", t.label), "")
	}
	return nil
}

// ── Service generator ───────────────────────────────────────────────

func generateServices(sp *common.Spinner, data *TemplateData) error {
	base := filepath.Join("src", data.ModuleName, "services")
	tpls := []struct {
		tpl   string
		out   string
		label string
	}{
		{"templates/res/service/service.go.tpl", filepath.Join(base, data.ModuleName+".service.go"), "service base"},
		{"templates/res/service/create.go.tpl", filepath.Join(base, "create.go"), "create method"},
		{"templates/res/service/findone.go.tpl", filepath.Join(base, "findone.go"), "findone method"},
		{"templates/res/service/find.go.tpl", filepath.Join(base, "find.go"), "find method"},
		{"templates/res/service/delete.go.tpl", filepath.Join(base, "delete.go"), "delete method"},
	}

	for _, t := range tpls {
		sp.Start(fmt.Sprintf("Creating %s...", t.label))
		if err := common.RenderToFile(assets(t.tpl), t.out, data); err != nil {
			sp.Fail(fmt.Sprintf("Failed to create %s: %v", t.label, err))
			return fmt.Errorf("service %s: %w", t.label, err)
		}
		sp.Step(fmt.Sprintf("✔ %s created", t.label), "")
	}
	return nil
}

// ── Controller generator ────────────────────────────────────────────

func generateController(sp *common.Spinner, data *TemplateData) error {
	base := filepath.Join("src", data.ModuleName, "controllers")
	out := filepath.Join(base, data.ModuleName+".controller.go")

	sp.Start("Creating controller...")
	if err := common.RenderToFile(assets("templates/res/controller/controller.go.tpl"), out, data); err != nil {
		sp.Fail(fmt.Sprintf("Failed to create controller: %v", err))
		return fmt.Errorf("controller: %w", err)
	}
	sp.Step("✔ controller created", "")
	return nil
}

// ── Module generator ────────────────────────────────────────────────

func generateModule(sp *common.Spinner, data *TemplateData) error {
	out := filepath.Join("src", data.ModuleName, data.ModuleName+".module.go")

	sp.Start("Creating module registration...")
	if err := common.RenderToFile(assets("templates/res/module.go.tpl"), out, data); err != nil {
		sp.Fail(fmt.Sprintf("Failed to create module: %v", err))
		return fmt.Errorf("module: %w", err)
	}
	sp.Step("✔ module created", "")
	return nil
}

// ── Single-purpose generators ───────────────────────────────────────

// runControllerOnly generates just the controller. It needs fields for nothing,
// but reuses the same template.
func runControllerOnly(sp *common.Spinner, modulePath string, data *TemplateData) error {
	common.Section("Generating Controller")
	return generateController(sp, data)
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
func printTree(module string) {
	base := filepath.Join("src", module)
	files := []string{
		filepath.Join(base, module+".module.go"),
		filepath.Join(base, "schema", module+".model.go"),
		filepath.Join(base, "schema", module+".repository.go"),
		filepath.Join(base, "schema", module+".repository.interface.go"),
		filepath.Join(base, "dto", "create.dto.go"),
		filepath.Join(base, "dto", "update.dto.go"),
		filepath.Join(base, "dto", "findone.dto.go"),
		filepath.Join(base, "dto", "find.dto.go"),
		filepath.Join(base, "controllers", module+".controller.go"),
		filepath.Join(base, "services", module+".service.go"),
		filepath.Join(base, "services", "create.go"),
		filepath.Join(base, "services", "findone.go"),
		filepath.Join(base, "services", "find.go"),
		filepath.Join(base, "services", "delete.go"),
	}
	for _, f := range files {
		fmt.Printf("    %s\n", f)
	}
}
