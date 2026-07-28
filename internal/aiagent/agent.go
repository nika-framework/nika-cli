package aiagent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/nika-framework/nika-cli/internal"
	"github.com/nika-framework/nika-cli/internal/nikaconf"
)

// EventKind labels what happened during a run, so a terminal and a browser can
// render the same stream differently.
type EventKind string

const (
	EventStatus   EventKind = "status"
	EventThinking EventKind = "thinking"
	EventToolCall EventKind = "tool_call"
	EventToolDone EventKind = "tool_result"
	EventMessage  EventKind = "message"
	EventError    EventKind = "error"
	EventDone     EventKind = "done"
)

// Event is one item in the run's progress stream.
type Event struct {
	Kind    EventKind `json:"kind"`
	Text    string    `json:"text,omitempty"`
	Tool    string    `json:"tool,omitempty"`
	Args    string    `json:"args,omitempty"`
	Result  string    `json:"result,omitempty"`
	Step    int       `json:"step,omitempty"`
	Failed  bool      `json:"failed,omitempty"`
	Changed []string  `json:"changed,omitempty"`
}

// Emit receives events as they happen.
type Emit func(Event)

// Agent runs a conversation against a project directory.
//
// It is stateful on purpose: `nika agent start` keeps one Agent per browser
// session so follow-up messages ("now add a price field too") see the earlier
// turns and the files that were already written.
type Agent struct {
	Provider *Provider
	Toolbox  *Toolbox
	MaxSteps int

	mu       sync.Mutex
	tools    map[string]Tool
	schemas  []ToolSchema
	messages []Message
}

// New builds an agent for the project in dir using the config found there.
func New(dir string) (*Agent, error) {
	config, exists, err := nikaconf.LoadFrom(dir)
	if err != nil {
		return nil, err
	}
	if !exists || strings.TrimSpace(config.Agent.Provider) == "" {
		return nil, fmt.Errorf("no AI provider configured in %s — run `nika agent init <ollama|chatgpt|9router|claude>` first", nikaconf.FileName)
	}
	provider, err := NewProvider(config.Agent)
	if err != nil {
		return nil, err
	}
	box, err := NewToolbox(dir)
	if err != nil {
		return nil, err
	}
	box.AllowCommands = config.Agent.AllowCommands

	maxSteps := config.Agent.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 25
	}

	agent := &Agent{Provider: provider, Toolbox: box, MaxSteps: maxSteps, tools: map[string]Tool{}}
	toolset := Tools()
	for _, tool := range toolset {
		agent.tools[tool.Name] = tool
	}
	agent.schemas = Schemas(toolset)
	agent.messages = []Message{{Role: RoleSystem, Content: agent.systemPrompt()}}
	return agent, nil
}

// Reset clears the conversation but keeps the provider and toolbox.
func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = []Message{{Role: RoleSystem, Content: a.systemPrompt()}}
	a.Toolbox.Changed = map[string]bool{}
}

// History returns a copy of the conversation, excluding the system prompt.
func (a *Agent) History() []Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.messages) <= 1 {
		return nil
	}
	return append([]Message(nil), a.messages[1:]...)
}

