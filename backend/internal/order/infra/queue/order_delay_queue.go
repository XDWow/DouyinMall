package queue

import (
	"context"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/order/infra/cache"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

const (
	orderDelayQueueKey = "order:delay:queue"
	claimBatchSize     = 100
)

type OrderDelayQueue struct {
	cache cache.OrderCache
	log   logger.LoggerV1
}

func NewOrderDelayQueue(cache cache.OrderCache, log logger.LoggerV1) *OrderDelayQueue {
	return &OrderDelayQueue{
		cache: cache,
		log:   log,
	}
}

func (q *OrderDelayQueue) Enqueue(ctx context.Context, orderID int64, executeAt time.Time) error {
	return q.cache.ZAdd(ctx, orderDelayQueueKey, map[string]float64{
		formatOrderID(orderID): float64(executeAt.UnixMilli()),
	}, 0)
}

func (q *OrderDelayQueue) DrainDue(ctx context.Context, now time.Time) ([]int64, error) {
	deadline := strconv.FormatInt(now.UnixMilli(), 10)
	orderIDs := make([]int64, 0)

	for {
		members, err := q.cache.ZClaimByScore(ctx, orderDelayQueueKey, deadline, claimBatchSize)
		if err != nil {
			return nil, err
		}
		if len(members) == 0 {
			return orderIDs, nil
		}

		for _, member := range members {
			orderID, convErr := parseOrderID(member)
			if convErr != nil {
				q.log.Warn("璁㈠崟寤舵椂闃熷垪涓瓨鍦ㄩ潪娉曟垚鍛?,
					logger.String("member", member),
					logger.Error(convErr))
				continue
			}
			orderIDs = append(orderIDs, orderID)
		}

		if len(members) < claimBatchSize {
			return orderIDs, nil
		}
	}
}

func formatOrderID(orderID int64) string {
	return strconv.FormatInt(orderID, 10)
}

func parseOrderID(member string) (int64, error) {
	return strconv.ParseInt(member, 10, 64)
}


