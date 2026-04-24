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
	// 榛樿閰嶇疆鍏滃簳
	c := config.DBConfig{
		Host:     "localhost",
		Port:     13306,
		User:     "root",
		Database: "coupon",
	}
	err := viper.UnmarshalKey("db", &c)
	if err != nil {
		panic(fmt.Errorf("鏁版嵁搴撳垵濮嬪寲璇诲彇閰嶇疆澶辫触: %w", err))
	}

	dsn, err := mysqlconfig.BuildDSN(mysqlconfig.Config{
		Host:     c.Host,
		Port:     c.Port,
		User:     c.User,
		Password: c.Password,
		Database: c.Database,
		Params:   c.Params,
	})
	if err != nil {
		panic(fmt.Errorf("鏁版嵁搴撳垵濮嬪寲閰嶇疆澶辫触: %w", err))
	}

	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("鏁版嵁搴撳垵濮嬪寲杩炴帴澶辫触: %w", err))
	}

	// 鍒濆鍖栬〃缁撴瀯
	err = db.InitTables(database)
	if err != nil {
		panic(fmt.Errorf("鏁版嵁搴撳垵濮嬪寲琛ㄥけ璐? %w", err))
	}

	return database
}
