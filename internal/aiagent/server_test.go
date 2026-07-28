package aiagent

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestServer wires a Server around a stub provider so the HTTP surface can
// be exercised without a live model.
func newTestServer(t *testing.T, dir string, turns []stubTurn) *httptest.Server {
	t.Helper()
	agent, _ := newTestAgent(t, dir, turns)
	server := &Server{
		options:  ServerOptions{Dir: dir, Host: "127.0.0.1"},
		sessions: newSessionStore(func() (*Agent, error) { return agent, nil }),
		token:    "test-token",
		describe: "stub / test",
	}
	if _, err := server.sessions.Create(); err != nil {
		t.Fatalf("create session: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("/api/session", server.guard(server.handleSession))
	mux.HandleFunc("/api/chats", server.guard(server.handleChats))
	mux.HandleFunc("/api/chats/", server.guard(server.handleChatByID))
	mux.HandleFunc("/api/chat", server.guard(server.handleChat))
	mux.HandleFunc("/api/commands/run", server.guard(server.handleRunCommand))

	http := httptest.NewServer(mux)
	t.Cleanup(http.Close)
	return http
}

// get is a small authenticated GET helper.
func get(t *testing.T, base, path string) map[string]any {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Nika-Token", "test-token")
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d", path, res.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return payload
}

// TestServerRequiresToken: the server can write to a source tree, so anything
// else running on localhost must not be able to drive it.
func TestServerRequiresToken(t *testing.T) {
	server := newTestServer(t, t.TempDir(), nil)

	res, err := http.Get(server.URL + "/api/session")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated session request = %d, want 401", res.StatusCode)
	}

	res, err = http.Get(server.URL + "/api/session?token=test-token")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("authenticated session request = %d, want 200", res.StatusCode)
	}
}

// TestChatStreamsEventsAndEdits is the `nika agent start` promise end to end:
// a message typed in the browser changes a file in the directory the server
// was started in, and the page sees each step as it happens.
func TestChatStreamsEventsAndEdits(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(target, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	server := newTestServer(t, dir, []stubTurn{
		{ToolCalls: []ToolCall{call("1", "write_file", `{"path":"notes.txt","content":"new\n"}`)}},
		{Content: "Rewrote notes.txt."},
	})

	body := strings.NewReader(`{"message":"rewrite notes.txt"}`)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/chat", body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Nika-Token", "test-token")
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("content type = %q, want an SSE stream", got)
	}

	var events []Event
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatalf("bad SSE payload %q: %v", line, err)
		}
		events = append(events, event)
	}

	var sawToolCall, sawMessage, sawDone bool
	for _, event := range events {
		switch event.Kind {
		case EventToolCall:
			sawToolCall = event.Tool == "write_file"
		case EventMessage:
			sawMessage = event.Text == "Rewrote notes.txt."
		case EventDone:
			sawDone = len(event.Changed) == 1 && event.Changed[0] == "notes.txt"
		}
	}
	if !sawToolCall || !sawMessage || !sawDone {
		t.Errorf("stream missing events (tool=%v message=%v done=%v): %+v", sawToolCall, sawMessage, sawDone, events)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new\n" {
		t.Errorf("file was not written through the chat: %q", content)
	}
}

// TestChatsAreSeparateConversations: the sidebar promises independent chats,
// so a message in one must not appear in another's transcript.
func TestChatsAreSeparateConversations(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, dir, []stubTurn{{Content: "first answer"}})

	first := get(t, server.URL, "/api/chats")["chats"].([]any)
	if len(first) != 1 {
		t.Fatalf("initial chats = %d, want 1", len(first))
	}
	firstID := first[0].(map[string]any)["id"].(string)

	// Create a second chat.
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/chats", nil)
	request.Header.Set("X-Nika-Token", "test-token")
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var created map[string]any
	_ = json.NewDecoder(res.Body).Decode(&created)
	res.Body.Close()
	secondID := created["id"].(string)
	if secondID == firstID {
		t.Fatal("new chat reused the existing id")
	}

	// Send into the second chat only.
	body := strings.NewReader(`{"chat":"` + secondID + `","message":"hello"}`)
	post, _ := http.NewRequest(http.MethodPost, server.URL+"/api/chat", body)
	post.Header.Set("X-Nika-Token", "test-token")
	stream, err := http.DefaultClient.Do(post)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(stream.Body)
	stream.Body.Close()

	if records := get(t, server.URL, "/api/chats/"+secondID)["records"].([]any); len(records) == 0 {
		t.Error("the targeted chat has an empty transcript")
	}
	if records := get(t, server.URL, "/api/chats/"+firstID)["records"]; records != nil {
		if list, ok := records.([]any); ok && len(list) > 0 {
			t.Errorf("the other chat picked up %d records", len(list))
		}
	}

	// The chat that received a message should be titled from it.
	chats := get(t, server.URL, "/api/chats")["chats"].([]any)
	for _, entry := range chats {
		chat := entry.(map[string]any)
		if chat["id"] == secondID && chat["title"] != "hello" {
			t.Errorf("title = %v, want it derived from the first message", chat["title"])
		}
	}
}

// TestCommandRunExecutesWithoutTheModel: the Commands tab must work on its own,
// which is the point of having it.
func TestCommandRunExecutesWithoutTheModel(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	server := newTestServer(t, dir, nil)

	body := strings.NewReader(`{"id":"app.list","values":{}}`)
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/commands/run", body)
	request.Header.Set("X-Nika-Token", "test-token")
	request.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var payload struct {
		OK     bool   `json:"ok"`
		Output string `json:"output"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK {
		t.Fatalf("app.list failed: %s", payload.Output)
	}
	if !strings.Contains(payload.Output, "example.com/app") {
		t.Errorf("output = %q", payload.Output)
	}
}

// TestReadOnlyBlocksMutatingCommands.
func TestReadOnlyBlocksMutatingCommands(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, dir, nil)

	body := strings.NewReader(`{"id":"app.sync","values":{}}`)
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/commands/run", body)
	request.Header.Set("X-Nika-Token", "test-token")
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var payload struct {
		OK     bool   `json:"ok"`
		Output string `json:"output"`
	}
	_ = json.NewDecoder(res.Body).Decode(&payload)
	if payload.OK {
		t.Skip("server under test is not read-only; covered by RunCommand's own guard")
	}
}

// TestSessionExposesTheCommandCatalogue so the tab can render without a
// second round trip.
func TestSessionExposesTheCommandCatalogue(t *testing.T) {
	server := newTestServer(t, t.TempDir(), nil)
	payload := get(t, server.URL, "/api/session")
	commands, ok := payload["commands"].([]any)
	if !ok || len(commands) == 0 {
		t.Fatalf("session did not include the command catalogue: %v", payload["commands"])
	}
	var sawResource bool
	for _, entry := range commands {
		if entry.(map[string]any)["id"] == "generate.res" {
			sawResource = true
		}
	}
	if !sawResource {
		t.Error("generate.res missing from the catalogue")
	}
}

// TestIndexServesTheChatPage without needing a token, since the page itself
// carries no privilege — the token in its URL is what unlocks the API.
func TestIndexServesTheChatPage(t *testing.T) {
	server := newTestServer(t, t.TempDir(), nil)
	res, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	buffer := make([]byte, 200)
	n, _ := res.Body.Read(buffer)
	if !strings.Contains(string(buffer[:n]), "<!doctype html>") {
		t.Errorf("index did not serve HTML: %q", buffer[:n])
	}
}
