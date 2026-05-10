package activityconsumer

import (
	"context"
	"sync"
	"time"
)

type queuedTask struct {
	ctx context.Context
	msg Message
}

type ActivityState struct {
	mu        sync.Mutex
	inflight  int
	mailbox   []queuedTask
	lastTouch time.Time
}

func newActivityState(now time.Time, mailboxCap int) *ActivityState {
	return &ActivityState{
		mailbox:   make([]queuedTask, 0, mailboxCap),
		lastTouch: now,
	}
}

func (s *ActivityState) touchLocked(now time.Time) {
	s.lastTouch = now
}

func (s *ActivityState) mailboxLenLocked() int {
	return len(s.mailbox)
}

func (s *ActivityState) enqueueLocked(task queuedTask, now time.Time) {
	s.mailbox = append(s.mailbox, task)
	s.lastTouch = now
}

func (s *ActivityState) prependBatchLocked(tasks []queuedTask, now time.Time) {
	if len(tasks) == 0 {
		s.lastTouch = now
		return
	}
	mailbox := make([]queuedTask, 0, len(tasks)+len(s.mailbox))
	mailbox = append(mailbox, tasks...)
	mailbox = append(mailbox, s.mailbox...)
	s.mailbox = mailbox
	s.lastTouch = now
}

// drainMailboxLocked reserves as many mailbox tasks as the activity still has room for.
func (s *ActivityState) drainMailboxLocked(limit int, now time.Time) []queuedTask {
	room := limit - s.inflight
	if room <= 0 || len(s.mailbox) == 0 {
		s.lastTouch = now
		return nil
	}

	if room > len(s.mailbox) {
		room = len(s.mailbox)
	}

	tasks := make([]queuedTask, room)
	copy(tasks, s.mailbox[:room])
	copy(s.mailbox, s.mailbox[room:])
	s.mailbox = s.mailbox[:len(s.mailbox)-room]
	s.inflight += room
	s.lastTouch = now
	return tasks
}

func (s *ActivityState) isIdleLocked(now time.Time, ttl time.Duration) bool {
	return s.inflight == 0 && len(s.mailbox) == 0 && now.Sub(s.lastTouch) > ttl
}
