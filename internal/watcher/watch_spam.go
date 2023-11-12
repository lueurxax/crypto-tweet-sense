package watcher

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder"
)

type SpamWatcher interface {
	Watch()
}

type spamWatcher struct {
	finder
	published map[string]struct{}
	logger    log.Logger
	spamWords []string
}

func (w *spamWatcher) Watch() {
	go w.watch()
}

func (w *spamWatcher) watch() {
	ctx := context.Background()
	w.run(ctx)
	tick := time.NewTicker(time.Minute * 10)
	for range tick.C {
		w.run(ctx)
	}
}

func (w *spamWatcher) run(ctx context.Context) {
	start := time.Now().AddDate(0, 0, -1)
	w.runWithQuery(ctx, "bitcoin", start)
	w.runWithQuery(ctx, "crypto", start)
	w.runWithQuery(ctx, "cryptocurrency", start)
	w.runWithQuery(ctx, "BTC", start)
	w.logger.Debug("watcher checked news")
}

func (w *spamWatcher) runWithQuery(ctx context.Context, query string, start time.Time) {
	tweets, err := w.finder.FindAll(
		ctx,
		&start,
		nil,
		query,
	)
	if err != nil {
		if errors.Is(err, tweetfinder.NoTops) {
			return
		}
		panic(err)
	}
	for _, tweet := range tweets {
		if _, ok := w.published[tweet.ID]; ok {
			continue
		}
		w.published[tweet.ID] = struct{}{}
		for _, word := range w.spamWords {
			if strings.Contains(tweet.Text, word) {
				w.logger.
					WithField("username", tweet.Username).
					WithField("tweet", tweet.PermanentURL).
					Info("found spam")
			}
		}
	}
}

func NewSpamWatcher(finder finder, spamWords []string, logger log.Logger) SpamWatcher {
	return &spamWatcher{
		finder:    finder,
		spamWords: spamWords,
		published: map[string]struct{}{},
		logger:    logger,
	}
}
