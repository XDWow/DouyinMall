package dao

import (
	"gorm.io/gorm"
	"time"
)

type CartDAO interface {
}

type GORMCartDAO struct {
	db *gorm.DB
}

type Cart struct {
	ID     int64 `gorm:"primaryKey;autoIncrement"`
	UserID int64 `gorm:"index;not null"`
	Total  int64

	Items []CartItem `gorm:"foreignKey:CartID;constraint:OnDelete:CASCADE"`
}

type CartItem struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	CartID    int64 `gorm:"not null;uniqueIndex:idx_cart_product"`
	ProductID int64 `gorm:"not null;uniqueIndex:idx_cart_product"`
	Quantity  int64 `gorm:"not null"`
	Price     int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
