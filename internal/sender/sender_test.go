package sender

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"gopkg.in/telebot.v3"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/sender/mocks"
)

func Test_sender_send(t *testing.T) {
	t.Run("short message", func(t *testing.T) {
		mockclient := mocks.NewMockclient(gomock.NewController(t))
		mockclient.EXPECT().Send(&telebot.User{ID: 1}, "test", telebot.ModeMarkdownV2).Return(nil, nil).Times(1)
		s := NewSender(mockclient, &telebot.User{ID: 1}, log.NewLogger(logrus.New()))
		ch := make(chan string)
		s.Send(context.Background(), ch)
		ch <- "test"
		close(ch)
		time.Sleep(time.Second)
	})

	t.Run("long message", func(t *testing.T) {
		mockclient := mocks.NewMockclient(gomock.NewController(t))
		// test text longer then 4096 symbols
		builder := strings.Builder{}
		for i := 0; i < maxLen; i++ {
			builder.WriteString("a")
		}

		textMoreThanMaxLen := builder.String()

		mockclient.EXPECT().Send(&telebot.User{ID: 1}, textMoreThanMaxLen, telebot.ModeMarkdownV2).Return(nil, nil).Times(2)
		s := NewSender(mockclient, &telebot.User{ID: 1}, log.NewLogger(logrus.New()))
		ch := make(chan string)
		s.Send(context.Background(), ch)
		ch <- strings.Join([]string{textMoreThanMaxLen, textMoreThanMaxLen}, "")
		close(ch)
		time.Sleep(time.Second)
	})
}