// Run executes one user instruction to completion.
//
// The loop is the whole point: the model may call tools as many times as it
// needs, seeing each result before deciding the next move, and only stops when
// it answers without requesting another tool. That is what lets "add a price
// field to the product model" work — search, read, edit, build, report — with
// no special-case code for that phrasing.
func (a *Agent) Run(ctx context.Context, instruction string, emit Emit) (string, error) {
	if emit == nil {
		emit = func(Event) {}
	}
	if strings.TrimSpace(instruction) == "" {
		return "", fmt.Errorf("empty instruction")
	}

	a.mu.Lock()
	a.messages = append(a.messages, Message{Role: RoleUser, Content: instruction})
	a.mu.Unlock()

	emit(Event{Kind: EventStatus, Text: "Thinking with " + a.Provider.Describe()})

	// Weak models — and reasoning models pushed into a tool role — often answer
	// the first turn with prose describing the edits they would make, complete
	// with invented file paths, instead of calling a tool. Nudging once
	// recovers the run; nudging repeatedly would just burn the step budget.
	toolsUsed := false
	nudged := false

	// Changed accumulates over the whole session, so "did this run change
	// anything" has to be measured against where it started — otherwise one
	// successful edit would suppress the warning for every later turn.
	changedAtStart := len(a.Toolbox.Changed)

	for step := 1; step <= a.MaxSteps; step++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		a.mu.Lock()
		conversation := append([]Message(nil), a.messages...)
		a.mu.Unlock()

		reply, err := a.Provider.Chat(ctx, conversation, a.schemas)
		if err != nil {
			emit(Event{Kind: EventError, Text: err.Error()})
			return "", err
		}

		a.mu.Lock()
		a.messages = append(a.messages, reply)
		a.mu.Unlock()

		if text := strings.TrimSpace(reply.Content); text != "" && len(reply.ToolCalls) > 0 {
			// Narration alongside tool calls: show it as progress, not as the
			// final answer.
			emit(Event{Kind: EventThinking, Text: text, Step: step})
		}

		if len(reply.ToolCalls) == 0 {
			answer := strings.TrimSpace(reply.Content)

			if !toolsUsed && !nudged && looksLikeUnexecutedWork(answer) {
				nudged = true
				note := "The model described the change instead of making it — asking it to use the tools"

				// Some models advertise native function calling and then never
				// emit a call, narrating the work instead. The emulated
				// protocol states the contract far more bluntly ("reply with a
				// single JSON object and nothing else"), so switching is worth
				// more here than repeating the same request.
				if a.Provider.Tools == ToolsNative {
					a.Provider.Tools = ToolsEmulated
					note += ", using the explicit JSON protocol"
				}

				emit(Event{Kind: EventStatus, Text: note, Step: step})
				a.mu.Lock()
				a.messages = append(a.messages, Message{Role: RoleUser, Content: nudgePrompt})
				a.mu.Unlock()
				continue
			}

			if answer == "" {
				answer = "(the model returned an empty response)"
			}
			// A model claiming it edited files when nothing was written is
			// worse than a model that fails: the user walks away believing the
			// change landed. Contradict it with the ground truth.
			if len(a.Toolbox.Changed) == changedAtStart && claimsChanges(answer) {
				answer += "\n\n⚠ Nothing was actually changed — no file was written during this run. " +
					"The summary above describes work the model did not carry out. Try a narrower instruction, " +
					"or a stronger model (`nika agent models`)."
			}
			emit(Event{Kind: EventMessage, Text: answer, Step: step})
			emit(Event{Kind: EventDone, Changed: a.ChangedFiles()})
			return answer, nil
		}

		toolsUsed = true
		for _, call := range reply.ToolCalls {
			result, failed := a.execute(ctx, call, step, emit)
			a.mu.Lock()
			a.messages = append(a.messages, Message{
				Role:       RoleTool,
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Content:    result,
			})
			a.mu.Unlock()
			_ = failed
		}
	}

	message := fmt.Sprintf("stopped after %d steps without a final answer — ask again with a narrower instruction, or raise agent.max_steps in %s",
		a.MaxSteps, nikaconf.FileName)
	emit(Event{Kind: EventError, Text: message})
	return "", fmt.Errorf("%s", message)
}

// nudgePrompt is sent once when the model answered without touching anything.
const nudgePrompt = `You have not called any tool yet, so nothing in the project has changed and you have not read any real file — anything you wrote above about the code is a guess.

Do not describe edits, diffs, or SEARCH/REPLACE blocks. Make the change by calling tools:

1. project_info to learn the real layout
2. search or read_file to find the actual code
3. edit_file or write_file to change it
4. run_command with "go build ./..." to verify

Start now with a single tool call. If the request genuinely needs no tools, say so in one sentence and stop.`

// unexecutedWorkMarkers are the shapes a model produces when it narrates an
// edit rather than performing one.
var unexecutedWorkMarkers = []string{
	"```", "<<<<<<<", ">>>>>>>", "search\n", "replace\n",
	"you can", "you should", "you need to", "here's how", "here is how",
	"let's update", "let's add", "first,", "step 1", "i would",
}

