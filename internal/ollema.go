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

type agentRuntime struct {
	Provider  string
	Model     string
	BaseURL   string
	APIKeyEnv string
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
	return askAgent(agentRuntime{Provider: "ollama", Model: model, BaseURL: os.Getenv("OLLAMA_HOST")}, prompt, format)
}

func askAgent(runtime agentRuntime, prompt, format string) (ollemaResponse, error) {
	if runtime.Provider == "ollama" {
		return askOllamaRuntime(runtime, prompt, format)
	}
	return askOpenAICompatibleRuntime(runtime, prompt, format)
}

func askOllamaRuntime(runtime agentRuntime, prompt, format string) (ollemaResponse, error) {
	body, err := json.Marshal(ollemaRequest{Model: runtime.Model, Prompt: prompt, Stream: false, Format: format})
	if err != nil {
		return ollemaResponse{}, fmt.Errorf("encode Ollama request: %w", err)
	}
	endpoint := strings.TrimRight(runtime.BaseURL, "/")
	if endpoint == "" {
		endpoint = defaultOllemaHost
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	req, err := http.NewRequest(http.MethodPost, endpoint+"/api/generate", bytes.NewReader(body))
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

type openAIRequest struct {
	Model          string                `json:"model"`
	Messages       []openAIMessage       `json:"messages"`
	ResponseFormat *openAIResponseFormat `json:"response_format,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func askOpenAICompatibleRuntime(runtime agentRuntime, prompt, format string) (ollemaResponse, error) {
	apiKey := strings.TrimSpace(os.Getenv(runtime.APIKeyEnv))
	if apiKey == "" {
		return ollemaResponse{}, fmt.Errorf("environment variable %s is not set", runtime.APIKeyEnv)
	}
	request := openAIRequest{
		Model:    runtime.Model,
		Messages: []openAIMessage{{Role: "user", Content: prompt}},
	}
	if format == "json" {
		request.ResponseFormat = &openAIResponseFormat{Type: "json_object"}
	}
	body, err := json.Marshal(request)
	if err != nil {
		return ollemaResponse{}, fmt.Errorf("encode agent request: %w", err)
	}
	endpoint := strings.TrimRight(runtime.BaseURL, "/")
	if endpoint == "" {
		return ollemaResponse{}, fmt.Errorf("agent base_url is empty")
	}
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ollemaResponse{}, fmt.Errorf("create agent request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	res, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return ollemaResponse{}, fmt.Errorf("connect to %s: %w", runtime.Provider, err)
	}
	defer res.Body.Close()
	var response openAIResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return ollemaResponse{}, fmt.Errorf("decode agent response: %w", err)
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		if response.Error.Message != "" {
			return ollemaResponse{}, fmt.Errorf("%s returned HTTP %s: %s", runtime.Provider, res.Status, response.Error.Message)
		}
		return ollemaResponse{}, fmt.Errorf("%s returned HTTP %s", runtime.Provider, res.Status)
	}
	if len(response.Choices) == 0 {
		return ollemaResponse{}, fmt.Errorf("%s returned no choices", runtime.Provider)
	}
	return ollemaResponse{Response: response.Choices[0].Message.Content}, nil
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
