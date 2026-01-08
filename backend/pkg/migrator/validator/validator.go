package validator

import (
	"context"
	"github.com/XDWow/ScholarNet/cmd/pkg/logger"
	"github.com/XDWow/ScholarNet/cmd/pkg/migrator"
	events "github.com/XDWow/ScholarNet/cmd/pkg/migrator/events"
	"github.com/ecodeclub/ekit/slice"
	"github.com/ecodeclub/ekit/syncx/atomicx"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"reflect"
	"time"
)

// 这部分代码做的是全量校验/增量校验+发修复信息:
// id: 告诉消费者这个 id 的数据有问题
// direction: 以谁为base(src or dst)

// Validator T 必须实现了 Entity 接口
type Validator[T migrator.Entity] struct {
	// 以 base 为基准
	base   *gorm.DB
	target *gorm.DB

	p         events.Producer
	batchSize int
	l         logger.LoggerV1
	highLoad  *atomicx.Value[bool]
	direction string

	utime int64
	// <=0 说明直接退出校验循环
	// > 0 真的 sleep
	sleepInterval time.Duration

	fromBase   func(ctx context.Context, offset int) (T, error)
	fromTarget func(ctx context.Context, offset int) ([]int64, error)
}

func NeValidator[T migrator.Entity](
	base *gorm.DB,
	target *gorm.DB,
	direction string,
	p events.Producer,
	l logger.LoggerV1,
	batchSize int,
) *Validator[T] {
	highLoad := atomicx.NewValueOf[bool](false)
	go func() {
		// 在这里，去查询数据库的状态
		// 你的校验代码不太可能是性能瓶颈，性能瓶颈一般在数据库
		// 你也可以结合本地的 CPU，内存负载来判定
	}()
	res := &Validator[T]{
		base:      base,
		target:    target,
		direction: direction,
		p:         p,
		l:         l,
		highLoad:  highLoad,
		batchSize: batchSize,
	}
	res.fromBase = res.fullFromBase
	res.fromTarget = res.fullFromTarget
	return res
}

func (v *Validator[T]) SleepInterval(i time.Duration) *Validator[T] {
	v.sleepInterval = i
	return v
}

func (v *Validator[T]) Utime(utime int64) *Validator[T] {
	v.utime = utime
	return v
}

// 第一个全量校验，是为了同步初始化目标表结构、数据这段时间，base表的插入、删除操作
func (v *Validator[T]) Validate(ctx context.Context) error {
	var eg errgroup.Group
	eg.Go(func() error {
		v.validateBaseToTarget(ctx)
		// 即使这个校验出错了，我也不希望另一个校验停下来
		// 这里用errgroup.Group的目的是方便wait()
		// 也可以sync.wait()
		return nil
	})
	eg.Go(func() error {
		v.validateTargetToBase(ctx)
		return nil
	})
	return eg.Wait()
}

func (v *Validator[T]) validateBaseToTarget(ctx context.Context) {
	offset := 0
	for {
		if v.highLoad.Load() {
			//挂起
		}
		// 去源库找数据
		// 全量校验和增量校验的区别在于取数据，所以这个 fromBase 做成了可修改的
		src, err := v.fromBase(ctx, offset)
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return

		case gorm.ErrRecordNotFound:
			// 比完了。没数据了，全量校验结束了
			// 同时支持全量校验和增量校验，你这里就不能直接返回
			// 在这里，你要考虑：有些情况下，用户希望退出，有些情况下。用户希望继续
			// 当用户希望继续的时候，你要 sleep 一下
			if v.sleepInterval <= 0 {
				return
			}
			time.Sleep(v.sleepInterval)
			// continue 就是不走 offset++，不挪
			continue

		case nil:
			var dst T
			err = v.target.Where("id = ?", src.ID()).First(&dst).Error
			switch err {
			case nil:
				// 比较
				// 1. src == dst 错
				// 2.原则上是可以利用反射来比
				//if reflect.DeepEqual(src, dst) {
				//}
				// 3.用它自定义的比较逻辑
				// 4. 动态选择
				// 这个写法比较有意思，断言其是否实现了CompareTo（）方法，不过要any类型才能这样断言
				var srcAny any = src
				if c1, ok := srcAny.(interface {
					// 有没有自定义的比较逻辑
					CompareTo(c2 migrator.Entity) bool
				}); ok {
					// 有，我就用它的
					if !c1.CompareTo(dst) {
						// 不相等，上报Kafka：数据不一致
						// 信息是什么？看消费者需要什么->定义相关事件event
						v.notify(ctx, src.ID(), events.InconsistentEventTypeNEQ)
					}
				} else {
					// 没有，我就用反射
					if !reflect.DeepEqual(src, dst) {
						v.notify(ctx, src.ID(), events.InconsistentEventTypeNEQ)
					}
				}
			case gorm.ErrRecordNotFound:
				// target 中少了数据
				v.notify(ctx, src.ID(), events.InconsistentEventTypeTargetMissing)
			case context.Canceled, context.DeadlineExceeded:
				// 超时或被取消,结束
				return
			default:
				v.l.Error("查询 target 数据失败", logger.Error(err))
			}

		default:
			v.l.Error("校验数据，查询 base 出错",
				logger.Error(err))
			// default 是出错了，offset 动不动？
			// 如果动了，漏一条数据
			// 如果不动，万一这条数据一直出错误，永远卡在这，影响更大
		}
		offset++
	}
}

