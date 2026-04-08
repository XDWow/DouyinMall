package global

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAccessGuardNodeInvokeValidatesInput(t *testing.T) {
	node := NewAccessGuardNode("tenant_1", 10, nil, nil)

	if _, err := node.Invoke(context.Background(), AccessGuardInput{UserID: 1}); err == nil {
		t.Fatal("expected message validation error")
	}
	if _, err := node.Invoke(context.Background(), AccessGuardInput{Message: "hello"}); err == nil {
		t.Fatal("expected user_id validation error")
	}
}

func TestAccessGuardNodeInvokeRateLimit(t *testing.T) {
	node := NewAccessGuardNode("tenant_1", 10, stubRateLimiter{allowed: false}, nil)

	result, err := node.Invoke(context.Background(), AccessGuardInput{
		Message: "hello",
		UserID:  1,
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.ErrorCode != "rate_limit" {
		t.Fatalf("ErrorCode = %q, want rate_limit", result.ErrorCode)
	}
	if result.FinalAnswer == "" {
		t.Fatal("expected rate limit reply")
	}
}

func TestAccessGuardNodeInvokeResumeCheckpoint(t *testing.T) {
	t.Run("checkpoint exists", func(t *testing.T) {
		node := NewAccessGuardNode("tenant_1", 10, nil, stubCheckpointStore{ok: true})

		result, err := node.Invoke(context.Background(), AccessGuardInput{
			Message:     "hello",
			UserID:      1,
			ResumeToken: "cp_1",
		})
		if err != nil {
			t.Fatalf("Invoke() error = %v", err)
		}
		if !result.ResumeFromCP {
			t.Fatal("expected ResumeFromCP = true")
		}
	})

	t.Run("checkpoint missing", func(t *testing.T) {
		node := NewAccessGuardNode("tenant_1", 10, nil, stubCheckpointStore{ok: false})

		_, err := node.Invoke(context.Background(), AccessGuardInput{
			Message:     "hello",
			UserID:      1,
			ResumeToken: "cp_1",
		})
		if err == nil {
			t.Fatal("expected checkpoint not found error")
		}
	})

	t.Run("checkpoint store error", func(t *testing.T) {
		node := NewAccessGuardNode("tenant_1", 10, nil, stubCheckpointStore{err: errors.New("boom")})

		_, err := node.Invoke(context.Background(), AccessGuardInput{
			Message:     "hello",
			UserID:      1,
			ResumeToken: "cp_1",
		})
		if err == nil {
			t.Fatal("expected checkpoint store error")
		}
	})
}

type stubRateLimiter struct {
	allowed bool
	err     error
}

func (s stubRateLimiter) AllowUser(context.Context, int64, int64, time.Duration) (bool, error) {
	return s.allowed, s.err
}

type stubCheckpointStore struct {
	ok  bool
	err error
}

func (s stubCheckpointStore) Get(context.Context, string) ([]byte, bool, error) {
	return nil, s.ok, s.err
}

func (s stubCheckpointStore) Set(context.Context, string, []byte) error {
	return nil
}
