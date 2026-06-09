package ai

import "context"

const noopAIClientName = "noopai"

type NoOpAIClient struct{ nopCloser }

func (c *NoOpAIClient) Configure(_ IAIConfig) error { return nil }
func (c *NoOpAIClient) GetName() string             { return noopAIClientName }

func (c *NoOpAIClient) GetCompletion(_ context.Context, _ string) (string, error) {
	return "[AI disabled — use --explain to enable]", nil
}
func (c *NoOpAIClient) GetCompletionStream(_ context.Context, _ string, onToken func(string)) error {
	onToken("[AI disabled]")
	return nil
}
func (c *NoOpAIClient) GetChatCompletion(_ context.Context, _ []ChatMessage) (string, error) {
	return "[AI disabled]", nil
}
func (c *NoOpAIClient) GetChatCompletionStream(_ context.Context, _ []ChatMessage, onToken func(string)) error {
	onToken("[AI disabled]")
	return nil
}
