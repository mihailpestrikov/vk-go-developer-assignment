package subpub

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewWorkerPool(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		wantSize int
	}{
		{
			name:     "Positive size",
			size:     5,
			wantSize: 5,
		},
		{
			name:     "Zero size",
			size:     0,
			wantSize: 2,
		},
		{
			name:     "Negative size",
			size:     -1,
			wantSize: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewWorkerPool(tt.size)
			if got.size != tt.wantSize {
				t.Errorf("NewWorkerPool() size = %v, want %v", got.size, tt.wantSize)
			}
			if got.tasks == nil {
				t.Error("NewWorkerPool() tasks channel is nil")
			}
			if got.quit == nil {
				t.Error("NewWorkerPool() quit channel is nil")
			}
		})
	}
}

func TestWorkerPool_Start_Stop(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{
			name: "Normal pool",
			size: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wp := NewWorkerPool(tt.size)

			if wp.started {
				t.Error("WorkerPool should not be started initially")
			}

			wp.Start()

			if !wp.started {
				t.Error("WorkerPool should be started after calling Start()")
			}

			wp.Start()

			wp.Stop()

			if wp.started {
				t.Error("WorkerPool should not be started after calling Stop()")
			}

			wp.Stop()
		})
	}
}

func TestWorkerPool_Submit(t *testing.T) {
	tests := []struct {
		name      string
		size      int
		taskCount int
	}{
		{
			name:      "Few tasks",
			size:      2,
			taskCount: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wp := NewWorkerPool(tt.size)
			wp.Start()
			defer wp.Stop()

			var counter int32
			var wg sync.WaitGroup
			wg.Add(tt.taskCount)

			for i := 0; i < tt.taskCount; i++ {
				wp.Submit(func() {
					atomic.AddInt32(&counter, 1)
					wg.Done()
				})
			}

			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:

			case <-time.After(time.Second):
				t.Fatal("Tasks execution timeout")
			}

			if atomic.LoadInt32(&counter) != int32(tt.taskCount) {
				t.Errorf("Expected %d tasks to be executed, got %d", tt.taskCount, counter)
			}
		})
	}
}

func TestWorkerPool_SubmitAfterStop(t *testing.T) {
	wp := NewWorkerPool(2)
	wp.Start()

	var taskExecuted atomic.Bool

	wp.Stop()

	wp.Submit(func() {
		taskExecuted.Store(true)
	})

	time.Sleep(100 * time.Millisecond)

	if taskExecuted.Load() {
		t.Error("Task should not be executed after pool is stopped")
	}
}
func TestWorkerPool_ConcurrentTasks(t *testing.T) {
	wp := NewWorkerPool(4)
	wp.Start()
	defer wp.Stop()

	const taskCount = 20
	resultCh := make(chan int, taskCount)

	for i := 0; i < taskCount; i++ {
		i := i
		wp.Submit(func() {

			time.Sleep(time.Millisecond * time.Duration(i%10))
			resultCh <- i
		})
	}

	results := make([]int, 0, taskCount)
	timeout := time.After(time.Second)

	for i := 0; i < taskCount; i++ {
		select {
		case res := <-resultCh:
			results = append(results, res)
		case <-timeout:
			t.Fatalf("Timeout waiting for task results, got %d of %d", len(results), taskCount)
		}
	}

	if len(results) != taskCount {
		t.Errorf("Expected %d results, got %d", taskCount, len(results))
	}

	resultsMap := make(map[int]bool)
	for _, res := range results {
		resultsMap[res] = true
	}

	for i := 0; i < taskCount; i++ {
		if !resultsMap[i] {
			t.Errorf("Missing result: %d", i)
		}
	}
}
