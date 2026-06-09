package chat

import "time"

type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`    // user | assistant | system | tool
	Content   string    `json:"content"` // display text
	Data      any       `json:"data,omitempty"` // structured payload (table, list, etc.)
	DataType  string    `json:"dataType,omitempty"` // "table" | "list" | "yaml" | "text"
	Timestamp time.Time `json:"timestamp"`
	Error     bool      `json:"error,omitempty"`
}

type Session struct {
	ID              string    `json:"id"`
	Namespace       string    `json:"namespace"`        // current context namespace
	Messages        []Message `json:"messages"`
	LastResource    string    `json:"lastResource"`     // last mentioned pod/deploy name
	LastResourceNS  string    `json:"lastResourceNS"`   // namespace of last resource
	LastResourceKind string   `json:"lastResourceKind"` // kind of last resource
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type TableRow map[string]string

type TableData struct {
	Headers []string   `json:"headers"`
	Rows    []TableRow `json:"rows"`
	Title   string     `json:"title,omitempty"`
}

type StreamChunk struct {
	Type     string   `json:"type"`             // "token" | "data" | "done" | "error" | "confirm"
	Content  string   `json:"content"`
	Data     any      `json:"data,omitempty"`
	DataType string   `json:"dataType,omitempty"`
	Actions  []Action `json:"actions,omitempty"` // for "confirm" type
}

type Action struct {
	Label   string `json:"label"`
	Command string `json:"command"` // sent back as a user message when clicked
	Style   string `json:"style"`   // "primary" | "danger" | "ghost"
}
