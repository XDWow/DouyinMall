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

// Create 鍒涘缓鎿嶄綔璁板綍锛堝敮涓€绱㈠紩淇濊瘉骞傜瓑锛?
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
		// 濡傛灉杩濆弽鍞竴绱㈠紩绾︽潫锛岃鏄庡凡缁忓瓨鍦紙骞傜瓑锛?
		// MySQL: "Error 1062: Duplicate entry"
		// 杩欓噷绠€鍗曡繑鍥為敊璇紝涓婂眰澶勭悊骞傜瓑閫昏緫
		return err
	}

	return nil
}

// GetByOperationID 鏍规嵁骞傜瓑閿煡璇㈡搷浣滆褰?
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


