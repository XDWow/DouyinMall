package activityconsumer

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/require"
)

func TestConsumerSingleActivityLimit(t *testing.T) {
	release := make(chan struct{})
	var running atomic.Int64
	var maxRunning atomic.Int64
	var processed atomic.Int64

	c, err := NewConsumer(testConfig(16, 8, 8), func(context.Context, Message) error {
		cur := running.Add(1)
		updateMax(&maxRunning, cur)
		<-release
		running.Add(-1)
		processed.Add(1)
		return nil
	})
	require.NoError(t, err)
	defer c.Close()

	for i := 0; i < 10; i++ {
		require.NoError(t, c.HandleMessage(context.Background(), Message{
			ActivityID: 1,
			RequestID:  strconv.Itoa(i),
		}))
	}

	require.Eventually(t, func() bool {
		return running.Load() == 8
	}, time.Second, 10*time.Millisecond)
	require.LessOrEqual(t, maxRunning.Load(), int64(8))

	close(release)
	require.Eventually(t, func() bool {
		return processed.Load() == 10 && c.TotalInflight() == 0
	}, 2*time.Second, 10*time.Millisecond)
}

func TestConsumerAllowsDifferentActivitiesToRunConcurrently(t *testing.T) {
	var running atomic.Int64
	var maxRunning atomic.Int64
	release := make(chan struct{})

	c, err := NewConsumer(testConfig(8, 1, 4), func(context.Context, Message) error {
		cur := running.Add(1)
		updateMax(&maxRunning, cur)
		<-release
		running.Add(-1)
		return nil
	})
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 101, RequestID: "a"}))
	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 102, RequestID: "b"}))
	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 103, RequestID: "c"}))

	require.Eventually(t, func() bool {
		return running.Load() == 3
	}, time.Second, 10*time.Millisecond)
	require.EqualValues(t, 3, maxRunning.Load())

	close(release)
	require.Eventually(t, func() bool {
		return c.TotalInflight() == 0
	}, time.Second, 10*time.Millisecond)
}

func TestConsumerDrainsMailboxOnTaskCompletion(t *testing.T) {
	started := make(chan string, 3)
	releaseOne := make(chan struct{})
	var finished atomic.Int64

	c, err := NewConsumer(testConfig(4, 1, 2), func(_ context.Context, msg Message) error {
		started <- msg.RequestID
		<-releaseOne
		finished.Add(1)
		return nil
	})
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 7, RequestID: "r1"}))
	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 7, RequestID: "r2"}))
	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 7, RequestID: "r3"}))

	require.Equal(t, "r1", waitStarted(t, started))
	require.Eventually(t, func() bool {
		n, ok := c.ActivityMailboxLen(7)
		return ok && n == 2
	}, time.Second, 10*time.Millisecond)

	releaseOne <- struct{}{}
	require.Equal(t, "r2", waitStarted(t, started))
	require.Eventually(t, func() bool {
		n, ok := c.ActivityMailboxLen(7)
		return ok && n == 1
	}, time.Second, 10*time.Millisecond)

	releaseOne <- struct{}{}
	require.Equal(t, "r3", waitStarted(t, started))
	require.Eventually(t, func() bool {
		n, ok := c.ActivityMailboxLen(7)
		return ok && n == 0
	}, time.Second, 10*time.Millisecond)

	releaseOne <- struct{}{}
	require.Eventually(t, func() bool {
		return finished.Load() == 3 && c.TotalInflight() == 0
	}, time.Second, 10*time.Millisecond)
}

func TestConsumerReturnsMailboxFull(t *testing.T) {
	release := make(chan struct{})
	c, err := NewConsumer(testConfig(2, 1, 1), func(context.Context, Message) error {
		<-release
		return nil
	})
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 9, RequestID: "r1"}))
	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 9, RequestID: "r2"}))
	err = c.HandleMessage(context.Background(), Message{ActivityID: 9, RequestID: "r3"})
	require.ErrorIs(t, err, ErrMailboxFull)

	stats := c.Stats()
	require.EqualValues(t, 1, stats.MailboxFullCount)

	close(release)
	require.Eventually(t, func() bool {
		return c.TotalInflight() == 0
	}, time.Second, 10*time.Millisecond)
}

