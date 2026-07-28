package aiagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ServerOptions configures `nika agent start`.
type ServerOptions struct {
	// Dir is the project the chat operates on. Every instruction typed in the
	// browser is applied here — the directory the command was started in.
	Dir string
	// Port to listen on. 0 picks a free one.
	Port int
	// Host to bind. Defaults to 127.0.0.1; the server holds write access to a
	// source tree, so it must not be reachable from the network by accident.
	Host string
	// Open launches the system browser.
	Open bool
	// ReadOnly refuses every mutating tool.
	ReadOnly bool
	// AllowAnyCommand disables the run_command allowlist.
	AllowAnyCommand bool
}

// Server hosts the chat UI and the command console.
type Server struct {
	options  ServerOptions
	sessions *sessionStore
	token    string
	describe string

	mu      sync.Mutex
	running bool
}

// NewServer builds the chat server for a project directory.
func NewServer(options ServerOptions) (*Server, error) {
	if strings.TrimSpace(options.Dir) == "" {
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		options.Dir = dir
	}
	if strings.TrimSpace(options.Host) == "" {
		options.Host = "127.0.0.1"
	}

	newAgent := func() (*Agent, error) {
		agent, err := New(options.Dir)
		if err != nil {
			return nil, err
		}
		agent.Toolbox.ReadOnly = options.ReadOnly
		agent.Toolbox.AllowAnyCommand = options.AllowAnyCommand
		return agent, nil
	}

	// Build one agent up front so a bad configuration fails at start rather
	// than on the user's first message.
	probe, err := newAgent()
	if err != nil {
		return nil, err
	}

	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}

	server := &Server{
		options:  options,
		sessions: newSessionStore(newAgent),
		token:    hex.EncodeToString(raw),
		describe: probe.Provider.Describe(),
	}
	if _, err := server.sessions.Create(); err != nil {
		return nil, err
	}
	return server, nil
}

// ListenAndServe starts the chat server and blocks until interrupted.
func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.options.Host, s.options.Port))
	if err != nil {
		return fmt.Errorf("listen on %s:%d: %w", s.options.Host, s.options.Port, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/session", s.guard(s.handleSession))
	mux.HandleFunc("/api/chats", s.guard(s.handleChats))
	mux.HandleFunc("/api/chats/", s.guard(s.handleChatByID))
	mux.HandleFunc("/api/chat", s.guard(s.handleChat))
	mux.HandleFunc("/api/commands/run", s.guard(s.handleRunCommand))

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	url := fmt.Sprintf("http://%s/?token=%s", listener.Addr().String(), s.token)
	fmt.Println()
	fmt.Println("  🤖 Nika agent chat is running")
	fmt.Printf("     Project : %s\n", s.options.Dir)
	fmt.Printf("     Model   : %s\n", s.describe)
	if s.options.ReadOnly {
		fmt.Println("     Mode    : read-only (no files will be changed)")
	}
	fmt.Printf("     Open    : %s\n", url)
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println()

	if s.options.Open {
		openBrowser(url)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	errs := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errs <- err
		}
	}()

	select {
	case err := <-errs:
		return err
	case <-stop:
		fmt.Println("\n  🛑 Stopping agent chat...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
}

// guard rejects requests without the session token.
func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Nika-Token")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token != s.token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(chatPage))
}

type sessionInfo struct {
	Project  string    `json:"project"`
	Model    string    `json:"model"`
	Provider string    `json:"provider"`
	ReadOnly bool      `json:"read_only"`
	Apps     []string  `json:"apps"`
	Commands []Command `json:"commands"`
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	info := sessionInfo{
		Project:  s.options.Dir,
		Model:    s.describe,
		ReadOnly: s.options.ReadOnly,
		Commands: Commands(),
	}
	if apps, err := loadApps(s.options.Dir); err == nil {
		info.Apps = apps
	}
	writeJSON(w, info)
}

// handleChats lists the chats or creates one.
func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"chats": s.sessions.List()})
	case http.MethodPost:
		session, err := s.sessions.Create()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, session)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleChatByID returns one chat's transcript or deletes it.
func (s *Server) handleChatByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/chats/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		session, ok := s.sessions.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{"chat": session, "records": session.Transcript()})
	case http.MethodDelete:
		if !s.sessions.Delete(id) {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{"chats": s.sessions.List()})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleChat streams one run back as server-sent events.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Chat    string `json:"chat"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Message) == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	session, err := s.sessions.GetOrCreate(body.Chat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// One run at a time across the whole server: two agents editing the same
	// tree would interleave edits and corrupt files, even from different chats.
	if !s.acquire() {
		http.Error(w, "the agent is already working on a message", http.StatusConflict)
		return
	}
	defer s.release()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	var writeMu sync.Mutex
	send := func(event Event) {
		session.appendEvent(event)
		writeMu.Lock()
		defer writeMu.Unlock()
		payload, err := json.Marshal(event)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}

	session.appendUser(body.Message)
	s.sessions.Touch(session.ID)
	fmt.Printf("  💬 %s\n", strings.TrimSpace(body.Message))

	if _, err := session.agent.Run(r.Context(), body.Message, send); err != nil {
		send(Event{Kind: EventDone, Changed: session.agent.ChangedFiles()})
	}
}

// handleRunCommand executes a Commands-tab form.
func (s *Server) handleRunCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input CommandInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Generators change the same tree the agent edits, so they take the same
	// lock rather than racing it.
	if !s.acquire() {
		http.Error(w, "the agent is busy — wait for it to finish", http.StatusConflict)
		return
	}
	defer s.release()

	fmt.Printf("  ⚙ %s\n", input.ID)
	output, err := RunCommand(s.options.Dir, input, s.options.ReadOnly)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "output": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "output": output})
}

// acquire takes the single-runner lock.
func (s *Server) acquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *Server) release() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

// openBrowser best-effort opens the chat page.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("  ⚠ Could not open a browser automatically: %v\n", err)
	}
}
