package aiagent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Emulated tool calling.
//
// Plenty of useful local models — gemma3, llama3, most older builds — expose
// no native function-calling API. Refusing to run on them means the agent only
// works after a multi-gigabyte download, which is a poor answer when the model
// on disk can follow a JSON protocol perfectly well.
//
// In emulated mode the tool schemas are described in the prompt, the model is
// asked to reply with a single JSON object, and the reply is parsed back into
// the same ToolCall structs the native path produces. Everything above the
// provider — the loop, the tools, the UI — is unchanged.

// emulationInstructions renders the tool catalogue and the reply contract.
func emulationInstructions(tools []ToolSchema) string {
	var out strings.Builder
	out.WriteString(`# How to reply

You cannot call functions directly, so you use a strict text protocol instead.

Every reply must be a SINGLE JSON object and nothing else. No prose before it,
no prose after it, no markdown fences.

To use a tool:

{"tool": "<tool name>", "arguments": { ... }}

To give your final answer once the task is done:

{"answer": "<what you did, in the user's language>"}

Only one JSON object per reply. After a tool call you will receive its result
and can then reply again. Never invent a tool result yourself — always wait for
it.

# Available tools

`)
	for _, tool := range tools {
		fmt.Fprintf(&out, "## %s\n%s\n", tool.Function.Name, tool.Function.Description)
		if schema, err := json.Marshal(tool.Function.Parameters); err == nil {
			fmt.Fprintf(&out, "arguments schema: %s\n", schema)
		}
		out.WriteString("\n")
	}

	out.WriteString(`# Examples

{"tool": "project_info", "arguments": {}}
{"tool": "read_file", "arguments": {"path": "src/user/schema/user.model.go"}}
{"tool": "edit_file", "arguments": {"path": "src/user/schema/user.model.go", "old_string": "\tName string ` + "`db:\\\"name\\\"`" + `", "new_string": "\tName string ` + "`db:\\\"name\\\"`" + `\n\tAge int ` + "`db:\\\"age\\\"`" + `"}}
{"answer": "Added an Age field to the User model and rebuilt the project."}
`)
	return out.String()
}

// toEmulatedMessages rewrites the conversation for a model with no tool role.
//
// Assistant tool calls are replayed as the JSON the model was asked to emit,
// and results come back as user turns, so the transcript stays internally
// consistent with the protocol it is being asked to follow.
func toEmulatedMessages(messages []Message, tools []ToolSchema) []Message {
	converted := make([]Message, 0, len(messages)+1)
	instructionsAdded := false

	for _, message := range messages {
		switch message.Role {
		case RoleSystem:
			content := message.Content
			if !instructionsAdded {
				content += "\n\n" + emulationInstructions(tools)
				instructionsAdded = true
			}
			converted = append(converted, Message{Role: RoleSystem, Content: content})

		case RoleAssistant:
			if len(message.ToolCalls) == 0 {
				converted = append(converted, Message{Role: RoleAssistant, Content: message.Content})
				continue
			}
			for _, toolCall := range message.ToolCalls {
				replay, err := json.Marshal(map[string]any{
					"tool":      toolCall.Function.Name,
					"arguments": json.RawMessage(toolCall.Function.Arguments.String()),
				})
				if err != nil {
					continue
				}
				converted = append(converted, Message{Role: RoleAssistant, Content: string(replay)})
			}

		case RoleTool:
			name := message.Name
			if name == "" {
				name = "tool"
			}
			converted = append(converted, Message{
				Role:    RoleUser,
				Content: fmt.Sprintf("Result of %s:\n\n%s\n\nReply with the next JSON object.", name, message.Content),
			})

		default:
			converted = append(converted, message)
		}
	}

	if !instructionsAdded {
		converted = append([]Message{{Role: RoleSystem, Content: emulationInstructions(tools)}}, converted...)
	}
	return converted
}

// thinkBlock matches the reasoning wrapper that r1-style models emit.
var thinkBlock = regexp.MustCompile(`(?s)<think>.*?</think>`)

// fencedBlock matches a ``` fence, optionally tagged.
var fencedBlock = regexp.MustCompile("(?s)```(?:json|JSON)?\\s*(.*?)```")

// parseEmulatedReply turns a text reply into the same shape a native
// tool-calling provider returns.
func parseEmulatedReply(text string, index int) Message {
	cleaned := strings.TrimSpace(thinkBlock.ReplaceAllString(text, ""))

	for _, candidate := range jsonCandidates(cleaned) {
		var payload struct {
			Tool      string          `json:"tool"`
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
			Args      json.RawMessage `json:"args"`
			Answer    string          `json:"answer"`
			Response  string          `json:"response"`
		}
		if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
			continue
		}

		// Accept "name" as well as "tool": models drift toward the OpenAI
		// vocabulary they were trained on, and rejecting that wastes a step.
		name := strings.TrimSpace(payload.Tool)
		if name == "" {
			name = strings.TrimSpace(payload.Name)
		}
		if name != "" {
			arguments := payload.Arguments
			if len(arguments) == 0 {
				arguments = payload.Args
			}
			if len(arguments) == 0 {
				arguments = json.RawMessage("{}")
			}
			return Message{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{{
					ID:       fmt.Sprintf("call_%d", index),
					Type:     "function",
					Function: FunctionCall{Name: name, Arguments: Arguments{Raw: arguments}},
				}},
			}
		}

		answer := payload.Answer
		if answer == "" {
			answer = payload.Response
		}
		if answer != "" {
			return Message{Role: RoleAssistant, Content: answer}
		}
	}

	// No JSON at all: treat the whole reply as the final answer. A model that
	// simply answered a question in prose is not an error.
	return Message{Role: RoleAssistant, Content: cleaned}
}

// jsonCandidates extracts the plausible JSON objects from a reply, best first:
// the whole string, then fenced blocks, then each balanced brace span.
func jsonCandidates(text string) []string {
	seen := map[string]bool{}
	var candidates []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if len(value) < 2 || value[0] != '{' || seen[value] {
			return
		}
		seen[value] = true
		candidates = append(candidates, value)
	}

	add(text)
	for _, match := range fencedBlock.FindAllStringSubmatch(text, -1) {
		add(match[1])
	}
	for _, span := range balancedObjects(text) {
		add(span)
	}
	return candidates
}

// balancedObjects returns every top-level {...} span, skipping braces that sit
// inside a JSON string so an escaped quote cannot throw the scan off.
func balancedObjects(text string) []string {
	var spans []string
	depth, start := 0, -1
	inString, escaped := false, false

	for i, r := range text {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString:
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// nothing to do
		case r == '{':
			if depth == 0 {
				start = i
			}
			depth++
		case r == '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					spans = append(spans, text[start:i+1])
					start = -1
				}
			}
		}
	}
	return spans
}
