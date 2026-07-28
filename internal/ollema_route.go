package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nika-framework/nika-cli/common"
)

type aiRouteSpec struct {
	Operation  string                     `json:"operation"`
	Module     string                     `json:"module"`
	RouteName  string                     `json:"route_name"`
	HTTPMethod string                     `json:"http_method"`
	Path       string                     `json:"path"`
	Values     map[string]json.RawMessage `json:"values"`
	Questions  []aiRouteQuestion          `json:"questions"`
}

type aiRouteQuestion struct {
	Field    string `json:"field"`
	Question string `json:"question"`
}

type controllerInfo struct {
	Type string
	File string
}

// RunOllemaRoute finds a module controller and adds an AI-planned mock-data route.
func RunOllemaRoute(model, userPrompt string, output io.Writer) error {
	return runOllemaRoute(agentRuntime{Provider: "ollama", Model: model}, userPrompt, output)
}

func runOllemaRoute(runtime agentRuntime, userPrompt string, output io.Writer) error {
	plan, err := planOllemaRoute(runtime, userPrompt, "")
	if err != nil {
		return err
	}
	plan.Module = strings.ToLower(strings.TrimSpace(plan.Module))
	if !isValidModule(plan.Module) {
		return fmt.Errorf("Ollama returned invalid module name %q", plan.Module)
	}
	moduleName := plan.Module

	workspace, err := LoadWorkspace()
	if err != nil {
		return err
	}
	// Look for the module in every app before asking. In a workspace the
	// module name alone usually identifies the service, and a prompt the user
	// can answer wrong is worse than a search that cannot.
	app, ok := workspace.FindModule(plan.Module)
	if !ok {
		app, err = workspace.SelectApp("")
		if err != nil {
			return err
		}
	}
	moduleDir := app.ModuleDir(plan.Module)
	if info, err := os.Stat(moduleDir); err != nil || !info.IsDir() {
		return fmt.Errorf("module %q was not found at %s", plan.Module, moduleDir)
	}

	modelPath, err := findModuleModel(moduleDir, plan.Module)
	if err != nil {
		return err
	}
	modelSource, err := common.ReadFile(modelPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", modelPath, err)
	}
	controllers, err := findControllers(filepath.Join(moduleDir, "controllers"))
	if err != nil {
		return err
	}
	if len(controllers) == 0 {
		return fmt.Errorf("no controllers found for module %q", plan.Module)
	}
	selected := controllers[0]
	if len(controllers) > 1 {
		options := make([]string, len(controllers))
		for i, controller := range controllers {
			options[i] = controller.Type + " (" + controller.File + ")"
		}
		choice := common.SelectOption("Which controller should receive the route?", options)
		for i, option := range options {
			if option == choice {
				selected = controllers[i]
				break
			}
		}
	}

	context := fmt.Sprintf("Module model source:\n%s\n\nSelected controller: %s\nUser request:\n%s", modelSource, selected.Type, userPrompt)
	plan, err = planOllemaRoute(runtime, userPrompt, context)
	if err != nil {
		return err
	}
	if err := validateRoutePlan(&plan, moduleName); err != nil {
		return err
	}
	plan.Module = moduleName

	for _, question := range plan.Questions {
		if strings.TrimSpace(question.Field) == "" || strings.TrimSpace(question.Question) == "" {
			continue
		}
		answer := common.PromptRequired(question.Question)
		raw, _ := json.Marshal(answer)
		plan.Values[question.Field] = raw
	}

	routeSource, err := buildMockRoute(plan, selected.Type, plan.Module, modelSource, workspace.ModulePath, app.SrcImport())
	if err != nil {
		return err
	}
	routeFile := filepath.Join(moduleDir, "controllers", "mock.go")
	if _, err := os.Stat(routeFile); err == nil {
		return fmt.Errorf("route file already exists: %s", routeFile)
	}
	if err := common.WriteFile(routeFile, routeSource); err != nil {
		return fmt.Errorf("write route: %w", err)
	}
	if err := registerControllerRoute(selected, plan, moduleDir); err != nil {
		return fmt.Errorf("register route: %w (the new route file was kept at %s)", err, routeFile)
	}

	_, err = fmt.Fprintf(output, "Route %s %s added to %s in module %q.\n", plan.HTTPMethod, plan.Path, selected.Type, plan.Module)
	return err
}

