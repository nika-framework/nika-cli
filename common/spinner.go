package common

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// spinnerFrames is the animation characters.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner provides a terminal-friendly step-by-step loading animation.
type Spinner struct {
	mu      sync.Mutex
	active  bool
	frame   int
	message string
	done    chan struct{}
}

// NewSpinner creates a ready-to-use Spinner.
func NewSpinner() *Spinner {
	return &Spinner{}
}

// startLoop runs the animation in a background goroutine.
// It prints until s.active is set to false.
func (s *Spinner) startLoop() {
	s.done = make(chan struct{})
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				s.mu.Lock()
				if !s.active {
					s.mu.Unlock()
					return
				}
				frame := spinnerFrames[s.frame%len(spinnerFrames)]
				s.frame++
				msg := s.message
				s.mu.Unlock()

				fmt.Printf("\r%s %s", frame, msg)
			}
		}
	}()
}

// stopLoop signals the background goroutine to stop and waits for it.
func (s *Spinner) stopLoop() {
	s.mu.Lock()
	s.active = false
	s.mu.Unlock()
	if s.done != nil {
		close(s.done)
		// Give the goroutine a tick to finish printing.
		time.Sleep(100 * time.Millisecond)
	}
}

// Start begins the animation for the given message (non-blocking).
func (s *Spinner) Start(message string) {
	s.mu.Lock()
	s.frame = 0
	s.message = message
	s.active = true
	s.mu.Unlock()
	s.startLoop()
}

// Step stops the current animation, prints a completion line, then starts
// the next step.
func (s *Spinner) Step(completedMsg, nextMessage string) {
	s.stopLoop()
	fmt.Printf("\r  ✔ %s\n", completedMsg)

	if nextMessage == "" {
		return
	}

	s.mu.Lock()
	s.frame = 0
	s.message = nextMessage
	s.active = true
	s.mu.Unlock()
	s.startLoop()
}

// Stop halts the animation and prints a final success message.
func (s *Spinner) Stop(finalMsg string) {
	s.stopLoop()
	fmt.Printf("\r  ✔ %s\n", finalMsg)
}

// Fail halts the animation and prints an error message.
func (s *Spinner) Fail(errMsg string) {
	s.stopLoop()
	fmt.Printf("\r  ✖ %s\n", errMsg)
}

// Section prints a styled section header.
func Section(title string) {
	width := len(title) + 4
	fmt.Println()
	fmt.Println(strings.Repeat("─", width))
	fmt.Printf("  %s\n", title)
	fmt.Println(strings.Repeat("─", width))
}
