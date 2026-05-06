package pool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/require"
)

func TestKeyedConsumerPoolCapsPerActivityConcurrency(t *testing.T) {
	p, err := NewKeyedConsumerPool(KeyedConsumerPoolOptions{
		GlobalWorkerNum:        8,
		PerActivityConcurrency: 2,
	}, logger.NewNopLogger())
	require.NoError(t, err)
	defer p.Close()

	release := make(chan struct{})
	var running atomic.Int64
	var maxRunning atomic.Int64

	for i := 0; i < 2; i++ {
		err = p.Submit(context.Background(), "activity-1", func(context.Context) error {
			cur := running.Add(1)
			for {
				prev := maxRunning.Load()
				if cur <= prev || maxRunning.CompareAndSwap(prev, cur) {
					break
				}
			}
			defer running.Add(-1)
			<-release
			return nil
		})
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool { return running.Load() == 2 }, time.Second, 10*time.Millisecond)

	err = p.Submit(context.Background(), "activity-1", func(context.Context) error {
		t.Fatal("busy activity task should not be submitted")
		return nil
	})
	require.ErrorIs(t, err, ErrActivityBusy)
	require.EqualValues(t, 2, maxRunning.Load())

	close(release)
	require.Eventually(t, func() bool { return running.Load() == 0 }, time.Second, 10*time.Millisecond)
}

func TestKeyedConsumerPoolAllowsDifferentActivitiesToRunConcurrently(t *testing.T) {
	p, err := NewKeyedConsumerPool(KeyedConsumerPoolOptions{
		GlobalWorkerNum:        2,
		PerActivityConcurrency: 1,
	}, logger.NewNopLogger())
	require.NoError(t, err)
	defer p.Close()

	release := make(chan struct{})
	var running atomic.Int64
	var maxRunning atomic.Int64

	handler := func(context.Context) error {
		cur := running.Add(1)
		for {
			prev := maxRunning.Load()
			if cur <= prev || maxRunning.CompareAndSwap(prev, cur) {
				break
			}
		}
		defer running.Add(-1)
		<-release
		return nil
	}

	require.NoError(t, p.Submit(context.Background(), "activity-1", handler))
	require.NoError(t, p.Submit(context.Background(), "activity-2", handler))

	require.Eventually(t, func() bool { return running.Load() == 2 }, time.Second, 10*time.Millisecond)
	require.EqualValues(t, 2, maxRunning.Load())

	close(release)
	require.Eventually(t, func() bool { return running.Load() == 0 }, time.Second, 10*time.Millisecond)
}

func TestKeyedConsumerPoolDoesNotSubmitWhenActivityBusy(t *testing.T) {
	p, err := NewKeyedConsumerPool(KeyedConsumerPoolOptions{
		GlobalWorkerNum:        4,
		PerActivityConcurrency: 1,
	}, logger.NewNopLogger())
	require.NoError(t, err)
	defer p.Close()

	release := make(chan struct{})
	var started atomic.Int64

	require.NoError(t, p.Submit(context.Background(), "activity-1", func(context.Context) error {
		started.Add(1)
		<-release
		return nil
	}))

	require.Eventually(t, func() bool { return started.Load() == 1 }, time.Second, 10*time.Millisecond)

	err = p.Submit(context.Background(), "activity-1", func(context.Context) error {
		started.Add(1)
		return nil
	})
	require.ErrorIs(t, err, ErrActivityBusy)

	time.Sleep(50 * time.Millisecond)
	require.EqualValues(t, 1, started.Load())

	close(release)
}

func TestKeyedConsumerPoolReleasesPermitWhenAntsSubmitFails(t *testing.T) {
	p, err := NewKeyedConsumerPool(KeyedConsumerPoolOptions{
		GlobalWorkerNum:        1,
		PerActivityConcurrency: 1,
	}, logger.NewNopLogger())
	require.NoError(t, err)
	defer p.Close()

	release := make(chan struct{})
	done := make(chan struct{})
	require.NoError(t, p.Submit(context.Background(), "activity-1", func(context.Context) error {
		defer close(done)
		<-release
		return nil
	}))

	require.Eventually(t, func() bool { return p.pool.Running() == 1 }, time.Second, 10*time.Millisecond)

	err = p.Submit(context.Background(), "activity-2", func(context.Context) error {
		t.Fatal("task should not run when ants pool is overloaded")
		return nil
	})
	require.ErrorIs(t, err, ants.ErrPoolOverload)

	close(release)
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	called := make(chan struct{}, 1)
	require.NoError(t, p.Submit(context.Background(), "activity-2", func(context.Context) error {
		called <- struct{}{}
		return nil
	}))
	require.Eventually(t, func() bool { return len(called) == 1 }, time.Second, 10*time.Millisecond)
}

