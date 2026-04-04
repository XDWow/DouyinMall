package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/aftersale/domain"
	"github.com/google/uuid"
)

type CreateAfterSaleRequestUseCase struct {
	repo domain.RequestRepository
	now  func() time.Time
}

type CreateAfterSaleRequestCmd struct {
	UserID      int64
	OrderID     int64
	ItemID      int64
	RequestType string
	Reason      string
	SessionID   string
	TraceID     string
	Metadata    map[string]any
}

func NewCreateAfterSaleRequestUseCase(repo domain.RequestRepository) *CreateAfterSaleRequestUseCase {
	return &CreateAfterSaleRequestUseCase{
		repo: repo,
		now:  time.Now,
	}
}

func (uc *CreateAfterSaleRequestUseCase) Execute(ctx context.Context, cmd CreateAfterSaleRequestCmd) (*domain.Request, error) {
	if cmd.UserID <= 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	if cmd.OrderID <= 0 {
		return nil, fmt.Errorf("order_id is required")
	}
	reason := strings.TrimSpace(cmd.Reason)
	if reason == "" {
		return nil, fmt.Errorf("reason is required")
	}

	requestType := domain.NormalizeRequestType(cmd.RequestType)
	existing, err := uc.repo.FindOpenByUserOrder(ctx, cmd.UserID, cmd.OrderID, requestType)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	metadata := ""
	if len(cmd.Metadata) > 0 {
		if payload, marshalErr := json.Marshal(cmd.Metadata); marshalErr == nil {
			metadata = string(payload)
		}
	}

	now := uc.now()
	request := &domain.Request{
		RequestNo:   newRequestNo(),
		UserID:      cmd.UserID,
		OrderID:     cmd.OrderID,
		ItemID:      cmd.ItemID,
		RequestType: requestType,
		Reason:      reason,
		Status:      domain.StatusPendingReview,
		SessionID:   strings.TrimSpace(cmd.SessionID),
		TraceID:     strings.TrimSpace(cmd.TraceID),
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := uc.repo.Create(ctx, request); err != nil {
		return nil, err
	}
	return request, nil
}

func newRequestNo() string {
	return "AS-" + strings.ToUpper(uuid.NewString()[:8])
}


