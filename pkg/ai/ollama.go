package ai

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	ollama "github.com/ollama/ollama/api"
)

const (
	ollamaClientName   = "ollama"
	defaultOllamaURL   = "http://localhost:11434"
	defaultOllamaModel = "qwen2.5-coder:14b-instruct-q4_K_M"
)

type OllamaClient struct {
	nopCloser
	client      *ollama.Client
	model       string
	temperature float32
	topP        float32
}

func (c *OllamaClient) Configure(cfg IAIConfig) error {
	baseURL := cfg.GetBaseURL()
	if baseURL == "" {
		baseURL = defaultOllamaURL
	}
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	httpClient := http.DefaultClient
	if proxy := cfg.GetProxyEndpoint(); proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return err
		}
		httpClient = &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		}
	}

	c.client = ollama.NewClient(parsedURL, httpClient)
	if c.client == nil {
		return errors.New("failed to create Ollama client")
	}
	c.model = cfg.GetModel()
	if c.model == "" {
		c.model = defaultOllamaModel
	}
	c.temperature = cfg.GetTemperature()
	c.topP = cfg.GetTopP()
	return nil
}

func (c *OllamaClient) GetCompletion(ctx context.Context, prompt string) (string, error) {
	streamFalse := false
	req := &ollama.GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: &streamFalse,
		Options: map[string]interface{}{
			"temperature": c.temperature,
			"top_p":       c.topP,
		},
	}
	var result string
	err := c.client.Generate(ctx, req, func(resp ollama.GenerateResponse) error {
		result = resp.Response
		return nil
	})
	return result, err
}

func (c *OllamaClient) GetCompletionStream(ctx context.Context, prompt string, onToken func(string)) error {
	streamTrue := true
	req := &ollama.GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: &streamTrue,
		Options: map[string]interface{}{
			"temperature": c.temperature,
			"top_p":       c.topP,
		},
	}
	return c.client.Generate(ctx, req, func(resp ollama.GenerateResponse) error {
		if resp.Response != "" {
			onToken(resp.Response)
		}
		return nil
	})
}

func (c *OllamaClient) GetChatCompletion(ctx context.Context, messages []ChatMessage) (string, error) {
	streamFalse := false
	ollamaMessages := toOllamaMessages(messages)
	req := &ollama.ChatRequest{
		Model:   c.model,
		Stream:  &streamFalse,
		Messages: ollamaMessages,
		Options: map[string]interface{}{
			"temperature": c.temperature,
			"top_p":       c.topP,
		},
	}
	var result string
	err := c.client.Chat(ctx, req, func(resp ollama.ChatResponse) error {
		result += resp.Message.Content
		return nil
	})
	return result, err
}

func (c *OllamaClient) GetChatCompletionStream(ctx context.Context, messages []ChatMessage, onToken func(string)) error {
	streamTrue := true
	ollamaMessages := toOllamaMessages(messages)
	req := &ollama.ChatRequest{
		Model:   c.model,
		Stream:  &streamTrue,
		Messages: ollamaMessages,
		Options: map[string]interface{}{
			"temperature": c.temperature,
			"top_p":       c.topP,
		},
	}
	return c.client.Chat(ctx, req, func(resp ollama.ChatResponse) error {
		if resp.Message.Content != "" {
			onToken(resp.Message.Content)
		}
		return nil
	})
}

func (c *OllamaClient) GetName() string { return ollamaClientName }

func toOllamaMessages(msgs []ChatMessage) []ollama.Message {
	out := make([]ollama.Message, len(msgs))
	for i, m := range msgs {
		out[i] = ollama.Message{Role: m.Role, Content: m.Content}
	}
	return out
}
