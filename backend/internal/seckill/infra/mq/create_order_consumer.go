package mq

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
	orderdomain "github.com/XDWow/DouyinMall/backend/internal/order/domain"
	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/pool"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
)

const (
	seckillActivityWorkerCount     = 32
	seckillActivityWorkerQueueSize = 1024
	seckillPartitionMaxInFlight    = 256
	seckillCreateOrderRetryTimes   = 3
)

type SeckillConsumer struct {
	client           sarama.Client
	producer         sarama.SyncProducer
	orderClient      orderservice.Client
	requestRepo      seckilldomain.RequestRepository
	activityRepo     seckilldomain.ActivityRepository
	cache            seckilldomain.Cache
	logger           logger.LoggerV1
	activityTaskPool *pool.GroupedWorkerPool
	consumerGrp      sarama.ConsumerGroup
}

func NewSeckillConsumer(client sarama.Client, producer sarama.SyncProducer, orderClient orderservice.Client, requestRepo seckilldomain.RequestRepository, activityRepo seckilldomain.ActivityRepository, cache seckilldomain.Cache, l logger.LoggerV1) *SeckillConsumer {
	consumer := &SeckillConsumer{
		client:       client,
		producer:     producer,
		orderClient:  orderClient,
		requestRepo:  requestRepo,
		activityRepo: activityRepo,
		cache:        cache,
		logger:       l,
	}
	consumer.activityTaskPool = pool.NewGroupedWorkerPool(
		seckillActivityWorkerCount,
		seckillActivityWorkerQueueSize,
		consumer.handleCreateOrderTask,
	)
	return consumer
}

