package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/config"
	db2 "github.com/XDWow/DouyinMall/backend/internal/seckill/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/mysqlconfig"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	c := config.DBConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Database: "douyin_mall",
	}
	if err := viper.UnmarshalKey("db", &c); err != nil {
		panic(fmt.Errorf("数据库初始化读取配置失败: %w", err))
	}
	c.Password = viper.GetString("db.password")

	dsn, err := mysqlconfig.BuildDSN(mysqlconfig.Config{
		Host:     c.Host,
		Port:     c.Port,
		User:     c.User,
		Password: c.Password,
		Database: c.Database,
		Params:   c.Params,
	})
	if err != nil {
		panic(fmt.Errorf("数据库初始化配置失败: %w", err))
	}

	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("数据库初始化连接失败: %w", err))
	}
	if err = db2.InitTables(database); err != nil {
		panic(fmt.Errorf("秒杀表初始化失败: %w", err))
	}
	return database
}
