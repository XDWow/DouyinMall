package pool

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"
	"time"

	keyedsemaphore "github.com/MonsieurTib/keyed-semaphore"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/panjf2000/ants/v2"
)

const (
	defaultGlobalWorkerNum  = 32
	defaultPerActivityLimit = 8
)

var (
	ErrActivityBusy = errors.New("activity busy")
	ErrNilTask      = errors.New("nil task")
	ErrPoolClosed   = errors.New("pool closed")
)

type KeyedConsumerPoolOptions struct {
	GlobalWorkerNum        int
	PerActivityConcurrency int
	TaskTimeout            time.Duration
}

func (o KeyedConsumerPoolOptions) withDefaults() KeyedConsumerPoolOptions {
	if o.GlobalWorkerNum <= 0 {
		o.GlobalWorkerNum = defaultGlobalWorkerNum
	}
	if o.PerActivityConcurrency <= 0 {
		o.PerActivityConcurrency = defaultPerActivityLimit
	}
	return o
}

type KeyedConsumerPool struct {
	pool *ants.Pool
	sem  *keyedsemaphore.KeyedSemaphore[string]
	log  logger.LoggerV1
	mu   sync.Mutex
	wg   sync.WaitGroup

	taskTimeout time.Duration
	closed      bool
}

func NewKeyedConsumerPool(options KeyedConsumerPoolOptions, l logger.LoggerV1) (*KeyedConsumerPool, error) {
	options = options.withDefaults()
	if l == nil {
		l = logger.NewNopLogger()
	}

	workerPool, err := ants.NewPool(options.GlobalWorkerNum, ants.WithNonblocking(true))
	if err != nil {
		return nil, err
	}

	return &KeyedConsumerPool{
		pool:        workerPool,
		sem:         keyedsemaphore.NewKeyedSemaphore[string](options.PerActivityConcurrency),
		log:         l,
		taskTimeout: options.TaskTimeout,
	}, nil
}

func (p *KeyedConsumerPool) Submit(ctx context.Context, activityKey string, task func(context.Context) error) error {
	if task == nil {
		return ErrNilTask
	}
	if ctx == nil {
		ctx = context.Background()
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrPoolClosed
	}

	ok := p.sem.TryWait(ctx, activityKey)
	if !ok {
		p.mu.Unlock()
		return ErrActivityBusy
	}

	p.wg.Add(1)
	p.mu.Unlock()
	err := p.pool.Submit(func() {
		defer func() {
			p.wg.Done()
			_ = p.sem.Release(activityKey)
		}()

		defer func() {
			if r := recover(); r != nil {
				p.log.Error("keyed consumer pool handler panic",
					logger.String("activityKey", activityKey),
					logger.Field{Key: "panic", Value: r},
					logger.Field{Key: "stack", Value: string(debug.Stack())},
				)
			}
		}()

		taskCtx := ctx
		cancel := func() {}
		if p.taskTimeout > 0 {
			taskCtx, cancel = context.WithTimeout(ctx, p.taskTimeout)
		}
		defer cancel()

		_ = task(taskCtx)
	})
	if err != nil {
		p.wg.Done()
		_ = p.sem.Release(activityKey)
		return err
	}

	return nil
}

func (p *KeyedConsumerPool) Close() {
	if p == nil || p.pool == nil {
		return
	}
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.pool.Release()
}

func (p *KeyedConsumerPool) CloseWithTimeout(timeout time.Duration) bool {
	if p == nil || p.pool == nil {
		return true
	}
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	defer p.pool.Release()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	if timeout <= 0 {
		<-done
		return true
	}

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
