package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/product/config"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
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
		panic(fmt.Errorf("unmarshal product db config failed: %w", err))
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
		panic(fmt.Errorf("build product db dsn failed: %w", err))
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("open product db failed: %w", err))
	}
	if err := mysqlconfig.ApplyPool(db, mysqlconfig.PoolConfig{}); err != nil {
		panic(fmt.Errorf("configure product db pool failed: %w", err))
	}

	if err := dao.InitTables(db); err != nil {
		panic(fmt.Errorf("init product tables failed: %w", err))
	}
	return db
}
