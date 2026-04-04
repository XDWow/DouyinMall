package repository

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"gorm.io/gorm"
)

type couponTemplateRepository struct {
	db *gorm.DB
	l  logger.LoggerV1
}

func NewCouponTemplateRepository(database *gorm.DB, l logger.LoggerV1) domain.CouponTemplateRepository {
	return &couponTemplateRepository{
		db: database,
		l:  l,
	}
}

func (repo *couponTemplateRepository) GetByID(ctx context.Context, id int64) (domain.CouponTemplate, error) {
	var model db.CouponTemplate
	err := repo.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.CouponTemplate{}, domain.ErrCouponTemplateNotFound
		}
		return domain.CouponTemplate{}, err
	}

	return *templateEntityToDomain(&model), nil
}

// IncrIssuedCount 鍘熷瓙澧炲姞宸插彂鏀炬暟閲忥紙浣跨敤鏁版嵁搴撳眰闈㈢殑鍘熷瓙鎿嶄綔锛?
func (repo *couponTemplateRepository) IncrIssuedCount(ctx context.Context, id int64) error {
	// 浣跨敤 UPDATE ... SET issued_count = issued_count + 1 瀹炵幇鍘熷瓙鎿嶄綔
	return repo.db.WithContext(ctx).
		Model(&db.CouponTemplate{}).
		Where("id = ?", id).
		UpdateColumn("issued_count", gorm.Expr("issued_count + ?", 1)).
		Error
}


