package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type aiModuleSpec struct {
	Module   string        `json:"module"`
	Database string        `json:"database"`
	Fields   []aiFieldSpec `json:"fields"`
}

type aiFieldSpec struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// RunOllemaModule asks Ollama for a resource definition, then uses Nika's
// normal generator so generated files follow the existing templates.
func RunOllemaModule(model, userPrompt string, output io.Writer) error {
	return runOllemaModule(agentRuntime{Provider: "ollama", Model: model}, userPrompt, output)
}

func runOllemaModule(runtime agentRuntime, userPrompt string, output io.Writer) error {
	instruction := `Convert the user's module request into JSON only. Do not use markdown.
The JSON must have this exact shape:
{"module":"lowercase_english_module_name","database":"postgres","fields":[{"name":"snake_case_name","type":"string","required":true}]}
Allowed databases: mongodb, postgres, mysql, sqlite.
MongoDB field types: string, int, int64, float64, bool, time.Time, primitive.ObjectID, []string, map[string]any.
PostgreSQL, MySQL, and SQLite field types: string, int, int64, float64, bool, time.Time.
Translate non-English module and field names into clear English names. Include every requested field.

User request:
` + userPrompt

	response, err := askAgent(runtime, instruction, "json")
	if err != nil {
		return err
	}
	var spec aiModuleSpec
	if err := json.Unmarshal([]byte(response.Response), &spec); err != nil {
		return fmt.Errorf("Ollama returned invalid module JSON: %w", err)
	}
	if err := validateAISpec(&spec); err != nil {
		return err
	}

	database := ParseDatabaseType(spec.Database)
	fields := make([]Field, 0, len(spec.Fields))
	for _, field := range spec.Fields {
		generatedField, err := newField(field.Name, field.Type, field.Required, database)
		if err != nil {
			return err
		}
		fields = append(fields, generatedField)
	}
	if err := RunGenerate(&GenerateConfig{Type: GenResource, Module: spec.Module, Database: spec.Database, Fields: fields}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "AI module %q generated successfully.\n", spec.Module)
	return err
}

func validateAISpec(spec *aiModuleSpec) error {
	spec.Module = strings.ToLower(strings.TrimSpace(spec.Module))
	if !isValidModule(spec.Module) {
		return fmt.Errorf("Ollama returned invalid module name %q", spec.Module)
	}
	if len(spec.Fields) == 0 {
		return fmt.Errorf("Ollama returned no module fields")
	}
	if strings.TrimSpace(spec.Database) == "" {
		spec.Database = string(DatabaseMongo)
	}
	database := ParseDatabaseType(spec.Database)
	if database == "" {
		return fmt.Errorf("Ollama returned unsupported database %q", spec.Database)
	}
	spec.Database = string(database)
	for i := range spec.Fields {
		field := &spec.Fields[i]
		field.Name = strings.ToLower(strings.TrimSpace(field.Name))
		if !isValidModule(field.Name) {
			return fmt.Errorf("Ollama returned invalid field name %q", field.Name)
		}
		if !containsString(supportedFieldTypes(database), field.Type) {
			return fmt.Errorf("Ollama returned unsupported type %q for field %q with %s", field.Type, field.Name, database.DisplayName())
		}
	}
	return nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