func planOllemaRoute(runtime agentRuntime, userPrompt, context string) (aiRouteSpec, error) {
	instruction := `Return JSON only for a request to add a mock-data route to an existing Go Nika module.
Use this exact shape:
{"operation":"mock_data","module":"news","route_name":"CreateMock","http_method":"POST","path":"/newss/mock","values":{"title":"Mock title"},"questions":[]}
The values object must contain JSON values suitable for the model fields. Use questions only when a value cannot be reasonably inferred; each question has a field and question.
The route must create one document by calling the module service's Create method and return the normal response.
Use an exported PascalCase route_name ending in Mock, HTTP method POST, and a path beginning with /.

User request:
` + userPrompt
	if context != "" {
		instruction += "\n\nAdditional project context:\n" + context
	}
	response, err := askAgent(runtime, instruction, "json")
	if err != nil {
		return aiRouteSpec{}, err
	}
	var plan aiRouteSpec
	if err := json.Unmarshal([]byte(response.Response), &plan); err != nil {
		return aiRouteSpec{}, fmt.Errorf("Ollama returned invalid route JSON: %w", err)
	}
	return plan, nil
}

func validateRoutePlan(plan *aiRouteSpec, module string) error {
	if plan.Operation != "mock_data" {
		return fmt.Errorf("Ollama returned unsupported route operation %q", plan.Operation)
	}
	if plan.Module != "" && strings.ToLower(strings.TrimSpace(plan.Module)) != module {
		return fmt.Errorf("Ollama changed the module from %q to %q", module, plan.Module)
	}
	if !isValidIdentifier(plan.RouteName) || !strings.HasSuffix(plan.RouteName, "Mock") {
		return fmt.Errorf("Ollama returned invalid route name %q", plan.RouteName)
	}
	if strings.ToUpper(plan.HTTPMethod) != "POST" {
		return fmt.Errorf("mock-data route must use POST, got %q", plan.HTTPMethod)
	}
	plan.HTTPMethod = "POST"
	if !strings.HasPrefix(plan.Path, "/") || strings.ContainsAny(plan.Path, "\r\n\"`") {
		return fmt.Errorf("Ollama returned invalid route path %q", plan.Path)
	}
	if plan.Values == nil {
		plan.Values = make(map[string]json.RawMessage)
	}
	return nil
}

func findControllers(dir string) ([]controllerInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read controllers directory: %w", err)
	}
	pattern := regexp.MustCompile(`(?m)^type\s+([A-Za-z][A-Za-z0-9_]*)Controller\s+struct\s*\{`)
	var result []controllerInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file := filepath.Join(dir, entry.Name())
		source, err := common.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read controller %s: %w", file, err)
		}
		match := pattern.FindStringSubmatch(source)
		if len(match) == 2 {
			result = append(result, controllerInfo{Type: match[1] + "Controller", File: file})
		}
	}
	return result, nil
}

func buildMockRoute(plan aiRouteSpec, controllerType, module, modelSource, modulePath, srcImport string) (string, error) {
	modelType := toPascalCase(module)
	fieldPattern := regexp.MustCompile(`(?m)^\s*([A-Z][A-Za-z0-9_]*)\s+([^\s` + "`" + `]+)\s+`)
	matches := fieldPattern.FindAllStringSubmatch(modelSource, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("could not read fields from model")
	}

	var fields []string
	needsTime, needsPrimitive := false, false
	for _, match := range matches {
		name, fieldType := match[1], match[2]
		if name == "ID" || name == "CreatedAt" || name == "UpdatedAt" {
			continue
		}
		value, ok := plan.Values[strings.ToLower(name)]
		if !ok {
			value = plan.Values[name]
		}
		literal, timeImport, primitiveImport, err := mockLiteral(fieldType, name, value)
		if err != nil {
			return "", err
		}
		needsTime = needsTime || timeImport
		needsPrimitive = needsPrimitive || primitiveImport
		fields = append(fields, fmt.Sprintf("\t\t%s: %s,", name, literal))
	}

	if srcImport == "" {
		srcImport = "src"
	}
	if modulePath == "" {
		modulePath = modulePathForRoute()
	}
	var imports []string
	imports = append(imports,
		fmt.Sprintf("\t\"%s/%s/%s/dto\"", modulePath, srcImport, module),
		fmt.Sprintf("\tres \"%s/%s/%s/response\"", modulePath, srcImport, module),
		"\t\"github.com/gin-gonic/gin\"",
		"\t\"github.com/nika-framework/nika/common/response\"",
	)
	if needsTime {
		imports = append(imports, "\t\"time\"")
	}
	if needsPrimitive {
		imports = append(imports, "\t\"go.mongodb.org/mongo-driver/bson/primitive\"")
	}

	handlerName := plan.RouteName + "Handler"
	return fmt.Sprintf(`package controllers

import (
%s
)

// %s creates one mock %s document.
//
// @Summary Create mock %s
// @Description Create one mock %s document for development and testing
// @Tags %ss
// @Produce json
// @Success 201 {object} res.CreateResponse
// @Failure 400 {object} response.Error "Mock creation error"
// @Router %s [post]
func (c *%s) %s(ctx *gin.Context) {
	item, err := c.service.Create(ctx, dto.Create%sDto{
%s
	})
	if err != nil {
		response.BadRequest(ctx, "MOCK_CREATION_FAILED", err.Error())
		return
	}
	response.Create(ctx, res.CreateResponse{
		Success: true,
		Message: "MOCK_CREATE_SUCCESS",
		Data: res.To%sResponse(item),
	})
}
`, strings.Join(imports, "\n"), plan.RouteName, modelType, modelType, modelType, module, plan.Path, controllerType, handlerName, modelType, strings.Join(fields, "\n"), modelType), nil
}

