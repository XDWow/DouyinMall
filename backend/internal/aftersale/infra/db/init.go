package db

import "gorm.io/gorm"

func InitTables(gdb *gorm.DB) error {
	return gdb.AutoMigrate(&AfterSaleRequest{})
}
