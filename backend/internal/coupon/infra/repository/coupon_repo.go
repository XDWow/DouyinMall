package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"gorm.io/gorm"
)

type couponRepository struct {
	db *gorm.DB
	l  logger.LoggerV1
}

func NewCouponRepository(db *gorm.DB, l logger.LoggerV1) domain.CouponRepository {
	return &couponRepository{
		db: db,
		l:  l,
	}
}

func (repo *couponRepository) Issue(ctx context.Context, coupon *domain.Coupon) (int64, error) {
	model := domainToEntity(coupon)
	err := repo.db.WithContext(ctx).Create(model).Error
	if err != nil {
		return 0, err
	}
	return model.ID, nil
}

func (repo *couponRepository) ListByUserID(ctx context.Context, userID int64, status domain.CouponStatus, page, pageSize int) ([]*domain.Coupon, int32, error) {
	var models []db.Coupon
	err := repo.db.WithContext(ctx).
		Model(&db.Coupon{}).
		Preload("Template"). // 鍏宠仈鏌ヨ浼樻儬鍒告ā鏉?
		Where("user_id = ? AND status = ?", userID, status.AsUint8()).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&models).Error
	if err != nil {
		return nil, 0, err
	}

	// 缁熻鎬绘暟
	var total int64
	err = repo.db.WithContext(ctx).
		Model(&db.Coupon{}).
		Where("user_id = ? AND status = ?", userID, status.AsUint8()).
		Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	coupons := make([]*domain.Coupon, 0, len(models))
	for i := range models {
		coupons = append(coupons, entityToDomain(&models[i]))
	}

	return coupons, int32(total), nil
}

func (repo *couponRepository) ListAvailableByUserID(ctx context.Context, userID int64) ([]*domain.Coupon, error) {
	var models []db.Coupon
	now := time.Now()

	err := repo.db.WithContext(ctx).
		Preload("Template").
		Where("user_id = ? AND status = ? AND valid_from <= ? AND valid_to > ?", // 杩欓噷涔熸煡鏃堕棿锛屼弗璋紝閬垮厤瀹氭椂浠诲姟鎵ц绌洪殭
			userID,
			domain.UserCouponStatusUnused.AsUint8(),
			now,
			now,
		).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	coupons := make([]*domain.Coupon, 0, len(models))
	for i := range models {
		coupons = append(coupons, entityToDomain(&models[i]))
	}

	return coupons, nil
}

func (repo *couponRepository) GetAvailableByIDs(ctx context.Context, userID int64, couponIDs []int64) ([]*domain.Coupon, error) {
	if len(couponIDs) == 0 {
		return []*domain.Coupon{}, nil
	}

	var models []db.Coupon
	now := time.Now()

	err := repo.db.WithContext(ctx).
		Preload("Template").
		Where("id IN ? AND user_id = ? AND status = ? AND valid_from <= ? AND valid_to > ?",
			couponIDs,
			userID,
			domain.UserCouponStatusUnused.AsUint8(),
			now,
			now,
		).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	coupons := make([]*domain.Coupon, 0, len(models))
	for i := range models {
		coupons = append(coupons, entityToDomain(&models[i]))
	}

	return coupons, nil
}

func (repo *couponRepository) CountByUserAndTemplate(ctx context.Context, userID, templateID int64) (int32, error) {
	var count int64
	err := repo.db.WithContext(ctx).Model(&db.Coupon{}).
		Where("user_id = ? AND template_id = ?", userID, templateID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

// 鎵归噺棰勫崰锛氬皢鐘舵€佹敼涓?Locked锛屽苟璁剧疆 order_id
func (repo *couponRepository) BatchReserve(ctx context.Context, couponIDs []int64, orderID int64) error {
	if len(couponIDs) == 0 {
		return nil
	}

	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&db.Coupon{}).
			Where("id IN ? AND status = ?", couponIDs, domain.UserCouponStatusUnused.AsUint8()).
			Updates(map[string]interface{}{
				"status":   domain.UserCouponStatusLocked.AsUint8(),
				"order_id": orderID,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(couponIDs)) {
			return domain.ErrCouponNotAvailable
		}
		return nil
	})
}

func (repo *couponRepository) UpdateStatusByOrderID(ctx context.Context, orderID int64, fromStatus, toStatus domain.CouponStatus) error {
	updates := map[string]interface{}{
		"status": toStatus.AsUint8(),
	}
	switch toStatus {
	case domain.UserCouponStatusUsed:
		updates["used_at"] = time.Now()
	case domain.UserCouponStatusUnused:
		updates["order_id"] = nil
		updates["used_at"] = nil
	}

	return repo.db.WithContext(ctx).
		Model(&db.Coupon{}).
		Where("order_id = ? AND status = ?", orderID, fromStatus.AsUint8()).
		Updates(updates).Error
}

