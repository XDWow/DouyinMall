package product

import (
	"fmt"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()

}

func initViperWatch() {
	cfile := pflag.String("config",
		"internal/user/config/dev.yaml", "閰嶇疆鏂囦欢璺緞")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("璇诲彇閰嶇疆鏂囦欢澶辫触: %w", err))
	}

	// 鏀寔鐜鍙橀噺瑕嗙洊閰嶇疆鏂囦欢锛堢幆澧冨彉閲忎紭鍏堬級
	viper.AutomaticEnv()
	// 璁剧疆鐜鍙橀噺鍓嶇紑锛堝彲閫夛級
	// viper.SetEnvPrefix("USER_SERVICE")

	// 鎵嬪姩缁戝畾鐜鍙橀噺鍒伴厤缃敭
	viper.BindEnv("db.dsn", "DB_DSN")
	viper.BindEnv("redis.addr", "REDIS_ADDR")
	viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
}


