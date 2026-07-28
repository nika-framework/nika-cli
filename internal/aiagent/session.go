package aiagent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Record is one entry in a chat's replayable transcript.
//
// The browser needs to redraw a conversation when the user switches back to
// it, and re-rendering from the model's message list would lose the tool trace
// that makes the run legible. So the stream the UI consumed live is what gets
// stored.
type Record struct {
	Kind  string `json:"kind"` // "user" or "event"
	Text  string `json:"text,omitempty"`
	Event *Event `json:"event,omitempty"`
}

// Session is one conversation with its own agent state.
type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  int       `json:"messages"`

	agent   *Agent
	records []Record
	mu      sync.Mutex
}

// Transcript returns a copy of the records for replay.
func (s *Session) Transcript() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Record(nil), s.records...)
}

func (s *Session) appendUser(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, Record{Kind: "user", Text: text})
	s.Messages++
	s.UpdatedAt = time.Now()
	if s.Title == "" || s.Title == untitledChat {
		s.Title = titleFrom(text)
	}
}

func (s *Session) appendEvent(event Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := event
	s.records = append(s.records, Record{Kind: "event", Event: &copied})
	s.UpdatedAt = time.Now()
}

func (s *Session) reset() {
	s.mu.Lock()
	s.records = nil
	s.Messages = 0
	s.Title = untitledChat
	s.mu.Unlock()
	s.agent.Reset()
}

const untitledChat = "New chat"

// titleFrom derives a sidebar label from the first message.
func titleFrom(text string) string {
	title := strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	for strings.Contains(title, "  ") {
		title = strings.ReplaceAll(title, "  ", " ")
	}
	// Count runes, not bytes: a Persian or Arabic title is multi-byte and a
	// byte slice would cut through a character.
	runes := []rune(title)
	if len(runes) > 48 {
		return string(runes[:48]) + "…"
	}
	if title == "" {
		return untitledChat
	}
	return title
}

// sessionStore holds every chat in one browser-side workspace.
//
// Each session gets its own Agent so conversations do not bleed into each
// other, but they share one project directory: the point of the tool is to act
// on the directory it was started in, and a per-chat sandbox would break that.
type sessionStore struct {
	mu       sync.Mutex
	order    []string
	byID     map[string]*Session
	newAgent func() (*Agent, error)
}

func newSessionStore(newAgent func() (*Agent, error)) *sessionStore {
	return &sessionStore{byID: map[string]*Session{}, newAgent: newAgent}
}

// Create starts a new chat and returns it.
func (s *sessionStore) Create() (*Session, error) {
	agent, err := s.newAgent()
	if err != nil {
		return nil, err
	}
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	session := &Session{ID: id, Title: untitledChat, CreatedAt: now, UpdatedAt: now, agent: agent}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[id] = session
	s.order = append([]string{id}, s.order...)
	return session, nil
}

// Get returns a session by ID.
func (s *sessionStore) Get(id string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.byID[id]
	return session, ok
}

// GetOrCreate returns the named session, the most recent one, or a new one.
func (s *sessionStore) GetOrCreate(id string) (*Session, error) {
	if id != "" {
		if session, ok := s.Get(id); ok {
			return session, nil
		}
	}
	s.mu.Lock()
	if len(s.order) > 0 {
		session := s.byID[s.order[0]]
		s.mu.Unlock()
		return session, nil
	}
	s.mu.Unlock()
	return s.Create()
}

// List returns every chat, newest activity first.
func (s *sessionStore) List() []*Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessions := make([]*Session, 0, len(s.order))
	for _, id := range s.order {
		sessions = append(sessions, s.byID[id])
	}
	return sessions
}

// Delete removes a chat. The last remaining one is reset instead of removed,
// so the UI always has something to show.
func (s *sessionStore) Delete(id string) bool {
	s.mu.Lock()
	session, ok := s.byID[id]
	if !ok {
		s.mu.Unlock()
		return false
	}
	if len(s.order) == 1 {
		s.mu.Unlock()
		session.reset()
		return true
	}
	delete(s.byID, id)
	remaining := make([]string, 0, len(s.order)-1)
	for _, existing := range s.order {
		if existing != id {
			remaining = append(remaining, existing)
		}
	}
	s.order = remaining
	s.mu.Unlock()
	return true
}

// Touch moves a session to the top of the list.
func (s *sessionStore) Touch(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.order) == 0 || s.order[0] == id {
		return
	}
	reordered := []string{id}
	for _, existing := range s.order {
		if existing != id {
			reordered = append(reordered, existing)
		}
	}
	s.order = reordered
}

func randomID() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
