package common

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

const bufSize = 64 * 1024 // 64 KB buffer for file I/O

// ReadFile reads a file using a buffered reader to avoid loading everything
// at once. Returns the content as a string.
func ReadFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file %s: %w", path, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	r := bufio.NewReaderSize(f, bufSize)
	if _, err := io.Copy(&buf, r); err != nil {
		return "", fmt.Errorf("reading file %s: %w", path, err)
	}
	return buf.String(), nil
}

// WriteFile writes content to a file using a buffered writer.
func WriteFile(path, content string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", path, err)
	}
	defer f.Close()

	w := bufio.NewWriterSize(f, bufSize)
	if _, err := w.WriteString(content); err != nil {
		return fmt.Errorf("writing file %s: %w", path, err)
	}
	return w.Flush()
}

// ReplaceInFile reads a file, replaces all occurrences of old with new,
// and writes it back — all via buffered I/O.
func ReplaceInFile(path, old, repl string) error {
	content, err := ReadFile(path)
	if err != nil {
		return err
	}

	updated := strings.ReplaceAll(content, old, repl)
	if updated == content {
		return fmt.Errorf("replacement pattern %q not found in %s", old, path)
	}

	return WriteFile(path, updated)
}
