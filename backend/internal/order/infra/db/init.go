package db

import "gorm.io/gorm"

func InitTables(db *gorm.DB) error {
	return db.AutoMigrate(
		&OrderModel{},
		&OrderItemModel{},
		&OutboxEventModel{},
	)
}


