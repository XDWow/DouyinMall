package validator

import (
	"context"
	"reflect"
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/migrator"
	events "github.com/XDWow/DouyinMall/backend/pkg/migrator/events"
	"github.com/ecodeclub/ekit/slice"
	"github.com/ecodeclub/ekit/syncx/atomicx"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

// 杩欓儴鍒嗕唬鐮佸仛鐨勬槸鍏ㄩ噺鏍￠獙/澧為噺鏍￠獙+鍙戜慨澶嶄俊鎭?
// id: 鍛婅瘔娑堣垂鑰呰繖涓?id 鐨勬暟鎹湁闂
// direction: 浠ヨ皝涓篵ase(src or dst)

// Validator T 蹇呴』瀹炵幇浜?Entity 鎺ュ彛
type Validator[T migrator.Entity] struct {
	// 浠?base 涓哄熀鍑?
	base   *gorm.DB
	target *gorm.DB

	p         events.Producer
	batchSize int
	l         logger.LoggerV1
	highLoad  *atomicx.Value[bool]
	direction string

	utime int64
	// <=0 璇存槑鐩存帴閫€鍑烘牎楠屽惊鐜?
	// > 0 鐪熺殑 sleep
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
		// 鍦ㄨ繖閲岋紝鍘绘煡璇㈡暟鎹簱鐨勭姸鎬?
		// 浣犵殑鏍￠獙浠ｇ爜涓嶅お鍙兘鏄€ц兘鐡堕锛屾€ц兘鐡堕涓€鑸湪鏁版嵁搴?
		// 浣犱篃鍙互缁撳悎鏈湴鐨?CPU锛屽唴瀛樿礋杞芥潵鍒ゅ畾
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

// 绗竴涓叏閲忔牎楠岋紝鏄负浜嗗悓姝ュ垵濮嬪寲鐩爣琛ㄧ粨鏋勩€佹暟鎹繖娈垫椂闂达紝base琛ㄧ殑鎻掑叆銆佸垹闄ゆ搷浣?
func (v *Validator[T]) Validate(ctx context.Context) error {
	var eg errgroup.Group
	eg.Go(func() error {
		v.validateBaseToTarget(ctx)
		// 鍗充娇杩欎釜鏍￠獙鍑洪敊浜嗭紝鎴戜篃涓嶅笇鏈涘彟涓€涓牎楠屽仠涓嬫潵
		// 杩欓噷鐢╡rrgroup.Group鐨勭洰鐨勬槸鏂逛究wait()
		// 涔熷彲浠ync.wait()
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
			//鎸傝捣
		}
		// 鍘绘簮搴撴壘鏁版嵁
		// 鍏ㄩ噺鏍￠獙鍜屽閲忔牎楠岀殑鍖哄埆鍦ㄤ簬鍙栨暟鎹紝鎵€浠ヨ繖涓?fromBase 鍋氭垚浜嗗彲淇敼鐨?
		src, err := v.fromBase(ctx, offset)
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return

		case gorm.ErrRecordNotFound:
			// 姣斿畬浜嗐€傛病鏁版嵁浜嗭紝鍏ㄩ噺鏍￠獙缁撴潫浜?
			// 鍚屾椂鏀寔鍏ㄩ噺鏍￠獙鍜屽閲忔牎楠岋紝浣犺繖閲屽氨涓嶈兘鐩存帴杩斿洖
			// 鍦ㄨ繖閲岋紝浣犺鑰冭檻锛氭湁浜涙儏鍐典笅锛岀敤鎴峰笇鏈涢€€鍑猴紝鏈変簺鎯呭喌涓嬨€傜敤鎴峰笇鏈涚户缁?
			// 褰撶敤鎴峰笇鏈涚户缁殑鏃跺€欙紝浣犺 sleep 涓€涓?
			if v.sleepInterval <= 0 {
				return
			}
			time.Sleep(v.sleepInterval)
			// continue 灏辨槸涓嶈蛋 offset++锛屼笉鎸?
			continue

		case nil:
			var dst T
			err = v.target.Where("id = ?", src.ID()).First(&dst).Error
			switch err {
			case nil:
				// 姣旇緝
				// 1. src == dst 閿?
				// 2.鍘熷垯涓婃槸鍙互鍒╃敤鍙嶅皠鏉ユ瘮
				//if reflect.DeepEqual(src, dst) {
				//}
				// 3.鐢ㄥ畠鑷畾涔夌殑姣旇緝閫昏緫
				// 4. 鍔ㄦ€侀€夋嫨
				// 杩欎釜鍐欐硶姣旇緝鏈夋剰鎬濓紝鏂█鍏舵槸鍚﹀疄鐜颁簡CompareTo锛堬級鏂规硶锛屼笉杩囪any绫诲瀷鎵嶈兘杩欐牱鏂█
				var srcAny any = src
				if c1, ok := srcAny.(interface {
					// 鏈夋病鏈夎嚜瀹氫箟鐨勬瘮杈冮€昏緫
					CompareTo(c2 migrator.Entity) bool
				}); ok {
					// 鏈夛紝鎴戝氨鐢ㄥ畠鐨?
					if !c1.CompareTo(dst) {
						// 涓嶇浉绛夛紝涓婃姤Kafka锛氭暟鎹笉涓€鑷?
						// 淇℃伅鏄粈涔堬紵鐪嬫秷璐硅€呴渶瑕佷粈涔?>瀹氫箟鐩稿叧浜嬩欢event
						v.notify(ctx, src.ID(), events.InconsistentEventTypeNEQ)
					}
				} else {
					// 娌℃湁锛屾垜灏辩敤鍙嶅皠
					if !reflect.DeepEqual(src, dst) {
						v.notify(ctx, src.ID(), events.InconsistentEventTypeNEQ)
					}
				}
			case gorm.ErrRecordNotFound:
				// target 涓皯浜嗘暟鎹?
				v.notify(ctx, src.ID(), events.InconsistentEventTypeTargetMissing)
			case context.Canceled, context.DeadlineExceeded:
				// 瓒呮椂鎴栬鍙栨秷,缁撴潫
				return
			default:
				v.l.Error("鏌ヨ target 鏁版嵁澶辫触", logger.Error(err))
			}

		default:
			v.l.Error("鏍￠獙鏁版嵁锛屾煡璇?base 鍑洪敊",
				logger.Error(err))
			// default 鏄嚭閿欎簡锛宱ffset 鍔ㄤ笉鍔紵
			// 濡傛灉鍔ㄤ簡锛屾紡涓€鏉℃暟鎹?
			// 濡傛灉涓嶅姩锛屼竾涓€杩欐潯鏁版嵁涓€鐩村嚭閿欒锛屾案杩滃崱鍦ㄨ繖锛屽奖鍝嶆洿澶?
		}
		offset++
	}
}

// 璋冪敤杩欎釜鏂规硶鍒囨崲澧為噺鏍￠獙
func (v *Validator[T]) Incr() *Validator[T] {
	v.fromBase = v.IncrFromBase
	v.fromTarget = v.IncrFromTarget
	return v
}

// 杩欎竴閬嶆槸涓轰簡鎵惧嚭 base 涓垹鎺夌殑鏁版嵁锛屾秷璐硅€呮嬁鍒版秷鎭幓鍒犻櫎target涓殑鏁版嵁
func (v *Validator[T]) validateTargetToBase(ctx context.Context) {
	// 鍏堟壘 target锛屽啀鎵?base锛屾壘鍑?base 涓凡缁忚鍒犻櫎鐨?
	// 鐞嗚涓婃潵璇达紝灏辨槸 target 閲岄潰涓€鏉℃潯鎵撅紝涓嶈繃杩欓噷鍙互浼樺寲
	offset := 0
	for {
		dbCtx, cancel := context.WithTimeout(ctx, time.Second)
		dstIds, err := v.fromTarget(dbCtx, offset)
		cancel()
		// gorm 鍦?Find 鏂规硶鎺ユ敹鐨勬槸鍒囩墖鐨勬椂鍊欙紝涓嶄細杩斿洖 gorm.ErrRecordNotFound,鎵€浠ラ€氳繃杩欐牱鍒ゆ柇
		if len(dstIds) == 0 {
			// 娌℃暟鎹簡锛岃繑鍥?
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
				v.l.Error("鏌ヨbase:target涓湁鐨勬暟鎹け璐?)
			}
		default:
			v.l.Error("鏌ヨtarget 澶辫触", logger.Error(err))
		}
		// 娉ㄦ剰杩欓噷涓嶆槸 + limit锛宭imit鏄渶澶у€硷紝瀹為檯涓妉en(dstTs)鏉?
		offset += len(dstIds)
		// 娌℃湁涓嬩竴鎵逛簡
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
	// 瑕佹寜鐓?id 鍗囧簭鏉ユ壘
	// 濡傛灉闄嶅簭锛屾柊鎻掑叆鐨勬暟鎹亶鍘嗕笉鍒?
	// 濡傛灉娌℃湁 id 杩欎釜鍒楋紝鎵句竴涓被浼肩殑鍒楋紝濡?ctime, utime涓嶈锛屽洜涓轰細涓嶅仠鍙橈紝浼氭紡鏁版嵁
	// 浣滀笟锛氭敼鎴愭壒閲忥紝鎬ц兘浼氬ソ寰堝
	err := v.base.WithContext(dbCtx).Order("id").
		Offset(offset).
		First(&src).Error
	return src, err
}

func (v *Validator[T]) IncrFromBase(ctx context.Context, offset int) (T, error) {
	dbCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var src T
	// 澧為噺鏍￠獙鍙兘鎸?utime 鏉ュ彇
	// 杩欓噷杩樻槸鏈夐棶棰? utime鐨勯棶棰?
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
	// 澧為噺鏍￠獙鍙兘鎸?utime 鏉ュ彇
	// 杩欓噷杩樻槸鏈夐棶棰? utime鐨勯棶棰?
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
		v.l.Error("鍙戦€佹暟鎹笉涓€鑷寸殑娑堟伅澶辫触", logger.Error(err))
	}
}


