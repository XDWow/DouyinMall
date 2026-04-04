package ioc

import (
	"log"

	"github.com/XDWow/DouyinMall/backend/internal/search/infra/es"
	"github.com/spf13/viper"
)

// InitES 鍒濆鍖?ES8 瀹㈡埛绔苟鍒涘缓绱㈠紩
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
		panic("鍒濆鍖?ES 瀹㈡埛绔け璐? " + err.Error())
	}

	log.Println("姝ｅ湪鍒濆鍖?ES 绱㈠紩...")
	if err := es.InitIndices(client); err != nil {
		panic("鍒濆鍖?ES 绱㈠紩澶辫触: " + err.Error())
	}
	log.Println("ES 绱㈠紩鍒濆鍖栨垚鍔?)

	return client
}


