package domain

type ChatCommand struct {
	SessionID   string
	UserID      int64
	Message     string
	ResumeToken string
	Metadata    map[string]string
}

type CreateSessionCommand struct {
	UserID int64
}

type HistoryQuery struct {
	SessionID string
	Limit     int
	Offset    int
}

type SessionListQuery struct {
	UserID int64
	Limit  int
	Offset int
}

type ClearSessionCommand struct {
	SessionID string
}
