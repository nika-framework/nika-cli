package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type aiModuleSpec struct {
	Module string        `json:"module"`
	Fields []aiFieldSpec `json:"fields"`
}

type aiFieldSpec struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// RunOllemaModule asks Ollama for a resource definition, then uses Nika's
// normal generator so generated files follow the existing templates.
func RunOllemaModule(model, userPrompt string, output io.Writer) error {
	instruction := `Convert the user's module request into JSON only. Do not use markdown.
The JSON must have this exact shape:
{"module":"lowercase_english_module_name","fields":[{"name":"snake_case_name","type":"string","required":true}]}
Allowed field types: string, int, int64, float64, bool, time.Time, primitive.ObjectID, []string, map[string]any.
Translate non-English module and field names into clear English names. Include every requested field.

User request:
` + userPrompt

	response, err := askOllema(model, instruction, "json")
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

	fields := make([]Field, 0, len(spec.Fields))
	for _, field := range spec.Fields {
		validate := mongoTypeDefaultValidate(field.Type, field.Required)
		fields = append(fields, Field{
			Name:      toPascalCase(field.Name),
			BsonName:  field.Name,
			Type:      field.Type,
			Required:  field.Required,
			ModelTag:  fmt.Sprintf(`bson:"%s" json:"%s"`, field.Name, field.Name),
			JsonTag:   fmt.Sprintf(`json:"%s"`, field.Name),
			CreateTag: fmt.Sprintf(`json:"%s" validate:"%s"`, field.Name, validate),
			UpdateTag: fmt.Sprintf(`json:"%s,omitempty" validate:"omitempty"`, field.Name),
		})
	}
	if err := RunGenerate(&GenerateConfig{Type: GenResource, Module: spec.Module, Fields: fields}); err != nil {
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
	for i := range spec.Fields {
		field := &spec.Fields[i]
		field.Name = strings.ToLower(strings.TrimSpace(field.Name))
		if !isValidModule(field.Name) {
			return fmt.Errorf("Ollama returned invalid field name %q", field.Name)
		}
		if !containsString(mongoTypes, field.Type) {
			return fmt.Errorf("Ollama returned unsupported type %q for field %q", field.Type, field.Name)
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