// looksLikeUnexecutedWork reports whether a tool-free reply is describing work
// instead of reporting it.
//
// A short direct answer to a question is left alone; the target is the long
// reply full of code fences and instructions that the user cannot act on
// because the paths in it were invented.
func looksLikeUnexecutedWork(answer string) bool {
	if answer == "" {
		return true
	}
	// Short replies are almost always genuine answers or clarifying questions.
	if len([]rune(answer)) < 200 {
		return false
	}
	lowered := strings.ToLower(answer)
	for _, marker := range unexecutedWorkMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

// changeClaimMarkers are the verbs a model uses when reporting work on files.
//
// These are stems, not full words: a model writes "added", "adds", "add" and
// "adding" interchangeably, and matching only the past tense lets the most
// common phrasing ("these changes add a price field") slip through.
var changeClaimMarkers = []string{
	"add", "updat", "creat", "chang", "modif", "remov", "renam",
	"implement", "generat", "wrote", "writ", "fix",
	"اضافه", "به‌روزرسانی", "بروزرسانی", "ایجاد", "تغییر", "حذف", "ساخت", "انجام شد",
}

// claimsChanges reports whether an answer talks about modifying files.
func claimsChanges(answer string) bool {
	lowered := strings.ToLower(answer)
	for _, marker := range changeClaimMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	// A narrated diff is a claim even without one of those verbs.
	return looksLikeUnexecutedWork(answer)
}

// execute runs one tool call and returns the content to feed back.
//
// Tool errors are returned as content rather than aborting the run: a wrong
// path or a stale old_string is something the model can correct on the next
// step, and killing the run would leave the project half-edited.
func (a *Agent) execute(ctx context.Context, call ToolCall, step int, emit Emit) (string, bool) {
	name := call.Function.Name
	args := call.Function.Arguments
	emit(Event{Kind: EventToolCall, Tool: name, Args: args.String(), Step: step})

	tool, ok := a.tools[name]
	if !ok {
		result := jsonError(fmt.Errorf("unknown tool %q; available: %s", name, strings.Join(a.toolNames(), ", ")))
		emit(Event{Kind: EventToolDone, Tool: name, Result: result, Step: step, Failed: true})
		return result, true
	}
	if tool.Mutates && a.Toolbox.ReadOnly {
		result := jsonError(fmt.Errorf("%s is disabled: this session is read-only", name))
		emit(Event{Kind: EventToolDone, Tool: name, Result: result, Step: step, Failed: true})
		return result, true
	}

	output, err := tool.Run(ctx, a.Toolbox, args)
	if err != nil {
		result := jsonError(err)
		emit(Event{Kind: EventToolDone, Tool: name, Result: result, Step: step, Failed: true})
		return result, true
	}
	emit(Event{Kind: EventToolDone, Tool: name, Result: truncate(output, 4000), Step: step})
	return output, false
}

func (a *Agent) toolNames() []string {
	names := make([]string, 0, len(a.tools))
	for name := range a.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ChangedFiles lists what the session has written so far.
func (a *Agent) ChangedFiles() []string {
	files := make([]string, 0, len(a.Toolbox.Changed))
	for file := range a.Toolbox.Changed {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "\n… truncated"
}

// systemPrompt describes the framework and the working agreement. It embeds a
// live snapshot of the project layout so the model does not have to spend its
// first two steps discovering that this is a workspace.
func (a *Agent) systemPrompt() string {
	var layout strings.Builder
	if workspace, err := internal.LoadWorkspaceAt(a.Toolbox.Root); err == nil {
		fmt.Fprintf(&layout, "\n\n# This project\n\nGo module: %s\n", workspace.ModulePath)
		if workspace.Microservice {
			layout.WriteString("Layout: microservice workspace — each app under apps/ is a separate process with its own src/ and its own app.module.go.\n")
		} else {
			layout.WriteString("Layout: single application — modules live in src/.\n")
		}
		layout.WriteString("Apps:\n")
		for _, app := range workspace.Apps {
			fmt.Fprintf(&layout, "  - %s → source in %s, import prefix %s/%s",
				app.Name, app.SrcDir, workspace.ModulePath, app.SrcImport())
			if modules := workspace.Modules(app); len(modules) > 0 {
				fmt.Fprintf(&layout, ", existing modules: %s", strings.Join(modules, ", "))
			}
			layout.WriteString("\n")
		}
		if workspace.Microservice {
			layout.WriteString("\nWhen a request does not say which app it means and more than one could match, ask the user before writing anything.\n")
		}
	}

	return `You are the Nika CLI agent. You work inside a Go backend project built on the Nika framework, and you carry out the user's instruction by calling tools — not by describing what should be done.

# How Nika projects are laid out

A module lives in one folder and has a fixed shape:

  <src>/<module>/
    <module>.module.go          registers controllers and providers
    <pkg>/<module>.model.go             the persistence model
    <pkg>/<module>.repository.go        repository implementation
    <pkg>/<module>.repository.interface.go
    dto/create.dto.go, update.dto.go, findone.dto.go, find.dto.go
    controllers/<module>.controller.go  plus create.go, find.go, find-one.go, update.go, delete.go
    services/<module>.service.go        plus one file per CRUD method
    response/<module>.response.go, <module>.mapper.go

<pkg> is "entity" for PostgreSQL, MySQL and SQLite modules, and "schema" for MongoDB modules. The folder and the Go package name are always the same. List the module folder to see which one it uses rather than assuming — modules generated before the rename may still use schema/.

<src> is "src" in a single-app project and "apps/<app>/src" in a microservice workspace. Import paths always mirror the folder path: "<module path>/<src>/<module>/dto".

Layer rules:
- Models: MongoDB uses bson+json tags and primitive.ObjectID IDs; PostgreSQL, MySQL and SQLite use db+json tags with int64 IDs.
- DTOs carry validate tags, e.g. validate:"required,min=1".
- Controllers are Gin handlers. Routes are declared as struct field tags: Create func(*gin.Context) ` + "`route:\"POST:/products\"`" + `, and the constructor assigns c.Create = c.CreateHandler.
- Services hold the business logic and talk to the repository.
- Responses are mapped from the model by the mapper — never return the model directly.
- A module is only live once it is listed in the app's app.module.go Imports().

# How to work

1. Understand before changing. Use project_info, list_dir, search and read_file. Never guess a file's contents.
2. Make the change with edit_file for existing files and write_file for new ones. Use nika_generate for a whole new resource — it produces framework-correct code and registers the module for you.
3. A change is not one file. Adding a field to a model usually also means the create and update DTOs, the response struct, the mapper, and — for SQL — a migration. Follow it through every layer.
4. Verify. After editing Go files run ` + "`go build ./...`" + ` and fix what it reports.
5. Answer in the user's language, briefly, listing the files you changed. Say plainly if something did not work.

Do not ask for permission for ordinary edits — the user invoked you to make them. Do ask when the instruction is genuinely ambiguous about which app or module it refers to.` + layout.String()
}
