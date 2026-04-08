package global

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

func TestSessionLoadNodeInvokeLoadsRecentMessagesAndMetadataSlots(t *testing.T) {
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
		messages: []domain.Message{
			{SessionID: "sess_1", Role: domain.RoleUser, Content: "hello"},
			{SessionID: "sess_1", Role: domain.RoleAssistant, Content: "hi"},
		},
	})

	result, err := node.Invoke(context.Background(), SessionLoadInput{
		Request: domain.ChatCommand{
			SessionID: "sess_1",
			UserID:    123,
			Metadata: map[string]string{
				"order_id":   "order-10001",
				"product_id": "sku-20002",
			},
		},
		TraceID: "trace_1",
		ExistingSlots: map[string]any{
			"reason": "damaged",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.SessionID != "sess_1" {
		t.Fatalf("SessionID = %q", result.SessionID)
	}
	if len(result.RecentMessages) != 2 {
		t.Fatalf("RecentMessages len = %d, want 2", len(result.RecentMessages))
	}
	if result.Slots["order_id"] != "10001" {
		t.Fatalf("order_id = %v", result.Slots["order_id"])
	}
	if result.Slots["product_id"] != "20002" {
		t.Fatalf("product_id = %v", result.Slots["product_id"])
	}
	if result.Slots["sku_id"] != "30003" {
		t.Fatalf("sku_id = %v", result.Slots["sku_id"])
	}
	if result.Slots["reason"] != "damaged" {
		t.Fatalf("reason = %v", result.Slots["reason"])
	}
	if result.SessionMeta == nil || result.SessionMeta.Slots["product_id"] != "20002" {
		t.Fatalf("session meta slots = %+v", result.SessionMeta)
	}
}

func TestSessionLoadNodeInvokeCreatesSessionWhenMissing(t *testing.T) {
	mem := &stubSessionMemory{}
	node := NewSessionLoadNode(mem)

	result, err := node.Invoke(context.Background(), SessionLoadInput{
		Request: domain.ChatCommand{
			UserID: 456,
		},
		TraceID: "trace_new",
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.SessionID != "sess_trace_new" {
		t.Fatalf("SessionID = %q", result.SessionID)
	}
	if mem.created == nil || mem.created.SessionID != "sess_trace_new" {
		t.Fatalf("expected session to be auto created, got %+v", mem.created)
	}
}

type stubSessionMemory struct {
	session  *domain.Session
	messages []domain.Message
	created  *domain.Session
}

func (s *stubSessionMemory) LoadSession(context.Context, string) (*domain.Session, []domain.Message, error) {
	return s.session, s.messages, nil
}

func (s *stubSessionMemory) CreateSession(_ context.Context, session domain.Session) error {
	cloned := session
	s.created = &cloned
	return nil
}

func (s *stubSessionMemory) RecentSchemaMessages(messages []domain.Message) []*schema.Message {
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
