package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client implements core.EmbedProvider using Ollama's local HTTP API.
type Client struct {
	baseURL string
	model   string
	timeout time.Duration
	http    *http.Client
}

// New creates an Ollama embedding client.
func New(baseURL, model string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		timeout: timeout,
		http:    &http.Client{},
	}
}

type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed generates a 768-dim vector for the given text.
// Implements core.EmbedProvider.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body, err := json.Marshal(embedRequest{Model: c.model, Prompt: text})
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: unexpected status %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("ollama: empty embedding returned")
	}

	return result.Embedding, nil
}

// CheckHealth pings Ollama to verify it is reachable.
func (c *Client) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("ollama: build health request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check failed: status %d", resp.StatusCode)
	}
	return nil
}
