package domain

// MessageTurn is the compact cross-turn conversation history stored in session.
type MessageTurn struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}
