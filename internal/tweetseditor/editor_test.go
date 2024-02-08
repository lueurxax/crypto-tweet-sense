package tweetseditor

import (
	"context"
	"os"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Text:         `$50 Giveaway || Ends in 24 Hrs 🤯`,
		PermanentURL: "https://twitter.com/krugermacro/status/1413122239264561156",
	},
}

var moreTestTweets = []common.Tweet{
	{
		Text: `🚨 BREAKING: 

	BRICS Currency Agreement Nearing as Consensus Is Close!

	#XRP 🤝🏼 BRICS currency `,
		PermanentURL: "https://twitter.com/krugermacro/status/1413122239264561156",
	},
}

func TestNewEditor(t *testing.T) {
	t.Run("some tweets request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := openai.NewClient(os.Getenv("CHAT_GPT_TOKEN"))
		logrusLogger := logrus.New()
		logrusLogger.SetLevel(logrus.TraceLevel)
		logger := log.NewLogger(logrusLogger)
		ed := NewEditor(client, logger)
		output := make(chan string, 100)

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-output:
					t.Log(msg)
				}
			}
		}()

		err := ed.Edit(ctx, testTweets, output)
		assert.NoError(t, err)

		err = ed.Edit(ctx, moreTestTweets, output)
		assert.NoError(t, err)
		cancel()
	})
}

func TestLongStory(t *testing.T) {
	logrusLogger := logrus.New()
	logrusLogger.SetLevel(logrus.TraceLevel)
	logger := log.NewLogger(logrusLogger)

	t.Run("some tweets request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := openai.NewClient(os.Getenv("CHAT_GPT_TOKEN"))
		ed := NewEditor(client, logger)
		output := make(chan string, 100)

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-output:
					t.Log(msg)
				}
			}
		}()

		_, err := ed.EditLongStory(ctx, testTweets, output)
		assert.NoError(t, err)

		_, err = ed.EditLongStory(ctx, moreTestTweets, output)
		assert.NoError(t, err)
		cancel()
	})

	t.Run("processTweetsFromJson", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := openai.NewClient(os.Getenv("CHAT_GPT_TOKEN"))

		data, err := os.ReadFile("./test/fixtures/tweets.json")
		require.NoError(t, err)

		var tweets []common.Tweet
		err = jsoniter.Unmarshal(data, &tweets)
		require.NoError(t, err)

		ed := NewEditor(client, logger)
		output := make(chan string, 100)

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-output:
					t.Log(msg)
				}
			}
		}()

		_, err = ed.EditLongStory(ctx, tweets[:20], output)
		assert.NoError(t, err)

		_, err = ed.EditLongStory(ctx, tweets[20:], output)
		assert.NoError(t, err)
		cancel()
	})
}

func TestRusLongStory(t *testing.T) {
	t.Run("some tweets request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := openai.NewClient(os.Getenv("CHAT_GPT_TOKEN"))
		logrusLogger := logrus.New()
		logrusLogger.SetLevel(logrus.TraceLevel)
		logger := log.NewLogger(logrusLogger)
		ed := NewEditor(client, logger)
		output := make(chan string, 100)

		content, err := ed.EditLongStory(ctx, testTweets, output)
		assert.NoError(t, err)

		translatedContent, err := ed.TranslateLongStory(ctx, content)
		assert.NoError(t, err)

		t.Log(translatedContent)

		content, err = ed.EditLongStory(ctx, moreTestTweets, output)
		assert.NoError(t, err)

		if content != "" {
			translatedContent, err = ed.TranslateLongStory(ctx, content)
			assert.NoError(t, err)

			t.Log(translatedContent)
		}

		cancel()
	})
}
