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
	// 榛樿閰嶇疆鍏滃簳
	c := config.DBConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Database: "mysql",
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

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("鏁版嵁搴撳垵濮嬪寲杩炴帴澶辫触: %w", err))
	}

	// 鍒濆鍖?db 鐨勮〃
	err = dao.InitTables(db)
	if err != nil {
		panic(fmt.Errorf("鏁版嵁搴撳垵濮嬪寲琛ㄥけ璐? %w", err))
	}
	return db
}
