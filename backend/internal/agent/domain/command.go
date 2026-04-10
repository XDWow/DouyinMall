package domain

// TurnInput 本轮用户入参（只读语义），与共享上下文 State 分离。
type TurnInput = ChatCommand

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