// 调用这个方法切换增量校验
func (v *Validator[T]) Incr() *Validator[T] {
	v.fromBase = v.IncrFromBase
	v.fromTarget = v.IncrFromTarget
	return v
}

// 这一遍是为了找出 base 中删掉的数据，消费者拿到消息去删除target中的数据
func (v *Validator[T]) validateTargetToBase(ctx context.Context) {
	// 先找 target，再找 base，找出 base 中已经被删除的
	// 理论上来说，就是 target 里面一条条找，不过这里可以优化
	offset := 0
	for {
		dbCtx, cancel := context.WithTimeout(ctx, time.Second)
		dstIds, err := v.fromTarget(dbCtx, offset)
		cancel()
		// gorm 在 Find 方法接收的是切片的时候，不会返回 gorm.ErrRecordNotFound,所以通过这样判断
		if len(dstIds) == 0 {
			// 没数据了，返回
			if v.sleepInterval <= 0 {
				return
			}
			time.Sleep(v.sleepInterval)
			continue
		}
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return
		case nil:
			var srcTs []T
			err = v.base.Where("id IN (?)", dstIds).Find(&srcTs).Error
			if len(srcTs) == 0 {
				v.notifyBaseMissing(ctx, dstIds)
			}
			switch err {
			case context.Canceled, context.DeadlineExceeded:
				return
			case nil:
				srcIds := slice.Map(srcTs, func(idx int, t T) int64 { return t.ID() })
				diff := slice.DiffSet(dstIds, srcIds)
				v.notifyBaseMissing(ctx, diff)
			default:
				v.l.Error("查询base:target中有的数据失败")
			}
		default:
			v.l.Error("查询target 失败", logger.Error(err))
		}
		// 注意这里不是 + limit，limit是最大值，实际上len(dstTs)条
		offset += len(dstIds)
		// 没有下一批了
		if len(dstIds) < v.batchSize {
			if v.sleepInterval <= 0 {
				return
			}
			time.Sleep(v.sleepInterval)
		}
	}
}

func (v *Validator[T]) fullFromBase(ctx context.Context, offset int) (T, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var src T
	// 要按照 id 升序来找
	// 如果降序，新插入的数据遍历不到
	// 如果没有 id 这个列，找一个类似的列，如 ctime, utime不行，因为会不停变，会漏数据
	// 作业：改成批量，性能会好很多
	err := v.base.WithContext(dbCtx).Order("id").
		Offset(offset).
		First(&src).Error
	return src, err
}

func (v *Validator[T]) IncrFromBase(ctx context.Context, offset int) (T, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var src T
	// 增量校验只能按 utime 来取
	// 这里还是有问题, utime的问题
	err := v.base.WithContext(dbCtx).
		Where("utime > ?", v.utime).
		Order("utime").
		Offset(offset).
		First(&src).Error
	return src, err
}

func (v *Validator[T]) fullFromTarget(ctx context.Context, offset int) ([]int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var ids []int64
	err := v.target.WithContext(dbCtx).
		Select("id").
		Order("id").
		Offset(offset).
		Limit(v.batchSize).
		Find(&ids).Error
	return ids, err
}

func (v *Validator[T]) IncrFromTarget(ctx context.Context, offset int) ([]int64, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var ids []int64
	// 增量校验只能按 utime 来取
	// 这里还是有问题, utime的问题
	err := v.target.WithContext(dbCtx).
		Where("utime > ?", v.utime).
		Order("utime").
		Select("id").
		Offset(offset).
		Limit(v.batchSize).
		Find(&ids).Error
	return ids, err
}

func (v *Validator[T]) notifyBaseMissing(ctx context.Context, ids []int64) {
	for _, id := range ids {
		v.notify(ctx, id, events.InconsistentEventTypeBaseMissing)
	}
}

func (v *Validator[T]) notify(ctx context.Context, id int64, typ string) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	err := v.p.ProduceInconsistentEvent(ctx,
		events.InconsistentEvent{
			ID:        id,
			Direction: v.direction,
			Type:      typ,
		})
	if err != nil {
		v.l.Error("发送数据不一致的消息失败", logger.Error(err))
	}
}