func domainToEntity(coupon *domain.Coupon) *db.Coupon {
	var orderID *int64
	if coupon.OrderID != 0 {
		orderID = &coupon.OrderID
	}

	var usedAt *time.Time
	if !coupon.UsedAt.IsZero() {
		usedAt = &coupon.UsedAt
	}

	return &db.Coupon{
		ID:         coupon.ID,
		UserID:     coupon.UserID,
		OrderID:    orderID,
		TemplateID: coupon.TemplateID,
		Status:     coupon.Status.AsUint8(),
		ValidFrom:  coupon.ValidStartTime,
		ValidTo:    coupon.ValidEndTime,
		UsedAt:     usedAt,
		CreatedAt:  coupon.CreatedAt,
	}
}

func entityToDomain(model *db.Coupon) *domain.Coupon {
	coupon := &domain.Coupon{
		ID:             model.ID,
		UserID:         model.UserID,
		TemplateID:     model.TemplateID,
		Status:         domain.CouponStatus(model.Status),
		ValidStartTime: model.ValidFrom,
		ValidEndTime:   model.ValidTo,
		CreatedAt:      model.CreatedAt,
	}

	// 澶勭悊鍙负绌虹殑瀛楁
	if model.OrderID != nil {
		coupon.OrderID = *model.OrderID
	}

	if model.UsedAt != nil {
		coupon.UsedAt = *model.UsedAt
	}

	// 杞崲鍏宠仈鐨勬ā鏉?
	if model.Template.ID != 0 {
		coupon.Template = templateEntityToDomain(&model.Template)
	}

	return coupon
}

func templateEntityToDomain(model *db.CouponTemplate) *domain.CouponTemplate {
	template := &domain.CouponTemplate{
		ID:            model.ID,
		Name:          model.Name,
		Type:          domain.CouponType(model.CouponType),
		DiscountValue: int64(model.DiscountValue),
		TotalCount:    model.TotalCount,
		IssuedCount:   model.IssuedCount,
		PerUserLimit:  model.PerUserLimit,
		Enabled:       model.Status == 1,
	}

	// 澶勭悊鍙负绌虹殑瀛楁
	if model.MinOrderAmount != nil {
		template.Threshold = int64(*model.MinOrderAmount)
	}

	if model.MaxDiscountAmount != nil {
		template.MaxDiscount = int64(*model.MaxDiscountAmount)
	}

	// 澶勭悊鏈夋晥鏈?
	if model.ValidDays != nil && *model.ValidDays > 0 {
		template.ValidDays = int64(*model.ValidDays)
	} else {
		if model.ValidStartTime != nil {
			template.ValidStartTime = model.ValidStartTime.Unix()
		}
		if model.ValidEndTime != nil {
			template.ValidEndTime = model.ValidEndTime.Unix()
		}
	}

	// 瑙ｆ瀽閫傜敤鑼冨洿锛堟牴鎹紭鍏堢骇鍒ゆ柇 Scope锛?
	// 1. 鍟嗗鍒?
	if model.MerchantID != nil && *model.MerchantID > 0 {
		template.Scope = domain.CouponScopeMerchant
		template.MerchantIDs = []int64{*model.MerchantID}
		return template
	}

	// 2. 鍟嗗搧鍒?
	var productIDs []int64
	if model.ApplicableProductIDs != "" && model.ApplicableProductIDs != "null" {
		_ = json.Unmarshal([]byte(model.ApplicableProductIDs), &productIDs)
	}
	if len(productIDs) > 0 {
		template.Scope = domain.CouponScopeProduct
		template.ProductIDs = productIDs
		return template
	}

	// 3. 鍝佺被鍒?
	var categoryIDs []int64
	if model.ApplicableCategoryIDs != "" && model.ApplicableCategoryIDs != "null" {
		_ = json.Unmarshal([]byte(model.ApplicableCategoryIDs), &categoryIDs)
	}
	if len(categoryIDs) > 0 {
		template.Scope = domain.CouponScopeCategory
		template.CategoryIDs = categoryIDs
		return template
	}

	// 4. 鍏ㄥ満鍒革紙榛樿锛?
	template.Scope = domain.CouponScopeAll
	return template
}

func (repo *couponRepository) MarkExpiredCoupons(ctx context.Context) (int64, error) {
	result := repo.db.WithContext(ctx).
		Model(&db.Coupon{}).
		Where("status = ? AND valid_to < NOW()", domain.UserCouponStatusUnused.AsUint8()).
		Update("status", domain.UserCouponStatusExpired.AsUint8())

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}
