package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunOllema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var request ollemaRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "test-model" || request.Prompt != "hello" || request.Stream {
			t.Fatalf("unexpected request body: %+v", request)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"hello back"}`))
	}))
	defer server.Close()
	t.Setenv("OLLAMA_HOST", server.URL)

	var output strings.Builder
	if err := RunOllema("test-model", "hello", &output); err != nil {
		t.Fatalf("RunOllema() error = %v", err)
	}
	if output.String() != "hello back\n" {
		t.Fatalf("output = %q, want %q", output.String(), "hello back\n")
	}
}

func TestRunOllemaRejectsEmptyArguments(t *testing.T) {
	for _, test := range []struct {
		name   string
		model  string
		prompt string
	}{
		{name: "empty model", prompt: "hello"},
		{name: "empty prompt", model: "test-model"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := RunOllema(test.model, test.prompt, &strings.Builder{}); err == nil {
				t.Fatal("RunOllema() error = nil, want validation error")
			}
		})
	}
}
