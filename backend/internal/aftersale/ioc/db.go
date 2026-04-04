package ioc

import (
	aftersaleconfig "github.com/XDWow/DouyinMall/backend/internal/aftersale/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB(cfg aftersaleconfig.Config) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(cfg.DB.DSN), &gorm.Config{})
}


