package subpub

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewSubPub(t *testing.T) {

	t.Run("Default options", func(t *testing.T) {
		sp := NewSubPub()
		if sp == nil {
			t.Fatal("Expected non-nil SubPub")
		}
	})

	t.Run("Custom options", func(t *testing.T) {
		sp := NewSubPub(
			WithBufferSize(200),
			WithDropStrategy(DropOldest),
			WithWorkerPool(5),
		)
		if sp == nil {
			t.Fatal("Expected non-nil SubPub")
		}

		spImpl := sp.(*subPub)
		if spImpl.bufferSize != 200 {
			t.Errorf("Expected bufferSize to be 200, got %d", spImpl.bufferSize)
		}
		if spImpl.dropStrategy != DropOldest {
			t.Errorf("Expected dropStrategy to be DropOldest, got %v", spImpl.dropStrategy)
		}
		if spImpl.workerPool.size != 5 {
			t.Errorf("Expected workerPool size to be 5, got %d", spImpl.workerPool.size)
		}
	})
}

func TestSubPub_SubscribePublish(t *testing.T) {
	t.Run("Basic subscribe and publish", func(t *testing.T) {
		sp := NewSubPub()

		msgCh1 := make(chan interface{}, 2)
		msgCh2 := make(chan interface{}, 2)
		msgCh3 := make(chan interface{}, 2)

		sub1, err := sp.Subscribe("test.topic", func(msg interface{}) {
			msgCh1 <- msg
		})
		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		sub2, err := sp.Subscribe("test.topic", func(msg interface{}) {
			msgCh2 <- msg
		})
		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		sub3, err := sp.Subscribe("other.topic", func(msg interface{}) {
			msgCh3 <- msg
		})
		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}
		defer sub3.Unsubscribe()

		err = sp.Publish("test.topic", "hello")
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		select {
		case msg := <-msgCh1:
			if msg != "hello" {
				t.Errorf("Expected 'hello', got %v", msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for message in subscriber 1")
		}

		select {
		case msg := <-msgCh2:
			if msg != "hello" {
				t.Errorf("Expected 'hello', got %v", msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for message in subscriber 2")
		}

		select {
		case msg := <-msgCh3:
			t.Errorf("Unexpected message in other.topic: %v", msg)
		case <-time.After(50 * time.Millisecond):

		}

		err = sp.Publish("other.topic", "world")
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		select {
		case msg := <-msgCh3:
			if msg != "world" {
				t.Errorf("Expected 'world', got %v", msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for message in subscriber 3")
		}

		sub1.Unsubscribe()

		err = sp.Publish("test.topic", "again")
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		select {
		case msg := <-msgCh1:
			t.Errorf("Unexpected message in unsubscribed subscriber: %v", msg)
		case <-time.After(50 * time.Millisecond):

		}

		select {
		case msg := <-msgCh2:
			if msg != "again" {
				t.Errorf("Expected 'again', got %v", msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for message in subscriber 2")
		}

		sub2.Unsubscribe()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := sp.Close(ctx); err != nil {
			t.Fatalf("Failed to close SubPub: %v", err)
		}
	})

	t.Run("Subscribe to closed system", func(t *testing.T) {
		sp := NewSubPub()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := sp.Close(ctx); err != nil {
			t.Fatalf("Failed to close SubPub: %v", err)
		}

		_, err := sp.Subscribe("test", func(msg interface{}) {})
		if err != ErrSubPubClosed {
			t.Errorf("Expected ErrSubPubClosed, got %v", err)
		}

		err = sp.Publish("test", "hello")
		if err != ErrSubPubClosed {
			t.Errorf("Expected ErrSubPubClosed, got %v", err)
		}
	})
}

func TestSubPub_SlowSubscriber(t *testing.T) {
	t.Run("Slow subscriber with DropNewest", func(t *testing.T) {
		sp := NewSubPub(
			WithBufferSize(2),
			WithDropStrategy(DropNewest),
		)

		receivedCh := make(chan string, 5)

		sub, err := sp.Subscribe("test", func(msg interface{}) {

			time.Sleep(50 * time.Millisecond)

			receivedCh <- msg.(string)
		})
		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		for i := 0; i < 5; i++ {
			msg := string(rune('a' + i))
			err = sp.Publish("test", msg)
			if err != nil {
				t.Fatalf("Failed to publish: %v", err)
			}
		}

		var receivedMsgs []string
		timer := time.NewTimer(300 * time.Millisecond)

	CollectLoop:
		for {
			select {
			case msg := <-receivedCh:
				receivedMsgs = append(receivedMsgs, msg)
			case <-timer.C:
				break CollectLoop
			}
		}

		sub.Unsubscribe()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		sp.Close(ctx)

		if len(receivedMsgs) < 2 {
			t.Errorf("Expected at least 2 messages, got %d", len(receivedMsgs))
		} else {

			hasA := false
			hasB := false

			for _, msg := range receivedMsgs {
				if msg == "a" {
					hasA = true
				} else if msg == "b" {
					hasB = true
				}
			}

			if !hasA || !hasB {
				t.Errorf("Expected to receive both 'a' and 'b' messages, got %v", receivedMsgs)
			}
		}
	})
}

func TestSubPub_Close(t *testing.T) {
	t.Run("Close with context", func(t *testing.T) {
		sp := NewSubPub()

		for i := 0; i < 3; i++ {
			_, err := sp.Subscribe("test", func(msg interface{}) {

				time.Sleep(10 * time.Millisecond)
			})
			if err != nil {
				t.Fatalf("Failed to subscribe: %v", err)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := sp.Close(ctx); err != nil {
			t.Errorf("Close() failed: %v", err)
		}

		spImpl := sp.(*subPub)
		if !spImpl.closed {
			t.Error("SubPub should be closed")
		}
	})

	t.Run("Close with canceled context", func(t *testing.T) {
		sp := NewSubPub()

		_, err := sp.Subscribe("test", func(msg interface{}) {
			time.Sleep(500 * time.Millisecond)
		})
		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = sp.Close(ctx)
		if err == nil {
			t.Error("Expected error from Close() with canceled context, got nil")
		}
	})
}

func TestSubPub_FIFO(t *testing.T) {
	t.Run("Preserve message order", func(t *testing.T) {

		sp := NewSubPub(
			WithWorkerPool(1),
		)

		const messagesCount = 20
		receivedMsgs := make([]int, 0, messagesCount)
		var mu sync.Mutex
		var wg sync.WaitGroup
		wg.Add(messagesCount)

		sub, err := sp.Subscribe("test", func(msg interface{}) {

			time.Sleep(time.Millisecond)

			value := msg.(int)
			mu.Lock()
			receivedMsgs = append(receivedMsgs, value)
			mu.Unlock()
			wg.Done()
		})
		if err != nil {
			t.Fatalf("Failed to subscribe: %v", err)
		}

		for i := 0; i < messagesCount; i++ {
			err = sp.Publish("test", i)
			if err != nil {
				t.Fatalf("Failed to publish message: %v", err)
			}
			time.Sleep(time.Millisecond)
		}

		waitCh := make(chan struct{})
		go func() {
			wg.Wait()
			close(waitCh)
		}()

		select {
		case <-waitCh:

			mu.Lock()
			defer mu.Unlock()

			if len(receivedMsgs) != messagesCount {
				t.Errorf("Expected %d messages, got %d", messagesCount, len(receivedMsgs))
			}

			outOfOrderCount := 0
			missingValues := make(map[int]bool)
			duplicateValues := make(map[int]bool)

			for i := 0; i < messagesCount; i++ {
				missingValues[i] = true
			}

			for _, val := range receivedMsgs {
				if val >= 0 && val < messagesCount {
					if !missingValues[val] {
						duplicateValues[val] = true
					}
					missingValues[val] = false
				}
			}

			missing := make([]int, 0)
			for i := 0; i < messagesCount; i++ {
				if missingValues[i] {
					missing = append(missing, i)
				}
			}

			if len(missing) > 0 {
				t.Errorf("Missing values: %v", missing)
			}

			duplicates := make([]int, 0)
			for i := 0; i < messagesCount; i++ {
				if duplicateValues[i] {
					duplicates = append(duplicates, i)
				}
			}

			if len(duplicates) > 0 {
				t.Errorf("Duplicate values: %v", duplicates)
			}

			for i := 0; i < len(receivedMsgs)-1; i++ {
				if receivedMsgs[i] > receivedMsgs[i+1] {
					outOfOrderCount++
					t.Errorf("Message order violation at position %d: %d > %d",
						i, receivedMsgs[i], receivedMsgs[i+1])
				}
			}

		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for messages")
		}

		sub.Unsubscribe()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		sp.Close(ctx)
	})
}
