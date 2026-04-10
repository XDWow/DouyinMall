package global

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
)

func TestSessionLoadNodeLoadsPersistedSlotsAndHistory(t *testing.T) {
	node := NewSessionLoadNode(&stubSessionMemory{
		session: &domain.Session{
			SessionID: "sess_1",
			UserID:    123,
			Status:    domain.SessionStatusActive,
			Slots: map[string]any{
				"order_id": "99999",
				"sku_id":   "30003",
			},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			TotalTurns: 2,
		},
		messages: []domain.SessionMessage{
			{SessionID: "sess_1", Role: domain.RoleUser, Content: "hello"},
			{SessionID: "sess_1", Role: domain.RoleAssistant, Content: "hi"},
		},
	})

	st := domain.NewState(domain.ChatCommand{SessionID: "sess_1", UserID: 123, Message: "x"}, nil, nil)
	if err := node.PrepareSession(context.Background(), st); err != nil {
		t.Fatalf("PrepareSession() error = %v", err)
	}
	if st.Session.SessionID != "sess_1" {
		t.Fatalf("SessionID = %q", st.Session.SessionID)
	}
	if len(st.Session.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2", len(st.Session.Messages))
	}
	if st.Session.Slots["order_id"] != "99999" {
		t.Fatalf("order_id = %v", st.Session.Slots["order_id"])
	}
	if st.Session.Slots["sku_id"] != "30003" {
		t.Fatalf("sku_id = %v", st.Session.Slots["sku_id"])
	}
	if _, ok := st.Session.Slots["reason"]; ok {
		t.Fatalf("expected no reason slot from non-persisted merge")
	}
	if st.PersistedSession == nil || st.PersistedSession.Slots == nil {
		t.Fatal("expected persisted slots merged for storage shape")
	}
}

func TestSessionLoadNodeCreatesSessionWhenMissing(t *testing.T) {
	mem := &stubSessionMemory{}
	node := NewSessionLoadNode(mem)

	st := domain.NewState(domain.ChatCommand{UserID: 456, Message: "x"}, nil, nil)
	st.TraceID = "trace_new"
	if err := node.PrepareSession(context.Background(), st); err != nil {
		t.Fatalf("PrepareSession() error = %v", err)
	}
	if st.Session.SessionID != "sess_trace_new" {
		t.Fatalf("SessionID = %q", st.Session.SessionID)
	}
	if mem.created == nil || mem.created.SessionID != "sess_trace_new" {
		t.Fatalf("expected session to be auto created, got %+v", mem.created)
	}
}

type stubSessionMemory struct {
	session  *domain.Session
	messages []domain.SessionMessage
	created  *domain.Session
}

func (s *stubSessionMemory) LoadSnapshot(context.Context, string) (*agentsession.Snapshot, error) {
	if s.session == nil {
		return nil, nil
	}
	return &agentsession.Snapshot{
		PersistedSession: *s.session,
		Messages:         append([]domain.SessionMessage(nil), s.messages...),
	}, nil
}

func (s *stubSessionMemory) CreateSession(_ context.Context, session domain.Session) error {
	cloned := session
	s.created = &cloned
	return nil
}

func (s *stubSessionMemory) BuildRecentHistory(messages []domain.SessionMessage) []*schema.Message {
	out := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case domain.RoleAssistant:
			out = append(out, schema.AssistantMessage(message.Content, nil))
		default:
			out = append(out, schema.UserMessage(message.Content))
		}
	}
	return out
}
