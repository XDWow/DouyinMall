package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/cart/config"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	// 默认配置兜底
	c := config.DBConfig{
		DSN: "root:root@tcp(localhost:3306)/mysql",
	}
	err := viper.UnmarshalKey("db", &c)
	if err != nil {
		panic(fmt.Errorf("数据库初始化读取配置失败: %w", err))
	}

	db, err := gorm.Open(mysql.Open(c.DSN), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("数据库初始化连接失败: %w", err))
	}

	// 初始化 db 的表
	err = dao.InitTables(db)
	if err != nil {
		panic(fmt.Errorf("数据库初始化表失败: %w", err))
	}
	return db
}
