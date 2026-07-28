package aiagent

import (
	"fmt"
	"strings"
)

// Whitespace-tolerant editing.
//
// edit_file requires old_string to appear verbatim, which is the right default:
// it is what makes an edit unambiguous. But a model reconstructing a line from
// memory gets the words right far more often than the spacing — one tab versus
// four spaces, or a struct tag written `db:"password"` when the file says
// `db:"password" json:"password"`. Failing those outright wastes the run.
//
// So on an exact miss the match is retried line by line with whitespace
// collapsed. The replacement is re-indented to the file's actual indentation,
// never the model's.

// normalizeLine reduces a line to its significant content.
func normalizeLine(line string) string {
	return strings.Join(strings.Fields(line), " ")
}

// indentOf returns the leading whitespace of a line.
func indentOf(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// replaceIgnoringWhitespace finds old within source comparing lines with
// whitespace collapsed, and splices in replacement.
//
// Returns an error when there is no match, or when there are several and
// replaceAll is false — an ambiguous fuzzy edit is exactly the case where
// guessing corrupts a file.
func replaceIgnoringWhitespace(source, old, replacement string, replaceAll bool) (string, int, error) {
	sourceLines := strings.Split(source, "\n")
	oldLines := strings.Split(strings.Trim(old, "\n"), "\n")
	if len(oldLines) == 0 {
		return "", 0, fmt.Errorf("old_string is empty")
	}

	normalizedOld := make([]string, len(oldLines))
	blank := true
	for i, line := range oldLines {
		normalizedOld[i] = normalizeLine(line)
		if normalizedOld[i] != "" {
			blank = false
		}
	}
	if blank {
		return "", 0, fmt.Errorf("old_string has no content to match")
	}

	var starts []int
	for start := 0; start+len(normalizedOld) <= len(sourceLines); start++ {
		matched := true
		for offset, want := range normalizedOld {
			if normalizeLine(sourceLines[start+offset]) != want {
				matched = false
				break
			}
		}
		if matched {
			starts = append(starts, start)
		}
	}

	if len(starts) == 0 {
		return "", 0, fmt.Errorf("no match")
	}
	if len(starts) > 1 && !replaceAll {
		return "", 0, fmt.Errorf("matches %d places when whitespace is ignored — include more surrounding lines", len(starts))
	}
	if !replaceAll {
		starts = starts[:1]
	}

	// Splice back to front so earlier indices stay valid.
	result := append([]string(nil), sourceLines...)
	for i := len(starts) - 1; i >= 0; i-- {
		start := starts[i]
		fileIndent := indentOf(result[start])
		oldIndent := indentOf(oldLines[0])
		newLines := reindent(strings.Split(replacement, "\n"), oldIndent, fileIndent)
		result = append(result[:start], append(newLines, result[start+len(oldLines):]...)...)
	}
	return strings.Join(result, "\n"), len(starts), nil
}

// reindent shifts replacement lines from the indentation the model assumed to
// the one the file actually uses, so an edit written with spaces lands
// correctly in a tab-indented file.
func reindent(lines []string, from, to string) []string {
	if from == to {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			out[i] = ""
			continue
		}
		if from != "" && strings.HasPrefix(line, from) {
			out[i] = to + line[len(from):]
			continue
		}
		// The model used some other indentation: replace whatever leading
		// whitespace it chose with the file's.
		out[i] = to + strings.TrimLeft(line, " \t")
	}
	return out
}

// editNotFoundError explains a failed match and shows the lines that came
// closest, so the next attempt can copy real text instead of guessing again.
func editNotFoundError(path, source, old string, cause error) error {
	if strings.Contains(cause.Error(), "matches") {
		return fmt.Errorf("old_string %s in %s", cause.Error(), path)
	}

	firstLine := ""
	for _, line := range strings.Split(strings.Trim(old, "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			firstLine = normalizeLine(line)
			break
		}
	}

	var hints []string
	if firstLine != "" {
		// Anchor on the first token — usually the field or function name — and
		// show what the file really contains there.
		anchor := strings.Fields(firstLine)[0]
		for number, line := range strings.Split(source, "\n") {
			if strings.Contains(line, anchor) {
				hints = append(hints, fmt.Sprintf("  %d: %s", number+1, line))
				if len(hints) >= 5 {
					break
				}
			}
		}
	}

	message := fmt.Sprintf("old_string was not found in %s, even ignoring whitespace", path)
	if len(hints) > 0 {
		message += ".\nThe file contains these similar lines — copy one of them exactly:\n" + strings.Join(hints, "\n")
	} else {
		message += " — read the file again and copy the exact text"
	}
	return fmt.Errorf("%s", message)
}
