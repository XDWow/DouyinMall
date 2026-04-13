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
				c.logger.Error("秒杀下单消费者消费出错", logger.Error(err))
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
	// Kafka 按 partition 调用 ConsumeClaim。
	// 为当前 partition 创建专用处理器，负责：
	// 1. 读取本 partition 消息
	// 2. 按 activity_id 投递到分组协程池
	// 3. 接收 worker 处理结果
	// 4. 按 offset 连续区间推进提交
	processor := newSeckillPartitionProcessor(c, session, claim)

	// 当前 partition 处理器退出时，通知尚未回写结果的 worker 停止回写。
	defer close(processor.processorDone)
	return processor.run()
}

type createOrderTask struct {
	Message  *sarama.ConsumerMessage
	Event    seckilldomain.Event
	ResultCh chan<- createOrderResult
	ClaimCtx context.Context

	// ProcessorDone 在当前 partition 处理器退出时关闭
	// worker 回写结果前会先监听它，避免 partition 已结束后仍往结果通道写
	ProcessorDone <-chan struct{}
}

type createOrderResult struct {
	// Message 保留 Kafka 原始位点，后续写死信与提交 offset 都要用。
	Message *sarama.ConsumerMessage
	Event   seckilldomain.Event

	// Err 为本地重试后的最终结果：
	// nil 表示处理成功；非 nil 表示需要进死信。
	Err error
}

type seckillPartitionProcessor struct {
	consumer *SeckillConsumer
	session  sarama.ConsumerGroupSession
	claim    sarama.ConsumerGroupClaim

	messageCh <-chan *sarama.ConsumerMessage

	// taskResultCh 只允许当前 partition 处理器自己读
	// 这样 offset 提交逻辑始终只在一个 goroutine 内，顺序最稳定
	taskResultCh chan createOrderResult

	// commitWindow 记录哪些 offset 已完成，以及当前最多能提交到哪里
	commitWindow *partitionCommitWindow

	// processorDone 是当前 partition 处理器的退出信号
	processorDone chan struct{}

	// pendingTaskCount 表示「已交给 worker、但尚未处理完」的任务数
	// 也用于给单个热点 partition 做背压
	pendingTaskCount int

	// messageStreamClosed 表示 claim.Messages() 已被 Sarama 关闭
	// 即本轮 claim 不会再有新消息进来
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
				// 当前 claim 已无新消息，但仍需把已投递的任务收完
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
	// 正常退出只依赖「消息流结束」这条路径
	// session 结束则在 select 里直接 return
	return p.messageStreamClosed && p.pendingTaskCount == 0 // 优雅退出
}

// messageSelectCh 返回当前 select 应监听的消费通道。
// 当前 partition 不应继续读消息时返回 nil，让 select 暂停读消息。
func (p *seckillPartitionProcessor) messageSelectCh() <-chan *sarama.ConsumerMessage {
	// 以下三种情况不再继续读：
	// 1. session 已结束，应优先退出。
	// 2. Sarama 已关闭当前 claim 的消息流。
	// 3. 当前 partition 未完成任务过多，先做背压。
	if p.session.Context().Err() != nil || p.messageStreamClosed || p.pendingTaskCount >= seckillPartitionMaxInFlight {
		return nil
	}
	return p.messageCh
}

