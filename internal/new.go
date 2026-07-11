package internal

import (
	"fmt"

	"github.com/nika-framework/nika-cli/common"
)

// CreateApp holds the parameters needed to scaffold a new Nika project.
type CreateApp struct {
	Name string
}

// Runner abstracts shell/git operations for testability.
type Runner interface {
	GitAvailable() bool
	GitClone(repoURL, targetDir string) error
	GitRemoveDir(dir string) error
	GitInit(dir string) error
}

// FileOps abstracts file system operations for testability.
type FileOps interface {
	ReplaceInFile(path, old, repl string) error
}

// realRunner implements Runner using the common package functions.
type realRunner struct{}

func (r *realRunner) GitAvailable() bool          { return common.IsGitAvailable() }
func (r *realRunner) GitClone(u, d string) error  { return common.GitClone(u, d) }
func (r *realRunner) GitRemoveDir(d string) error { return common.RemoveGitDir(d) }
func (r *realRunner) GitInit(d string) error      { return common.GitInit(d) }

// realFileOps implements FileOps using the common package functions.
type realFileOps struct{}

func (f *realFileOps) ReplaceInFile(p, o, n string) error {
	return common.ReplaceInFile(p, o, n)
}

// RunNewProject executes the full step-by-step project creation flow.
func RunNewProject(app *CreateApp, runner Runner, fops FileOps) error {
	if runner == nil {
		runner = &realRunner{}
	}
	if fops == nil {
		fops = &realFileOps{}
	}
	sp := common.NewSpinner()
	// ── Step 1: Validate name ──────────────────────────────────────
	common.Section("Project Validation")
	sp.Start(fmt.Sprintf("Validating project name \"%s\"...", app.Name))
	appName, err := common.ValidateProjectName(app.Name)
	if err != nil {
		return fmt.Errorf("validation: %w", err)
	}
	sp.Step("Project name is valid", "")

	// ── Step 2: Check git ──────────────────────────────────────────
	common.Section("Environment Check")
	sp.Start("Checking git availability...")
	if !runner.GitAvailable() {
		sp.Stop("Git not found — continuing without initial commit")
	} else {
		sp.Step("Git is available", "")
	}

	// ── Step 3: Clone ──────────────────────────────────────────────
	common.Section("Scaffolding")
	sp.Start(fmt.Sprintf("Cloning Nika template into %q...", appName))
	if err := runner.GitClone("https://github.com/nika-framework/nika-app.git", "./"+appName); err != nil {
		sp.Fail(fmt.Sprintf("Clone failed: %v", err))
		return fmt.Errorf("clone: %w", err)
	}
	sp.Step("Template cloned successfully", "Cleaning up template .git history...")

	// ── Step 4: Remove old .git ────────────────────────────────────
	if err := runner.GitRemoveDir(appName); err != nil {
		sp.Fail(fmt.Sprintf("Failed to remove .git: %v", err))
		return fmt.Errorf("remove .git: %w", err)
	}
	sp.Step("Template .git removed", "Initializing fresh git repository...")

	// ── Step 5: Git init ──────────────────────────────────────────
	if runner.GitAvailable() {
		if err := runner.GitInit(appName); err != nil {
			sp.Fail(fmt.Sprintf("git init failed: %v", err))
			return fmt.Errorf("git init: %w", err)
		}
		sp.Step("Fresh git repository initialized", "Customizing project files...")
	} else {
		sp.Step("Skipped git init (git not found)", "Customizing project files...")
	}

	// ── Step 6: Customize go.mod ──────────────────────────────────
	moduleName := appName
	goModPath := appName + "/go.mod"
	if err := fops.ReplaceInFile(goModPath, "module NikaApp", "module "+moduleName); err != nil {
		sp.Fail(fmt.Sprintf("Failed to update go.mod: %v", err))
		return fmt.Errorf("go.mod: %w", err)
	}
	sp.Step("go.mod updated", "Updating import paths...")

	// ── Step 7: Fix app.controller.go import ─────────────────────
	controllerPath := appName + "/src/app.controller.go"
	if err := fops.ReplaceInFile(controllerPath, "NikaApp/src/dto", appName+"/src/dto"); err != nil {
		sp.Fail(fmt.Sprintf("Failed to update app.controller.go: %v", err))
		return fmt.Errorf("app.controller.go: %w", err)
	}
	sp.Step("app.controller.go imports updated", "Updating main.go...")

	// ── Step 8: Fix main.go import ─────────────────────────────────
	mainPath := appName + "/main.go"
	if err := fops.ReplaceInFile(mainPath, "NikaApp/src", appName+"/src"); err != nil {
		sp.Fail(fmt.Sprintf("Failed to update main.go: %v", err))
		return fmt.Errorf("main.go: %w", err)
	}
	sp.Stop("main.go imports updated")

	// ── Done ──────────────────────────────────────────────────────
	common.Section("Done!")
	fmt.Println()
	fmt.Printf("  📦 Project %q created successfully!\n", appName)
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Printf("    cd %s\n", appName)
	fmt.Println("    go mod download")
	fmt.Println("    go mod tidy")
	fmt.Println("    mv .env.example .env")
	fmt.Println("    nika start . --watch")
	fmt.Println()

	return nil
}
