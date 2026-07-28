// Package templates carries the code-generation templates inside the binary.
//
// They used to be located with runtime.Caller, which resolves to the path of
// the machine that compiled the CLI — so `nika g` only ever worked from a
// checkout of this repository and failed for anyone who ran `go install`.
// Embedding them makes a released binary self-contained.
package templates

import (
	"embed"
	"io/fs"
	"strings"
)

// FS holds every .tpl under templates/res (the resource generator) and
// templates/micro (the microservice scaffolder). Dot-prefixed files
// (.DS_Store) are excluded by the embed rules.
//
//go:embed res
//go:embed micro
var FS embed.FS

// Read returns the contents of a template.
//
// Both "res/dto/create.dto.go.tpl" and the legacy "templates/res/..." form are
// accepted, and OS-native separators are normalized, so callers can keep using
// filepath.Join to build template names.
func Read(name string) (string, error) {
	content, err := FS.ReadFile(Normalize(name))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// Normalize converts a template reference into an embed-FS path.
func Normalize(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "templates/")
	return name
}

// List returns every embedded template path, for diagnostics.
func List() []string {
	var names []string
	_ = fs.WalkDir(FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	return names
}
