package subpub

import (
	"sync/atomic"
)

type SubscriptionParent interface {
	RemoveSubscription(subject string, sub Subscription)
	Submit(task func())
}

type subscription struct {
	subject      string
	callback     MessageHandler
	parent       SubscriptionParent
	messageCh    chan interface{}
	quit         chan struct{}
	active       int32
	dropStrategy DropStrategy
}

func newSubscription(subject string, cb MessageHandler, parent SubscriptionParent, bufferSize int, dropStrategy DropStrategy) *subscription {
	sub := &subscription{
		subject:      subject,
		callback:     cb,
		parent:       parent,
		messageCh:    make(chan interface{}, bufferSize),
		quit:         make(chan struct{}),
		active:       1,
		dropStrategy: dropStrategy,
	}

	go sub.processMessages()

	return sub
}

func (s *subscription) IsActive() bool {
	return atomic.LoadInt32(&s.active) == 1
}

func (s *subscription) SendMessage(msg interface{}) {
	if !s.IsActive() {
		return
	}

	switch s.dropStrategy {
	case DropNone:
		// Блокируемся до тех пор, пока сообщение не будет отправлено
		select {
		case s.messageCh <- msg:
			// Сообщение отправлено
		case <-s.quit:
			// Подписка отменена
		}

	case DropNewest:
		// Не блокируемся, пропускаем новое сообщение, если буфер полон или подписка отменена
		select {
		case s.messageCh <- msg:
			// Сообщение отправлено
		case <-s.quit:
			// Подписка отменена
		default:
			// Буфер полон, пропускаем сообщение
		}

	case DropOldest:
		// Если буфер полон, удаляем самое старое сообщение и добавляем новое
		select {
		case s.messageCh <- msg:
			// Сообщение отправлено
		case <-s.quit:
			// Подписка отменена
		default:
			// Буфер полон, удаляем самое старое сообщение
			select {
			case <-s.messageCh: // Удаляем старое сообщение
				select {
				case s.messageCh <- msg: // Добавляем новое
					// Сообщение отправлено
				case <-s.quit:
					// Подписка отменена
				}
			case <-s.quit:
				// Подписка отменена
			}
		}
	}
}

func (s *subscription) Unsubscribe() {
	if !atomic.CompareAndSwapInt32(&s.active, 1, 0) {
		return
	}

	close(s.quit)

	s.parent.RemoveSubscription(s.subject, s)
}

func (s *subscription) processMessages() {
	for {
		select {
		case msg := <-s.messageCh:
			s.parent.Submit(func() {
				if s.IsActive() {
					s.callback(msg)
				}
			})

		case <-s.quit:
			return
		}
	}
}
