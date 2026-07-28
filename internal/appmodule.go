package internal

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"

	"github.com/nika-framework/nika-cli/common"
)

// RegisterModule adds a generated module to the app's app.module.go: an import
// line and an entry in the Imports() slice.
//
// This used to be a printed reminder ("don't forget to import UserModule"),
// which meant every generated module was dead code until someone edited the
// file by hand — and in a microservice workspace, edited the *right* one of
// four such files. Returns whether the file was changed.
func RegisterModule(app AppTarget, modulePath, moduleName, typeName string) (bool, error) {
	path := app.AppModulePath()
	source, err := common.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("%s does not exist", path)
		}
		return false, err
	}

	importPath := fmt.Sprintf("%s/%s/%s", modulePath, app.SrcImport(), moduleName)
	constructor := fmt.Sprintf("%s.New%sModule()", moduleName, typeName)

	updated, changed, err := addModuleToSource(source, importPath, moduleName, constructor)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	if err := common.WriteFile(path, updated); err != nil {
		return false, err
	}
	return true, nil
}

// addModuleToSource performs the two edits on an app.module.go source.
//
// It is separated from the file I/O so the surgery — the part that can
// silently corrupt a user's file — is directly testable. The insertion point
// comes from the AST rather than a regex, because Imports() is written both as
// a multi-line method and as a one-liner, and a regex that handles one tends
// to mangle the other.
func addModuleToSource(source, importPath, packageAlias, constructor string) (string, bool, error) {
	// Parse first: refusing to touch a file that does not compile is cheaper
	// than writing a second syntax error into it.
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "app.module.go", source, parser.ParseComments)
	if err != nil {
		return "", false, fmt.Errorf("app.module.go does not parse: %w", err)
	}

	alreadyImported := hasImport(file, importPath)
	alreadyRegistered := strings.Contains(source, constructor)
	if alreadyImported && alreadyRegistered {
		return source, false, nil
	}

	updated := source
	if !alreadyRegistered {
		// Register before importing: the offsets below come from the parse of
		// `source`, and inserting the import first would shift them.
		updated, err = appendToModuleSlice(fset, file, source, constructor)
		if err != nil {
			return "", false, err
		}
	}
	if !alreadyImported {
		// Re-parse: the slice edit above shifted every offset in the file.
		shifted := token.NewFileSet()
		reparsed, err := parser.ParseFile(shifted, "app.module.go", updated, parser.ParseComments)
		if err != nil {
			return "", false, fmt.Errorf("edit would have produced invalid Go: %w", err)
		}
		updated, err = insertImport(shifted, reparsed, updated, importPath, packageAlias, importAliasNeeded(file, packageAlias))
		if err != nil {
			return "", false, err
		}
	}

	// Parse again so a botched edit never reaches disk.
	if _, err := parser.ParseFile(token.NewFileSet(), "app.module.go", updated, parser.ParseComments); err != nil {
		return "", false, fmt.Errorf("edit would have produced invalid Go: %w", err)
	}
	return tidy(source, updated), true, nil
}

// tidy runs gofmt over the result, but only when the original was already
// gofmt-clean. Sorting the import group we just added to is worth doing;
// reformatting a file the user deliberately keeps unformatted is not.
func tidy(original, updated string) string {
	if formattedOriginal, err := format.Source([]byte(original)); err != nil || string(formattedOriginal) != original {
		return updated
	}
	formatted, err := format.Source([]byte(updated))
	if err != nil {
		return updated
	}
	return string(formatted)
}

// appendToModuleSlice inserts the constructor call as the last element of the
// slice returned by Imports().
func appendToModuleSlice(fset *token.FileSet, file *ast.File, source, constructor string) (string, error) {
	literal := findImportsLiteral(file)
	if literal == nil {
		return "", fmt.Errorf("could not find an Imports() []nika.Module method returning a slice literal")
	}

	closing := fset.Position(literal.Rbrace).Offset
	if closing <= 0 || closing > len(source) {
		return "", fmt.Errorf("could not locate the end of the Imports() slice")
	}

	if len(literal.Elts) == 0 {
		// Empty literal — keep it on one line if that is how it was written.
		opening := fset.Position(literal.Lbrace).Offset
		if !strings.Contains(source[opening:closing], "\n") {
			return source[:closing] + constructor + ", " + source[closing:], nil
		}
		return source[:closing] + "\t" + constructor + ",\n" + source[closing:], nil
	}

	last := literal.Elts[len(literal.Elts)-1]
	lastEnd := fset.Position(last.End()).Offset
	indent := elementIndent(source, fset.Position(last.Pos()).Offset)

	// Everything between the final element and the closing brace: possibly a
	// trailing comma and a line comment, then the newline the brace sits on.
	gap := source[lastEnd:closing]

	// Step past the existing trailing comma if there is one, so the new entry
	// lands after it rather than splitting it off.
	cursor := lastEnd
	needComma := true
	if idx := strings.IndexByte(gap, ','); idx >= 0 {
		cursor = lastEnd + idx + 1
		needComma = false
	}

	prefix := ""
	if needComma {
		prefix = ","
	}

	// Insert at the end of that line, which keeps any trailing comment
	// attached to the element it belongs to.
	if idx := strings.IndexByte(source[cursor:closing], '\n'); idx >= 0 {
		at := cursor + idx
		return source[:at] + prefix + "\n" + indent + constructor + "," + source[at:], nil
	}
	// A one-line literal: stay on the line.
	return source[:cursor] + prefix + " " + constructor + "," + source[cursor:], nil
}

