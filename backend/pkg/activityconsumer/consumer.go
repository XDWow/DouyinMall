package activityconsumer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
)

type activityShard struct {
	mu     sync.Mutex
	states map[int64]*ActivityState
}

// Consumer limits per-activity concurrency inside a single process.
//
// Locking rule:
// 1. Find/create state under shard lock.
// 2. Acquire the activity state lock while shard lock is still held.
// 3. Release shard lock after state lock is held.
//
// No path is allowed to take state lock first and then shard lock.
type Consumer struct {
	cfg     Config
	process ProcessFunc
	pool    *ants.Pool
	shards  []activityShard

	closed    atomic.Bool
	closeOnce sync.Once

	gcMu      sync.Mutex
	gcStopCh  chan struct{}
	gcDoneCh  chan struct{}
	gcRunning bool

	metrics consumerMetrics
}

func NewConsumer(cfg Config, process ProcessFunc) (*Consumer, error) {
	if process == nil {
		return nil, ErrNilProcessFunc
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	pool, err := ants.NewPool(cfg.PoolSize, ants.WithNonblocking(true))
	if err != nil {
		return nil, fmt.Errorf("create ants pool: %w", err)
	}

	c := &Consumer{
		cfg:     cfg,
		process: process,
		pool:    pool,
		shards:  make([]activityShard, cfg.ShardCount),
	}
	for i := range c.shards {
		c.shards[i].states = make(map[int64]*ActivityState)
	}

	c.StartGC()
	return c, nil
}

func (c *Consumer) HandleMessage(ctx context.Context, msg Message) error {
	if c.closed.Load() {
		return ErrConsumerClosed
	}

	task := queuedTask{
		ctx: detachContext(ctx),
		msg: msg,
	}
	now := time.Now()
	state := c.lockOrCreateState(msg.ActivityID, now)

	switch {
	case state.inflight < c.cfg.PerActivityLimit:
		state.inflight++
		state.touchLocked(now)
		state.mu.Unlock()

		c.metrics.directDispatches.Add(1)
		c.metrics.totalInflight.Add(1)
		if err := c.submitDirect(msg.ActivityID, task); err != nil {
			return err
		}
		return nil
	case c.cfg.PerActivityMailboxSize == 0:
		state.touchLocked(now)
		state.mu.Unlock()
		c.metrics.activityBusyCount.Add(1)
		return ErrActivityBusy
	case state.mailboxLenLocked() >= c.cfg.PerActivityMailboxSize:
		state.touchLocked(now)
		state.mu.Unlock()
		c.metrics.mailboxFullCount.Add(1)
		return ErrMailboxFull
	default:
		state.enqueueLocked(task, now)
		state.mu.Unlock()

		c.metrics.mailboxEnqueues.Add(1)
		c.metrics.totalBuffered.Add(1)
		return nil
	}
}

func (c *Consumer) Close() error {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		c.StopGC()
		if c.pool != nil && !c.pool.IsClosed() {
			c.pool.Release()
		}
	})
	return nil
}

func (c *Consumer) submitDirect(activityID int64, task queuedTask) error {
	if err := c.pool.Submit(c.makeWorker(task)); err != nil {
		c.metrics.submitFailureCount.Add(1)
		if c.cfg.OnSubmitError != nil {
			c.cfg.OnSubmitError(task.ctx, task.msg, err)
		}
		c.rollbackDirectSubmit(activityID)
		return err
	}
	return nil
}

func (c *Consumer) submitMailboxBatch(activityID int64, tasks []queuedTask) {
	for i, task := range tasks {
		if err := c.pool.Submit(c.makeWorker(task)); err != nil {
			c.metrics.submitFailureCount.Add(1)
			if c.cfg.OnSubmitError != nil {
				c.cfg.OnSubmitError(task.ctx, task.msg, err)
			}
			c.rollbackMailboxBatch(activityID, tasks[i:])
			return
		}
	}
}

func (c *Consumer) rollbackDirectSubmit(activityID int64) {
	state := c.lockExistingState(activityID)
	if state.inflight > 0 {
		state.inflight--
	}
	state.touchLocked(time.Now())
	state.mu.Unlock()
	c.metrics.totalInflight.Add(-1)
}

// rollbackMailboxBatch puts mailbox-derived tasks back at the front of the mailbox.
//
// These tasks had already reserved inflight slots while the state lock was held in
// drainMailboxLocked. If ants refuses them, we must release those inflight slots and
// return the tasks to the mailbox so they are not lost.
func (c *Consumer) rollbackMailboxBatch(activityID int64, tasks []queuedTask) {
	if len(tasks) == 0 {
		return
	}

	state := c.lockExistingState(activityID)
	if state.inflight >= len(tasks) {
		state.inflight -= len(tasks)
	} else {
		state.inflight = 0
	}
	state.prependBatchLocked(tasks, time.Now())
	state.mu.Unlock()

	c.metrics.totalInflight.Add(-int64(len(tasks)))
	c.metrics.totalBuffered.Add(int64(len(tasks)))
}

func (c *Consumer) makeWorker(task queuedTask) func() {
	return func() {
		var procErr error

		defer func() {
			if r := recover(); r != nil {
				procErr = fmt.Errorf("process panic for activity=%d request=%s: %v", task.msg.ActivityID, task.msg.RequestID, r)
			}
			if procErr != nil {
				c.metrics.processErrorCount.Add(1)
				if c.cfg.OnProcessError != nil {
					c.cfg.OnProcessError(task.ctx, task.msg, procErr)
				}
			}
			c.onTaskDone(task.msg.ActivityID)
		}()

		procErr = c.process(task.ctx, task.msg)
	}
}

func (c *Consumer) onTaskDone(activityID int64) {
	state := c.lockExistingState(activityID)
	now := time.Now()

	if state.inflight > 0 {
		state.inflight--
	}
	state.touchLocked(now)
	next := state.drainMailboxLocked(c.cfg.PerActivityLimit, now)
	state.mu.Unlock()

	c.metrics.totalInflight.Add(-1)
	if len(next) == 0 {
		return
	}

	c.metrics.totalInflight.Add(int64(len(next)))
	c.metrics.totalBuffered.Add(-int64(len(next)))
	c.submitMailboxBatch(activityID, next)
}

func (c *Consumer) lockOrCreateState(activityID int64, now time.Time) *ActivityState {
	shard := c.shardFor(activityID)
	shard.mu.Lock()
	state, ok := shard.states[activityID]
	if !ok {
		state = newActivityState(now, c.cfg.PerActivityMailboxSize)
		shard.states[activityID] = state
		c.metrics.activeActivities.Add(1)
	}
	state.mu.Lock()
	shard.mu.Unlock()
	return state
}

func (c *Consumer) lockExistingState(activityID int64) *ActivityState {
	shard := c.shardFor(activityID)
	shard.mu.Lock()
	state, ok := shard.states[activityID]
	if !ok {
		shard.mu.Unlock()
		panic(fmt.Sprintf("activity state missing for inflight activity %d", activityID))
	}
	state.mu.Lock()
	shard.mu.Unlock()
	return state
}

func (c *Consumer) lockStateIfPresent(activityID int64) (*ActivityState, bool) {
	shard := c.shardFor(activityID)
	shard.mu.Lock()
	state, ok := shard.states[activityID]
	if !ok {
		shard.mu.Unlock()
		return nil, false
	}
	state.mu.Lock()
	shard.mu.Unlock()
	return state, true
}

func (c *Consumer) shardFor(activityID int64) *activityShard {
	idx := int(uint64(activityID) % uint64(len(c.shards)))
	return &c.shards[idx]
}

func detachContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}
