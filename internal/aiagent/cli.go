package aiagent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// RunOptions configures a terminal agent run.
type RunOptions struct {
	Dir             string
	ReadOnly        bool
	AllowAnyCommand bool
	// Quiet suppresses the tool-by-tool trace and prints only the answer.
	Quiet bool
}

// RunPrompt executes a single instruction and prints the trace to output.
func RunPrompt(ctx context.Context, prompt string, output io.Writer, options RunOptions) error {
	agent, err := newFor(options)
	if err != nil {
		return err
	}
	_, err = agent.Run(ctx, prompt, terminalEmitter(output, options.Quiet))
	return err
}

// RunInteractive starts a REPL against the project, so the terminal gets the
// same multi-turn loop the browser chat has.
func RunInteractive(ctx context.Context, input io.Reader, output io.Writer, options RunOptions) error {
	agent, err := newFor(options)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "\n  🤖 Nika agent — %s\n", agent.Provider.Describe())
	fmt.Fprintf(output, "     Project: %s\n", agent.Toolbox.Root)
	if options.ReadOnly {
		fmt.Fprintln(output, "     Mode: read-only")
	}
	fmt.Fprintln(output, "     Type your instruction. /reset clears the conversation, /exit quits.")
	fmt.Fprintln(output)

	emit := terminalEmitter(output, options.Quiet)
	reader := bufio.NewScanner(input)
	reader.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for {
		fmt.Fprint(output, "\n› ")
		if !reader.Scan() {
			fmt.Fprintln(output)
			return reader.Err()
		}
		line := strings.TrimSpace(reader.Text())
		switch {
		case line == "":
			continue
		case line == "/exit" || line == "/quit":
			return nil
		case line == "/reset":
			agent.Reset()
			fmt.Fprintln(output, "  conversation cleared")
			continue
		case line == "/changed":
			changed := agent.ChangedFiles()
			if len(changed) == 0 {
				fmt.Fprintln(output, "  nothing changed yet")
				continue
			}
			for _, file := range changed {
				fmt.Fprintf(output, "  %s\n", file)
			}
			continue
		}
		if _, err := agent.Run(ctx, line, emit); err != nil {
			fmt.Fprintf(output, "  ✖ %v\n", err)
		}
	}
}

func newFor(options RunOptions) (*Agent, error) {
	dir := options.Dir
	if strings.TrimSpace(dir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dir = cwd
	}
	agent, err := New(dir)
	if err != nil {
		return nil, err
	}
	agent.Toolbox.ReadOnly = options.ReadOnly
	agent.Toolbox.AllowAnyCommand = options.AllowAnyCommand
	return agent, nil
}

// terminalEmitter renders the event stream as a compact trace.
func terminalEmitter(output io.Writer, quiet bool) Emit {
	return func(event Event) {
		switch event.Kind {
		case EventStatus:
			if !quiet {
				fmt.Fprintf(output, "  · %s\n", event.Text)
			}
		case EventThinking:
			if !quiet {
				fmt.Fprintf(output, "  · %s\n", firstLine(event.Text))
			}
		case EventToolCall:
			if !quiet {
				fmt.Fprintf(output, "  ⚙ %s %s\n", event.Tool, summarizeArgs(event.Args))
			}
		case EventToolDone:
			if quiet {
				return
			}
			if event.Failed {
				fmt.Fprintf(output, "    ✖ %s\n", firstLine(event.Result))
				return
			}
			fmt.Fprintf(output, "    ✔ %s\n", firstLine(event.Result))
		case EventMessage:
			fmt.Fprintf(output, "\n%s\n", event.Text)
		case EventError:
			// Skipped on purpose: Run returns this same error, and the command
			// layer prints it. Emitting here too would show it twice.
		case EventDone:
			if len(event.Changed) > 0 {
				fmt.Fprintf(output, "\n  📝 changed: %s\n", strings.Join(event.Changed, ", "))
			}
		}
	}
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx] + " …"
	}
	if len(text) > 160 {
		text = text[:160] + "…"
	}
	return text
}

// summarizeArgs keeps the trace to one line by showing only the argument that
// identifies what the call is about.
func summarizeArgs(args string) string {
	args = strings.TrimSpace(args)
	if args == "" || args == "{}" {
		return ""
	}
	if len(args) > 120 {
		return args[:120] + "…"
	}
	return args
}
