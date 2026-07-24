package internal

import (
	"fmt"
	"os"

	"github.com/nika-framework/nika-cli/common"
)

// SetupAgent writes agent custom settings and instructions to the target project.
func SetupAgent() error {
	sp := common.NewSpinner()

	// Check for go.mod to ensure we're inside a project root
	sp.Start("Checking for project environment...")
	if _, err := os.Stat("go.mod"); err != nil {
		sp.Fail("go.mod not found — run this command inside a Nika project root")
		return fmt.Errorf("not in a Go project (no go.mod)")
	}
	sp.Step("Found project environment (go.mod)", "Setting up AI agent configuration...")

	// Create directories: .github/agents, .github/instructions, .github/prompts, .github/skills/nika-gen-skill
	dirs := []string{
		".github/agents",
		".github/instructions",
		".github/prompts",
		".github/skills/nika-gen-skill",
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			sp.Fail(fmt.Sprintf("Failed to create directory %s: %v", d, err))
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	// 1. Write .github/agents/nika-agent.agent.md
	agentPath := ".github/agents/nika-agent.agent.md"
	agentContent := `---
name: nika-agent
description: "An intelligent agent for generating and developing modules and resources for the Nika Framework with custom fields"
tools: [read, edit, search, execute]
---
You are an intelligent assistant and expert in developing the Nika Framework. Your job is to create or extend modules, controllers, services, DTOs, and database models precisely based on the user's requirements.

When a user asks you to build a module with specific fields (for example, a book module with title and price fields), follow these steps:
1. First, inspect the Nika project structure (there must be a src folder and a go.mod file).
2. Choose the module name in lowercase and a proper format (for example, book).
3. Use file creation tools to generate the following files in the user's project:
   - Model and repository at src/<module>/schema/<module>.model.go, src/<module>/schema/<module>.repository.go, and src/<module>/schema/<module>.repository.interface.go
   - DTOs at src/<module>/dto/create.dto.go, src/<module>/dto/update.dto.go, src/<module>/dto/findone.dto.go, and src/<module>/dto/find.dto.go
   - Controller at src/<module>/controllers/<module>.controller.go and the CRUD methods (create.go, find.go, find-one.go, update.go, delete.go)
   - Service at src/<module>/services/<module>.service.go and the CRUD methods
   - Responses at src/<module>/response/<module>.response.go and src/<module>/response/<module>.mapper.go
   - The registry module at src/<module>/<module>.module.go
4. Import and register the new module in src/app.module.go so it is added to the application.

Important rules:
- Include all user-requested fields with the proper data type in the database model, responses, DTOs, mapper functions, and services.
- Before generating a module, identify its database: MongoDB, PostgreSQL, MySQL, or SQLite. MongoDB models use ` + "`bson`" + ` and ` + "`json`" + ` tags with ` + "`primitive.ObjectID`" + ` IDs; SQL models use ` + "`db`" + ` and ` + "`json`" + ` tags with ` + "`int64`" + ` IDs and ` + "`common/sqldb/repository`" + `.
- Map database types correctly (for example string, int, float64, time.Time, bool).
- **Multilingual support (Localization / Translation)**: When the user requests multilingual or translatable fields (for example: translate:[{lang:"fa", title:"اتوبوسرانی"},{lang:"en", title:"Bus"}]):
  - In the database model, define an array field named Translations containing items with a Lang field and the translated fields (e.g. Title).
  - In controllers, read the language header (e.g. Accept-Language or a custom lang header) from the incoming request via ctx.GetHeader("Accept-Language").
  - Pass that language value down to the service layer, and from there to the response mapper (either as an argument or through the context).
  - In the mapper (response/<module>.mapper.go), apply the filter: if a language header was provided, extract only the translation matching that language and populate the main response fields with it. Otherwise (no language header sent), return the full list of translations.
  - Update the Swagger annotations on controller methods so the language header is documented and accepted (e.g. @Param Accept-Language header string false "Response language").
  - Add the Accept-Language header param to **every** CRUD endpoint (create, find-one, find, update, delete) so language filtering works across the whole module.
- Always make sure generated code matches the current project structure.
- Package names must match the folder structure (for example package controllers or package services).
- Fix all imports according to the module path declared in go.mod.
- After building the module, register it in the main project module (src/app.module.go).
- Prefer writing files directly and deliver the work complete and ready to run.
- Provide clear answers in English.
- Never leave a change half-finished.
`

	if err := common.WriteFile(agentPath, agentContent); err != nil {
		sp.Fail(fmt.Sprintf("Failed to write %s: %v", agentPath, err))
		return fmt.Errorf("write %s: %w", agentPath, err)
	}
	fmt.Println("  ✔ Created .github/agents/nika-agent.agent.md")

	// 2. Write .github/instructions/nika.instructions.md
	instPath := ".github/instructions/nika.instructions.md"
	instContent := `---
name: nika-instructions
description: "General guidelines for developing modules and features in Nika Framework projects"
applyTo: "src/**/*.go"
---
# Nika Framework Development Guidelines

When editing or creating Go files inside the ` + "`src`" + ` folder of a Nika project, follow these rules:

1. **Modular architecture**:
   - Each module must live in its own folder under ` + "`src`" + ` (for example ` + "`src/product`" + `).
   - Each module's internal structure contains the sub-folders ` + "`schema`" + ` (database/repository), ` + "`dto`" + ` (input definitions), ` + "`controllers`" + ` (handlers/routes), ` + "`services`" + ` (business logic), and ` + "`response`" + ` (outputs).

2. **Layer rules**:
	- **Schema/Model**: Match the module database. MongoDB uses ` + "`bson`" + ` and ` + "`json`" + ` tags with ` + "`primitive.ObjectID`" + `; PostgreSQL, MySQL, and SQLite use ` + "`db`" + ` and ` + "`json`" + ` tags with ` + "`int64`" + ` IDs and the SQL repository.
   - **DTOs**: The ` + "`validator`" + ` library is supported. For example use tags like ` + "`validate:\"required,min=1\"`" + `.
   - **Controllers**: They are written using the Gin framework. Route binding is declared via a tag on the controller's function fields (for example ` + "`route:\"POST:/products\"`" + `).
   - **Services**: All core business logic, including communication with the repository, must live in the service layer.
   - **Response**: Data must always be mapped to response models — never send the database model directly to the output. Use mapper functions.

3. **Module registration**:
   - The module name must be added to ` + "`src/app.module.go`" + ` inside the ` + "`Imports`" + ` method so it is properly wired into the Nika dependency injection cycle.

4. **Multilingual / translation fields**:
   - When a field is translatable (e.g. a ` + "`Title`" + ` with multiple languages), store it as an array under ` + "`Translations`" + ` where each item has a ` + "`Lang`" + ` code and the translated value (e.g. ` + "`{lang:\"fa\", title:\"اتوبوسرانی\"}`" + `).
   - Read the requested language from the ` + "`Accept-Language`" + ` request header in controllers (` + "`ctx.GetHeader(\"Accept-Language\")`" + `).
   - Forward the language to the service and then to the response mapper. In the mapper, if a language is provided, return only the matching translation in the main fields; otherwise return the full ` + "`Translations`" + ` array.
   - Document the header in Swagger with ` + "`@Param Accept-Language header string false \"Response language\"`" + ` on every CRUD endpoint.
`

	if err := common.WriteFile(instPath, instContent); err != nil {
		sp.Fail(fmt.Sprintf("Failed to write %s: %v", instPath, err))
		return fmt.Errorf("write %s: %w", instPath, err)
	}
	fmt.Println("  ✔ Created .github/instructions/nika.instructions.md")

	// 3. Write .github/prompts/generate-module.prompt.md
	promptPath := ".github/prompts/generate-module.prompt.md"
	promptContent := `---
description: "Automatically generate a Nika module with custom fields using AI"
argument-hint: "Write the module name and its fields. Example: book module with a title (string) and price (int)"
agent: "nika-agent"
---
Please create a complete Nika module for the following entity:
{{.argument}}

Requirements:
- Create all parts: model, service, controller, responses, DTOs, and the main module file.
- Implement the requested fields correctly across the structs and mappers.
- If the entity has multilingual fields (e.g. ` + "`translate:[{lang:\"fa\", title:\"اتوبوسرانی\"},{lang:\"en\", title:\"Bus\"}]`" + `), store them in a ` + "`Translations`" + ` array, read the ` + "`Accept-Language`" + ` header in every controller, forward it to the mapper, and return only the matching translation when a language is provided (otherwise return all translations). Document the header in the Swagger annotations.
- Register the created module as an import in ` + "`src/app.module.go`" + `.
- After finishing, list the files that were created.
`

	if err := common.WriteFile(promptPath, promptContent); err != nil {
		sp.Fail(fmt.Sprintf("Failed to write %s: %v", promptPath, err))
		return fmt.Errorf("write %s: %w", promptPath, err)
	}
	fmt.Println("  ✔ Created .github/prompts/generate-module.prompt.md")

	// 4. Write .github/skills/nika-gen-skill/SKILL.md
	skillPath := ".github/skills/nika-gen-skill/SKILL.md"
	skillContent := `---
name: nika-gen-skill
description: "Develop and create Nika Framework modules. Use this skill when the user wants to generate custom fields."
user-invocable: true
---
# Nika Module Generation Skill

This skill helps the AI agent build new modules in a Nika project with fields specified by the user.

## When to use
- When the user says: "build a module with these fields"
- When the user needs to generate a controller, service, or a whole resource automatically.

## Execution steps
1. Inspect the project ` + "`src`" + ` folder and read the module path from ` + "`go.mod`" + `.
2. Create the folder for the new module (for example ` + "`src/todo`" + `).
3. Create the required files based on the Nika templates:
   - Model and repository in ` + "`src/<module>/schema/`" + `
   - Input DTOs in ` + "`src/<module>/dto/`" + `
   - Controller and routes in ` + "`src/<module>/controllers/`" + `
   - Services in ` + "`src/<module>/services/`" + `
   - Responses and data mappers in ` + "`src/<module>/response/`" + `
   - The module class in ` + "`src/<module>/<module>.module.go`" + `
4. Add the module import to the main application settings file (` + "`src/app.module.go`" + `).

## Multilingual fields
- When the user requests a translatable field (e.g. ` + "`translate:[{lang:\"fa\", title:\"اتوبوسرانی\"},{lang:\"en\", title:\"Bus\"}]`" + `):
  - Store translations as a ` + "`Translations`" + ` array on the model, where each entry has a ` + "`Lang`" + ` and the translated field (e.g. ` + "`Title`" + `).
  - Read the ` + "`Accept-Language`" + ` header in every controller handler (` + "`ctx.GetHeader(\"Accept-Language\")`" + `).
  - Forward the language to the mapper: if a language is provided, return only the matching translation in the main response fields; otherwise return the full translations list.
  - Document the header in Swagger annotations on every endpoint: ` + "`@Param Accept-Language header string false \"Response language\"`" + `.
`

	if err := common.WriteFile(skillPath, skillContent); err != nil {
		sp.Fail(fmt.Sprintf("Failed to write %s: %v", skillPath, err))
		return fmt.Errorf("write %s: %w", skillPath, err)
	}
	fmt.Println("  ✔ Created .github/skills/nika-gen-skill/SKILL.md")

	sp.Stop("AI Agent configurations installed successfully!")
	fmt.Println("\n  🎉 You can now use AI agents in your editor (Cursor/Copilot) to automatically generate Nika modules!")
	return nil
}