func (p *seckillPartitionProcessor) handleMessage(msg *sarama.ConsumerMessage) error {
	// 用当前 partition 第一条见到的消息初始化提交窗口
	p.commitWindow.observe(msg.Offset)

	var evt seckilldomain.Event
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		p.consumer.logger.Error("秒杀消息反序列化失败",
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

	// 这里只表示任务已成功交给 worker。
	// 能否提交 offset 要等 worker 处理结束后再看。
	p.pendingTaskCount++
	return nil
}

func (p *seckillPartitionProcessor) handleResult(result createOrderResult) error {
	p.pendingTaskCount--

	if result.Err != nil {
		// 业务失败不能把整条 partition 永远卡死
		// 本地重试耗尽后转死信；只有连死信都发不出去时，才拒绝提交 offset
		if err := p.consumer.sendCreateOrderDeadLetter(p.session.Context(), result); err != nil {
			p.consumer.logger.Error("秒杀下单失败消息发送死信失败",
				logger.Error(err),
				logger.Int64("activityID", result.Event.ActivityID),
				logger.String("requestNo", result.Event.RequestNo),
				logger.Int64("offset", result.Message.Offset),
				logger.Int64("partition", int64(result.Message.Partition)))
			return err
		}
	}
	// 忍忍吧，秒杀抢不到没关系
	p.commitWindow.markDone(p.session, result.Message.Offset)
	return nil
}

type partitionCommitWindow struct {
	topic     string
	partition int32

	initialized bool

	// nextCommit 表示 Kafka 下一次可以从哪个 offset 继续消费。
	// 例如 10 与 11 都完成后，nextCommit 会推进到 12。
	nextCommit int64

	// doneOffsets 记录「已完成，但因前面还有空洞，暂时还不能提交」的 offset。
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

	// 检查有没有形成连续区间，如果形成了，提交 offset
	advanced := false
	for {
		// 只有形成连续完成区间，提交窗口才能继续向后推进。
		if _, ok := w.doneOffsets[w.nextCommit]; !ok {
			break
		}
		delete(w.doneOffsets, w.nextCommit)
		w.nextCommit++
		advanced = true
	}
	if advanced {
		// Kafka 提交的是「下一条待消费 offset」，不是刚刚处理完的那一条。
		session.MarkOffset(w.topic, w.partition, w.nextCommit, "")
	}
}

// 处理 task 的任务
func (c *SeckillConsumer) handleCreateOrderTask(_ context.Context, _ int64, task interface{}) error {
	createTask, ok := task.(createOrderTask)
	if !ok {
		return errors.New("任务无效")
	}

	// 秒杀下单业务链路在此执行：
	// 幂等检查 -> 一人一单校验 -> 扣库存 -> 创建订单 -> 标记成功，失败时走补偿
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
	// 按 activity_id 路由到分组协程池
	// 同一活动落到同一 worker，在单机上串行执行
	// 不同活动可落到不同 worker，并行执行
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

	// 死信里保留原始事件与 Kafka 位点，便于后续排查与人工补单。
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
	c.logger.Error("秒杀下单处理失败，已投递死信消息",
		logger.Int64("activityID", result.Event.ActivityID),
		logger.String("requestNo", result.Event.RequestNo),
		logger.Int("attempts", seckillCreateOrderRetryTimes),
		logger.String("topic", result.Message.Topic),
		logger.Error(result.Err))
	return nil
}

// 带重试的创建秒杀订单
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
			c.logger.Warn("秒杀下单处理失败，准备重试", fields...)
			continue
		}
		c.logger.Error("秒杀下单处理失败，已达重试上限", fields...)
	}
	return err
}

