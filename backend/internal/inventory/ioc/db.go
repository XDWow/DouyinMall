package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/config"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/db"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	// 默认配置兜底
	c := config.DBConfig{
		DSN: "root:root@tcp(localhost:3306)/douyin_mall",
	}
	err := viper.UnmarshalKey("db", &c)
	if err != nil {
		panic(fmt.Errorf("数据库初始化读取配置失败: %w", err))
	}

	database, err := gorm.Open(mysql.Open(c.DSN), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("数据库初始化连接失败: %w", err))
	}

	// 初始化 db 的表
	err = db.InitTables(database)
	if err != nil {
		panic(fmt.Errorf("数据库初始化表失败: %w", err))
	}
	return database
}
