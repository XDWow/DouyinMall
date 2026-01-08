package wego

import (
	"github.com/XDWow/DouyinMall/backend/cmd/pkg/ginx"
	"github.com/XDWow/DouyinMall/backend/cmd/pkg/grpcx"
	"github.com/XDWow/DouyinMall/backend/cmd/pkg/saramax"
	"github.com/robfig/cron/v3"
)

// App 当你在 wire 里面使用这个结构体的时候，要注意不是所有的服务都需要全部字段，
// 那么在 wire 的时候就不要使用 * 了
type App struct {
	GRPCServer *grpcx.Server
	WebServer  *ginx.Server
	Consumers  []saramax.Consumer
	Cron       *cron.Cron
}
