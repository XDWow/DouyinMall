package queue

import (
	"context"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestOrderDelayQueueEnqueueAndDrainDue(t *testing.T) {
	cache := newStubOrderCache()
	delayQueue := NewOrderDelayQueue(cache, logger.NewNopLogger())
	now := time.Now()

	err := delayQueue.Enqueue(context.Background(), 1001, now.Add(-time.Second))
	require.NoError(t, err)

	err = delayQueue.Enqueue(context.Background(), 1002, now.Add(time.Minute))
	require.NoError(t, err)

	orderIDs, err := delayQueue.DrainDue(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, []int64{1001}, orderIDs)

	orderIDs, err = delayQueue.DrainDue(context.Background(), now)
	require.NoError(t, err)
	require.Empty(t, orderIDs)
}

func TestOrderDelayQueueDrainDueClaimsInBatches(t *testing.T) {
	cache := newStubOrderCache()
	delayQueue := NewOrderDelayQueue(cache, logger.NewNopLogger())
	now := time.Now()

	for i := 0; i < claimBatchSize+5; i++ {
		err := delayQueue.Enqueue(context.Background(), int64(2000+i), now.Add(-time.Second))
		require.NoError(t, err)
	}

	orderIDs, err := delayQueue.DrainDue(context.Background(), now)
	require.NoError(t, err)
	require.Len(t, orderIDs, claimBatchSize+5)

	orderIDs, err = delayQueue.DrainDue(context.Background(), now)
	require.NoError(t, err)
	require.Empty(t, orderIDs)
}

type stubOrderCache struct {
	scores map[string]map[string]float64
}

func newStubOrderCache() *stubOrderCache {
	return &stubOrderCache{
		scores: make(map[string]map[string]float64),
	}
}

func (s *stubOrderCache) Set(context.Context, string, any, time.Duration) error {
	return nil
}

func (s *stubOrderCache) Get(context.Context, string) (string, error) {
	return "", nil
}

func (s *stubOrderCache) MGet(context.Context, ...string) ([]*string, error) {
	return nil, nil
}

func (s *stubOrderCache) Del(context.Context, ...string) error {
	return nil
}

func (s *stubOrderCache) ZAdd(_ context.Context, key string, members map[string]float64, _ time.Duration) error {
	if _, ok := s.scores[key]; !ok {
		s.scores[key] = make(map[string]float64)
	}
	for member, score := range members {
		s.scores[key][member] = score
	}
	return nil
}

func (s *stubOrderCache) ZAddWithLimit(context.Context, string, map[string]float64, int64, time.Duration) error {
	return nil
}

func (s *stubOrderCache) ZRange(context.Context, string, int64, int64, bool) ([]string, error) {
	return nil, nil
}

func (s *stubOrderCache) ZRangeByScore(_ context.Context, key, _ string, max string, limit int64) ([]string, error) {
	maxScore, err := parseOrderID(max)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, limit)
	for member, score := range s.scores[key] {
		if int64(score) <= maxScore {
			result = append(result, member)
		}
	}
	if limit > 0 && int64(len(result)) > limit {
		return result[:limit], nil
	}
	return result, nil
}

func (s *stubOrderCache) ZClaimByScore(_ context.Context, key, max string, limit int64) ([]string, error) {
	members, err := s.ZRangeByScore(context.Background(), key, "-inf", max, limit)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		delete(s.scores[key], member)
	}
	return members, nil
}

func (s *stubOrderCache) ZRem(_ context.Context, key string, members ...string) error {
	for _, member := range members {
		delete(s.scores[key], member)
	}
	return nil
}


