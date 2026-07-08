package internal

import (
	"fmt"
	"strings"

	"github.com/nika-framework/nika-cli/common"
)

// GenerateType represents the kind of resource to generate.
type GenerateType string

const (
	// GenResource generates everything: schema + dto + controller + service + module.
	GenResource GenerateType = "res"
	GenRes      GenerateType = "r"
	// GenController generates only the controller.
	GenController GenerateType = "controller"
	GenC          GenerateType = "c"
	// GenService generates only the service.
	GenService GenerateType = "service"
	GenS       GenerateType = "s"
	// GenDTO generates only the DTO.
	GenDTO GenerateType = "dto"
	GenD    GenerateType = "d"
)

// ParseGenerateType normalizes a type string to a GenerateType.
// Returns empty string if the type is unknown.
func ParseGenerateType(raw string) GenerateType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "res", "r":
		return GenResource
	case "controller", "c":
		return GenController
	case "service", "s":
		return GenService
	case "dto", "d":
		return GenDTO
	}
	return ""
}

// String returns the canonical (long) name of the type.
func (t GenerateType) String() string {
	switch t {
	case GenResource, GenRes:
		return "resource"
	case GenController, GenC:
		return "controller"
	case GenService, GenS:
		return "service"
	case GenDTO, GenD:
		return "dto"
	}
	return string(t)
}

// ResolveModulePath reads go.mod in the current directory and returns the module name.
func ResolveModulePath() (string, error) {
	content, err := common.ReadFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("reading go.mod: %w", err)
	}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	return "", fmt.Errorf("no module directive found in go.mod")
}
