package mq

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/require"
)

func TestPartitionCommitWindowMarkDoneInOrder(t *testing.T) {
	window := newPartitionCommitWindow(TopicSeckillCreateOrder, 2)
	session := &recordingSession{ctx: context.Background()}

	window.observe(10)
	window.markDone(session, 11)
	require.Empty(t, session.markedOffsets)

	window.markDone(session, 10)
	require.Equal(t, []recordedOffset{{
		topic:     TopicSeckillCreateOrder,
		partition: 2,
		offset:    12,
	}}, session.markedOffsets)

	window.markDone(session, 12)
	require.Equal(t, []recordedOffset{
		{
			topic:     TopicSeckillCreateOrder,
			partition: 2,
			offset:    12,
		},
		{
			topic:     TopicSeckillCreateOrder,
			partition: 2,
			offset:    13,
		},
	}, session.markedOffsets)
}

type recordedOffset struct {
	topic     string
	partition int32
	offset    int64
}

type recordingSession struct {
	ctx           context.Context
	markedOffsets []recordedOffset
}

func (s *recordingSession) Claims() map[string][]int32 {
	return nil
}

func (s *recordingSession) MemberID() string {
	return ""
}

func (s *recordingSession) GenerationID() int32 {
	return 0
}

func (s *recordingSession) MarkOffset(topic string, partition int32, offset int64, _ string) {
	s.markedOffsets = append(s.markedOffsets, recordedOffset{
		topic:     topic,
		partition: partition,
		offset:    offset,
	})
}

func (s *recordingSession) Commit() {}

func (s *recordingSession) ResetOffset(string, int32, int64, string) {}

func (s *recordingSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {}

func (s *recordingSession) Context() context.Context {
	return s.ctx
}

var _ sarama.ConsumerGroupSession = (*recordingSession)(nil)


