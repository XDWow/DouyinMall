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

// Scheduler 鐢ㄦ潵缁熶竴绠＄悊鏁翠釜杩佺Щ杩囩▼
// 瀹冧笉鏄繀椤荤殑锛屼綘鍙互鐞嗚В涓鸿繖鏄负浜嗘柟渚跨敤鎴锋搷浣滐紙鍜屼綘鐞嗚В锛夎€屽紩鍏ョ殑銆?
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

// 杩欎竴涓篃涓嶆槸蹇呴』鐨勶紝灏辨槸浣犲彲浠ヨ€冭檻鍒╃敤閰嶇疆涓績锛岀洃鍚厤缃腑蹇冪殑鍙樺寲
// 鎶婂叏閲忔牎楠岋紝澧為噺鏍￠獙鍋氭垚鍒嗗竷寮忎换鍔★紝鍒╃敤鍒嗗竷寮忎换鍔¤皟搴﹀钩鍙版潵璋冨害
func (s *Scheduler[T]) RegisterRoutes(server *gin.RouterGroup) {
	// 灏嗚繖涓毚闇蹭负 HTTP 鎺ュ彛
	// 浣犲彲浠ラ厤涓婂搴旂殑 UI
	server.POST("/src_only", ginx.Wrap(s.SrcOnly))
	server.POST("/src_first", ginx.Wrap(s.SrcFirst))
	server.POST("/dst_first", ginx.Wrap(s.DstFirst))
	server.POST("/dst_only", ginx.Wrap(s.DstOnly))
	server.POST("/full/start", ginx.Wrap(s.StartFullValidation))
	server.POST("/full/stop", ginx.Wrap(s.StopFullValidation))
	server.POST("/incr/stop", ginx.Wrap(s.StopIncrValidation))
	server.POST("/incr/start", ginx.WrapReq[StartIncrRequest](s.StartIncrValidation))
}

// ---- 涓嬮潰鏄洓涓樁娈?---- //
// 鍒囨崲鐨勫疄璐ㄦ槸鏀瑰彉 connpool 鐨勬搷浣滐細淇敼 pattern
// SrcOnly 鍙鍐欐簮琛?
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
	// 杩欓噷閿佸ぇ鏈夌敤澶?
	// 涓庡垏鎹㈡柟娉曞叡浜繖涓攣锛屼繚璇佷簡鍦ㄦ牎楠屾椂鏃犳硶鍒囨崲妯″紡锛屼竴瀹氱▼搴︿笂淇濇姢浜嗘暟鎹纭€?
	s.lock.Lock()
	defer s.lock.Unlock()
	// 鍑嗗鍙栨秷涓婁竴娆＄殑ctx锛岄噴鏀捐祫婧?
	cancel := s.cancelFull
	v, err := s.newValidator()
	if err != nil {
		return ginx.Result{}, err
	}
	var ctx context.Context
	ctx, s.cancelFull = context.WithCancel(context.Background())
	// 寮傛鏍￠獙锛屼富绾跨▼杩斿洖缁撴灉
	go func() {
		cancel()
		err := v.Validate(ctx)
		if err != nil {
			s.l.Warn("閫€鍑哄叏閲忔牎楠?, logger.Error(err))
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
	// 杩欓噷閿佸ぇ鏈夌敤澶?
	// 1銆侀槻姝㈠涓嚎绋嬭繃鏉ラ兘寮€鍚牎楠岋紝浜哄鎼炲埌涓€鍗婏紝浣犲啀鏉ヤ竴璧锋牎楠屾病鎰忎箟锛屾垨鑰呭彇娑堝埆浜虹殑鏍￠獙涔熶笉琛?
	// 2銆佷笌鍒囨崲鏂规硶鍏变韩杩欎釜閿侊紝淇濊瘉浜嗗湪鏍￠獙鏃舵棤娉曞垏鎹㈡ā寮忥紝涓€瀹氱▼搴︿笂淇濇姢浜嗘暟鎹纭€?
	s.lock.Lock()
	defer s.lock.Unlock()
	// 鍑嗗鍙栨秷涓婁竴娆＄殑ctx锛岄噴鏀捐祫婧?
	cancel := s.cancelFull
	v, err := s.newValidator()
	if err != nil {
		return ginx.Result{}, err
	}
	// 淇敼妯″紡涓哄閲忔牎楠岋紝骞朵紶鍏time,SleepInterval
	v.Incr().Utime(req.Utime).SleepInterval(time.Duration(req.Interval) * time.Millisecond)
	var ctx context.Context
	ctx, s.cancelFull = context.WithCancel(context.Background())
	// 寮傛鏍￠獙锛屼富绾跨▼杩斿洖缁撴灉
	go func() {
		cancel()
		err := v.Validate(ctx)
		if err != nil {
			s.l.Warn("閫€鍑哄閲忔牎楠?, logger.Error(err))
		}
	}()
	return ginx.Result{
		Code: 200,
		Msg:  "鍚姩澧為噺鏍￠獙鎴愬姛",
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
	// 姣鏁?
	// json 涓嶈兘姝ｇ‘澶勭悊 time.Duration 绫诲瀷
	Interval int64 `json:"interval"`
}


