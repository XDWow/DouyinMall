package activityconsumer

import "time"

func (c *Consumer) StartGC() {
	if c.closed.Load() {
		return
	}

	c.gcMu.Lock()
	if c.gcRunning {
		c.gcMu.Unlock()
		return
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	c.gcStopCh = stopCh
	c.gcDoneCh = doneCh
	c.gcRunning = true
	c.gcMu.Unlock()

	go c.gcLoop(stopCh, doneCh)
}

func (c *Consumer) StopGC() {
	c.gcMu.Lock()
	if !c.gcRunning {
		c.gcMu.Unlock()
		return
	}
	stopCh := c.gcStopCh
	doneCh := c.gcDoneCh
	c.gcStopCh = nil
	c.gcDoneCh = nil
	c.gcRunning = false
	close(stopCh)
	c.gcMu.Unlock()

	<-doneCh
}

func (c *Consumer) gcLoop(stopCh <-chan struct{}, doneCh chan<- struct{}) {
	ticker := time.NewTicker(c.cfg.GCInterval)
	defer func() {
		ticker.Stop()
		close(doneCh)
	}()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			c.runGC(time.Now())
		}
	}
}

func (c *Consumer) runGC(now time.Time) {
	for i := range c.shards {
		shard := &c.shards[i]
		shard.mu.Lock()
		for activityID, state := range shard.states {
			state.mu.Lock()
			idle := state.isIdleLocked(now, c.cfg.ActivityTTL)
			if idle {
				if current, ok := shard.states[activityID]; ok && current == state {
					delete(shard.states, activityID)
					c.metrics.activeActivities.Add(-1)
				}
			}
			state.mu.Unlock()
		}
		shard.mu.Unlock()
	}
}
