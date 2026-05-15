package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/user/config"
	"github.com/XDWow/DouyinMall/backend/internal/user/repo/dao"
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
	err := viper.UnmarshalKey("db", &c)
	if err != nil {
		panic(fmt.Errorf("load user db config: %w", err))
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
		panic(fmt.Errorf("build user db dsn: %w", err))
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("open user db connection: %w", err))
	}

	if err = dao.InitTables(db); err != nil {
		panic(fmt.Errorf("migrate user tables: %w", err))
	}
	return db
}
