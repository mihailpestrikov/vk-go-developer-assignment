package subpub

import (
	"sync"
)

type WorkerPool struct {
	tasks   chan func()
	wg      sync.WaitGroup
	quit    chan struct{}
	size    int
	started bool
	mu      sync.Mutex
}

func NewWorkerPool(size int) *WorkerPool {
	if size <= 0 {
		size = 2
	}

	return &WorkerPool{
		tasks: make(chan func(), 1000),
		quit:  make(chan struct{}),
		size:  size,
	}
}

func (wp *WorkerPool) Start() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.started {
		return
	}

	wp.started = true

	for i := 0; i < wp.size; i++ {
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()

			for {
				select {
				case task, ok := <-wp.tasks:
					if !ok {
						return
					}
					task()

				case <-wp.quit:
					return
				}
			}
		}()
	}
}

func (wp *WorkerPool) Submit(task func()) {
	select {
	case wp.tasks <- task:
	case <-wp.quit:
	}
}

func (wp *WorkerPool) Stop() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if !wp.started {
		return
	}

	close(wp.quit)
	close(wp.tasks)

	wp.wg.Wait()
	wp.started = false
}
