package activityconsumer

import (
	"context"
	"log"
	"sync"
	"time"
)

func ExampleConsumer() {
	var wg sync.WaitGroup
	wg.Add(1)

	cfg := Config{
		PoolSize:               16,
		PerActivityLimit:       8,
		PerActivityMailboxSize: 32,
		ActivityTTL:            time.Minute,
		GCInterval:             10 * time.Second,
		ShardCount:             32,
		OnProcessError: func(ctx context.Context, msg Message, err error) {
			log.Printf("process failed activity=%d request=%s err=%v", msg.ActivityID, msg.RequestID, err)
		},
		OnSubmitError: func(ctx context.Context, msg Message, err error) {
			log.Printf("submit failed activity=%d request=%s err=%v", msg.ActivityID, msg.RequestID, err)
		},
	}

	consumer, err := NewConsumer(cfg, func(ctx context.Context, msg Message) error {
		defer wg.Done()
		// Do the real business work here, for example:
		// 1. Update seckill request state.
		// 2. Deduct DB stock.
		// 3. Create the order.
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	err = consumer.HandleMessage(context.Background(), Message{
		ActivityID: 10001,
		RequestID:  "req-1",
		Payload:    map[string]any{"user_id": 42},
	})
	if err != nil {
		log.Fatal(err)
	}

	wg.Wait()
}
