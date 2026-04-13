package db

import "gorm.io/gorm"

func InitTables(db *gorm.DB) error {
	return db.AutoMigrate(
		&SeckillActivity{},
		&SeckillRequest{},
		&SeckillOperation{},
		&SeckillSuccess{},
	)
}


