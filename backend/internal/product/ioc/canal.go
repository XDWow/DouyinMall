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
	if password == "" {
		password = "root"
	}

	cfg := canal.NewDefaultConfig()
	cfg.Addr = fmt.Sprintf("%s:%d", host, port)
	cfg.User = user
	cfg.Password = password
	// 设置 server_id（用于 binlog replication，每个 Canal 实例需要唯一）
	cfg.ServerID = uint32(viper.GetInt("canal.server_id"))
	if cfg.ServerID == 0 {
		cfg.ServerID = 1234 // 默认值
	}
	cfg.Dump.ExecutionPath = ""
	cfg.Dump.TableDB = "DouyinMall"

	positionDao := dao.NewGormPositionDao(db)

	canalProducer, err := prod.NewCanalProducer(cfg, kafkaProducer, logger, positionDao)
	if err != nil {
		panic(fmt.Errorf("初始化 Canal Producer 失败: %w", err))
	}

	return canalProducer
}
