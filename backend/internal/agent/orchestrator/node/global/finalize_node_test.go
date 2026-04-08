package global

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	agentsession "github.com/XDWow/DouyinMall/backend/internal/agent/session"
)

func TestFinalizeNodeAddsLowConfidenceNoticeAndStreamsReply(t *testing.T) {
	writer := &stubStreamWriter{}
	state := graphstate.NewState(domain.ChatCommand{
		SessionID: "sess_finalize",
		UserID:    1,
		Message:   "help",
	}, writer, graphstate.InitOptions{})
	state.Session.SessionID = "sess_finalize"
	state.Session.Intent = domain.IntentFallback
	state.Session.FinalAnswer = "这是当前查询结果。"
	state.Session.IntentConfidence = 0.1

	node := NewFinalizeNode(nil, nil, nil, nil, nil, nil, nil, nil, 0)
	if _, err := node.Invoke(context.Background(), state); err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if got := state.Response.Reply; got == "" || got == "这是当前查询结果。" {
		t.Fatalf("reply = %q, want low confidence notice", got)
	}
	if len(writer.events) != 1 {
		t.Fatalf("streamed events = %d, want 1", len(writer.events))
	}
	if writer.events[0].Event != "token" {
		t.Fatalf("event = %q, want token", writer.events[0].Event)
	}
	if !state.Answer.Streamed {
		t.Fatal("expected reply to be marked as streamed")
	}
}

func TestFinalizeNodeWritesSessionCacheBeforeReturning(t *testing.T) {
	repo := &stubSessionRepo{}
	service := agentsession.NewService(repo, 5)
	state := graphstate.NewState(domain.ChatCommand{
		SessionID: "sess_cache",
		UserID:    7,
		Message:   "where is my order",
	}, nil, graphstate.InitOptions{})
	state.Session.SessionID = "sess_cache"
	state.Session.Intent = domain.IntentOrderQuery
	state.Session.FinalAnswer = "订单正在配送中。"
	state.SessionMeta = &domain.Session{
		SessionID:  "sess_cache",
		UserID:     7,
		Status:     domain.SessionStatusActive,
		TotalTurns: 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	node := NewFinalizeNode(nil, nil, nil, nil, service, nil, nil, nil, 0)
	if _, err := node.Invoke(context.Background(), state); err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.cacheSession.SessionID != "sess_cache" {
		t.Fatalf("cache session id = %q", repo.cacheSession.SessionID)
	}
	if len(repo.cacheMessages) != 2 {
		t.Fatalf("cache messages = %d, want 2", len(repo.cacheMessages))
	}
	if repo.cacheMessages[0].Role != domain.RoleUser {
		t.Fatalf("first role = %s, want user", repo.cacheMessages[0].Role)
	}
	if repo.cacheMessages[1].Role != domain.RoleAssistant {
		t.Fatalf("second role = %s, want assistant", repo.cacheMessages[1].Role)
	}
}

type stubStreamWriter struct {
	events []graphstate.StreamEvent
}

func (s *stubStreamWriter) Send(_ context.Context, event graphstate.StreamEvent) error {
	s.events = append(s.events, event)
	return nil
}

type stubSessionRepo struct {
	mu            sync.Mutex
	cacheSession  domain.Session
	cacheMessages []domain.Message
}

func (s *stubSessionRepo) Load(context.Context, string) (*domain.Session, []domain.Message, error) {
	return nil, nil, nil
}

func (s *stubSessionRepo) Create(context.Context, domain.Session) error {
	return nil
}

func (s *stubSessionRepo) SaveRound(context.Context, domain.Session, domain.Message, domain.Message) error {
	return nil
}

func (s *stubSessionRepo) SaveMessages(context.Context, string, []domain.Message) error {
	return nil
}

func (s *stubSessionRepo) LoadAllMessages(context.Context, string) ([]domain.Message, error) {
	return nil, nil
}

func (s *stubSessionRepo) Clear(context.Context, string) error {
	return nil
}

func (s *stubSessionRepo) ListByUser(context.Context, int64, int, int) ([]domain.Session, int, error) {
	return nil, 0, nil
}

func (s *stubSessionRepo) SaveCacheSnapshot(_ context.Context, session domain.Session, messages []domain.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheSession = session
	s.cacheMessages = append([]domain.Message(nil), messages...)
	return nil
}

func (s *stubSessionRepo) SaveRoundPersistent(context.Context, domain.Session, domain.Message, domain.Message) error {
	return nil
}
