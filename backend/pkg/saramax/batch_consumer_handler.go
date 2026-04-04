package saramax

import (
	"context"
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type BatchHandler[T any] struct {
	l  logger.LoggerV1
	fn func(msgs []*sarama.ConsumerMessage, ts []T) error
	// 鐢?option 妯″紡鏉ヨ缃繖涓?batchSize 鍜?duration
	batchSize     int
	batchDuration time.Duration

	maxConcurrency int
}

func NewBatchHandler[T any](l logger.LoggerV1, fn func(msgs []*sarama.ConsumerMessage, ts []T) error, batchsize int) *BatchHandler[T] {
	return &BatchHandler[T]{l: l, fn: fn, batchDuration: time.Second, batchSize: 10, maxConcurrency: 16}
}

func (b *BatchHandler[T]) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (b *BatchHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// 鎷挎秷鎭粰浣犲啓濂戒簡, 鎻愪氦涔熷府浣犲啓濂戒簡, 閮芥槸閫氱敤鐨?
// 鍙渶浼犲叆浣犵殑娑堣垂淇℃伅涓氬姟 fn()
// session 鏄湰娆℃秷璐逛細璇濈殑涓婁笅鏂囷紝璐熻矗鎻愪氦 Offset 鍜岃幏鍙栫粍鍐呭厓淇℃伅
// claim 鎻愪緵鍒嗛厤缁欏綋鍓嶅疄渚嬬殑鏌愪釜鍒嗗尯鐨勪俊鎭拰璇ュ垎鍖虹殑娑堟伅閫氶亾
// ConsumeClaim 鍙互鑰冭檻鍦ㄨ繖涓皝瑁呴噷闈㈡彁渚涚粺涓€鐨勯噸璇曟満鍒?
// 鎵归噺鎺ュ彛
func (h *BatchHandler[T]) ConsumeClaim(session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim) error {
	msgsCh := claim.Messages()
	// 杩欎釜鍙互鍋氭垚鍙傛暟
	const batchSize = 10
	for {
		msgs := make([]*sarama.ConsumerMessage, 0, batchSize)
		ts := make([]T, 0, batchSize)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		done := false
		for i := 0; i < batchSize && !done; i++ {
			select {
			case <-ctx.Done():
				// 杩欎竴鎵规宸茬粡瓒呮椂浜嗭紝
				// 鎴栬€咃紝鏁翠釜 consumer 琚叧闂簡
				// 涓嶅啀灏濊瘯鍑戝涓€鎵逛簡
				done = true
			case msg, ok := <-msgsCh:
				if !ok {
					cancel()
					// channel 琚叧闂簡
					return nil
				}
				msgs = append(msgs, msg)
				var t T
				err := json.Unmarshal(msg.Value, &t)
				if err != nil {
					// 娑堟伅鏍煎紡閮戒笉瀵癸紝娌″暐濂藉鐞嗙殑
					// 浣嗘槸涔熶笉鑳界洿鎺ヨ繑鍥烇紝鍦ㄧ嚎涓婄殑鏃跺€欒缁х画澶勭悊涓嬪幓
					h.l.Error("鍙嶅簭鍒楀寲娑堟伅浣撳け璐?,
						logger.String("topic", msg.Topic),
						logger.Int32("partition", msg.Partition),
						logger.Int64("offset", msg.Offset),
						// 杩欓噷涔熷彲浠ヨ€冭檻鎵撳嵃 msg.Value锛屼絾鏄湁浜涙椂鍊?msg 鏈韩涔熷寘鍚晱鎰熸暟鎹?
						logger.Error(err))
					// 涓嶄腑鏂紝缁х画涓嬩竴涓?
					session.MarkMessage(msg, "")
					continue
				}
				ts = append(ts, t)
			}
		}
		err := h.fn(msgs, ts)
		if err == nil {
			// 杩欒竟灏辫閮芥彁浜や簡
			for _, msg := range msgs {
				session.MarkMessage(msg, "")
			}
		} else {
			// 杩欓噷鍙互鑰冭檻閲嶈瘯锛屼篃鍙互鍦ㄥ叿浣撶殑涓氬姟閫昏緫閲岄潰閲嶈瘯
			// 涔熷氨鏄?eg.Go 閲岄潰閲嶈瘯
		}
		cancel()
	}
}

// 寮傛娑堣垂+鎵归噺鎺ュ彛瀹炵幇锛岀粡鍏哥殑閿欒锛屾爣鍑嗙殑0鍒?
func (b *BatchHandler[T]) ConsumeClaimFalse(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	msgsCh := claim.Messages()
	sem := make(chan struct{}, b.maxConcurrency)

	for {
		// 鎵归噺娑堟伅鏁版嵁锛岃鏈変釜瓒呮椂 context 锛岄伩鍏嶄竴鐩寸瓑寰呭噾榻愪竴鎵规秷鎭?
		ctx, cancel := context.WithTimeout(context.Background(), b.batchDuration)
		// 鏈秴鏃?
		done := false
		// 鍒濆鍖栨秷鎭垏鐗?
		msgs := make([]*sarama.ConsumerMessage, 0, b.batchSize)
		ts := make([]T, 0, b.batchSize)
		for i := 0; i < b.batchSize && !done; i++ {
			select {
			case <-ctx.Done():
				done = true
			case msg, ok := <-msgsCh:
				// 閫氶亾鍏抽棴浜?閫€鍑烘秷璐?
				if !ok {
					cancel()
					return nil
				}
				var t T
				err := json.Unmarshal(msg.Value, &t)
				if err != nil {
					b.l.Error("鍙嶅簭鍒楀寲澶辫触",
						logger.Error(err),
						logger.String("topic", msg.Topic),
						logger.Int64("partition", int64(msg.Partition)),
						logger.Int64("offset", msg.Offset))
					// 鍚庨潰涓嶆墽琛屼簡锛岃烦鍒颁笅涓€鏉℃秷鎭?
					continue
				}
				msgs = append(msgs, msg)
				ts = append(ts, t)
			}
		}
		// 涓€鎵规暟鎹嬁瀹屼簡
		// 鎵归噺娑堣垂
		cancel()
		// 涓€涓秷鎭兘娌℃嬁鍒帮紝涓嶈兘鎵ц娑堣€梖n,缁х画寰幆绛夋秷鎭惂
		if len(msgs) == 0 {
			continue
		}
		// 鎺у埗骞跺彂鏁?
		sem <- struct{}{}
		go func(msgs []*sarama.ConsumerMessage, ts []T) {
			defer func() { <-sem }()
			err := b.fn(msgs, ts)
			if err != nil {
				b.l.Error("璋冪敤涓氬姟鎵归噺鎺ュ彛澶辫触",
					logger.Error(err))
				// 浣犺繖閲屾暣涓壒娆￠兘瑕佽涓嬫潵

				// 杩樿缁х画寰€鍓嶆秷璐?
				return
			}
			for _, msg := range msgs {
				// 杩欐牱锛屼竾鏃犱竴澶?
				session.MarkMessage(msg, "")
			}
		}(msgs, ts)
	}
}


