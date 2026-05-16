package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/inventory/config"
	"github.com/XDWow/DouyinMall/backend/internal/inventory/infra/db"
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
	err := viper.UnmarshalKey("db", &c)
	if err != nil {
		panic(fmt.Errorf("load inventory db config: %w", err))
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
		panic(fmt.Errorf("build inventory db dsn: %w", err))
	}

	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("open inventory db connection: %w", err))
	}
	if err := mysqlconfig.ApplyPool(database, mysqlconfig.PoolConfig{}); err != nil {
		panic(fmt.Errorf("configure inventory db pool: %w", err))
	}

	if err = db.InitTables(database); err != nil {
		panic(fmt.Errorf("migrate inventory tables: %w", err))
	}
	return database
}