func (c *SeckillConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("seckill-create-order-consumer", c.client)
	if err != nil {
		return err
	}
	c.consumerGrp = cg

	go func() {
		for {
			err := cg.Consume(context.Background(), []string{TopicSeckillCreateOrder}, c)
			if err != nil {
				if errors.Is(err, sarama.ErrClosedConsumerGroup) {
					return
				}
				c.logger.Error("绉掓潃涓嬪崟娑堣垂鑰呴€€鍑?, logger.Error(err))
			}
		}
	}()
	return nil
}

func (c *SeckillConsumer) Stop() error {
	var firstErr error
	if c.consumerGrp != nil {
		err := c.consumerGrp.Close()
		if err != nil && !errors.Is(err, sarama.ErrClosedConsumerGroup) {
			firstErr = err
		}
	}
	if c.activityTaskPool != nil {
		c.activityTaskPool.Shutdown()
	}
	return firstErr
}

func (c *SeckillConsumer) Setup(_ sarama.ConsumerGroupSession) error {
	return nil
}

func (c *SeckillConsumer) Cleanup(_ sarama.ConsumerGroupSession) error {
	return nil
}

func (c *SeckillConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// Kafka 浼氭寜 partition 璋冪敤 ConsumeClaim銆?
	// 杩欓噷涓哄綋鍓?partition 鍒涘缓涓€涓笓灞炲鐞嗗櫒锛岃礋璐ｏ細
	// 1. 璇诲彇杩欎釜 partition 鐨勬秷鎭€?
	// 2. 鎸?activity_id 鍒嗗彂鍒板垎缁勫崗绋嬫睜銆?
	// 3. 鎺ユ敹 worker 澶勭悊缁撴灉銆?
	// 4. 鎸?offset 杩炵画鍖洪棿鎺ㄨ繘鎻愪氦銆?
	processor := newSeckillPartitionProcessor(c, session, claim)

	// 褰撳墠 partition 澶勭悊鍣ㄩ€€鍑烘椂锛岄€氱煡杩樻病鏉ュ緱鍙婂洖鍐欑粨鏋滅殑 worker 鍋滄鍥炲啓銆?
	defer close(processor.processorDone)
	return processor.run()
}

type createOrderTask struct {
	Message  *sarama.ConsumerMessage
	Event    seckilldomain.Event
	ResultCh chan<- createOrderResult
	ClaimCtx context.Context

	// ProcessorDone 浼氬湪褰撳墠 partition 澶勭悊鍣ㄩ€€鍑烘椂鍏抽棴銆?
	// worker 鍥炲啓缁撴灉鍓嶄細鍏堢湅瀹冿紝閬垮厤 partition 宸茬粨鏉熷悗杩樺線缁撴灉閫氶亾閲屽啓銆?
	ProcessorDone <-chan struct{}
}

type createOrderResult struct {
	// Message 淇濈暀 Kafka 鍘熷浣嶇偣锛屽悗闈㈠啓姝讳俊鍜屾彁浜?offset 閮借鐢?
	Message *sarama.ConsumerMessage
	Event   seckilldomain.Event

	// Err 鏄湰鍦伴噸璇曞悗鐨勬渶缁堢粨鏋?
	// nil 琛ㄧず澶勭悊鎴愬姛锛岄潪 nil 琛ㄧず瑕佽繘姝讳俊
	Err error
}

type seckillPartitionProcessor struct {
	consumer *SeckillConsumer
	session  sarama.ConsumerGroupSession
	claim    sarama.ConsumerGroupClaim

	messageCh <-chan *sarama.ConsumerMessage

	// taskResultCh 鍙厑璁稿綋鍓?partition 澶勭悊鍣ㄨ嚜宸辫
	// 杩欐牱 offset 鎻愪氦閫昏緫濮嬬粓鍙湪涓€涓?goroutine 鍐咃紝椤哄簭鏈€绋冲畾
	taskResultCh chan createOrderResult

	// commitWindow 璁板綍鍝簺 offset 宸插畬鎴愶紝浠ュ強褰撳墠鏈€澶氳兘鎻愪氦鍒板摢
	commitWindow *partitionCommitWindow

	// processorDone 鏄綋鍓?partition 澶勭悊鍣ㄧ殑閫€鍑轰俊鍙?
	processorDone chan struct{}

	// pendingTaskCount 琛ㄧず鈥滃凡缁忎氦缁?worker锛屼絾杩樻病澶勭悊瀹屸€濈殑浠诲姟鏁般€?
	// 瀹冧篃鐢ㄦ潵缁欏崟涓儹鐐?partition 鍋氳儗鍘?
	pendingTaskCount int

	// messageStreamClosed 琛ㄧず claim.Messages() 宸茶 Sarama 鍏抽棴銆?
	// 杩欒〃绀哄綋鍓嶈繖杞?claim 宸茬粡娌℃湁鏂版秷鎭細鍐嶈繘鏉?
	messageStreamClosed bool
}

func newSeckillPartitionProcessor(consumer *SeckillConsumer, session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) *seckillPartitionProcessor {
	return &seckillPartitionProcessor{
		consumer:      consumer,
		session:       session,
		claim:         claim,
		messageCh:     claim.Messages(),
		taskResultCh:  make(chan createOrderResult, seckillPartitionMaxInFlight),
		commitWindow:  newPartitionCommitWindow(claim.Topic(), claim.Partition()),
		processorDone: make(chan struct{}),
	}
}

func (p *seckillPartitionProcessor) run() error {
	for {
		if p.shouldExit() {
			return nil
		}

		select {
		case <-p.session.Context().Done():
			return nil
		case result := <-p.taskResultCh:
			if err := p.handleResult(result); err != nil {
				return err
			}
		case msg, ok := <-p.messageSelectCh():
			if !ok {
				// 褰撳墠杩欒疆 claim 宸茬粡娌℃湁鏂版秷鎭簡锛屼絾杩樿鎶婂凡鎶曞嚭鍘荤殑浠诲姟鏀跺畬
				p.messageStreamClosed = true
				continue
			}
			if err := p.handleMessage(msg); err != nil {
				return err
			}
		}
	}
}

func (p *seckillPartitionProcessor) shouldExit() bool {
	// 姝ｅ父閫€鍑哄彧渚濊禆鈥滄秷鎭祦缁撴潫鈥濊繖鏉¤矾寰勩€?
	// session 缁撴潫鍒欏湪 select 閲岀洿鎺ラ€€鍑恒€?
	return p.messageStreamClosed && p.pendingTaskCount == 0
}

// messageSelectCh 杩斿洖褰撳墠 select 搴旇鐩戝惉鐨勬秷鎭€氶亾銆?
// 褰撳墠 partition 涓嶈缁х画璇绘秷鎭椂锛岃繖閲岃繑鍥?nil锛岃 select 鏆傚仠璇绘秷鎭€?
func (p *seckillPartitionProcessor) messageSelectCh() <-chan *sarama.ConsumerMessage {
	// 杩欎笁绉嶆儏鍐甸兘涓嶅啀缁х画璇伙細
	// 1. session 宸茬粨鏉燂紝搴旇浼樺厛閫€鍑恒€?
	// 2. Sarama 宸插叧闂綋鍓?claim 鐨勬秷鎭祦銆?
	// 3. 褰撳墠 partition 鏈畬鎴愪换鍔″お澶氾紝鍏堝仛鑳屽帇銆?
	if p.session.Context().Err() != nil || p.messageStreamClosed || p.pendingTaskCount >= seckillPartitionMaxInFlight {
		return nil
	}
	return p.messageCh
}

func (p *seckillPartitionProcessor) handleMessage(msg *sarama.ConsumerMessage) error {
	// 鐢ㄥ綋鍓?partition 绗竴鏉＄湅鍒扮殑娑堟伅鍒濆鍖栨彁浜ょ獥鍙ｃ€?
	p.commitWindow.observe(msg.Offset)

	var evt seckilldomain.Event
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		p.consumer.logger.Error("绉掓潃娑堟伅鍙嶅簭鍒楀寲澶辫触",
			logger.Error(err),
			logger.String("topic", msg.Topic),
			logger.Int32("partition", msg.Partition),
			logger.Int64("offset", msg.Offset))
		p.commitWindow.markDone(p.session, msg.Offset)
		return nil
	}

	if err := p.consumer.submitCreateOrderTask(msg, evt, p.taskResultCh, p.session.Context(), p.processorDone); err != nil {
		return err
	}

	// 杩欓噷鍙〃绀轰换鍔″凡缁忔垚鍔熶氦缁?worker銆?
	// 鐪熸鑳戒笉鑳芥彁浜?offset锛岃绛?worker 澶勭悊缁撴潫鍚庡啀鐪嬨€?
	p.pendingTaskCount++
	return nil
}