// 真正核心逻辑
func (c *SeckillConsumer) processCreateOrderEvent(evt seckilldomain.Event) error {
	// 幂等性检查
	_, canProcess, err := c.checkRequestIdempotency(context.Background(), evt)
	if err != nil {
		return err
	}
	if !canProcess {
		return nil
	}

	// 一人一单：与扣活动库存在同一事务（TryDeductStockAndClaimSuccess → seckill_success 唯一 activity_id+user_id）
	if err = c.activityRepo.TryDeductStockAndClaimSuccess(context.Background(), evt.ActivityID, evt.UserID, evt.RequestNo, evt.Quantity); err != nil {
		if errors.Is(err, seckilldomain.ErrOutOfStock) {
			return c.failAndCompensate(evt, seckilldomain.FailReasonOutOfStock, true)
		}
		if errors.Is(err, seckilldomain.ErrSeckillSuccessAlreadyClaimed) {
			return c.failAndCompensate(evt, seckilldomain.FailReasonUserAlreadySucceeded, true)
		}
		return err
	}

	orderID, err := strconv.ParseInt(evt.RequestNo, 10, 64)
	// 回滚
	if err != nil {
		_ = c.activityRepo.IncreaseStock(context.Background(), evt.ActivityID, "restore_"+evt.RequestNo+"_invalid_order", evt.Quantity)
		_ = c.activityRepo.DeleteSuccessClaim(context.Background(), evt.ActivityID, evt.UserID)
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
	createErr := err
	if createErr != nil && !isDuplicate(createErr) {
		_ = c.activityRepo.IncreaseStock(context.Background(), evt.ActivityID, "restore_"+evt.RequestNo+"_create_order", evt.Quantity)
		_ = c.activityRepo.DeleteSuccessClaim(context.Background(), evt.ActivityID, evt.UserID)
		return c.failAndCompensate(evt, seckilldomain.FailReasonCreateOrderFail, true)
	}
	if createErr != nil && isDuplicate(createErr) {
		// 区分：① 本 order_id 已落库（MQ 重试）② 其它唯一冲突（不应在 seckill_success 已占位后仍发生）
		getResp, gerr := c.orderClient.GetOrder(context.Background(), &orderv1.GetOrderReq{OrderId: orderID})
		if gerr != nil || getResp.GetOrder() == nil {
			_ = c.activityRepo.IncreaseStock(context.Background(), evt.ActivityID, "restore_"+evt.RequestNo+"_seckill_order_dup", evt.Quantity)
			_ = c.activityRepo.DeleteSuccessClaim(context.Background(), evt.ActivityID, evt.UserID)
			return c.failAndCompensate(evt, seckilldomain.FailReasonUserAlreadySucceeded, true)
		}
		ord := getResp.GetOrder()
		if ord.GetUserId() != evt.UserID || ord.GetActivityId() != evt.ActivityID || ord.GetOrderKind() != orderdomain.OrderKindSeckill {
			_ = c.activityRepo.IncreaseStock(context.Background(), evt.ActivityID, "restore_"+evt.RequestNo+"_create_order_mismatch", evt.Quantity)
			_ = c.activityRepo.DeleteSuccessClaim(context.Background(), evt.ActivityID, evt.UserID)
			return c.failAndCompensate(evt, seckilldomain.FailReasonCreateOrderFail, true)
		}
	}

	if err = c.activityRepo.UpdateSuccessOrderID(context.Background(), evt.ActivityID, evt.UserID, orderID); err != nil {
		return err
	}
	// status 修改为拿到资格，前端轮询过来，发现订单创建好了，就拿这个 orderID 去调支付
	if err = c.requestRepo.MarkQualified(context.Background(), evt.RequestNo); err != nil {
		return err
	}
	return c.cache.SetResult(context.Background(), seckilldomain.Result{
		RequestNo: evt.RequestNo,
		Status:    seckilldomain.RequestStatusQualified,
		OrderID:   orderID,
	})
}

// checkRequestIdempotency 以 request_no 为粒度做幂等：与「本条 Kafka 消息所代表的一次提交」一一对应
// seckill_request 表仅保证 request_no 唯一，记录每次抢购尝试；「一人一单」由 seckill_success
// 表唯一 (activity_id,user_id) 与 TryDeductStockAndClaimSuccess 同事务写入（失败补偿后可新 request_no）
//
// 失败与重投：failAndCompensate 等路径会 MarkFail；创单成功路径 MarkQualified
// 之后 rebalance、broker 重投等再次收到同一消息时，canProcess=false
//
// 返回 true 表示允许继续执行业务；false 表示本条消息已被幂等丢弃
func (c *SeckillConsumer) checkRequestIdempotency(ctx context.Context, evt seckilldomain.Event) (*seckilldomain.Request, bool, error) {
	req, err := c.requestRepo.FindByRequestNo(ctx, evt.RequestNo)
	if err == nil {
		canProcess := req.Status == seckilldomain.RequestStatusProcessing
		return req, canProcess, nil
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
	// request_no 唯一：并发下同一消息的重复插入，丢弃
	if errors.Is(err, seckilldomain.ErrDuplicateSeckill) {
		return nil, false, nil
	}
	return nil, false, err
}

// 设置 request_no 状态为失败，并补偿恢复 redis
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
