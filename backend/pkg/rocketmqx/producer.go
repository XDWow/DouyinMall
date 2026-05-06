package rocketmqx

import (
	"context"

	rmq_client "github.com/apache/rocketmq-clients/golang"
)

type Producer struct {
	producer rmq_client.Producer
}

func NewProducer(producer rmq_client.Producer) MessageProducer {
	return &Producer{producer: producer}
}

func (p *Producer) Send(ctx context.Context, msg *rmq_client.Message) error {
	_, err := p.producer.Send(ctx, msg)
	return err
}

func (p *Producer) SendBatch(ctx context.Context, msgs []*rmq_client.Message) []error {
	if len(msgs) == 0 {
		return nil
	}
	errs := make([]error, len(msgs))
	for i, msg := range msgs {
		if msg == nil {
			continue
		}
		_, err := p.producer.Send(ctx, msg)
		errs[i] = err
	}
	allNil := true
	for _, err := range errs {
		if err != nil {
			allNil = false
			break
		}
	}
	if allNil {
		return nil
	}
	return errs
}

func (p *Producer) GracefulStop() error {
	if p == nil || p.producer == nil {
		return nil
	}
	return p.producer.GracefulStop()
}
