package ioc

import (
	"log"

	"github.com/XDWow/DouyinMall/backend/internal/search/repo/es"
	"github.com/spf13/viper"
)

// InitES 初始化 Elasticsearch 客户端并创建索引（类似数据库的 AutoMigrate）
// 索引创建是幂等的：如果索引已存在，则跳过；如果不存在，则创建
func InitES() *es.ESClient {
	addresses := viper.GetStringSlice("elasticsearch.addresses")
	if len(addresses) == 0 {
		// 如果从环境变量读取（字符串），转换为数组
		if addr := viper.GetString("elasticsearch.addresses"); addr != "" {
			addresses = []string{addr}
		} else {
			addresses = []string{"http://localhost:9200"}
		}
	}

	client, err := es.NewESClient(addresses)
	if err != nil {
		panic("初始化 ES 客户端失败: " + err.Error())
	}

	log.Println("正在初始化 ES 索引...")
	if err := es.InitIndices(client); err != nil {
		panic("初始化 ES 索引失败: " + err.Error())
	}
	log.Println("ES 索引初始化成功")

	return client
}
