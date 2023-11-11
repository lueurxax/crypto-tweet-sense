package tweets_editor

import (
	"os"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

var testTweets = []string{
	`Suspect a lot of crypto folks underweight and been waiting for a good pullback to buy since $BTC sliced right through $31k.   Strong markets have a way of locking people out.`,
	`Bizim kıytırık tek çizgi yine ipten aldı❤️`,
	`#Bitcoin is breaking back into the bull market territorial!`,
	`Time to freshen up, make some coffee, and then....record a new video update on $BTC?

	What do we think fam? Interested in a new vid update or nah? 🤔`,
	`📢📢📢📢📢 OFFICIAL LAUNCH: November 8  08:00 PM UTC📢📢📢📢📢

	OPYx [Opportunity Crypto DAO] : OPYx is DAO project born to make collective investments and have more economic power in this bearish phase, we buy and accumulate assets in Bear in… `,
	`I am not betting against the cartel`,
	`🚨 BREAKING: 

	BRICS Currency Agreement Nearing as Consensus Is Close!

	#XRP 🤝🏼 BRICS currency `,
	"This is the part of the cycle where holders will out perform day traders over the next 18-24 months",
	"CME Futures Open Interest just surpassed 100,000 BTC for the first time ever",
	`One of the most powerful supply shocks is loading, there are approximately two million #BTC remaining to buy on exchanges only 

	I believe that the incoming bull next year will be the strongest bull in BTC history, and also the last ‚huge‘ bull market in BTC history",
	"New video update is a little longer this time but its A BANGER! 

	Covered low, mid and high timeframe thoughts on $BTC as well as thoughts on $ETH and $XRP.

	Likes/shares/comments appreciated.`,
}

func TestNewEditor(t *testing.T) {
	t.Run("some tweets request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		client := openai.NewClient(os.Getenv("token"))
		ed := NewEditor(client, time.Second, log.NewLogger(logrus.New()))
		input := make(chan string)
		ctx = ed.Edit(ctx, input)
		output := ed.SubscribeEdited()
		go func(input chan string) {
			for _, tweet := range testTweets {
				input <- tweet
			}
		}(input)
		t.Log(<-output)
		cancel()
	})
}
