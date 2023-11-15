package watcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder"
	"github.com/lueurxax/crypto-tweet-sense/pkg/utils"
)

const (
	subscribersKey = "subscribers"
	tweetKey       = "tweet"
)

type Watcher interface {
	Watch()
	Subscribe() <-chan string
	RawSubscribe() <-chan string
}

type finder interface {
	FindAll(ctx context.Context, start, end *time.Time, search string) ([]common.TweetSnapshot, error)
	Find(ctx context.Context, id string) (*common.TweetSnapshot, error)
}

type repo interface {
	Save(ctx context.Context, tweets []common.TweetSnapshot) error
	DeleteTweet(ctx context.Context, id string) error
	GetFastestGrowingTweet(ctx context.Context) (*common.TweetSnapshot, error)
	GetOldestTopReachableTweet(ctx context.Context, top float64) (*common.TweetSnapshot, error)
	GetOldestSyncedTweet(ctx context.Context) (*common.TweetSnapshot, error)
	GetTweetsOlderThen(ctx context.Context, after time.Time) ([]*common.TweetSnapshot, error)
}

type ratingChecker interface {
	Check(ctx context.Context, tweet *common.TweetSnapshot) (bool, float64, error)
	CurrentTop() float64
}

type watcher struct {
	queries map[string]time.Time

	finder
	repo
	ratingChecker

	subMu          sync.RWMutex
	subscribers    []chan string
	rawSubscribers []chan string

	published map[string]struct{}

	logger log.Logger
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
	for query := range w.queries {
		go w.searchAll(query)
	}
	go w.updateTop()
	go w.updateOldestFast()
	go w.updateOldest()
	go w.cleanTooOld()
}

func (w *watcher) searchAll(query string) {
	ctx := context.Background()
	w.search(ctx, query)

	tick := time.NewTicker(time.Minute * 15)
	for range tick.C {
		w.search(ctx, query)
	}
}

func (w *watcher) search(ctx context.Context, query string) {
	w.searchWithQuery(ctx, query, w.queries[query])
	w.logger.WithField("query", query).Debug("watcher checked news")
}

func (w *watcher) searchWithQuery(ctx context.Context, query string, start time.Time) {
	tweets, err := w.finder.FindAll(
		ctx,
		&start,
		nil,
		query,
	)
	if err != nil {
		if errors.Is(err, tweetfinder.ErrNoTops) {
			return
		}

		panic(err)
	}

	lastTweet := start

	for i := range tweets {
		lastTweet, tweets[i].RatingGrowSpeed = w.processTweet(ctx, &tweets[i], lastTweet)
	}

	if err = w.repo.Save(ctx, tweets); err != nil {
		w.logger.WithError(err).Error("save tweets")
	}

	w.queries[query] = lastTweet
}

