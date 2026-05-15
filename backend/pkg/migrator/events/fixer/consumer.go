package fixer

import (
	"context"
	"errors"
	"time"

	"github.com/IBM/sarama"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/XDWow/DouyinMall/backend/pkg/migrator"
	"github.com/XDWow/DouyinMall/backend/pkg/migrator/events"
	"github.com/XDWow/DouyinMall/backend/pkg/migrator/fixer"
	"github.com/XDWow/DouyinMall/backend/pkg/saramax"
	"gorm.io/gorm"
)

type Consumer[T migrator.Entity] struct {
	client sarama.Client
	l      logger.LoggerV1
	// 娑堣垂閫昏緫: fix
	srcFirst *fixer.OverrideFixer[T]
	dstFirst *fixer.OverrideFixer[T]
	topic    string
}

func NewConsumer[T migrator.Entity](
	client sarama.Client,
	l logger.LoggerV1,
	src *gorm.DB,
	dst *gorm.DB,
	topic string) (*Consumer[T], error) {
	srcFirst, err := fixer.NewOverrideFixer[T](src, dst)
	if err != nil {
		return nil, err
	}
	dstFirst, err := fixer.NewOverrideFixer[T](dst, src)
	if err != nil {
		return nil, err
	}
	return &Consumer[T]{
		client:   client,
		l:        l,
		srcFirst: srcFirst,
		topic:    topic,
		dstFirst: dstFirst,
	}, err
}

// Start 灏辨槸鑷繁鍚姩 goroutine 浜?
func (c *Consumer[T]) Start() error {
	cg, err := sarama.NewConsumerGroupFromClient("migrator-fix", c.client)
	if err != nil {
		return err
	}
	go func() {
		err = cg.Consume(context.Background(),
			[]string{c.topic},
			saramax.NewHandler[events.InconsistentEvent](c.l, c.consume))
	}()
	return err
}

func (c *Consumer[T]) consume(msg *sarama.ConsumerMessage, t events.InconsistentEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	switch t.Direction {
	case "SRC":
		return c.srcFirst.Fix(ctx, t.ID)
	case "DST":
		return c.dstFirst.Fix(ctx, t.ID)
	default:
		return errors.New("unknown fix direction")
	}
}
