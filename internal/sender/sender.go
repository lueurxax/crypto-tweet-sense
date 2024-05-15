package sender

import (
	"context"

	"gopkg.in/telebot.v3"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/pkg/utils"
)

//go:generate mockgen -source=sender.go -destination=mocks/mock_sender.go -package=mocks

const (
	maxLen  = 4096
	whatKey = "what"
)

type Sender interface {
	Send(ctx context.Context, linkCh <-chan string) context.Context
}

type client interface {
	Send(recipient telebot.Recipient, what interface{}, options ...interface{}) (*telebot.Message, error)
}

type sender struct {
	client

	recipient telebot.Recipient

	log log.Logger
}

func (s *sender) Send(ctx context.Context, linkCh <-chan string) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	go s.send(cancel, linkCh)

	return ctx
}

func (s *sender) send(cancel context.CancelFunc, ch <-chan string) {
	for msg := range ch {
		for len(msg) > 0 {
			batchLen := min(maxLen, len(msg))
			if _, err := s.client.Send(s.recipient, msg[:batchLen], telebot.ModeMarkdownV2); err != nil {
				s.log.WithField(whatKey, msg).WithError(err).Warn("send error")

				if _, err = s.client.Send(s.recipient, utils.Escape(msg[:batchLen]), telebot.ModeMarkdownV2); err != nil {
					s.log.WithField(whatKey, msg).WithError(err).Error("escaped send error")
				}

				cancel()
			}

			msg = msg[batchLen:]
		}
	}
}

func NewSender(client client, recipient telebot.Recipient, logger log.Logger) Sender {
	return &sender{client: client, recipient: recipient, log: logger}
}
