package activityconsumer

import "sync/atomic"

type consumerMetrics struct {
	activeActivities   atomic.Int64
	totalInflight      atomic.Int64
	totalBuffered      atomic.Int64
	directDispatches   atomic.Int64
	mailboxEnqueues    atomic.Int64
	activityBusyCount  atomic.Int64
	mailboxFullCount   atomic.Int64
	submitFailureCount atomic.Int64
	processErrorCount  atomic.Int64
}

type Stats struct {
	ActiveActivities   int64
	TotalInflight      int64
	TotalBuffered      int64
	DirectDispatches   int64
	MailboxEnqueues    int64
	ActivityBusyCount  int64
	MailboxFullCount   int64
	SubmitFailureCount int64
	ProcessErrorCount  int64
	AntsCapacity       int
	AntsRunning        int
	AntsWaiting        int
}

func (c *Consumer) Stats() Stats {
	stats := Stats{
		ActiveActivities:   c.metrics.activeActivities.Load(),
		TotalInflight:      c.metrics.totalInflight.Load(),
		TotalBuffered:      c.metrics.totalBuffered.Load(),
		DirectDispatches:   c.metrics.directDispatches.Load(),
		MailboxEnqueues:    c.metrics.mailboxEnqueues.Load(),
		ActivityBusyCount:  c.metrics.activityBusyCount.Load(),
		MailboxFullCount:   c.metrics.mailboxFullCount.Load(),
		SubmitFailureCount: c.metrics.submitFailureCount.Load(),
		ProcessErrorCount:  c.metrics.processErrorCount.Load(),
	}
	if c.pool != nil {
		stats.AntsCapacity = c.pool.Cap()
		stats.AntsRunning = c.pool.Running()
		stats.AntsWaiting = c.pool.Waiting()
	}
	return stats
}

func (c *Consumer) ActiveActivities() int {
	return int(c.metrics.activeActivities.Load())
}

func (c *Consumer) TotalInflight() int {
	return int(c.metrics.totalInflight.Load())
}

func (c *Consumer) ActivityMailboxLen(activityID int64) (int, bool) {
	state, ok := c.lockStateIfPresent(activityID)
	if !ok {
		return 0, false
	}
	defer state.mu.Unlock()
	return state.mailboxLenLocked(), true
}
