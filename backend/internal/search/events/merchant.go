package events

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/internal/search/service"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
)

const topicSyncMerchant = "sync_merchant_event"

type MerchantConsumer struct {
	syncSvc service.SyncService
	client  sarama.Client
	l       logger.LoggerV1
}

func NewMerchantConsumer(client sarama.Client, l logger.LoggerV1, svc service.SyncService) *MerchantConsumer {
	return &MerchantConsumer{
		syncSvc: svc,
		client:  client,
		l:       l,
	}
}

func (m *MerchantConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("search-merchant-consumer", m.client)
	if err != nil {
		return err
	}
	go func() {
		err := cg.Consume(context.Background(),
			[]string{topicSyncMerchant},
			saramax.NewHandler(m.l, m.Consume))
		if err != nil {
			m.l.Error("退出了消费循环异常", logger.Error(err))
		}
	}()
	return nil
}

func (m *MerchantConsumer) Consume(msg *sarama.ConsumerMessage, evt domain.SyncEvent) error {
	if evt.Type != domain.EventTypeMerchant {
		return nil
	}
	ctx := context.Background()
	return m.syncSvc.Sync(ctx, evt)
}
