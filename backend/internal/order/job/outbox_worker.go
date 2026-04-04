package job

import (
	"context"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/domain"
	"github.com/XDWow/DouyinMall/backend/internal/order/infra/mq"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// OutboxWorkerJob 瀹氭湡鎵弿 pending 鐨?outboxEvent锛岃繘琛屽彂閫?
// 杩欐槸鎱㈣矾寰勭殑鍏滃簳鏈哄埗锛岀‘淇濆嵆浣垮揩璺緞澶辫触锛屾秷鎭渶缁堜篃鑳借鍙戦€侊紙鏈€缁堟暟鎹竴鑷存€э級
// Outbox 灏辨槸璁板綍寰呭彂閫佹秷鎭€侀噸璇曟鏁板拰涓嬫閲嶈瘯鏃堕棿锛屽啀鐢卞畾鏃朵换鍔℃寔缁壂鎻忥紝鎶婅繕鑳介噸璇曠殑娑堟伅缁х画鍙戝嚭鍘汇€?
type OutboxWorkerJob struct {
	outboxRepo domain.OutboxRepository
	producer   mq.SaramaProducer
	l          logger.LoggerV1
	batchSize  int
	maxRetry   int
}

func NewOutboxWorkerJob(
	outboxRepo domain.OutboxRepository,
	producer mq.SaramaProducer,
	l logger.LoggerV1,
) *OutboxWorkerJob {
	return &OutboxWorkerJob{
		outboxRepo: outboxRepo,
		producer:   producer,
		l:          l,
		batchSize:  100,
		maxRetry:   5,
	}
}

func (j *OutboxWorkerJob) Name() string {
	return "OutboxWorkerJob"
}

// 绠€鍗曠偣锛屽厛涓嶈€冭檻鍒嗗竷寮忓畾鏃朵换鍔?
func (j *OutboxWorkerJob) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	offset := 0
	for {
		outboxEvents, err := j.outboxRepo.ListPending(ctx, offset, j.batchSize)
		if err != nil {
			j.l.Error("鏌ヨ寰呭彂閫?outbox 浜嬩欢澶辫触", logger.Error(err))
			return err
		}
		if len(outboxEvents) == 0 {
			break
		}
		j.l.Info("鍙戠幇寰呭彂閫?outbox 浜嬩欢", logger.Int("count", len(outboxEvents)))
		// 鐢熶骇鑰呮壒閲忓彂閫?
		j.processBatch(ctx, outboxEvents)

		// 濡傛灉鏈壒娆℃暟閲忓皬浜巄atchSize锛岃鏄庡凡缁忔槸鏈€鍚庝竴鎵?
		if len(outboxEvents) < j.batchSize {
			break
		}

		offset += j.batchSize
	}

	return nil
}

// 鎬ц兘浼樺寲鍦ㄥ彂閫佸眰锛氫娇鐢ㄦ壒閲忓彂閫丄PI
// 澶辫触闅旂锛氭瘡涓秷鎭嫭绔嬶紝绮剧‘澶勭悊澶辫触
func (j *OutboxWorkerJob) processBatch(ctx context.Context, outboxEvents []domain.OutboxEvent) {
	events := make([]domain.OrderStatusUpdateEvent, 0, len(outboxEvents))
	for _, outboxEvent := range outboxEvents {
		events = append(events, outboxEvent.Event)
	}

	errs := j.producer.SendMessages(ctx, events)

	successIDs := make([]int64, 0, len(outboxEvents))
	failedIDs := make([]int64, 0)

	// 澶勭悊缁撴灉锛氬け璐ヨ闅旂鑰屼笉鏄斁澶?
	if errs == nil {
		// 鍏ㄩ儴鎴愬姛
		for _, outboxEvent := range outboxEvents {
			successIDs = append(successIDs, outboxEvent.ID)
		}
	} else {
		// 閫愪釜妫€鏌ョ粨鏋?
		for i, err := range errs {
			if err != nil {
				j.l.Error("鍙戦€乷utbox浜嬩欢澶辫触",
					logger.Error(err),
					logger.Int64("outboxID", outboxEvents[i].ID),
					logger.Int64("orderID", outboxEvents[i].Event.OrderID))

				failedIDs = append(failedIDs, outboxEvents[i].ID)

				// 澧炲姞閲嶈瘯鍙戦€佺殑娆℃暟锛岃繖閲屾湁鍒嗘敮鍒ゆ柇锛堟爣璁板け璐?DLQ+鍛婅锛夛紝鎵€浠ヤ笉鐢ㄦ壒閲?
				retry, err := j.outboxRepo.IncreaseRetry(ctx, outboxEvents[i].ID)
				if err != nil {
					j.l.Error("澧炲姞outbox閲嶈瘯娆℃暟澶辫触",
						logger.Error(err),
						logger.Int64("outboxID", outboxEvents[i].ID))
				} else if retry > j.maxRetry {
					j.l.Warn("outbox浜嬩欢閲嶈瘯娆℃暟杈惧埌涓婇檺锛岄渶浜哄伐浠嬪叆澶勭悊",
						logger.Int64("outboxID", outboxEvents[i].ID),
						logger.Int64("orderID", outboxEvents[i].Event.OrderID),
						logger.Int("maxRetry", j.maxRetry))
					err = j.outboxRepo.MarkFailed(ctx, outboxEvents[i].ID)
					if err != nil {
						j.l.Error("鏍囪outbox浜嬩欢涓哄け璐ョ姸鎬佸け璐?,
							logger.Error(err),
							logger.Int64("outboxID", outboxEvents[i].ID))
					}
					// 鍚庣画鍏ユ淇￠槦鍒?鍛婅
				}

			} else {
				successIDs = append(successIDs, outboxEvents[i].ID)
			}
		}
	}

	// 鎵归噺鏍囪鎴愬姛鍙戦€佺殑浜嬩欢
	if len(successIDs) > 0 {
		if err := j.outboxRepo.BatchMarkSent(ctx, successIDs); err != nil {
			j.l.Error("鎵归噺鏍囪outbox涓哄凡鍙戦€佸け璐?,
				logger.Error(err),
				logger.Int("successCount", len(successIDs)))
		} else {
			j.l.Info("鎵归噺鍙戦€乷utbox浜嬩欢鎴愬姛",
				logger.Int("successCount", len(successIDs)))
		}
	}

	if len(failedIDs) > 0 {
		j.l.Warn("閮ㄥ垎outbox浜嬩欢鍙戦€佸け璐ワ紝灏嗗湪涓嬫閲嶈瘯",
			logger.Int("failedCount", len(failedIDs)))
	}
}


