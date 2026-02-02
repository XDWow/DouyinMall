package job

type Job interface { // 定时任务（同步超时订单）
	Run() error
	Name() string
}
