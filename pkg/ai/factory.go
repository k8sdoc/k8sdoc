package ai

// clients is the global registry of all supported AI backends.
var clients = []IAI{
	&OllamaClient{},
	&NoOpAIClient{},
}

var Backends = []string{
	ollamaClientName,
	noopAIClientName,
}

// NewClient returns the IAI implementation for the given provider name
// Defaults to OllamaClient when provider is unknown
func NewClient(provider string) IAI {
	for _, c := range clients {
		if provider == c.GetName() {
			return c
		}
	}
	return &OllamaClient{}
}
