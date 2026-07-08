package common

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// RenderTemplate reads a .tpl file, parses it, and executes it with data.
// Returns the rendered string.
func RenderTemplate(tplPath string, data interface{}) (string, error) {
	absPath, err := filepath.Abs(tplPath)
	if err != nil {
		return "", fmt.Errorf("resolving template path %s: %w", tplPath, err)
	}

	content, err := ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("reading template %s: %w", absPath, err)
	}

	t, err := template.New(filepath.Base(tplPath)).Parse(content)
	if err != nil {
		return "", fmt.Errorf("parsing template %s: %w", absPath, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %s: %w", absPath, err)
	}

	return buf.String(), nil
}

// RenderToFile renders a template and writes the output to a file,
// creating parent directories as needed.
func RenderToFile(tplPath, outputPath string, data interface{}) error {
	rendered, err := RenderTemplate(tplPath, data)
	if err != nil {
		return err
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	return WriteFile(outputPath, rendered)
}
