package watcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweet_finder"
	"github.com/lueurxax/crypto-tweet-sense/pkg/utils"
)

type Watcher interface {
	Watch()
	Subscribe() <-chan string
	RawSubscribe() <-chan string
}

type finder interface {
	Find(ctx context.Context, start, end time.Time, search string) (string, error)
	FindAll(ctx context.Context, start, end *time.Time, search string) ([]twitterscraper.Tweet, error)
}

type watcher struct {
	finder
	subMu          sync.RWMutex
	subscribers    []chan string
	rawSubscribers []chan string
	published      map[string]struct{}
	logger         log.Logger
}

func (w *watcher) RawSubscribe() <-chan string {
	w.subMu.Lock()

	subscriber := make(chan string, 10)
	w.rawSubscribers = append(w.rawSubscribers, subscriber)

	w.subMu.Unlock()

	return subscriber
}

func (w *watcher) Subscribe() <-chan string {
	w.subMu.Lock()

	subscriber := make(chan string, 10)
	w.subscribers = append(w.subscribers, subscriber)

	w.subMu.Unlock()

	return subscriber
}

func (w *watcher) Watch() {
	go w.watch()
}

func (w *watcher) watch() {
	ctx := context.Background()
	w.run(ctx)

	tick := time.NewTicker(time.Minute * 10)
	for range tick.C {
		w.run(ctx)
	}
}

func (w *watcher) run(ctx context.Context) {
	start := time.Now().AddDate(0, 0, -1)
	w.runWithQuery(ctx, "bitcoin", start)
	w.runWithQuery(ctx, "crypto", start)
	w.runWithQuery(ctx, "cryptocurrency", start)
	w.runWithQuery(ctx, "BTC", start)
	w.logger.Debug("watcher checked news")
}

func (w *watcher) runWithQuery(ctx context.Context, query string, start time.Time) {
	tweets, err := w.finder.FindAll(
		ctx,
		&start,
		nil,
		query,
	)
	if err != nil {
		if errors.Is(err, tweet_finder.NoTops) {
			return
		}

		panic(err)
	}

	for _, tweet := range tweets {
		if _, ok := w.published[tweet.PermanentURL]; ok {
			continue
		}

		w.published[tweet.PermanentURL] = struct{}{}

		w.subMu.RLock()
		w.logger.WithField("subscribers", len(w.rawSubscribers)).Debug("send raw tweet")

		for i := range w.rawSubscribers {
			w.rawSubscribers[i] <- tweet.Text
		}

		w.logger.WithField("subscribers", len(w.subscribers)).Debug("send formatted tweet")

		for i := range w.subscribers {
			w.subscribers[i] <- w.formatTweet(tweet)
		}
		w.subMu.RUnlock()
	}
}

func (w *watcher) formatTweet(tweet twitterscraper.Tweet) (str string) {
	str = fmt.Sprintf("*%s*\n", utils.Escape(tweet.TimeParsed.Format(time.RFC3339)))
	str += fmt.Sprintf("%s\n", utils.Escape(tweet.Text))

	for _, photo := range tweet.Photos {
		str += fmt.Sprintf("[photo](%s)\n", utils.Escape(photo.URL))
	}

	for _, video := range tweet.Videos {
		str += fmt.Sprintf("[video](%s)\n", utils.Escape(video.URL))
	}

	str += fmt.Sprintf("[link](%s)\n", utils.Escape(tweet.PermanentURL))

	return
}

func NewWatcher(finder finder, initPublished map[string]struct{}, logger log.Logger) Watcher {
	return &watcher{
		finder:         finder,
		subMu:          sync.RWMutex{},
		subscribers:    make([]chan string, 0),
		rawSubscribers: make([]chan string, 0),
		published:      initPublished,
		logger:         logger,
	}
}