func (p *seckillPartitionProcessor) handleResult(result createOrderResult) error {
	p.pendingTaskCount--

	if result.Err != nil {
		// 涓氬姟澶辫触涓嶈兘鎶婃暣涓?partition 姘歌繙鍗℃銆?
		// 鏈湴閲嶈瘯鑰楀敖鍚庤浆姝讳俊锛涘彧鏈夎繛姝讳俊閮藉彂涓嶅嚭鍘绘椂锛屾墠鎷掔粷鎻愪氦 offset銆?
		if err := p.consumer.sendCreateOrderDeadLetter(p.session.Context(), result); err != nil {
			p.consumer.logger.Error("绉掓潃涓嬪崟澶辫触娑堟伅鍙戦€佹淇″け璐?,
				logger.Error(err),
				logger.Int64("activityID", result.Event.ActivityID),
				logger.String("requestNo", result.Event.RequestNo),
				logger.Int64("offset", result.Message.Offset),
				logger.Int64("partition", int64(result.Message.Partition)))
			return err
		}
	}

	p.commitWindow.markDone(p.session, result.Message.Offset)
	return nil
}

type partitionCommitWindow struct {
	topic     string
	partition int32

	initialized bool

	// nextCommit 琛ㄧず Kafka 涓嬩竴娆″彲浠ヤ粠鍝釜 offset 寮€濮嬬户缁秷璐广€?
	// 渚嬪 10 鍜?11 閮藉畬鎴愬悗锛宯extCommit 浼氭帹杩涘埌 12銆?
	nextCommit int64

	// doneOffsets 璁板綍鈥滃凡缁忓畬鎴愶紝浣嗗洜涓哄墠闈㈣繕鏈夌┖娲烇紝鏆傛椂杩樹笉鑳芥彁浜も€濈殑 offset銆?
	doneOffsets map[int64]struct{}
}

func newPartitionCommitWindow(topic string, partition int32) *partitionCommitWindow {
	return &partitionCommitWindow{
		topic:       topic,
		partition:   partition,
		doneOffsets: make(map[int64]struct{}),
	}
}

func (w *partitionCommitWindow) observe(offset int64) {
	if w.initialized {
		return
	}
	w.initialized = true
	w.nextCommit = offset
}

func (w *partitionCommitWindow) markDone(session sarama.ConsumerGroupSession, offset int64) {
	if !w.initialized {
		w.observe(offset)
	}
	if offset < w.nextCommit {
		return
	}

	w.doneOffsets[offset] = struct{}{}

	advanced := false
	for {
		// 鍙湁褰㈡垚杩炵画瀹屾垚鍖洪棿锛屾彁浜ょ獥鍙ｆ墠鑳界户缁悜鍚庢帹杩涖€?
		if _, ok := w.doneOffsets[w.nextCommit]; !ok {
			break
		}
		delete(w.doneOffsets, w.nextCommit)
		w.nextCommit++
		advanced = true
	}
	if advanced {
		// Kafka 鎻愪氦鐨勬槸鈥滀笅涓€鏉″緟娑堣垂 offset鈥濓紝涓嶆槸鍒氬垰澶勭悊瀹岀殑閭ｄ竴鏉°€?
		session.MarkOffset(w.topic, w.partition, w.nextCommit, "")
	}
}

