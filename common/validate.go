package common

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// reservedNames are names that conflict with OS devices or Go keywords.
var reservedNames = map[string]bool{
	"con":    true, "prn": true, "aux": true, "nul": true,
	"com0":   true, "com1": true, "com2": true, "com3": true,
	"com4":   true, "com5": true, "com6": true, "com7": true,
	"com8":   true, "com9": true,
	"lpt0":   true, "lpt1": true, "lpt2": true, "lpt3": true,
	"lpt4":   true, "lpt5": true, "lpt6": true, "lpt7": true,
	"lpt8":   true, "lpt9": true,
	// Go keywords
	"break": true, "case": true, "chan": true, "const": true,
	"continue": true, "default": true, "defer": true, "else": true,
	"fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true,
	"map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true,
	"var": true,
}

// validDirPattern allows lowercase letters, digits, hyphens, underscores,
// and dots — safe for directory names on all major OSes.
var validDirPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ValidateProjectName checks that a name is safe to use as both a directory
// name across operating systems and a Go module name.
// Returns the cleaned name or an error.
func ValidateProjectName(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("project name is required")
	}

	name := strings.TrimSpace(raw)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")

	// Strip anything that isn't a valid directory character.
	name = regexp.MustCompile(`[^a-z0-9._-]`).ReplaceAllString(name, "")

	if name == "" {
		return "", fmt.Errorf("invalid project name: must contain at least one letter or digit")
	}

	// Must start with a letter (directory + Go module convention).
	if name[0] < 'a' || name[0] > 'z' {
		return "", fmt.Errorf("invalid project name: must start with a lowercase letter")
	}

	if reservedNames[name] {
		return "", fmt.Errorf("invalid project name: %q is a reserved name", name)
	}

	if !validDirPattern.MatchString(name) {
		return "", fmt.Errorf("invalid project name: contains disallowed characters")
	}

	// Check for collision with existing directory.
	if _, err := os.Stat(name); err == nil {
		return "", fmt.Errorf("directory %q already exists in the current path", name)
	}

	return name, nil
}
