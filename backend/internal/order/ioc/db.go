package ioc

import (
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/order/config"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/db"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	// 榛樿閰嶇疆鍏滃簳
	c := config.DBConfig{
		DSN: "root:root@tcp(localhost:3306)/mysql",
	}
	err := viper.UnmarshalKey("db", &c)
	if err != nil {
		panic(fmt.Errorf("鏁版嵁搴撳垵濮嬪寲璇诲彇閰嶇疆澶辫触: %w", err))
	}

	database, err := gorm.Open(mysql.Open(c.DSN), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("鏁版嵁搴撳垵濮嬪寲杩炴帴澶辫触: %w", err))
	}

	// 鍒濆鍖?db 鐨勮〃
	err = db.InitTables(database)
	if err != nil {
		panic(fmt.Errorf("鏁版嵁搴撳垵濮嬪寲琛ㄥけ璐? %w", err))
	}
	return database
}