// findImportsLiteral returns the []nika.Module composite literal returned by
// the Imports() method, if there is one.
func findImportsLiteral(file *ast.File) *ast.CompositeLit {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != "Imports" || fn.Recv == nil || fn.Body == nil {
			continue
		}
		var literal *ast.CompositeLit
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if literal != nil {
				return false
			}
			ret, ok := node.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				return true
			}
			if composite, ok := ret.Results[0].(*ast.CompositeLit); ok {
				literal = composite
				return false
			}
			return true
		})
		if literal != nil {
			return literal
		}
	}
	return nil
}

// elementIndent recovers the leading whitespace of the line holding offset, so
// the inserted entry lines up with its siblings.
func elementIndent(source string, offset int) string {
	start := strings.LastIndexByte(source[:offset], '\n') + 1
	line := source[start:offset]
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// hasImport reports whether the file already imports path.
func hasImport(file *ast.File, path string) bool {
	for _, spec := range file.Imports {
		if strings.Trim(spec.Path.Value, `"`) == path {
			return true
		}
	}
	return false
}

// importAliasNeeded reports whether an explicit alias is required because some
// other import already binds that identifier. Workspaces hit this constantly:
// apps/api/src imports user-grpc, user-tcp and user-redis, all package "user".
func importAliasNeeded(file *ast.File, packageAlias string) bool {
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		name := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			name = path[idx+1:]
		}
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == packageAlias {
			return true
		}
	}
	return false
}

// packageClausePattern locates the package line in a file with no imports.
var packageClausePattern = regexp.MustCompile(`(?m)^package\s+[A-Za-z_][A-Za-z0-9_]*\s*$`)

// insertImport adds the import next to the project's other imports.
//
// Placement matters more than it looks: appending blindly to the end of the
// block drops a local package into the third-party group, which the next
// gofmt/goimports run — or the next reader — has to undo.
func insertImport(fset *token.FileSet, file *ast.File, source, importPath, packageAlias string, needsAlias bool) (string, error) {
	entry := fmt.Sprintf("%q", importPath)
	if needsAlias {
		entry = fmt.Sprintf("%s %q", uniqueAlias(source, packageAlias), importPath)
	}

	// Prefer to sit beside an import from the same top-level path segment,
	// i.e. another package of this same Go module.
	if anchor := siblingImport(file, importPath); anchor != nil {
		end := fset.Position(anchor.End()).Offset
		if idx := strings.IndexByte(source[end:], '\n'); idx >= 0 {
			at := end + idx
			indent := elementIndent(source, fset.Position(anchor.Pos()).Offset)
			return source[:at] + "\n" + indent + entry + source[at:], nil
		}
	}

	// Otherwise use the import declaration itself.
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		if gen.Lparen.IsValid() {
			closing := fset.Position(gen.Rparen).Offset
			return source[:closing] + "\t" + entry + "\n" + source[closing:], nil
		}
		// A single unparenthesised import: turn it into a group.
		if len(gen.Specs) == 1 {
			start := fset.Position(gen.Pos()).Offset
			end := fset.Position(gen.End()).Offset
			existing := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(source[start:end]), "import"))
			block := fmt.Sprintf("import (\n\t%s\n\t%s\n)", existing, entry)
			return source[:start] + block + source[end:], nil
		}
	}

	// No imports at all: create the block after the package clause.
	loc := packageClausePattern.FindStringIndex(source)
	if loc == nil {
		return "", fmt.Errorf("no package clause found")
	}
	return source[:loc[1]] + fmt.Sprintf("\n\nimport (\n\t%s\n)", entry) + source[loc[1]:], nil
}

// siblingImport finds the last existing import from the same Go module, which
// is where a newly generated module's import belongs.
func siblingImport(file *ast.File, importPath string) *ast.ImportSpec {
	prefix := importPath
	if idx := strings.IndexByte(prefix, '/'); idx >= 0 {
		prefix = prefix[:idx]
	}
	if prefix == "" {
		return nil
	}
	var found *ast.ImportSpec
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		if path == importPath {
			continue
		}
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			found = spec
		}
	}
	return found
}

// uniqueAlias derives an identifier that is not already used in the file.
func uniqueAlias(source, base string) string {
	candidate := strings.ReplaceAll(base, "-", "") + "module"
	for i := 2; strings.Contains(source, candidate+" \""); i++ {
		candidate = fmt.Sprintf("%s%dmodule", strings.ReplaceAll(base, "-", ""), i)
	}
	return candidate
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
