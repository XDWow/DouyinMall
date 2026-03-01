package events

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/internal/search/usecase"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
)

const topicSyncProduct = "sync_product_event"

type ProductConsumer struct {
	syncUC *usecase.SyncUseCase
	client sarama.Client
	l      logger.LoggerV1
}

func NewProductConsumer(client sarama.Client, l logger.LoggerV1, syncUC *usecase.SyncUseCase) *ProductConsumer {
	return &ProductConsumer{
		syncUC: syncUC,
		client: client,
		l:      l,
	}
}

func (p *ProductConsumer) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("search-product-consumer", p.client)
	if err != nil {
		return err
	}
	go func() {
		err := cg.Consume(context.Background(),
			[]string{topicSyncProduct},
			saramax.NewHandler(p.l, p.Consume))
		if err != nil {
			p.l.Error("退出了消费循环异常", logger.Error(err))
		}
	}()
	return nil
}

func (p *ProductConsumer) Consume(msg *sarama.ConsumerMessage, evt domain.SyncEvent) error {
	if evt.Type != domain.EventTypeProduct {
		return nil
	}
	ctx := context.Background()
	return p.syncUC.Sync(ctx, evt)
}
