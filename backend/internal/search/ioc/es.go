package ioc

import (
	"log"

	"github.com/XDWow/DouyinMall/backend/internal/search/infra/es"
	"github.com/spf13/viper"
)

// InitES 初始化 ES8 客户端并创建索引
func InitES() *es.ESClient {
	addresses := viper.GetStringSlice("elasticsearch.addresses")
	if len(addresses) == 0 {
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
