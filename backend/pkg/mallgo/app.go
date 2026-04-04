package mallgo

import (
	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/XDWow/DouyinMall/backend/pkg/grpcx"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	"github.com/robfig/cron/v3"
)

// App 褰撲綘鍦?wire 閲岄潰浣跨敤杩欎釜缁撴瀯浣撶殑鏃跺€欙紝瑕佹敞鎰忎笉鏄墍鏈夌殑鏈嶅姟閮介渶瑕佸叏閮ㄥ瓧娈碉紝
// 閭ｄ箞鍦?wire 鐨勬椂鍊欏氨涓嶈浣跨敤 * 浜?
type App struct {
	GRPCServer *grpcx.Server
	WebServer  *ginx.Server
	Consumers  []saramax.Consumer
	Cron       *cron.Cron
}