func (c *SeckillConsumer) handleCreateOrderTask(_ context.Context, _ int64, task interface{}) error {
	createTask, ok := task.(createOrderTask)
	if !ok {
		return errors.New("invalid seckill create order task")
	}

	// 鐪熸鐨勭鏉€涓氬姟閾捐矾鍦ㄨ繖閲屾墽琛岋細
	// 骞傜瓑妫€鏌?-> 鎵ｅ簱瀛?-> 鍒涘缓璁㈠崟 -> 鏍囪鎴愬姛锛涘け璐ユ椂璧拌ˉ鍋裤€?
	err := c.processCreateOrderWithRetry(createTask.Message, createTask.Event)
	select {
	case createTask.ResultCh <- createOrderResult{
		Message: createTask.Message,
		Event:   createTask.Event,
		Err:     err,
	}:
	case <-createTask.ClaimCtx.Done():
	case <-createTask.ProcessorDone:
	}
	return err
}

func (c *SeckillConsumer) submitCreateOrderTask(msg *sarama.ConsumerMessage, evt seckilldomain.Event, resultCh chan<- createOrderResult, claimCtx context.Context, processorDone <-chan struct{}) error {
	// 鎸?activity_id 璺敱鍒板垎缁勫崗绋嬫睜銆?
	// 鍚屼竴涓椿鍔ㄤ細钀藉埌鍚屼竴涓?worker锛屽湪鍗曟満鍐呬覆琛屾墽琛岋紱
	// 涓嶅悓娲诲姩鍙互钀藉埌涓嶅悓 worker锛屽苟琛屾墽琛屻€?
	return c.activityTaskPool.Submit(pool.GroupedTask{
		GroupID: evt.ActivityID,
		Task: createOrderTask{
			Message:       msg,
			Event:         evt,
			ResultCh:      resultCh,
			ClaimCtx:      claimCtx,
			ProcessorDone: processorDone,
		},
	})
}

func (c *SeckillConsumer) sendCreateOrderDeadLetter(ctx context.Context, result createOrderResult) error {
	if c.producer == nil {
		return result.Err
	}

	// 姝讳俊閲屼繚鐣欏師濮嬩簨浠跺拰 Kafka 浣嶇偣锛屾柟渚垮悗缁帓鏌ュ拰浜哄伐琛ュ崟銆?
	msg := seckillDeadLetterMessage{
		Event:           result.Event,
		Error:           result.Err.Error(),
		Attempts:        seckillCreateOrderRetryTimes,
		SourceTopic:     result.Message.Topic,
		SourcePartition: result.Message.Partition,
		SourceOffset:    result.Message.Offset,
		FailedAt:        time.Now(),
	}
	if err := publishSeckillDeadLetter(ctx, c.producer, msg); err != nil {
		return err
	}
	c.logger.Error("绉掓潃涓嬪崟澶勭悊澶辫触锛屽凡鎶曢€掓淇℃秷鎭?,
		logger.Int64("activityID", result.Event.ActivityID),
		logger.String("requestNo", result.Event.RequestNo),
		logger.Int("attempts", seckillCreateOrderRetryTimes),
		logger.String("topic", result.Message.Topic),
		logger.Error(result.Err))
	return nil
}

