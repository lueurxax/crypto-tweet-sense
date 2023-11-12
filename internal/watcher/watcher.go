package watcher

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweet_finder"
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
	defer w.subMu.Unlock()

	subscriber := make(chan string)
	w.rawSubscribers = append(w.rawSubscribers, subscriber)

	return subscriber
}

func (w *watcher) Subscribe() <-chan string {
	w.subMu.Lock()
	defer w.subMu.Unlock()

	subscriber := make(chan string)
	w.subscribers = append(w.subscribers, subscriber)

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
		for _, subscriber := range w.rawSubscribers {
			subscriber <- tweet.Text
		}

		for _, subscriber := range w.subscribers {
			subscriber <- w.formatTweet(tweet)
		}
		w.subMu.RUnlock()
	}
}

func (w *watcher) formatTweet(tweet twitterscraper.Tweet) (str string) {
	str = fmt.Sprintf("*%s*\n", escape(tweet.TimeParsed.Format(time.RFC3339)))
	str += fmt.Sprintf("%s\n", escape(tweet.Text))

	for _, photo := range tweet.Photos {
		str += fmt.Sprintf("[photo](%s)\n", escape(photo.URL))
	}

	for _, video := range tweet.Videos {
		str += fmt.Sprintf("[video](%s)\n", escape(video.URL))
	}

	str += fmt.Sprintf("[link](%s)\n", escape(tweet.PermanentURL))

	return
}

func escape(data string) string {
	res := data
	for _, symbol := range []string{"-", "]", "[", "{", "}", "(", ")", ">", "<", ".", "!", "*", "+", "=", "#", "~", "|", "`", "_"} {
		res = strings.ReplaceAll(res, symbol, "\\"+symbol)
	}

	return res
}

func NewWatcher(finder finder, initPublished map[string]struct{}, logger log.Logger) Watcher {
	return &watcher{
		finder:         finder,
		subMu:          sync.RWMutex{},
		subscribers:    make([]chan string, 10),
		rawSubscribers: make([]chan string, 10),
		published:      initPublished,
		logger:         logger,
	}
}
