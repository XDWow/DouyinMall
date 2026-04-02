package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/aftersale/domain"
)

type stubRepository struct {
	items []*domain.Request
}

func (r *stubRepository) Create(_ context.Context, request *domain.Request) error {
	r.items = append(r.items, request)
	return nil
}

func (r *stubRepository) FindOpenByUserOrder(_ context.Context, userID, orderID int64, requestType domain.RequestType) (*domain.Request, error) {
	for _, item := range r.items {
		if item.UserID == userID && item.OrderID == orderID && item.RequestType == requestType && item.Status == domain.StatusPendingReview {
			return item, nil
		}
	}
	return nil, nil
}

func (r *stubRepository) GetByRequestNo(_ context.Context, requestNo string) (*domain.Request, error) {
	for _, item := range r.items {
		if item.RequestNo == requestNo {
			return item, nil
		}
	}
	return nil, nil
}

func TestCreateRequestIsIdempotentForOpenOrder(t *testing.T) {
	repo := &stubRepository{}
	uc := NewCreateAfterSaleRequestUseCase(repo)
	uc.now = func() time.Time { return time.Unix(1710000000, 0) }

	first, err := uc.Execute(context.Background(), CreateAfterSaleRequestCmd{
		UserID:      1001,
		OrderID:     2002,
		RequestType: "return",
		Reason:      "item damaged",
	})
	if err != nil {
		t.Fatalf("create first request failed: %v", err)
	}

	second, err := uc.Execute(context.Background(), CreateAfterSaleRequestCmd{
		UserID:      1001,
		OrderID:     2002,
		RequestType: "return",
		Reason:      "item damaged",
	})
	if err != nil {
		t.Fatalf("create second request failed: %v", err)
	}

	if len(repo.items) != 1 {
		t.Fatalf("expected one persisted request, got %d", len(repo.items))
	}
	if first.RequestNo != second.RequestNo {
		t.Fatalf("expected idempotent request number, got %s and %s", first.RequestNo, second.RequestNo)
	}
}
