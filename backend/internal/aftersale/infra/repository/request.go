package repository

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/aftersale/domain"
	aftersaledb "github.com/XDWow/DouyinMall/backend/internal/aftersale/infra/db"
	"gorm.io/gorm"
)

type RequestRepository struct {
	db *gorm.DB
}

func NewRequestRepository(db *gorm.DB) *RequestRepository {
	return &RequestRepository{db: db}
}

func (r *RequestRepository) Create(ctx context.Context, request *domain.Request) error {
	item := toDBModel(request)
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return err
	}
	applyDomainModel(item, request)
	return nil
}

func (r *RequestRepository) FindOpenByUserOrder(ctx context.Context, userID, orderID int64, requestType domain.RequestType) (*domain.Request, error) {
	var item aftersaledb.AfterSaleRequest
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND order_id = ? AND request_type = ? AND status = ?", userID, orderID, string(requestType), string(domain.StatusPendingReview)).
		Order("id DESC").
		First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return toDomainModel(&item), nil
}

func (r *RequestRepository) GetByRequestNo(ctx context.Context, requestNo string) (*domain.Request, error) {
	var item aftersaledb.AfterSaleRequest
	err := r.db.WithContext(ctx).Where("request_no = ?", requestNo).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return toDomainModel(&item), nil
}

func toDBModel(request *domain.Request) *aftersaledb.AfterSaleRequest {
	if request == nil {
		return nil
	}
	return &aftersaledb.AfterSaleRequest{
		ID:          request.ID,
		RequestNo:   request.RequestNo,
		UserID:      request.UserID,
		OrderID:     request.OrderID,
		ItemID:      request.ItemID,
		RequestType: string(request.RequestType),
		Reason:      request.Reason,
		Status:      string(request.Status),
		SessionID:   request.SessionID,
		TraceID:     request.TraceID,
		Metadata:    request.Metadata,
		CreatedAt:   request.CreatedAt,
		UpdatedAt:   request.UpdatedAt,
	}
}

func toDomainModel(item *aftersaledb.AfterSaleRequest) *domain.Request {
	if item == nil {
		return nil
	}
	return &domain.Request{
		ID:          item.ID,
		RequestNo:   item.RequestNo,
		UserID:      item.UserID,
		OrderID:     item.OrderID,
		ItemID:      item.ItemID,
		RequestType: domain.RequestType(item.RequestType),
		Reason:      item.Reason,
		Status:      domain.RequestStatus(item.Status),
		SessionID:   item.SessionID,
		TraceID:     item.TraceID,
		Metadata:    item.Metadata,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func applyDomainModel(item *aftersaledb.AfterSaleRequest, request *domain.Request) {
	if item == nil || request == nil {
		return
	}
	request.ID = item.ID
	request.CreatedAt = item.CreatedAt
	request.UpdatedAt = item.UpdatedAt
}


