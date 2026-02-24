package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/config"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/db"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	// 默认配置兜底
	c := config.DBConfig{
		DSN: "root:123456@tcp(localhost:13306)/coupon?charset=utf8mb4&parseTime=True&loc=Local",
	}
	err := viper.UnmarshalKey("db", &c)
	if err != nil {
		panic(fmt.Errorf("数据库初始化读取配置失败: %w", err))
	}

	database, err := gorm.Open(mysql.Open(c.DSN), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("数据库初始化连接失败: %w", err))
	}

	// 初始化表结构
	err = db.InitTables(database)
	if err != nil {
		panic(fmt.Errorf("数据库初始化表失败: %w", err))
	}

	return database
}
