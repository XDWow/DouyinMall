package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CartDAO interface {
	FindCartByUserID(ctx context.Context, userID int64) ([]CartItem, error)
	FindCartItemByUserIDAndSKUID(ctx context.Context, userID, skuID int64) (CartItem, error)
	UpsertIncrement(ctx context.Context, item CartItem) error
	UpsertIncrementBatch(ctx context.Context, userID int64, items []CartItem) error
	Upsert(ctx context.Context, item CartItem) error
	Delete(ctx context.Context, userID, skuID int64) error
	DeleteBySKUIDs(ctx context.Context, userID int64, skuIDs []int64) error
	DeleteByUserID(ctx context.Context, userID int64) error
	IncrementQuantity(ctx context.Context, userID, skuID int64) error
	DecrementQuantity(ctx context.Context, userID, skuID int64) error
}

type GORMCartDAO struct {
	db *gorm.DB
}

func NewGORMCartDAO(db *gorm.DB) CartDAO {
	return &GORMCartDAO{db: db}
}

type CartItem struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64 `gorm:"not null;uniqueIndex:uidx_user_sku"`
	ProductID int64 `gorm:"not null"`
	SKUID     int64 `gorm:"column:sku_id;not null;uniqueIndex:uidx_user_sku"`
	Quantity  int64 `gorm:"not null;default:1"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CartItem) TableName() string {
	return "cart_items"
}

func (d *GORMCartDAO) FindCartByUserID(ctx context.Context, userID int64) ([]CartItem, error) {
	var items []CartItem
	err := d.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&items).Error
	return items, err
}

func (d *GORMCartDAO) FindCartItemByUserIDAndSKUID(ctx context.Context, userID, skuID int64) (CartItem, error) {
	var item CartItem
	err := d.db.WithContext(ctx).
		Where("user_id = ? AND sku_id = ?", userID, skuID).
		First(&item).Error
	return item, err
}

func (d *GORMCartDAO) UpsertIncrement(ctx context.Context, item CartItem) error {
	if item.Quantity <= 0 {
		item.Quantity = 1
	}
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "sku_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"product_id": item.ProductID,
			"quantity":   gorm.Expr("quantity + ?", item.Quantity),
			"updated_at": time.Now(),
		}),
	}).Create(&item).Error
}

func (d *GORMCartDAO) UpsertIncrementBatch(ctx context.Context, userID int64, items []CartItem) error {
	if len(items) == 0 {
		return nil
	}
	daoItems := make([]CartItem, 0, len(items))
	for _, item := range items {
		if item.Quantity <= 0 {
			item.Quantity = 1
		}
		item.UserID = userID
		daoItems = append(daoItems, item)
	}
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "sku_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"product_id": gorm.Expr("VALUES(product_id)"),
			"quantity":   gorm.Expr("quantity + VALUES(quantity)"),
			"updated_at": time.Now(),
		}),
	}).Create(&daoItems).Error
}

func (d *GORMCartDAO) Upsert(ctx context.Context, item CartItem) error {
	if item.Quantity <= 0 {
		return gorm.ErrInvalidData
	}
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "sku_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"product_id", "quantity", "updated_at"}),
	}).Create(&item).Error
}

func (d *GORMCartDAO) Delete(ctx context.Context, userID, skuID int64) error {
	return d.db.WithContext(ctx).
		Where("user_id = ? AND sku_id = ?", userID, skuID).
		Delete(&CartItem{}).Error
}

func (d *GORMCartDAO) DeleteBySKUIDs(ctx context.Context, userID int64, skuIDs []int64) error {
	if len(skuIDs) == 0 {
		return nil
	}
	return d.db.WithContext(ctx).
		Where("user_id = ? AND sku_id IN ?", userID, skuIDs).
		Delete(&CartItem{}).Error
}

func (d *GORMCartDAO) DeleteByUserID(ctx context.Context, userID int64) error {
	return d.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&CartItem{}).Error
}

func (d *GORMCartDAO) IncrementQuantity(ctx context.Context, userID, skuID int64) error {
	return d.db.WithContext(ctx).
		Model(&CartItem{}).
		Where("user_id = ? AND sku_id = ?", userID, skuID).
		UpdateColumn("quantity", gorm.Expr("quantity + ?", 1)).Error
}

func (d *GORMCartDAO) DecrementQuantity(ctx context.Context, userID, skuID int64) error {
	result := d.db.WithContext(ctx).
		Model(&CartItem{}).
		Where("user_id = ? AND sku_id = ? AND quantity > 1", userID, skuID).
		UpdateColumn("quantity", gorm.Expr("quantity - ?", 1))

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
