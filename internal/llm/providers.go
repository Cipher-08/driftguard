package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Settings configures which provider to use. The first one with credentials
// wins, in this order: explicit Provider override, then Groq, Gemini, Ollama.
type Settings struct {
	Provider string // optional explicit override: groq|gemini|ollama

	GroqAPIKey string
	GroqModel  string

	GeminiAPIKey string
	GeminiModel  string

	OllamaHost  string
	OllamaModel string
}

// New returns a configured Client, or nil if no provider is available.
func New(s Settings) Client {
	pick := s.Provider
	switch pick {
	case "groq":
		if s.GroqAPIKey != "" {
			return newGroq(s)
		}
	case "gemini":
		if s.GeminiAPIKey != "" {
			return newGemini(s)
		}
	case "ollama":
		if s.OllamaHost != "" {
			return newOllama(s)
		}
	}
	// Auto-detect when no (valid) explicit provider was given.
	switch {
	case s.GroqAPIKey != "":
		return newGroq(s)
	case s.GeminiAPIKey != "":
		return newGemini(s)
	case s.OllamaHost != "":
		return newOllama(s)
	}
	return nil
}

var httpClient = &http.Client{Timeout: 90 * time.Second}

// ---- Groq (OpenAI-compatible) ----

type groqClient struct {
	apiKey string
	model  string
}

func newGroq(s Settings) *groqClient {
	model := s.GroqModel
	if model == "" {
		model = "llama-3.3-70b-versatile"
	}
	return &groqClient{apiKey: s.GroqAPIKey, model: model}
}

func (g *groqClient) Name() string { return "groq" }

func (g *groqClient) GeneratePatch(ctx context.Context, req PatchRequest) (string, error) {
	body := map[string]interface{}{
		"model":       g.model,
		"temperature": 0.1,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": buildUserPrompt(req)},
		},
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := postJSON(ctx, "https://api.groq.com/openai/v1/chat/completions",
		map[string]string{"Authorization": "Bearer " + g.apiKey}, body, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("groq: empty response")
	}
	return stripFences(out.Choices[0].Message.Content), nil
}

// ---- Google Gemini ----

type geminiClient struct {
	apiKey string
	model  string
}

func newGemini(s Settings) *geminiClient {
	model := s.GeminiModel
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &geminiClient{apiKey: s.GeminiAPIKey, model: model}
}

func (g *geminiClient) Name() string { return "gemini" }

func (g *geminiClient) GeneratePatch(ctx context.Context, req PatchRequest) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.model, g.apiKey)
	body := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []map[string]string{{"text": systemPrompt}},
		},
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]string{{"text": buildUserPrompt(req)}}},
		},
		"generationConfig": map[string]interface{}{"temperature": 0.1},
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := postJSON(ctx, url, nil, body, &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty response")
	}
	return stripFences(out.Candidates[0].Content.Parts[0].Text), nil
}

// ---- Ollama (local, fully free) ----

type ollamaClient struct {
	host  string
	model string
}

func newOllama(s Settings) *ollamaClient {
	model := s.OllamaModel
	if model == "" {
		model = "llama3.1"
	}
	return &ollamaClient{host: s.OllamaHost, model: model}
}

func (o *ollamaClient) Name() string { return "ollama" }

func (o *ollamaClient) GeneratePatch(ctx context.Context, req PatchRequest) (string, error) {
	body := map[string]interface{}{
		"model":  o.model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": buildUserPrompt(req)},
		},
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := postJSON(ctx, o.host+"/api/chat", nil, body, &out); err != nil {
		return "", err
	}
	return stripFences(out.Message.Content), nil
}

// postJSON marshals body, POSTs it, and decodes the response into out.
func postJSON(ctx context.Context, url string, headers map[string]string, body, out interface{}) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	return json.Unmarshal(raw, out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
