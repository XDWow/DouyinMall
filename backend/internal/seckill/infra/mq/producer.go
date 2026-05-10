package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	rocketmq "github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
)

const (
	transactionUserTTLProperty = "SECKILL_USER_TTL_SECONDS"
	transactionDecisionTTL     = 10 * time.Minute
)

type transactionProducer interface {
	SendMessageInTransaction(ctx context.Context, msg *primitive.Message) (*primitive.TransactionSendResult, error)
	Shutdown() error
}

type Producer struct {
	producer transactionProducer
	listener *TransactionListener
	log      logger.LoggerV1
}

type transactionDecision struct {
	state     primitive.LocalTransactionState
	result    *seckilldomain.Result
	expiresAt time.Time
}

type TransactionListener struct {
	cache   seckilldomain.Cache
	soldOut seckilldomain.SoldOutMarker
	log     logger.LoggerV1

	mu        sync.Mutex
	decisions map[string]transactionDecision
}

func NewTransactionListener(cache seckilldomain.Cache, soldOut seckilldomain.SoldOutMarker, logs ...logger.LoggerV1) *TransactionListener {
	l := logger.NewNopLogger()
	if len(logs) > 0 && logs[0] != nil {
		l = logs[0]
	}
	if soldOut == nil {
		soldOut = seckilldomain.NewNopSoldOutMarker()
	}
	return &TransactionListener{
		cache:     cache,
		soldOut:   soldOut,
		log:       l,
		decisions: make(map[string]transactionDecision),
	}
}

func NewProducer(producer transactionProducer, listener *TransactionListener, logs ...logger.LoggerV1) *Producer {
	l := logger.NewNopLogger()
	if len(logs) > 0 && logs[0] != nil {
		l = logs[0]
	}
	return &Producer{
		producer: producer,
		listener: listener,
		log:      l,
	}
}

func (p *Producer) Submit(ctx context.Context, evt seckilldomain.Event, userTTLSeconds int64) (*seckilldomain.Result, error) {
	data, err := json.Marshal(evt)
	if err != nil {
		return nil, fmt.Errorf("marshal seckill event failed: %w", err)
	}

	msg := primitive.NewMessage(TopicSeckillRequest, data)
	msg.WithKeys([]string{evt.RequestNo})
	msg.WithProperty(transactionUserTTLProperty, strconv.FormatInt(userTTLSeconds, 10))

	res, err := p.producer.SendMessageInTransaction(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("send seckill transaction message failed: %w", err)
	}

	processing := &seckilldomain.Result{
		RequestNo: evt.RequestNo,
		Status:    seckilldomain.RequestStatusProcessing,
	}

	switch res.State {
	case primitive.CommitMessageState:
		return processing, nil
	case primitive.RollbackMessageState:
		if rollback, ok := p.listener.rollbackResult(evt.RequestNo); ok {
			return rollback, errorForFailReason(rollback.FailReason)
		}
		p.log.Warn("transaction rolled back without explicit fail reason",
			logger.String("requestNo", evt.RequestNo),
			logger.Int64("activityID", evt.ActivityID),
			logger.Int64("userID", evt.UserID))
		return &seckilldomain.Result{
			RequestNo: evt.RequestNo,
			Status:    seckilldomain.RequestStatusFailed,
		}, fmt.Errorf("seckill transaction rolled back")
	case primitive.UnknowState:
		return processing, nil
	default:
		return processing, nil
	}
}

func (p *Producer) Stop() error {
	if p == nil || p.producer == nil {
		return nil
	}
	return p.producer.Shutdown()
}

func (l *TransactionListener) ExecuteLocalTransaction(msg *primitive.Message) primitive.LocalTransactionState {
	evt, userTTLSeconds, err := decodeTransactionMessage(msg.Body, msg.GetProperty(transactionUserTTLProperty))
	if err != nil {
		l.log.Error("decode seckill transaction message failed",
			logger.Error(err))
		return primitive.RollbackMessageState
	}

	code, err := l.cache.AtomicReserve(context.Background(), evt.ActivityID, evt.UserID, evt.RequestNo, userTTLSeconds)
	if err != nil {
		l.log.Error("seckill redis reserve failed, wait broker transaction check",
			logger.Error(err),
			logger.String("requestNo", evt.RequestNo),
			logger.Int64("activityID", evt.ActivityID),
			logger.Int64("userID", evt.UserID))
		return primitive.UnknowState
	}

	switch code {
	case 0:
		return primitive.CommitMessageState
	case 1:
		// Redis 已经明确告诉我们卖完了，本机打一个售罄标记，减少后续尾流量冲击 Redis
		l.soldOut.MarkSoldOut(evt.ActivityID)
		l.log.Info("redis reserve reports sold out, mark local sold-out flag",
			logger.String("requestNo", evt.RequestNo),
			logger.Int64("activityID", evt.ActivityID),
			logger.Int64("userID", evt.UserID))
		l.recordRollbackResult(evt.RequestNo, failedResult(evt.RequestNo, seckilldomain.FailReasonOutOfStock))
		return primitive.RollbackMessageState
	case 2:
		l.recordRollbackResult(evt.RequestNo, failedResult(evt.RequestNo, seckilldomain.FailReasonDuplicate))
		return primitive.RollbackMessageState
	default:
		l.log.Warn("unexpected seckill reserve code, wait broker transaction check",
			logger.String("requestNo", evt.RequestNo),
			logger.Int64("activityID", evt.ActivityID),
			logger.Int64("userID", evt.UserID),
			logger.Int64("code", code))
		return primitive.UnknowState
	}
}

