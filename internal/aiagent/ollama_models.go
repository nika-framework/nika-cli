package aiagent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// OllamaModel is one locally installed model and what it can do.
type OllamaModel struct {
	Name         string
	Capabilities []string
	SizeBytes    int64
}

// SupportsTools reports whether Ollama advertises native function calling.
func (m OllamaModel) SupportsTools() bool {
	for _, capability := range m.Capabilities {
		if capability == "tools" {
			return true
		}
	}
	return false
}

// DefaultOllamaHost is where Ollama listens unless told otherwise.
const DefaultOllamaHost = "http://localhost:11434"

// OllamaHost resolves the endpoint from OLLAMA_HOST or the default.
func OllamaHost() string {
	host := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	if host == "" {
		return DefaultOllamaHost
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return strings.TrimRight(host, "/")
}

// ListOllamaModels returns the installed models with their capabilities.
//
// This exists because hard-coding a default model name guesses wrong on every
// machine that has not pulled that exact tag: `nika agent init ollama` used to
// write "qwen2.5-coder:7b" whether or not it was installed, and the first real
// prompt failed with a model-not-found error.
func ListOllamaModels(baseURL string) ([]OllamaModel, error) {
	if baseURL == "" {
		baseURL = OllamaHost()
	}
	baseURL = strings.TrimRight(baseURL, "/")
	client := &http.Client{Timeout: 10 * time.Second}

	res, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("connect to Ollama at %s: %w", baseURL, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned HTTP %s from /api/tags", res.Status)
	}

	var tags struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("decode Ollama model list: %w", err)
	}

	models := make([]OllamaModel, 0, len(tags.Models))
	for _, tag := range tags.Models {
		model := OllamaModel{Name: tag.Name, SizeBytes: tag.Size}
		model.Capabilities = ollamaCapabilities(client, baseURL, tag.Name)
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	return models, nil
}

// ollamaCapabilities asks /api/show what one model can do. A failure is
// reported as "unknown" rather than an error: the caller can still use the
// model, it just has to discover the hard way whether tools work.
func ollamaCapabilities(client *http.Client, baseURL, model string) []string {
	body, err := json.Marshal(map[string]string{"model": model})
	if err != nil {
		return nil
	}
	res, err := client.Post(baseURL+"/api/show", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil
	}
	var shown struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(res.Body).Decode(&shown); err != nil {
		return nil
	}
	return shown.Capabilities
}

// preferredOllamaModels are good agent drivers, best first. A model matching
// one of these prefixes is chosen over an unrecognised tool-capable model.
var preferredOllamaModels = []string{
	"qwen2.5-coder", "qwen3-coder", "qwen3", "qwen2.5",
	"llama3.3", "llama3.2", "llama3.1",
	"mistral-nemo", "mistral-small", "devstral",
	"deepseek-r1", "firefunction",
}

// PickOllamaModel chooses the best installed model to drive the agent.
//
// Tool-capable models win outright. Among those, a recognised coding or
// function-calling family wins; otherwise the largest one, as a rough proxy
// for capability. Returns ok=false when nothing is installed.
func PickOllamaModel(models []OllamaModel) (OllamaModel, bool) {
	var capable []OllamaModel
	for _, model := range models {
		if model.SupportsTools() {
			capable = append(capable, model)
		}
	}

	pool := capable
	if len(pool) == 0 {
		// Nothing supports tools natively; the agent will emulate them, so
		// still pick the strongest-looking model rather than giving up.
		pool = models
	}
	if len(pool) == 0 {
		return OllamaModel{}, false
	}

	// Within a matching family take the biggest variant: deepseek-r1:7b is a
	// far better agent driver than deepseek-r1:1.5b, and picking whichever
	// sorted first would hand the user the 1.5b.
	for _, preferred := range preferredOllamaModels {
		var matches []OllamaModel
		for _, model := range pool {
			if strings.HasPrefix(strings.ToLower(model.Name), preferred) {
				matches = append(matches, model)
			}
		}
		if len(matches) > 0 {
			return largest(matches), true
		}
	}
	return largest(pool), true
}

// largest returns the biggest model by on-disk size, a rough proxy for
// capability when nothing more specific is known.
func largest(models []OllamaModel) OllamaModel {
	best := models[0]
	for _, model := range models[1:] {
		if model.SizeBytes > best.SizeBytes {
			best = model
		}
	}
	return best
}

// DescribeOllamaModels renders the installed models for the CLI.
func DescribeOllamaModels(models []OllamaModel) string {
	if len(models) == 0 {
		return "  (no models installed — run `ollama pull qwen2.5-coder:7b`)"
	}
	var out strings.Builder
	for _, model := range models {
		mark := "emulated tools"
		if model.SupportsTools() {
			mark = "native tools"
		}
		fmt.Fprintf(&out, "  %-24s %6.1f GB  %s\n", model.Name, float64(model.SizeBytes)/(1<<30), mark)
	}
	return strings.TrimRight(out.String(), "\n")
}
