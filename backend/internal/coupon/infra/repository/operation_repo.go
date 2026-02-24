package repository

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"gorm.io/gorm"
)

type couponOperationRepository struct {
	db *gorm.DB
	l  logger.LoggerV1
}

func NewCouponOperationRepository(database *gorm.DB, l logger.LoggerV1) domain.CouponOperationRepository {
	return &couponOperationRepository{
		db: database,
		l:  l,
	}
}

// Create 创建操作记录（唯一索引保证幂等）
func (repo *couponOperationRepository) Create(ctx context.Context, op *domain.CouponOperation) error {
	model := &db.CouponOperation{
		OperationID:   op.OperationID,
		UserCouponID:  op.UserCouponID,
		OperationType: op.Type,
	}

	if op.OrderID != 0 {
		model.OrderID = &op.OrderID
	}

	err := repo.db.WithContext(ctx).Create(model).Error
	if err != nil {
		// 如果违反唯一索引约束，说明已经存在（幂等）
		// MySQL: "Error 1062: Duplicate entry"
		// 这里简单返回错误，上层处理幂等逻辑
		return err
	}

	return nil
}

// GetByOperationID 根据幂等键查询操作记录
func (repo *couponOperationRepository) GetByOperationID(ctx context.Context, operationID string) (*domain.CouponOperation, error) {
	var model db.CouponOperation
	err := repo.db.WithContext(ctx).
		Where("operation_id = ?", operationID).
		First(&model).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrOperationNotFound
		}
		return nil, err
	}

	op := &domain.CouponOperation{
		ID:           model.ID,
		OperationID:  model.OperationID,
		UserCouponID: model.UserCouponID,
		Type:         model.OperationType,
		CreatedAt:    model.CreatedAt.Unix(),
	}

	if model.OrderID != nil {
		op.OrderID = *model.OrderID
	}

	return op, nil
}
