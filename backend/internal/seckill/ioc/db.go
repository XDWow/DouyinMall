package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/config"
	db2 "github.com/XDWow/DouyinMall/backend/internal/seckill/infra/db"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	c := config.DBConfig{DSN: "root:root@tcp(localhost:3306)/douyin_mall"}
	if err := viper.UnmarshalKey("db", &c); err != nil {
		panic(fmt.Errorf("read db config failed: %w", err))
	}
	database, err := gorm.Open(mysql.Open(c.DSN), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("open db failed: %w", err))
	}
	if err = db2.InitTables(database); err != nil {
		panic(fmt.Errorf("init seckill tables failed: %w", err))
	}
	return database
}


