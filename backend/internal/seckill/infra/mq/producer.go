package mq

import (
	"context"
	"encoding/json"
	"fmt"

	seckilldomain "github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	rmq_client "github.com/apache/rocketmq-clients/golang"
)

type Producer struct {
	producer rmq_client.Producer
}

type transactionWrapper struct {
	tx rmq_client.Transaction
}

func NewProducer(producer rmq_client.Producer) *Producer {
	return &Producer{producer: producer}
}

func NewTransactionChecker(cache seckilldomain.Cache, l logger.LoggerV1) *rmq_client.TransactionChecker {
	return &rmq_client.TransactionChecker{
		Check: func(msg *rmq_client.MessageView) rmq_client.TransactionResolution {
			var evt seckilldomain.Event
			if err := json.Unmarshal(msg.GetBody(), &evt); err != nil {
				l.Error("transaction checker failed to decode seckill event", logger.Error(err))
				return rmq_client.ROLLBACK
			}

			resolution, err := cache.ResolveTransaction(context.Background(), evt.ActivityID, evt.UserID, evt.RequestNo)
			if err != nil {
				l.Warn("transaction checker resolve failed",
					logger.Error(err),
					logger.String("requestNo", evt.RequestNo))
				return rmq_client.UNKNOW
			}

			switch resolution {
			case seckilldomain.TransactionResolutionCommit:
				return rmq_client.COMMIT
			case seckilldomain.TransactionResolutionRollback:
				return rmq_client.ROLLBACK
			default:
				return rmq_client.UNKNOW
			}
		},
	}
}

func (p *Producer) Prepare(ctx context.Context, evt seckilldomain.Event) (seckilldomain.Transaction, error) {
	data, err := json.Marshal(evt)
	if err != nil {
		return nil, fmt.Errorf("marshal seckill event failed: %w", err)
	}

	msg := &rmq_client.Message{
		Topic: TopicSeckillRequest,
		Body:  data,
	}
	msg.SetKeys(evt.RequestNo)

	tx := p.producer.BeginTransaction()
	if _, err = p.producer.SendWithTransaction(ctx, msg, tx); err != nil {
		return nil, fmt.Errorf("send half seckill message failed: %w", err)
	}
	return &transactionWrapper{tx: tx}, nil
}

func (t *transactionWrapper) Commit() error {
	return t.tx.Commit()
}

func (t *transactionWrapper) Rollback() error {
	return t.tx.RollBack()
}

func (p *Producer) Stop() error {
	if p == nil || p.producer == nil {
		return nil
	}
	return p.producer.GracefulStop()
}
