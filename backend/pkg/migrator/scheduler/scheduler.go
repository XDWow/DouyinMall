package scheduler

import (
	"context"
	"fmt"
	"github.com/XDWow/ScholarNet/cmd/pkg/ginx"
	"github.com/XDWow/ScholarNet/cmd/pkg/gormx/connpool"
	"github.com/XDWow/ScholarNet/cmd/pkg/logger"
	"github.com/XDWow/ScholarNet/cmd/pkg/migrator"
	"github.com/XDWow/ScholarNet/cmd/pkg/migrator/events"
	"github.com/XDWow/ScholarNet/cmd/pkg/migrator/validator"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sync"
	"time"
)

// Scheduler 用来统一管理整个迁移过程
// 它不是必须的，你可以理解为这是为了方便用户操作（和你理解）而引入的。
type Scheduler[T migrator.Entity] struct {
	lock     sync.Mutex
	src      *gorm.DB
	dst      *gorm.DB
	pool     *connpool.DoubleWritePool
	l        logger.LoggerV1
	pattern  string
	producer events.Producer

	cancelFull func()
	cancelIncr func()
}

func NewScheduler[T migrator.Entity](
	src *gorm.DB,
	dst *gorm.DB,
	l logger.LoggerV1,
	pool *connpool.DoubleWritePool,
	producer events.Producer) *Scheduler[T] {
	return &Scheduler[T]{
		l:          l,
		src:        src,
		dst:        dst,
		pool:       pool,
		producer:   producer,
		cancelFull: func() {},
		cancelIncr: func() {},
		pattern:    connpool.PatternSrcOnly,
	}
}

// 这一个也不是必须的，就是你可以考虑利用配置中心，监听配置中心的变化
// 把全量校验，增量校验做成分布式任务，利用分布式任务调度平台来调度
func (s *Scheduler[T]) RegisterRoutes(server *gin.RouterGroup) {
	// 将这个暴露为 HTTP 接口
	// 你可以配上对应的 UI
	server.POST("/src_only", ginx.Wrap(s.SrcOnly))
	server.POST("/src_first", ginx.Wrap(s.SrcFirst))
	server.POST("/dst_first", ginx.Wrap(s.DstFirst))
	server.POST("/dst_only", ginx.Wrap(s.DstOnly))
	server.POST("/full/start", ginx.Wrap(s.StartFullValidation))
	server.POST("/full/stop", ginx.Wrap(s.StopFullValidation))
	server.POST("/incr/stop", ginx.Wrap(s.StopIncrValidation))
	server.POST("/incr/start", ginx.WrapBodyV1[StartIncrRequest](s.StartIncrValidation))
}

// ---- 下面是四个阶段 ---- //
// 切换的实质是改变 connpool 的操作：修改 pattern
// SrcOnly 只读写源表
func (s *Scheduler[T]) SrcOnly(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternSrcOnly
	s.pool.UpdatePattern(s.pattern)
	return ginx.Result{
		Code: 200,
		Msg:  "OK",
	}, nil
}

func (s *Scheduler[T]) SrcFirst(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternSrcFirst
	s.pool.UpdatePattern(s.pattern)
	return ginx.Result{
		Code: 200,
		Msg:  "OK",
	}, nil
}

func (s *Scheduler[T]) DstFirst(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternDstFirst
	s.pool.UpdatePattern(s.pattern)
	return ginx.Result{
		Code: 200,
		Msg:  "OK",
	}, nil
}

func (s *Scheduler[T]) DstOnly(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pattern = connpool.PatternDstOnly
	s.pool.UpdatePattern(s.pattern)
	return ginx.Result{
		Code: 200,
		Msg:  "OK",
	}, nil
}

func (s *Scheduler[T]) StartFullValidation(c *gin.Context) (ginx.Result, error) {
	// 这里锁大有用处
	// 与切换方法共享这个锁，保证了在校验时无法切换模式，一定程度上保护了数据正确性
	s.lock.Lock()
	defer s.lock.Unlock()
	// 准备取消上一次的ctx，释放资源
	cancel := s.cancelFull
	v, err := s.newValidator()
	if err != nil {
		return ginx.Result{}, err
	}
	var ctx context.Context
	ctx, s.cancelFull = context.WithCancel(context.Background())
	// 异步校验，主线程返回结果
	go func() {
		cancel()
		err := v.Validate(ctx)
		if err != nil {
			s.l.Warn("退出全量校验", logger.Error(err))
		}
	}()
	return ginx.Result{
		Code: 200,
		Msg:  "OK",
	}, nil
}

func (s *Scheduler[T]) StopFullValidation(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cancelFull()
	return ginx.Result{
		Code: 200,
		Msg:  "OK",
	}, nil
}

func (s *Scheduler[T]) StartIncrValidation(c *gin.Context, req StartIncrRequest) (ginx.Result, error) {
	// 这里锁大有用处
	// 1、防止多个线程过来都开启校验，人家搞到一半，你再来一起校验没意义，或者取消别人的校验也不行
	// 2、与切换方法共享这个锁，保证了在校验时无法切换模式，一定程度上保护了数据正确性
	s.lock.Lock()
	defer s.lock.Unlock()
	// 准备取消上一次的ctx，释放资源
	cancel := s.cancelFull
	v, err := s.newValidator()
	if err != nil {
		return ginx.Result{}, err
	}
	// 修改模式为增量校验，并传入utime,SleepInterval
	v.Incr().Utime(req.Utime).SleepInterval(time.Duration(req.Interval) * time.Millisecond)
	var ctx context.Context
	ctx, s.cancelFull = context.WithCancel(context.Background())
	// 异步校验，主线程返回结果
	go func() {
		cancel()
		err := v.Validate(ctx)
		if err != nil {
			s.l.Warn("退出增量校验", logger.Error(err))
		}
	}()
	return ginx.Result{
		Code: 200,
		Msg:  "启动增量校验成功",
	}, nil
}

func (s *Scheduler[T]) StopIncrValidation(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cancelFull()
	return ginx.Result{
		Code: 200,
		Msg:  "OK",
	}, nil
}

func (s *Scheduler[T]) newValidator() (*validator.Validator[T], error) {
	switch s.pattern {
	case connpool.PatternSrcOnly, connpool.PatternSrcFirst:
		return validator.NeValidator[T](s.src, s.dst, "SRC", s.producer, s.l, 10), nil
	case connpool.PatternDstFirst, connpool.PatternDstOnly:
		return validator.NeValidator[T](s.src, s.dst, "SRC", s.producer, s.l, 10), nil
	default:
		return nil, fmt.Errorf("invalid pattern: %s", s.pattern)
	}
}

type StartIncrRequest struct {
	Utime int64 `json:"utime"`
	// 毫秒数
	// json 不能正确处理 time.Duration 类型
	Interval int64 `json:"interval"`
}
