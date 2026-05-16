package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/payment/config"
	"github.com/XDWow/DouyinMall/backend/internal/payment/infra/db"
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
		Database: "mysql",
	}
	if err := viper.UnmarshalKey("db", &c); err != nil {
		panic(fmt.Errorf("payment db config read failed: %w", err))
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
		panic(fmt.Errorf("payment db config invalid: %w", err))
	}

	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("payment db connect failed: %w", err))
	}
	if err := mysqlconfig.ApplyPool(database, mysqlconfig.PoolConfig{}); err != nil {
		panic(fmt.Errorf("payment db configure pool failed: %w", err))
	}

	if err = db.InitTables(database); err != nil {
		panic(fmt.Errorf("payment db init tables failed: %w", err))
	}
	return database
}
