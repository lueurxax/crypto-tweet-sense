package tweetseditor

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"gopkg.in/telebot.v3"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/sender"
)

var testTweets = []common.Tweet{
	{
		Text:         `Suspect a lot of crypto folks underweight and been waiting for a good pullback to buy since $BTC sliced right through $31k.   Strong markets have a way of locking people out.`,
		PermanentURL: "https://twitter.com/krugermacro/status/1413122239264561156",
	},
	{
		Text:         `Bizim kıytırık tek çizgi yine ipten aldı❤️`,
		PermanentURL: "https://twitter.com/krugermacro/status/1413122239264561156",
	},
	{
		Text:         `I am not betting against the cartel`,
		PermanentURL: "https://twitter.com/krugermacro/status/1413122239264561156",
	},
	{
		Text:         `#Bitcoin is breaking back into the bull market territorial!`,
		PermanentURL: "https://twitter.com/krugermacro/status/1413122239264561156",
	},
	{
		Text: `Time to freshen up, make some coffee, and then....record a new video update on $BTC?

	What do we think fam? Interested in a new vid update or nah? 🤔`,
		PermanentURL: "https://twitter.com/krugermacro/status/1413122239264561156",
	},
	{
		Text: `📢📢📢📢📢 OFFICIAL LAUNCH: November 8  08:00 PM UTC📢📢📢📢📢

	OPYx [Opportunity Crypto DAO] : OPYx is DAO project born to make collective investments and have more economic power in this bearish phase, we buy and accumulate assets in Bear in… `,
		PermanentURL: "https://twitter.com/krugermacro/status/1413122239264561156",
	},
	{
		Text: `🚨 BREAKING: 

	BRICS Currency Agreement Nearing as Consensus Is Close!

	#XRP 🤝🏼 BRICS currency `,
		PermanentURL: "https://twitter.com/krugermacro/status/1413122239264561156",
	},
}

var moreTestTweets = []common.Tweet{
	{
		Text:         `$50 Giveaway || Ends in 24 Hrs 🤯`,
		PermanentURL: "https://twitter.com/krugermacro/status/1413122239264561156",
	},
}

func TestNewEditor(t *testing.T) {
	t.Run("some tweets request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := openai.NewClient(os.Getenv("openai_token"))
		logrusLogger := logrus.New()
		logrusLogger.SetLevel(logrus.TraceLevel)
		logger := log.NewLogger(logrusLogger)
		ed := NewEditor(client, time.Second, time.Hour*24, logger)
		input := make(chan *common.Tweet)
		ctx = ed.Edit(ctx, input)
		output := ed.SubscribeEdited()

		chatID, err := strconv.ParseInt(os.Getenv("chat_id"), 10, 64)
		require.NoError(t, err)

		api, err := telebot.NewBot(
			telebot.Settings{Token: os.Getenv("bot_token"), Poller: &telebot.LongPoller{Timeout: 10 * time.Second}},
		)
		require.NoError(t, err)
		s := sender.NewSender(api, &telebot.Chat{ID: chatID}, logger)
		s.Send(ctx, output)
		go func(input chan *common.Tweet) {
			for i := range testTweets {
				input <- &testTweets[i]
			}
		}(input)
		time.Sleep(time.Second)
		go func(input chan *common.Tweet) {
			for i := range moreTestTweets {
				input <- &testTweets[i]
			}
		}(input)
		time.Sleep(time.Minute)
		cancel()
	})
}
