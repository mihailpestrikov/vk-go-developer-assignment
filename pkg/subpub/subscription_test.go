package subpub

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockSubPub struct {
	workerPool       *WorkerPool
	removeCallCount  int32
	removeSubject    string
	removeSubscriber Subscription
}

func (m *mockSubPub) RemoveSubscription(subject string, sub Subscription) {
	atomic.AddInt32(&m.removeCallCount, 1)
	m.removeSubject = subject
	m.removeSubscriber = sub
}

func (m *mockSubPub) Submit(task func()) {
	if m.workerPool != nil {
		m.workerPool.Submit(task)
	} else {

		task()
	}
}

func newMockParent() *mockSubPub {
	mp := &mockSubPub{
		workerPool: NewWorkerPool(2),
	}
	mp.workerPool.Start()
	return mp
}

func Test_subscription_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		active int32
		want   bool
	}{
		{
			name:   "Active subscription",
			active: 1,
			want:   true,
		},
		{
			name:   "Inactive subscription",
			active: 0,
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := newMockParent()

			s := &subscription{
				subject:      "test",
				callback:     func(msg interface{}) {},
				parent:       parent,
				messageCh:    make(chan interface{}, 10),
				quit:         make(chan struct{}),
				active:       tt.active,
				dropStrategy: DropNewest,
			}
			if got := s.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_subscription_Unsubscribe(t *testing.T) {
	tests := []struct {
		name        string
		active      int32
		expectCalls int
		shouldClose bool
	}{
		{
			name:        "Active subscription",
			active:      1,
			expectCalls: 1,
			shouldClose: true,
		},
		{
			name:        "Already inactive subscription",
			active:      0,
			expectCalls: 0,
			shouldClose: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := newMockParent()

			s := &subscription{
				subject:      "test-subject",
				callback:     func(msg interface{}) {},
				parent:       parent,
				messageCh:    make(chan interface{}, 10),
				quit:         make(chan struct{}),
				active:       tt.active,
				dropStrategy: DropNewest,
			}

			if !tt.shouldClose && tt.active == 0 {
				close(s.quit)
			}

			s.Unsubscribe()

			if s.IsActive() {
				t.Error("subscription should be inactive after Unsubscribe()")
			}

			if tt.shouldClose {
				select {
				case _, ok := <-s.quit:
					if ok {
						t.Error("Quit channel should be closed after Unsubscribe()")
					}
				default:
					t.Error("Quit channel should be closed after Unsubscribe()")
				}
			}

			if int(atomic.LoadInt32(&parent.removeCallCount)) != tt.expectCalls {
				t.Errorf("Expected %d calls to RemoveSubscription, got %d", tt.expectCalls, parent.removeCallCount)
			}

			if tt.expectCalls > 0 {
				if parent.removeSubject != "test-subject" {
					t.Errorf("Expected subject 'test-subject', got '%s'", parent.removeSubject)
				}
				if parent.removeSubscriber != s {
					t.Error("RemoveSubscription was called with wrong subscription")
				}
			}
		})
	}
}

func Test_subscription_SendMessage_DropNewest(t *testing.T) {
	parent := newMockParent()

	s := &subscription{
		subject:      "test",
		callback:     func(msg interface{}) {},
		parent:       parent,
		messageCh:    make(chan interface{}, 2),
		quit:         make(chan struct{}),
		active:       1,
		dropStrategy: DropNewest,
	}

	s.SendMessage("msg1")
	s.SendMessage("msg2")

	s.SendMessage("msg3")

	if len(s.messageCh) != 2 {
		t.Errorf("Expected messageCh length to be 2, got %d", len(s.messageCh))
	}

	expected := []string{"msg1", "msg2"}
	for i := 0; i < 2; i++ {
		select {
		case msg := <-s.messageCh:
			if msg != expected[i] {
				t.Errorf("Expected message %s, got %s", expected[i], msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timeout waiting for message %d", i)
		}
	}

	s.Unsubscribe()
}

func Test_subscription_SendMessage_DropOldest(t *testing.T) {
	parent := newMockParent()

	s := &subscription{
		subject:      "test",
		callback:     func(msg interface{}) {},
		parent:       parent,
		messageCh:    make(chan interface{}, 2),
		quit:         make(chan struct{}),
		active:       1,
		dropStrategy: DropOldest,
	}

	s.SendMessage("msg1")
	s.SendMessage("msg2")

	s.SendMessage("msg3")

	if len(s.messageCh) != 2 {
		t.Errorf("Expected messageCh length to be 2, got %d", len(s.messageCh))
	}

	expected := []string{"msg2", "msg3"}
	for i := 0; i < 2; i++ {
		select {
		case msg := <-s.messageCh:
			if msg != expected[i] {
				t.Errorf("Expected message %s, got %s", expected[i], msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timeout waiting for message %d", i)
		}
	}

	s.Unsubscribe()
}

func Test_subscription_SendMessage_DropNone(t *testing.T) {
	parent := newMockParent()

	s := &subscription{
		subject:      "test",
		callback:     func(msg interface{}) {},
		parent:       parent,
		messageCh:    make(chan interface{}, 2),
		quit:         make(chan struct{}),
		active:       1,
		dropStrategy: DropNone,
	}

	s.SendMessage("msg1")
	s.SendMessage("msg2")

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		s.SendMessage("msg3")
	}()

	time.Sleep(100 * time.Millisecond)

	if len(s.messageCh) != 2 {
		t.Errorf("Expected messageCh length to be 2, got %d", len(s.messageCh))
	}

	<-s.messageCh

	time.Sleep(100 * time.Millisecond)

	if len(s.messageCh) != 2 {
		t.Errorf("Expected messageCh length to be 2, got %d", len(s.messageCh))
	}

	wg.Wait()

	s.Unsubscribe()
}

func Test_subscription_SendMessage_InactiveSubscription(t *testing.T) {
	parent := newMockParent()

	s := &subscription{
		subject:      "test",
		callback:     func(msg interface{}) {},
		parent:       parent,
		messageCh:    make(chan interface{}, 2),
		quit:         make(chan struct{}),
		active:       0,
		dropStrategy: DropNewest,
	}

	s.SendMessage("msg1")

	if len(s.messageCh) != 0 {
		t.Errorf("Expected messageCh to be empty, got length %d", len(s.messageCh))
	}
}

func Test_subscription_processMessages(t *testing.T) {
	var processed bool
	var wg sync.WaitGroup
	wg.Add(1)

	parent := newMockParent()

	callback := func(msg interface{}) {
		processed = true
		wg.Done()
	}

	s := &subscription{
		subject:      "test",
		callback:     callback,
		parent:       parent,
		messageCh:    make(chan interface{}, 2),
		quit:         make(chan struct{}),
		active:       1,
		dropStrategy: DropNewest,
	}

	go s.processMessages()

	s.messageCh <- "test message"

	wg.Wait()

	if !processed {
		t.Error("Message was not processed")
	}

	s.Unsubscribe()
}

func Test_newSubscription(t *testing.T) {
	parent := &subPub{
		workerPool: NewWorkerPool(2),
	}
	parent.workerPool.Start()

	callback := func(msg interface{}) {}

	sub := newSubscription("test", callback, parent, 10, DropNewest)

	if sub.subject != "test" {
		t.Errorf("Expected subject to be 'test', got %s", sub.subject)
	}

	if sub.callback == nil {
		t.Error("Callback should not be nil")
	}

	if sub.parent != parent {
		t.Error("Parent should be set correctly")
	}

	if cap(sub.messageCh) != 10 {
		t.Errorf("Expected messageCh capacity to be 10, got %d", cap(sub.messageCh))
	}

	if sub.quit == nil {
		t.Error("Quit channel should not be nil")
	}

	if !sub.IsActive() {
		t.Error("New subscription should be active")
	}

	if sub.dropStrategy != DropNewest {
		t.Errorf("Expected dropStrategy to be DropNewest, got %v", sub.dropStrategy)
	}

	sub.Unsubscribe()
}
