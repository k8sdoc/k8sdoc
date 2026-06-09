// Package ai defines the IAI interface and Ollama provider
package ai

import (
	"context"
	"net/http"
)

// IAI is implemented by every AI backend client.
type IAI interface {
	Configure(config IAIConfig) error
	// GetCompletion returns the full response (non-streaming)
	GetCompletion(ctx context.Context, prompt string) (string, error)
	// GetCompletionStream streams tokens via the callback. Used for chat UI
	GetCompletionStream(ctx context.Context, prompt string, onToken func(string)) error
	// GetChatCompletion sends a full conversation history (multi-turn)
	GetChatCompletion(ctx context.Context, messages []ChatMessage) (string, error)
	// GetChatCompletionStream streams a multi-turn conversation
	GetChatCompletionStream(ctx context.Context, messages []ChatMessage, onToken func(string)) error
	GetName() string
	Close()
}

// ChatMessage is a single turn in a multi-turn conversation.
type ChatMessage struct {
	Role    string `json:"role"`    // system | user | assistant
	Content string `json:"content"`
}

// IAIConfig is the read only view of provider settings.
type IAIConfig interface {
	GetModel() string
	GetBaseURL() string
	GetProxyEndpoint() string
	GetTemperature() float32
	GetTopP() float32
	GetMaxTokens() int
	GetCustomHeaders() []http.Header
}

// AIConfiguration is stored under the "ai" key in config.yaml
type AIConfiguration struct {
	Providers       []AIProvider `mapstructure:"providers"`
	DefaultProvider string       `mapstructure:"defaultprovider"`
}

type AIProvider struct {
	Name        string        `mapstructure:"name"`
	Model       string        `mapstructure:"model"`
	BaseURL     string        `mapstructure:"baseurl"     yaml:"baseurl,omitempty"`
	Temperature float32       `mapstructure:"temperature" yaml:"temperature,omitempty"`
	TopP        float32       `mapstructure:"topp"        yaml:"topp,omitempty"`
	MaxTokens   int           `mapstructure:"maxtokens"   yaml:"maxtokens,omitempty"`
	CustomHeaders []http.Header `mapstructure:"customHeaders"`
}

func (p *AIProvider) GetModel() string           { return p.Model }
func (p *AIProvider) GetBaseURL() string         { return p.BaseURL }
func (p *AIProvider) GetProxyEndpoint() string   { return "" }
func (p *AIProvider) GetTemperature() float32    { return p.Temperature }
func (p *AIProvider) GetTopP() float32           { return p.TopP }
func (p *AIProvider) GetMaxTokens() int          { return p.MaxTokens }
func (p *AIProvider) GetCustomHeaders() []http.Header { return p.CustomHeaders }

type nopCloser struct{}
func (nopCloser) Close() {}
