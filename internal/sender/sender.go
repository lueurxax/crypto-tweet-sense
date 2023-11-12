package sender

import (
	"context"

	"gopkg.in/telebot.v3"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

type Sender interface {
	Send(ctx context.Context, linkCh <-chan string) context.Context
}

type sender struct {
	client *telebot.Bot

	recipient telebot.Recipient

	log log.Logger
}

func (s *sender) Send(ctx context.Context, linkCh <-chan string) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	go s.send(cancel, linkCh)

	return ctx
}

func (s *sender) send(cancel context.CancelFunc, ch <-chan string) {
	for link := range ch {
		if _, err := s.client.Send(s.recipient, link, telebot.ModeMarkdownV2); err != nil {
			s.log.WithError(err).Error("send error")
			cancel()
		}
	}
}

func NewSender(client *telebot.Bot, recipient telebot.Recipient, logger log.Logger) Sender {
	return &sender{client: client, recipient: recipient, log: logger}
}
