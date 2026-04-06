package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// SessionLoadInput 描述会话装载阶段的输入。
type SessionLoadInput struct {
	Request     domain.ChatCommand
	TraceID     string
	SessionMeta *domain.Session
}

// SessionLoadNode 负责补齐会话标识，并整理请求携带的槽位。
type SessionLoadNode struct{}

func NewSessionLoadNode() *SessionLoadNode { return &SessionLoadNode{} }

type SessionLoadResult struct {
	SessionID   string
	SessionMeta *domain.Session
	OrderID     string
	ProductID   string
}

// Invoke 生成会话装载阶段的结果。
func (n *SessionLoadNode) Invoke(_ context.Context, input SessionLoadInput) (*SessionLoadResult, error) {
	sessionID := strings.TrimSpace(input.Request.SessionID)
	if sessionID == "" {
		sessionID = "sess_" + input.TraceID
	}

	sessionMeta := input.SessionMeta
	if sessionMeta == nil {
		sessionMeta = &domain.Session{
			SessionID: sessionID,
			UserID:    input.Request.UserID,
			Status:    domain.SessionStatusActive,
		}
	}
	if sessionMeta.UserID != 0 && sessionMeta.UserID != input.Request.UserID {
		return nil, fmt.Errorf("会话归属用户不匹配")
	}

	return &SessionLoadResult{
		SessionID:   sessionMeta.SessionID,
		SessionMeta: sessionMeta,
		OrderID:     support.DigitsOnlyID(support.MetadataValue(input.Request.Metadata, "order_id", "orderID")),
		ProductID:   support.DigitsOnlyID(support.MetadataValue(input.Request.Metadata, "product_id", "productID")),
	}, nil
}
