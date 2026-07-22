package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultOllemaHost = "http://localhost:11434"

type ollemaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"`
}

type ollemaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

// RunOllema sends one prompt to Ollama and writes the generated text to output.
func RunOllema(model, prompt string, output io.Writer) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("model name cannot be empty")
	}
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("prompt cannot be empty")
	}

	response, err := askOllema(model, prompt, "")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(output, response.Response)
	return err
}

func askOllema(model, prompt, format string) (ollemaResponse, error) {
	body, err := json.Marshal(ollemaRequest{Model: model, Prompt: prompt, Stream: false, Format: format})
	if err != nil {
		return ollemaResponse{}, fmt.Errorf("encode Ollama request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, ollemaGenerateURL(), bytes.NewReader(body))
	if err != nil {
		return ollemaResponse{}, fmt.Errorf("create Ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return ollemaResponse{}, fmt.Errorf("connect to Ollama: %w (is Ollama running?)", err)
	}
	defer res.Body.Close()

	var response ollemaResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return ollemaResponse{}, fmt.Errorf("decode Ollama response: %w", err)
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		if response.Error != "" {
			return ollemaResponse{}, fmt.Errorf("Ollama returned HTTP %s: %s", res.Status, response.Error)
		}
		return ollemaResponse{}, fmt.Errorf("Ollama returned HTTP %s", res.Status)
	}
	if response.Error != "" {
		return ollemaResponse{}, fmt.Errorf("Ollama error: %s", response.Error)
	}
	return response, nil
}

func ollemaGenerateURL() string {
	host := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	if host == "" {
		host = defaultOllemaHost
	} else if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return strings.TrimRight(host, "/") + "/api/generate"
}