func (l *TransactionListener) CheckLocalTransaction(msg *primitive.MessageExt) primitive.LocalTransactionState {
	evt, _, err := decodeTransactionMessage(msg.Body, msg.GetProperty(transactionUserTTLProperty))
	if err != nil {
		l.log.Error("decode seckill transaction check message failed",
			logger.Error(err))
		return primitive.RollbackMessageState
	}

	if decision, ok := l.lookupDecision(evt.RequestNo); ok {
		return decision.state
	}

	resolution, err := l.cache.ResolveTransaction(context.Background(), evt.ActivityID, evt.UserID, evt.RequestNo)
	if err != nil {
		l.log.Warn("resolve seckill transaction from redis failed",
			logger.Error(err),
			logger.String("requestNo", evt.RequestNo),
			logger.Int64("activityID", evt.ActivityID),
			logger.Int64("userID", evt.UserID))
		return primitive.UnknowState
	}

	switch resolution {
	case seckilldomain.TransactionResolutionCommit:
		return primitive.CommitMessageState
	case seckilldomain.TransactionResolutionRollback:
		return primitive.RollbackMessageState
	default:
		return primitive.UnknowState
	}
}

func (l *TransactionListener) rollbackResult(requestNo string) (*seckilldomain.Result, bool) {
	decision, ok := l.lookupDecision(requestNo)
	if !ok || decision.state != primitive.RollbackMessageState || decision.result == nil {
		return nil, false
	}
	copied := *decision.result
	return &copied, true
}

func (l *TransactionListener) recordRollbackResult(requestNo string, result seckilldomain.Result) {
	decision := transactionDecision{
		state:     primitive.RollbackMessageState,
		result:    &result,
		expiresAt: time.Now().Add(transactionDecisionTTL),
	}

	l.mu.Lock()
	l.cleanupExpiredLocked(time.Now())
	l.decisions[requestNo] = decision
	l.mu.Unlock()

	if err := l.cache.SetResult(context.Background(), result); err != nil {
		l.log.Warn("persist failed seckill transaction result to cache failed",
			logger.Error(err),
			logger.String("requestNo", result.RequestNo),
			logger.String("failReason", result.FailReason))
	}
}

func (l *TransactionListener) lookupDecision(requestNo string) (transactionDecision, bool) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanupExpiredLocked(now)

	decision, ok := l.decisions[requestNo]
	if !ok {
		return transactionDecision{}, false
	}
	return decision, true
}

func (l *TransactionListener) cleanupExpiredLocked(now time.Time) {
	for requestNo, decision := range l.decisions {
		if !decision.expiresAt.IsZero() && now.After(decision.expiresAt) {
			delete(l.decisions, requestNo)
		}
	}
}

func failedResult(requestNo, reason string) seckilldomain.Result {
	return seckilldomain.Result{
		RequestNo:  requestNo,
		Status:     seckilldomain.RequestStatusFailed,
		FailReason: reason,
	}
}

func errorForFailReason(reason string) error {
	switch reason {
	case seckilldomain.FailReasonOutOfStock:
		return seckilldomain.ErrOutOfStock
	case seckilldomain.FailReasonDuplicate:
		return seckilldomain.ErrDuplicateSeckill
	default:
		return fmt.Errorf("seckill transaction rolled back: %s", reason)
	}
}

func decodeTransactionMessage(body []byte, userTTL string) (seckilldomain.Event, int64, error) {
	var evt seckilldomain.Event
	if err := json.Unmarshal(body, &evt); err != nil {
		return seckilldomain.Event{}, 0, fmt.Errorf("unmarshal event: %w", err)
	}
	userTTLSeconds, err := strconv.ParseInt(userTTL, 10, 64)
	if err != nil || userTTLSeconds <= 0 {
		return seckilldomain.Event{}, 0, fmt.Errorf("parse user ttl seconds: %w", err)
	}
	return evt, userTTLSeconds, nil
}

// 编译期断言：确保生产者和事务监听器满足预期接口
var _ seckilldomain.Producer = (*Producer)(nil)
var _ primitive.TransactionListener = (*TransactionListener)(nil)
var _ transactionProducer = (rocketmq.TransactionProducer)(nil)
