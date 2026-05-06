package ioc

import (
	"log"

	"github.com/XDWow/DouyinMall/backend/internal/search/infra/es"
	"github.com/spf13/viper"
)

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
		panic("init es client failed: " + err.Error())
	}

	log.Println("initializing search indices")
	if err := es.InitIndices(client); err != nil {
		panic("init search indices failed: " + err.Error())
	}
	log.Println("search indices ready")

	return client
}
