package subpub

import (
	"errors"
)

var (
	// ErrSubPubClosed возникает при попытке подписаться на закрытую шину
	ErrSubPubClosed = errors.New("subpub: system is closed")

	// ErrPublishFailed возникает при неудачной публикации сообщения
	ErrPublishFailed = errors.New("subpub: failed to publish message")
)
