package pool

import (
	"context"
	"errors"
	"sync"
)

type GroupedTask struct {
	GroupID int64
	Task    interface{}
}

type TaskHandler func(ctx context.Context, groupID int64, task interface{}) error

type GroupedWorkerPool struct {
	ctx        context.Context
	cancel     context.CancelFunc
	numWorkers int
	handler    TaskHandler

	workerChs []chan GroupedTask
	wg        sync.WaitGroup
}

func NewGroupedWorkerPool(numWorkers, queueSize int, handler TaskHandler) *GroupedWorkerPool {
	if numWorkers <= 0 {
		numWorkers = 8
	}
	if queueSize <= 0 {
		queueSize = 1024
	}
	if handler == nil {
		panic("TaskHandler 为空")
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &GroupedWorkerPool{
		ctx:        ctx,
		cancel:     cancel,
		numWorkers: numWorkers,
		handler:    handler,
		workerChs:  make([]chan GroupedTask, numWorkers),
	}

	for i := 0; i < numWorkers; i++ {
		p.workerChs[i] = make(chan GroupedTask, queueSize)
		p.wg.Add(1)
		go p.worker(i)
	}

	return p
}

func (p *GroupedWorkerPool) Submit(task GroupedTask) error {
	select {
	case <-p.ctx.Done():
		return errors.New("协程池关闭")
	default:
	}

	workerID := int(task.GroupID % int64(p.numWorkers))
	if workerID < 0 {
		workerID = -workerID
	}

	select {
	case <-p.ctx.Done():
		return errors.New("协程池关闭")
	case p.workerChs[workerID] <- task:
		return nil
	}
}

func (p *GroupedWorkerPool) worker(workerID int) {
	defer p.wg.Done()

	ch := p.workerChs[workerID]
	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-ch:
			if !ok {
				return
			}
			_ = p.handler(p.ctx, task.GroupID, task.Task)
		}
	}
}

func (p *GroupedWorkerPool) Shutdown() {
	// 优雅关闭：先通知，再等待到所有协程真的退出
	p.cancel()
	p.wg.Wait()
}