func mockLiteral(fieldType, fieldName string, raw json.RawMessage) (string, bool, bool, error) {
	if len(raw) == 0 {
		switch fieldType {
		case "string":
			return strconv.Quote("Mock " + fieldName), false, false, nil
		case "[]string":
			return `[]string{"mock"}`, false, false, nil
		case "bool":
			return "false", false, false, nil
		case "int", "int64", "float64":
			return "0", false, false, nil
		case "time.Time":
			return "time.Now().UTC()", true, false, nil
		case "primitive.ObjectID":
			return "primitive.NilObjectID", false, true, nil
		case "map[string]any":
			return "map[string]any{}", false, false, nil
		}
	}
	switch fieldType {
	case "string":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", false, false, fmt.Errorf("invalid mock value for %s: %w", fieldName, err)
		}
		return strconv.Quote(value), false, false, nil
	case "[]string":
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil {
			return "", false, false, fmt.Errorf("invalid mock value for %s: %w", fieldName, err)
		}
		literals := make([]string, len(values))
		for i, value := range values {
			literals[i] = strconv.Quote(value)
		}
		return "[]string{" + strings.Join(literals, ", ") + "}", false, false, nil
	case "int", "int64", "float64":
		var number json.Number
		if err := json.Unmarshal(raw, &number); err != nil {
			return "", false, false, fmt.Errorf("invalid numeric mock value for %s: %w", fieldName, err)
		}
		return number.String(), false, false, nil
	case "bool":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", false, false, fmt.Errorf("invalid boolean mock value for %s: %w", fieldName, err)
		}
		return strconv.FormatBool(value), false, false, nil
	case "time.Time":
		return "time.Now().UTC()", true, false, nil
	case "primitive.ObjectID":
		return "primitive.NilObjectID", false, true, nil
	case "map[string]any":
		return "map[string]any{}", false, false, nil
	default:
		return "", false, false, fmt.Errorf("unsupported model field type %q", fieldType)
	}
}

func registerControllerRoute(controller controllerInfo, plan aiRouteSpec, moduleDir string) error {
	source, err := common.ReadFile(controller.File)
	if err != nil {
		return err
	}
	field := fmt.Sprintf("\n\t%s func(*gin.Context) `route:\"%s:%s\"`", plan.RouteName, plan.HTTPMethod, plan.Path)
	if strings.Contains(source, "\n\t"+plan.RouteName+" func(") {
		return fmt.Errorf("controller already contains route %s", plan.RouteName)
	}
	structPattern := regexp.MustCompile(`(?s)(type\s+` + regexp.QuoteMeta(controller.Type) + `\s+struct\s*\{.*?)(\n\})`)
	updated := structPattern.ReplaceAllString(source, "$1"+field+"$2")
	if updated == source {
		return fmt.Errorf("controller struct %s not found", controller.Type)
	}
	handlerName := plan.RouteName + "Handler"
	assignment := fmt.Sprintf("\n\tc.%s = c.%s", plan.RouteName, handlerName)
	if !strings.Contains(updated, "\n\treturn c") {
		return fmt.Errorf("constructor for %s not found", controller.Type)
	}
	updated = strings.Replace(updated, "\n\treturn c", assignment+"\n\treturn c", 1)
	return common.WriteFile(controller.File, updated)
}

func isValidIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if i == 0 && !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_') {
			return false
		}
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// modulePathForRoute is replaced with the project module path before writing.
// It is kept separate so route rendering stays easy to test.
func modulePathForRoute() string {
	path, err := ResolveModulePath()
	if err != nil {
		return ""
	}
	return path
}
