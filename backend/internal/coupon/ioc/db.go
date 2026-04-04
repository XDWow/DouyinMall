package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/config"
	"github.com/XDWow/DouyinMall/backend/internal/coupon/infra/db"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	// 榛樿閰嶇疆鍏滃簳
	c := config.DBConfig{
		DSN: "root:123456@tcp(localhost:13306)/coupon?charset=utf8mb4&parseTime=True&loc=Local",
	}
	err := viper.UnmarshalKey("db", &c)
	if err != nil {
		panic(fmt.Errorf("鏁版嵁搴撳垵濮嬪寲璇诲彇閰嶇疆澶辫触: %w", err))
	}

	database, err := gorm.Open(mysql.Open(c.DSN), &gorm.Config{})
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