func TestKeyedConsumerPoolReleasesPermitWhenHandlerPanics(t *testing.T) {
	p, err := NewKeyedConsumerPool(KeyedConsumerPoolOptions{
		GlobalWorkerNum:        1,
		PerActivityConcurrency: 1,
	}, logger.NewNopLogger())
	require.NoError(t, err)
	defer p.Close()

	require.NoError(t, p.Submit(context.Background(), "activity-1", func(context.Context) error {
		panic("boom")
	}))

	require.Eventually(t, func() bool {
		return p.Submit(context.Background(), "activity-1", func(context.Context) error {
			return nil
		}) == nil
	}, time.Second, 10*time.Millisecond)
}

func TestKeyedConsumerPoolReturnsErrActivityBusyForCanceledTryWait(t *testing.T) {
	p, err := NewKeyedConsumerPool(KeyedConsumerPoolOptions{
		GlobalWorkerNum:        1,
		PerActivityConcurrency: 1,
	}, logger.NewNopLogger())
	require.NoError(t, err)
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = p.Submit(ctx, "activity-1", func(context.Context) error {
		return errors.New("should not run")
	})
	require.ErrorIs(t, err, ErrActivityBusy)
}

func TestKeyedConsumerPoolPassesTimedTaskContext(t *testing.T) {
	p, err := NewKeyedConsumerPool(KeyedConsumerPoolOptions{
		GlobalWorkerNum:        1,
		PerActivityConcurrency: 1,
		TaskTimeout:            20 * time.Millisecond,
	}, logger.NewNopLogger())
	require.NoError(t, err)
	defer p.Close()

	done := make(chan error, 1)
	err = p.Submit(context.Background(), "activity-1", func(ctx context.Context) error {
		<-ctx.Done()
		done <- ctx.Err()
		return ctx.Err()
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(done) == 1
	}, time.Second, 10*time.Millisecond)
	require.ErrorIs(t, <-done, context.DeadlineExceeded)
}

func TestKeyedConsumerPoolCloseWithTimeoutWaitsForInflightTasks(t *testing.T) {
	p, err := NewKeyedConsumerPool(KeyedConsumerPoolOptions{
		GlobalWorkerNum:        1,
		PerActivityConcurrency: 1,
	}, logger.NewNopLogger())
	require.NoError(t, err)

	release := make(chan struct{})
	started := make(chan struct{})
	err = p.Submit(context.Background(), "activity-1", func(context.Context) error {
		close(started)
		<-release
		return nil
	})
	require.NoError(t, err)
	<-started

	closed := make(chan bool, 1)
	go func() {
		closed <- p.CloseWithTimeout(time.Second)
	}()

	select {
	case <-closed:
		t.Fatal("pool should wait for inflight task")
	case <-time.After(30 * time.Millisecond):
	}

	close(release)
	require.Eventually(t, func() bool { return len(closed) == 1 }, time.Second, 10*time.Millisecond)
	require.True(t, <-closed)
}

func TestKeyedConsumerPoolCloseWithTimeoutReturnsFalseWhenTaskStuck(t *testing.T) {
	p, err := NewKeyedConsumerPool(KeyedConsumerPoolOptions{
		GlobalWorkerNum:        1,
		PerActivityConcurrency: 1,
	}, logger.NewNopLogger())
	require.NoError(t, err)

	release := make(chan struct{})
	started := make(chan struct{})
	err = p.Submit(context.Background(), "activity-1", func(context.Context) error {
		close(started)
		<-release
		return nil
	})
	require.NoError(t, err)
	<-started

	require.False(t, p.CloseWithTimeout(20*time.Millisecond))
	close(release)
}