func TestConsumerGCReclaimsIdleActivityState(t *testing.T) {
	cfg := testConfig(2, 1, 1)
	cfg.ActivityTTL = 30 * time.Millisecond
	cfg.GCInterval = 10 * time.Millisecond

	var processed atomic.Int64
	c, err := NewConsumer(cfg, func(context.Context, Message) error {
		processed.Add(1)
		return nil
	})
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 22, RequestID: "r1"}))
	require.Eventually(t, func() bool {
		return processed.Load() == 1
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return c.ActiveActivities() == 0
	}, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 22, RequestID: "r2"}))
	require.Eventually(t, func() bool {
		return processed.Load() == 2
	}, time.Second, 10*time.Millisecond)
}

func TestConsumerRollsBackInflightOnDirectSubmitFailure(t *testing.T) {
	release := make(chan struct{})
	var processed atomic.Int64

	c, err := NewConsumer(testConfig(1, 1, 0), func(context.Context, Message) error {
		<-release
		processed.Add(1)
		return nil
	})
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 1, RequestID: "hold"}))
	require.Eventually(t, func() bool {
		return c.TotalInflight() == 1
	}, time.Second, 10*time.Millisecond)

	err = c.HandleMessage(context.Background(), Message{ActivityID: 2, RequestID: "overload"})
	require.ErrorIs(t, err, ants.ErrPoolOverload)
	require.Equal(t, 1, c.TotalInflight())

	n, ok := c.ActivityMailboxLen(2)
	require.True(t, ok)
	require.Equal(t, 0, n)

	close(release)
	require.Eventually(t, func() bool {
		return processed.Load() == 1 && c.TotalInflight() == 0
	}, time.Second, 10*time.Millisecond)
}

func TestConsumerRequeuesMailboxTasksWhenSubmitFailsAfterCompletion(t *testing.T) {
	release := make(chan struct{})

	c, err := NewConsumer(testConfig(1, 1, 1), func(context.Context, Message) error {
		<-release
		return nil
	})
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 55, RequestID: "r1"}))
	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 55, RequestID: "r2"}))
	require.Eventually(t, func() bool {
		n, ok := c.ActivityMailboxLen(55)
		return ok && n == 1
	}, time.Second, 10*time.Millisecond)

	c.pool.Release()
	close(release)

	require.Eventually(t, func() bool {
		stats := c.Stats()
		n, ok := c.ActivityMailboxLen(55)
		return ok && n == 1 && stats.TotalInflight == 0 && stats.SubmitFailureCount >= 1
	}, time.Second, 10*time.Millisecond)
}

func TestConsumerInvokesProcessErrorHookAndKeepsScheduling(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	var hookCount atomic.Int64

	cfg := testConfig(2, 1, 1)
	cfg.OnProcessError = func(_ context.Context, msg Message, err error) {
		if msg.RequestID == "bad" && errors.Is(err, errBoom) {
			hookCount.Add(1)
		}
	}

	c, err := NewConsumer(cfg, func(_ context.Context, msg Message) error {
		started <- msg.RequestID
		<-release
		if msg.RequestID == "bad" {
			return errBoom
		}
		return nil
	})
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 88, RequestID: "bad"}))
	require.NoError(t, c.HandleMessage(context.Background(), Message{ActivityID: 88, RequestID: "good"}))

	require.Equal(t, "bad", waitStarted(t, started))
	release <- struct{}{}
	require.Equal(t, "good", waitStarted(t, started))
	release <- struct{}{}

	require.Eventually(t, func() bool {
		return hookCount.Load() == 1 && c.TotalInflight() == 0
	}, time.Second, 10*time.Millisecond)
}

var errBoom = errors.New("boom")

func testConfig(poolSize, perActivityLimit, mailboxSize int) Config {
	return Config{
		PoolSize:               poolSize,
		PerActivityLimit:       perActivityLimit,
		PerActivityMailboxSize: mailboxSize,
		ActivityTTL:            time.Minute,
		GCInterval:             time.Minute,
		ShardCount:             8,
	}
}

func updateMax(max *atomic.Int64, cur int64) {
	for {
		prev := max.Load()
		if cur <= prev || max.CompareAndSwap(prev, cur) {
			return
		}
	}
}

func waitStarted(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for task start")
		return ""
	}
}