func (c *SeckillConsumer) processCreateOrderWithRetry(msg *sarama.ConsumerMessage, evt seckilldomain.Event) error {
	var err error
	for attempt := 1; attempt <= seckillCreateOrderRetryTimes; attempt++ {
		err = c.processCreateOrderEvent(evt)
		if err == nil {
			return nil
		}

		fields := []logger.Field{
			logger.Error(err),
			logger.Int64("activityID", evt.ActivityID),
			logger.String("requestNo", evt.RequestNo),
			logger.Int("attempt", attempt),
		}
		if msg != nil {
			fields = append(fields,
				logger.String("topic", msg.Topic),
				logger.Int32("partition", msg.Partition),
				logger.Int64("offset", msg.Offset))
		}
		if attempt < seckillCreateOrderRetryTimes {
			c.logger.Warn("绉掓潃涓嬪崟澶勭悊澶辫触锛屽噯澶囬噸璇?, fields...)
			continue
		}
		c.logger.Error("绉掓潃涓嬪崟澶勭悊澶辫触锛岃揪鍒伴噸璇曚笂闄?, fields...)
	}
	return err
}

func (c *SeckillConsumer) processCreateOrderEvent(evt seckilldomain.Event) error {
	_, canProcess, err := c.checkRequestIdempotency(context.Background(), evt)
	if err != nil {
		return err
	}
	if !canProcess {
		return nil
	}

	if err = c.activityRepo.DecreaseStock(context.Background(), evt.ActivityID, evt.RequestNo, evt.Quantity); err != nil {
		if errors.Is(err, seckilldomain.ErrOutOfStock) {
			return c.failAndCompensate(evt, seckilldomain.FailReasonOutOfStock, true)
		}
		return err
	}

	orderID, err := strconv.ParseInt(evt.RequestNo, 10, 64)
	if err != nil {
		_ = c.activityRepo.IncreaseStock(context.Background(), evt.ActivityID, "restore_"+evt.RequestNo+"_invalid_order", evt.Quantity)
		return c.failAndCompensate(evt, seckilldomain.FailReasonCreateOrderFail, true)
	}

	_, err = c.orderClient.CreateOrder(context.Background(), &orderv1.CreateOrderReq{
		OrderId:    orderID,
		UserId:     evt.UserID,
		Currency:   "CNY",
		OrderKind:  orderdomain.OrderKindSeckill,
		ActivityId: evt.ActivityID,
		Items: []*orderv1.OrderItem{{
			ProductId:        evt.ProductID,
			SkuId:            evt.SKUID,
			Quantity:         int64(evt.Quantity),
			SnapshotPrice:    evt.SeckillPrice,
			SnapshotCurrency: "CNY",
			ConvertedPrice:   evt.SeckillPrice,
		}},
	})
	if err != nil && !isDuplicate(err) {
		_ = c.activityRepo.IncreaseStock(context.Background(), evt.ActivityID, "restore_"+evt.RequestNo+"_create_order", evt.Quantity)
		return c.failAndCompensate(evt, seckilldomain.FailReasonCreateOrderFail, true)
	}

	if err = c.requestRepo.MarkSuccess(context.Background(), evt.RequestNo, orderID); err != nil {
		return err
	}
	return c.cache.SetResult(context.Background(), seckilldomain.Result{
		RequestNo: evt.RequestNo,
		Status:    seckilldomain.RequestStatusSuccess,
		OrderID:   orderID,
	})
}

// checkRequestIdempotency 鍋氱鏉€璇锋眰鐨勫箓绛夋鏌?
// 杩斿洖 true 琛ㄧず杩欐潯娑堟伅鍏佽缁х画鎵ц涓氬姟
// 杩斿洖 false 琛ㄧず宸茬粡琚箓绛夋尅鎺夛紝涓嶉渶瑕侀噸澶嶅鐞?
func (c *SeckillConsumer) checkRequestIdempotency(ctx context.Context, evt seckilldomain.Event) (*seckilldomain.Request, bool, error) {
	req, err := c.requestRepo.FindByRequestNo(ctx, evt.RequestNo)
	if err == nil {
		return req, req.Status == seckilldomain.RequestStatusProcessing, nil
	}
	if !errors.Is(err, seckilldomain.ErrRequestNotFound) {
		return nil, false, err
	}

	req = &seckilldomain.Request{
		RequestNo:  evt.RequestNo,
		ActivityID: evt.ActivityID,
		UserID:     evt.UserID,
		Quantity:   evt.Quantity,
		Status:     seckilldomain.RequestStatusProcessing,
	}
	if err = c.requestRepo.Create(ctx, req); err == nil {
		return req, true, nil
	}
	// 涓€浜轰竴鍗曞厹搴曟牎楠岋紝鎴戝湪 activity+uesrID 涓婂姞浜嗗敮涓€绱㈠紩
	if errors.Is(err, seckilldomain.ErrDuplicateSeckill) {
		// 鍛戒腑 request_no 鎴栤€滄椿鍔?+ 鐢ㄦ埛鈥濈殑鍞竴绾︽潫锛岄兘璇存槑杩欐璇锋眰搴旇骞傜瓑鎸℃帀銆?
		return nil, false, nil
	}
	return nil, false, err
}

func (c *SeckillConsumer) failAndCompensate(evt seckilldomain.Event, reason string, removeUser bool) error {
	if err := c.requestRepo.MarkFail(context.Background(), evt.RequestNo, reason); err != nil {
		return err
	}
	if err := c.cache.Compensate(context.Background(), evt.ActivityID, evt.UserID, evt.Quantity, removeUser); err != nil {
		return err
	}
	return c.cache.SetResult(context.Background(), seckilldomain.Result{
		RequestNo:  evt.RequestNo,
		Status:     seckilldomain.RequestStatusFail,
		FailReason: reason,
	})
}

func isDuplicate(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "1062") || strings.Contains(msg, "Duplicate entry")
}


