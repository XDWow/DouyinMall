package domain

// ChatInput only carries raw request input and trusted request-scoped context.
type ChatInput struct {
	SessionID   string            `json:"session_id,omitempty"`
	UserID      int64             `json:"user_id"`
	Message     string            `json:"message,omitempty"`
	ResumeToken string            `json:"resume_token,omitempty"`
	InterruptID string            `json:"interrupt_id,omitempty"`
	ResumeData  map[string]any    `json:"resume_data,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}
