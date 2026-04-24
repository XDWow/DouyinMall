//go:build legacy_agent

package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/agent/config"
	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/infra/db"
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
		Database: "douyinmall_agent",
	}
	_ = viper.UnmarshalKey("db", &c)

	dsn, err := mysqlconfig.BuildDSN(mysqlconfig.Config{
		Host:     c.Host,
		Port:     c.Port,
		User:     c.User,
		Password: c.Password,
		Database: c.Database,
		Params:   c.Params,
	})
	if err != nil {
		panic(fmt.Errorf("Agent DB config failed: %w", err))
	}

	gormDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("Agent DB 閺夆晝鍋炵敮瀛樺緞鏉堫偉袝: %w", err))
	}

	if err := db.InitTables(gormDB); err != nil {
		panic(fmt.Errorf("Agent 閻炴稏鍔岄崹鍨叏鐎ｎ亜顕у鎯扮簿鐟? %w", err))
	}
	return gormDB
}
