package dao

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CartDAO interface {
	FindCartByUserID(ctx context.Context, userID int64) ([]CartItem, error)
	FindCartItemByUserIDAndProductID(ctx context.Context, userID, productID int64) (CartItem, error)
	UpsertIncrement(ctx context.Context, userID, productID int64) error
	UpsertIncrementBatch(ctx context.Context, userID int64, productIDs []int64) error // 批量 INSERT ON CONFLICT +1
	Upsert(ctx context.Context, item CartItem) error
	Delete(ctx context.Context, userID, productID int64) error
	DeleteByProductIDs(ctx context.Context, userID int64, productIDs []int64) error // 批量 DELETE WHERE product_id IN
	DeleteByUserID(ctx context.Context, userID int64) error
	IncrementQuantity(ctx context.Context, userID, productID int64) error
	DecrementQuantity(ctx context.Context, userID, productID int64) error
}

type GORMCartDAO struct {
	db *gorm.DB
}

func NewGORMCartDAO(db *gorm.DB) CartDAO {
	return &GORMCartDAO{db: db}
}

type CartItem struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64 `gorm:"not null;uniqueIndex:uidx_user_product"`
	ProductID int64 `gorm:"not null;uniqueIndex:uidx_user_product"`
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

func (d *GORMCartDAO) FindCartItemByUserIDAndProductID(ctx context.Context, userID, productID int64) (CartItem, error) {
	var item CartItem
	err := d.db.WithContext(ctx).
		Where("user_id = ? AND product_id = ?", userID, productID).
		First(&item).Error
	return item, err
}

// 插入一项，如果存在，数量+1
func (d *GORMCartDAO) UpsertIncrement(ctx context.Context, userID, productID int64) error {
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "product_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"quantity":   gorm.Expr("quantity + ?", 1), // 用gorm表达式原子性+1
			"updated_at": time.Now(),
		}),
	}).Create(&CartItem{
		UserID:    userID,
		ProductID: productID,
		Quantity:  1,
	}).Error
}

// UpsertIncrementBatch 批量插入，已存在则数量+1，一条 SQL 完成
func (d *GORMCartDAO) UpsertIncrementBatch(ctx context.Context, userID int64, productIDs []int64) error {
	if len(productIDs) == 0 {
		return nil
	}
	items := make([]CartItem, len(productIDs))
	for i, pid := range productIDs {
		items[i] = CartItem{UserID: userID, ProductID: pid, Quantity: 1}
	}
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "product_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"quantity":   gorm.Expr("quantity + ?", 1),
			"updated_at": time.Now(),
		}),
	}).Create(&items).Error
}

func (d *GORMCartDAO) Upsert(ctx context.Context, item CartItem) error {
	if item.Quantity <= 0 {
		return gorm.ErrInvalidData
	}
	return d.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "product_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"quantity", "updated_at"}),
	}).Create(&item).Error
}

func (d *GORMCartDAO) Delete(ctx context.Context, userID, productID int64) error {
	return d.db.WithContext(ctx).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Delete(&CartItem{}).Error
}

// DeleteByProductIDs 批量删除，用 IN 子句一条 SQL 搞定
func (d *GORMCartDAO) DeleteByProductIDs(ctx context.Context, userID int64, productIDs []int64) error {
	if len(productIDs) == 0 {
		return nil
	}
	return d.db.WithContext(ctx).
		Where("user_id = ? AND product_id IN ?", userID, productIDs).
		Delete(&CartItem{}).Error
}

func (d *GORMCartDAO) DeleteByUserID(ctx context.Context, userID int64) error {
	return d.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&CartItem{}).Error
}

func (d *GORMCartDAO) IncrementQuantity(ctx context.Context, userID, productID int64) error {
	err := d.db.WithContext(ctx).
		Model(&CartItem{}).
		Where("user_id = ? AND product_id = ?", userID, productID).
		UpdateColumn("quantity", gorm.Expr("quantity + ?", 1)).Error
	return err
}

func (d *GORMCartDAO) DecrementQuantity(ctx context.Context, userID, productID int64) error {
	result := d.db.WithContext(ctx).
		Model(&CartItem{}).
		Where("user_id = ? AND product_id = ? AND quantity > 1", userID, productID).
		UpdateColumn("quantity", gorm.Expr("quantity - ?", 1))

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
