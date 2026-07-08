package common

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const promptBufSize = 64 * 1024

// A single shared reader so buffered-but-unconsumed input isn't lost
// between prompt calls.
var sharedReader = bufio.NewReaderSize(os.Stdin, promptBufSize)

// errEOF is returned by readLine when stdin reaches end-of-file.
var errEOF = errors.New("end of input")

// readLine reads a single line from the shared reader.
// Returns errEOF when there is no more input.
func readLine() (string, error) {
	line, err := sharedReader.ReadString('\n')
	if err != nil {
		// io.EOF with some content is still usable.
		if errors.Is(err, io.EOF) {
			if strings.TrimSpace(line) == "" {
				return "", errEOF
			}
			return strings.TrimSpace(line), nil
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Prompt reads a line from stdin with an optional default value.
// Returns the trimmed input, or the default if the user pressed Enter
// (or hit EOF).
func Prompt(label string, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, err := readLine()
	if err != nil {
		return def
	}
	if line == "" {
		return def
	}
	return line
}

// PromptRequired reads a line from stdin. Repeats until non-empty input.
// On EOF it returns the empty string (caller should handle gracefully).
func PromptRequired(label string) string {
	for {
		val := Prompt(label, "")
		if val != "" {
			return val
		}
		fmt.Println("  ⚠ This field is required.")
	}
}

// SelectOption presents a numbered list and returns the selected item.
// Returns the first option on EOF.
func SelectOption(label string, options []string) string {
	for {
		fmt.Printf("\n  %s\n", label)
		for i, opt := range options {
			fmt.Printf("  [%d] %s\n", i+1, opt)
		}
		fmt.Print("  Enter number: ")

		line, err := readLine()
		if err != nil {
			fmt.Printf("  ✔ Selected: %s\n", options[0])
			return options[0]
		}

		if num, perr := strconv.Atoi(line); perr == nil && num >= 1 && num <= len(options) {
			fmt.Printf("  ✔ Selected: %s\n", options[num-1])
			return options[num-1]
		}

		// Allow typing the option name directly.
		for _, opt := range options {
			if strings.EqualFold(line, opt) {
				fmt.Printf("  ✔ Selected: %s\n", opt)
				return opt
			}
		}

		fmt.Println("  ⚠ Invalid choice. Try again.")
	}
}

// ConfirmYesNo asks a yes/no question. Returns true for yes/Y.
// Defaults to true on EOF.
func ConfirmYesNo(label string) bool {
	for {
		fmt.Printf("  %s (Y/n): ", label)
		line, err := readLine()
		if err != nil {
			return true
		}
		line = strings.ToLower(line)
		if line == "" || line == "y" || line == "yes" {
			return true
		}
		if line == "n" || line == "no" {
			return false
		}
		fmt.Println("  ⚠ Please enter Y or N.")
	}
}
