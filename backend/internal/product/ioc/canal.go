package ioc

import (
	"fmt"

	"github.com/IBM/sarama"
	prod "github.com/XDWow/DouyinMall/backend/internal/product/producer"
	"github.com/XDWow/DouyinMall/backend/internal/product/repo/dao"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func InitCanalProducer(kafkaProducer sarama.SyncProducer, logger logger.LoggerV1, db *gorm.DB) prod.Producer {
	host := viper.GetString("canal.mysql.host")
	if host == "" {
		host = "localhost"
	}
	port := viper.GetInt("canal.mysql.port")
	if port == 0 {
		port = 3306
	}
	user := viper.GetString("canal.mysql.user")
	if user == "" {
		user = "root"
	}
	password := viper.GetString("canal.mysql.password")

	cfg := canal.NewDefaultConfig()
	cfg.Addr = fmt.Sprintf("%s:%d", host, port)
	cfg.User = user
	cfg.Password = password
	// 璁剧疆 server_id锛堢敤浜?binlog replication锛屾瘡涓?Canal 瀹炰緥闇€瑕佸敮涓€锛?
	cfg.ServerID = uint32(viper.GetInt("canal.server_id"))
	if cfg.ServerID == 0 {
		cfg.ServerID = 1234 // 榛樿鍊?
	}
	cfg.Dump.ExecutionPath = ""
	cfg.Dump.TableDB = "DouyinMall"

	positionDao := dao.NewGormPositionDao(db)

	canalProducer, err := prod.NewCanalProducer(cfg, kafkaProducer, logger, positionDao)
	if err != nil {
		panic(fmt.Errorf("鍒濆鍖?Canal Producer 澶辫触: %w", err))
	}

	return canalProducer
}
