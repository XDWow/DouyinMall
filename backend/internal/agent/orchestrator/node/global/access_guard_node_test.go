package global

import (
	"context"
	"testing"
	"time"
)

func TestAccessGuardNodeInvokeValidatesInput(t *testing.T) {
	node := NewAccessGuardNode("tenant_1", 10, nil)

	if _, err := node.Invoke(context.Background(), AccessGuardInput{UserID: 1}); err == nil {
		t.Fatal("expected message validation error")
	}
	if _, err := node.Invoke(context.Background(), AccessGuardInput{Message: "hello"}); err == nil {
		t.Fatal("expected user_id validation error")
	}
}

func TestAccessGuardNodeInvokeRateLimit(t *testing.T) {
	node := NewAccessGuardNode("tenant_1", 10, stubRateLimiter{allowed: false})

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

func TestAccessGuardNodeInvokeResumeTokenSetsResumeFromCP(t *testing.T) {
	node := NewAccessGuardNode("tenant_1", 10, nil)

	result, err := node.Invoke(context.Background(), AccessGuardInput{
		Message:     "hello",
		UserID:      1,
		ResumeToken: "cp_1",
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !result.ResumeFromCP {
		t.Fatal("expected ResumeFromCP = true when resume_token is non-empty")
	}
}

type stubRateLimiter struct {
	allowed bool
	err     error
}

func (s stubRateLimiter) AllowUser(context.Context, int64, int64, time.Duration) (bool, error) {
	return s.allowed, s.err
}
