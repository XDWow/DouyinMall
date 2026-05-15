package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/config"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/db"
	"github.com/XDWow/DouyinMall/backend/pkg/mysqlconfig"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	c := config.DBConfig{
		Host:     "localhost",
		Port:     13306,
		User:     "root",
		Database: "coupon",
	}
	err := viper.UnmarshalKey("db", &c)
	if err != nil {
		panic(fmt.Errorf("load coupon db config: %w", err))
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
		panic(fmt.Errorf("build coupon db dsn: %w", err))
	}

	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("open coupon db connection: %w", err))
	}

	if err = db.InitTables(database); err != nil {
		panic(fmt.Errorf("migrate coupon tables: %w", err))
	}

	return database
}
