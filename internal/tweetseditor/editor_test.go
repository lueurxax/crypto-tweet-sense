package tweetseditor

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
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

type testRepo struct {
	data []common.Tweet
}

func (t *testRepo) GetTweetForEdit(context.Context) ([]common.Tweet, error) {
	return t.data, nil
}

func (t *testRepo) DeleteEditedTweets(context.Context, []string) error {
	return nil
}

func TestNewEditor(t *testing.T) {
	t.Run("some tweets request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := openai.NewClient(os.Getenv("CHAT_GPT_TOKEN"))
		logrusLogger := logrus.New()
		logrusLogger.SetLevel(logrus.TraceLevel)
		logger := log.NewLogger(logrusLogger)
		r := &testRepo{data: testTweets}
		ed := NewEditor(client, r, time.Second, time.Hour*24, logger)
		ctx = ed.Edit(ctx)
		output := ed.SubscribeEdited()

		chatID, err := strconv.ParseInt(os.Getenv("CHAT_ID"), 10, 64)
		require.NoError(t, err)

		api, err := telebot.NewBot(
			telebot.Settings{Token: os.Getenv("BOT_TOKEN"), Poller: &telebot.LongPoller{Timeout: 10 * time.Second}},
		)
		require.NoError(t, err)
		s := sender.NewSender(api, &telebot.Chat{ID: chatID}, logger)
		s.Send(ctx, output)
		time.Sleep(time.Second)
		r.data = moreTestTweets
		time.Sleep(time.Minute)
		cancel()
	})
}

func TestLongStory(t *testing.T) {
	t.Run("some tweets request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := openai.NewClient(os.Getenv("CHAT_GPT_TOKEN"))
		logrusLogger := logrus.New()
		logrusLogger.SetLevel(logrus.TraceLevel)
		logger := log.NewLogger(logrusLogger)
		r := &testRepo{data: testTweets}
		ed := NewEditor(client, r, time.Second, time.Hour*24, logger)
		ctx = ed.Edit(ctx)
		output := ed.SubscribeLongStoryMessages()

		chatID, err := strconv.ParseInt(os.Getenv("CHAT_ID"), 10, 64)
		require.NoError(t, err)

		api, err := telebot.NewBot(
			telebot.Settings{Token: os.Getenv("BOT_TOKEN"), Poller: &telebot.LongPoller{Timeout: 10 * time.Second}},
		)
		require.NoError(t, err)
		s := sender.NewSender(api, &telebot.Chat{ID: chatID}, logger)
		s.Send(ctx, output)
		time.Sleep(time.Second)
		r.data = moreTestTweets
		time.Sleep(time.Minute)
		cancel()
	})
}
