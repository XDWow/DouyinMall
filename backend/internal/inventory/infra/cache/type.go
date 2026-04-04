package cache

import (
	"context"
	"time"
)

type InventoryCache interface {
	// Lua 鑴氭湰鎵ц锛堟牳蹇冿級
	// 鐢ㄤ簬锛歊eserveStock銆丷eleaseStock 鐨勫師瀛愭搷浣?
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)

	// Hash 鎵归噺璇诲彇
	// 鐢ㄤ簬锛歊efundStock 鎵归噺璇诲彇棰勬墸璁板綍
	// 鎵归噺涓€鑸繑鍥烇細[]interface{} 瀵瑰簲fields鐨勫€硷紝杩欐牱涓嶅瓨鍦ㄧ殑field杩斿洖nil锛屽彲浠ュ尯鍒?"鍜岀┖
	HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error)

	// 鏁板€煎鍔?
	// 鐢ㄤ簬锛歊efundStock 鎭㈠Redis棰勫簱瀛?
	IncrBy(ctx context.Context, key string, delta int32) (int64, error)

	// 鐢ㄤ簬锛欳acheRepairJob 鏌ヨ棰勫簱瀛?
	Get(ctx context.Context, key string) (string, error)

	// 鐢ㄤ簬锛欳acheRepairJob 淇棰勫簱瀛?
	Set(ctx context.Context, key string, value string, expiration time.Duration) (string, error)
}