func (w *watcher) formatTweet(tweet common.TweetSnapshot) (str string) {
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

func (w *watcher) processTweet(ctx context.Context, tweet *common.TweetSnapshot, lastTweet time.Time) (time.Time, float64) {
	if _, ok := w.published[tweet.PermanentURL]; ok {
		return lastTweet, 0
	}

	ok, ratingSpeed, err := w.ratingChecker.Check(ctx, tweet)
	if err != nil {
		w.logger.WithError(err).Error("check tweet")
		return lastTweet, 0
	}

	if ok {
		w.subMu.RLock()
		w.logger.WithField(subscribersKey, len(w.rawSubscribers)).Debug("send raw tweet")

		for j := range w.rawSubscribers {
			w.rawSubscribers[j] <- tweet.Text
		}

		w.logger.WithField(subscribersKey, len(w.subscribers)).Debug("send formatted tweet")

		for j := range w.subscribers {
			w.subscribers[j] <- w.formatTweet(*tweet)
		}
		w.subMu.RUnlock()

		w.logger.
			WithField("ts", tweet.TimeParsed).
			WithField("text", tweet.Text).
			Debug("found tweet")

		w.published[tweet.PermanentURL] = struct{}{}
	}

	if lastTweet.Before(tweet.TimeParsed) {
		lastTweet = tweet.TimeParsed
	}

	return lastTweet, ratingSpeed
}

func (w *watcher) updateTop() {
	tick := time.NewTicker(time.Second * 30)
	for range tick.C {
		ctx := context.Background()
		if err := w.updateTopTweet(ctx); err != nil {
			w.logger.WithError(err).Error("update top tweet")
		}
	}
}

func (w *watcher) updateTopTweet(ctx context.Context) error {
	tweet, err := w.repo.GetFastestGrowingTweet(ctx)
	if err != nil {
		return err
	}

	w.logger.WithField(tweetKey, tweet).Debug("top tweet")

	return w.updateTweet(ctx, tweet.ID)
}

func (w *watcher) updateOldestFast() {
	tick := time.NewTicker(time.Minute * 1)
	for range tick.C {
		ctx := context.Background()
		if err := w.updateOldestFastTweet(ctx); err != nil {
			w.logger.WithError(err).Error()
		}
	}
}

func (w *watcher) updateOldestFastTweet(ctx context.Context) error {
	tweet, err := w.repo.GetOldestTopReachableTweet(ctx, w.ratingChecker.CurrentTop())
	if err != nil {
		return err
	}

	w.logger.WithField(tweetKey, tweet).Debug("oldest fast tweet")

	return w.updateTweet(ctx, tweet.ID)
}

func (w *watcher) updateTweet(ctx context.Context, id string) error {
	tweet, err := w.finder.Find(ctx, id)
	if err != nil {
		if errors.Is(err, tweetfinder.ErrNotFound) {
			return w.repo.DeleteTweet(ctx, id)
		}

		return err
	}

	_, tweet.RatingGrowSpeed = w.processTweet(ctx, tweet, time.Now())
	if tweet.RatingGrowSpeed != 0 {
		return w.repo.Save(ctx, []common.TweetSnapshot{*tweet})
	}

	return w.repo.DeleteTweet(ctx, tweet.ID)
}

func (w *watcher) updateOldest() {
	tick := time.NewTicker(time.Minute)
	for range tick.C {
		ctx := context.Background()
		if err := w.updateOldestTweet(ctx); err != nil {
			w.logger.WithError(err).Error()
		}
	}
}

func (w *watcher) updateOldestTweet(ctx context.Context) error {
	tweet, err := w.repo.GetOldestSyncedTweet(ctx)
	if err != nil {
		return err
	}

	w.logger.WithField(tweetKey, tweet).Debug("oldest tweet")

	return w.updateTweet(ctx, tweet.ID)
}

func (w *watcher) cleanTooOld() {
	tick := time.NewTicker(time.Hour)
	for range tick.C {
		ctx := context.Background()
		if err := w.cleanTooOldTweets(ctx); err != nil {
			w.logger.WithError(err).Error()
		}
	}
}

func (w *watcher) cleanTooOldTweets(ctx context.Context) error {
	tweets, err := w.repo.GetTweetsOlderThen(ctx, time.Now().AddDate(0, 0, -1))
	if err != nil {
		return err
	}

	for _, tweet := range tweets {
		if err = w.repo.DeleteTweet(ctx, tweet.ID); err != nil {
			return err
		}
	}

	return nil
}

func NewWatcher(finder finder, repo repo, checker ratingChecker, initPublished map[string]struct{}, logger log.Logger) Watcher {
	start := time.Now().AddDate(0, 0, -1)

	return &watcher{
		queries: map[string]time.Time{
			"bitcoin":        start,
			"crypto":         start,
			"cryptocurrency": start,
			"BTC":            start,
		},
		finder:         finder,
		repo:           repo,
		ratingChecker:  checker,
		subMu:          sync.RWMutex{},
		subscribers:    make([]chan string, 0),
		rawSubscribers: make([]chan string, 0),
		published:      initPublished,
		logger:         logger,
	}
}
