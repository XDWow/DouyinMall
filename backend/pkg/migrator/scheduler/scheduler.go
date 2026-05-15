package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/XDWow/DouyinMall/backend/pkg/gormx/connpool"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/migrator"
	"github.com/XDWow/DouyinMall/backend/pkg/migrator/events"
	"github.com/XDWow/DouyinMall/backend/pkg/migrator/validator"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
	producer events.Producer,
) *Scheduler[T] {
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

func (s *Scheduler[T]) RegisterRoutes(server *gin.RouterGroup) {
	server.POST("/src_only", ginx.Wrap(s.SrcOnly))
	server.POST("/src_first", ginx.Wrap(s.SrcFirst))
	server.POST("/dst_first", ginx.Wrap(s.DstFirst))
	server.POST("/dst_only", ginx.Wrap(s.DstOnly))
	server.POST("/full/start", ginx.Wrap(s.StartFullValidation))
	server.POST("/full/stop", ginx.Wrap(s.StopFullValidation))
	server.POST("/incr/stop", ginx.Wrap(s.StopIncrValidation))
	server.POST("/incr/start", ginx.WrapReq[StartIncrRequest](s.StartIncrValidation))
}

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
	s.lock.Lock()
	defer s.lock.Unlock()

	cancel := s.cancelFull
	v, err := s.newValidator()
	if err != nil {
		return ginx.Result{}, err
	}

	var ctx context.Context
	ctx, s.cancelFull = context.WithCancel(context.Background())
	go func() {
		cancel()
		if err := v.Validate(ctx); err != nil {
			s.l.Warn("full validation stopped", logger.Error(err))
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
	s.lock.Lock()
	defer s.lock.Unlock()

	cancel := s.cancelIncr
	v, err := s.newValidator()
	if err != nil {
		return ginx.Result{}, err
	}

	v.Incr().Utime(req.Utime).SleepInterval(time.Duration(req.Interval) * time.Millisecond)

	var ctx context.Context
	ctx, s.cancelIncr = context.WithCancel(context.Background())
	go func() {
		cancel()
		if err := v.Validate(ctx); err != nil {
			s.l.Warn("incremental validation stopped", logger.Error(err))
		}
	}()

	return ginx.Result{
		Code: 200,
		Msg:  "incremental validation started",
	}, nil
}

func (s *Scheduler[T]) StopIncrValidation(c *gin.Context) (ginx.Result, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.cancelIncr()
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
	Utime    int64 `json:"utime"`
	Interval int64 `json:"interval"`
}
