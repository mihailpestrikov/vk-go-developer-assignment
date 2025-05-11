package subpub

import (
	"context"
	"sync"
	"sync/atomic"
)

type MessageHandler func(msg interface{})

type Subscription interface {
	Unsubscribe()
}

type SubPub interface {
	Subscribe(subject string, cb MessageHandler) (Subscription, error)
	Publish(subject string, msg interface{}) error
	Close(ctx context.Context) error
}

type DropStrategy int

const (
	// DropNone - не пропускать сообщения (блокировать публикацию)
	DropNone DropStrategy = iota

	// DropOldest - пропускать самые старые сообщения для освобождения места новым
	DropOldest

	// DropNewest - пропускать новые сообщения, если очередь полна
	DropNewest
)

type subPub struct {
	subjects sync.Map
	closed   bool
	mu       sync.RWMutex

	workerPool *WorkerPool

	bufferSize   int
	dropStrategy DropStrategy
}

type SubPubOption func(*subPub)

func WithBufferSize(size int) SubPubOption {
	return func(sp *subPub) {
		sp.bufferSize = size
	}
}

func WithDropStrategy(strategy DropStrategy) SubPubOption {
	return func(sp *subPub) {
		sp.dropStrategy = strategy
	}
}

func WithWorkerPool(size int) SubPubOption {
	return func(sp *subPub) {
		sp.workerPool = NewWorkerPool(size)
	}
}

func NewSubPub(opts ...SubPubOption) SubPub {
	sp := &subPub{
		bufferSize:   100,
		dropStrategy: DropNewest,
	}

	for _, opt := range opts {
		opt(sp)
	}

	if sp.workerPool == nil {
		sp.workerPool = NewWorkerPool(10)
	}

	sp.workerPool.Start()

	return sp
}

func (sp *subPub) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	sp.mu.RLock()
	if sp.closed {
		sp.mu.RUnlock()
		return nil, ErrSubPubClosed
	}
	sp.mu.RUnlock()

	sub := newSubscription(subject, cb, sp, sp.bufferSize, sp.dropStrategy)

	value, _ := sp.subjects.LoadOrStore(subject, &sync.Map{})
	subscribers := value.(*sync.Map)

	subscribers.Store(sub, struct{}{})

	return sub, nil
}

func (sp *subPub) Publish(subject string, msg interface{}) error {
	sp.mu.RLock()
	if sp.closed {
		sp.mu.RUnlock()
		return ErrSubPubClosed
	}
	sp.mu.RUnlock()

	value, ok := sp.subjects.Load(subject)
	if !ok {
		return nil
	}

	subscribers := value.(*sync.Map)

	var deliveryCount int32 = 0

	subscribers.Range(func(key, _ interface{}) bool {
		sub := key.(*subscription)
		if sub.IsActive() {
			sub.SendMessage(msg)
			atomic.AddInt32(&deliveryCount, 1)
		}
		return true
	})

	if atomic.LoadInt32(&deliveryCount) == 0 {
		return ErrPublishFailed
	}

	return nil
}

func (sp *subPub) Close(ctx context.Context) error {
	sp.mu.Lock()
	if sp.closed {
		sp.mu.Unlock()
		return nil
	}

	sp.closed = true
	sp.mu.Unlock()

	var wg sync.WaitGroup

	sp.subjects.Range(func(subject, value interface{}) bool {
		subscribers := value.(*sync.Map)

		subscribers.Range(func(key, _ interface{}) bool {
			sub := key.(*subscription)
			wg.Add(1)
			go func() {
				defer wg.Done()
				sub.Unsubscribe()
			}()
			return true
		})

		return true
	})

	sp.workerPool.Stop()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (sp *subPub) removeSubscription(subject string, sub *subscription) {
	value, ok := sp.subjects.Load(subject)
	if !ok {
		return
	}

	subscribers := value.(*sync.Map)
	subscribers.Delete(sub)

	// Если подписчиков больше нет, удаляем тему
	empty := true
	subscribers.Range(func(_, _ interface{}) bool {
		empty = false
		return false
	})

	if empty {
		sp.subjects.Delete(subject)
	}
}

func (sp *subPub) Submit(task func()) {
	sp.workerPool.Submit(task)
}

func (sp *subPub) RemoveSubscription(subject string, sub Subscription) {
	if s, ok := sub.(*subscription); ok {
		sp.removeSubscription(subject, s)
	}
}
