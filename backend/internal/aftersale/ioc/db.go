package ioc

import (
	aftersaleconfig "github.com/XDWow/DouyinMall/backend/internal/aftersale/config"
	"github.com/XDWow/DouyinMall/backend/pkg/mysqlconfig"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB(cfg aftersaleconfig.Config) (*gorm.DB, error) {
	dsn, err := mysqlconfig.BuildDSN(mysqlconfig.Config{
		Host:     cfg.DB.Host,
		Port:     cfg.DB.Port,
		User:     cfg.DB.User,
		Password: cfg.DB.Password,
		Database: cfg.DB.Database,
		Params:   cfg.DB.Params,
	})
	if err != nil {
		return nil, err
	}
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}
