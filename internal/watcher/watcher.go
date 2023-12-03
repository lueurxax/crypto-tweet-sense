package watcher

import (
	"context"
	"errors"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder"
)

const (
	tweetKey        = "tweet"
	timeout         = time.Minute * 5
	oldFastInterval = time.Second * 30
	oldestInterval  = time.Second * 10
	searchInterval  = time.Minute
)

type Watcher interface {
	Watch()
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
	CheckIfSentTweetExist(ctx context.Context, link string) (bool, error)
	SaveSentTweet(ctx context.Context, link string) error
	SaveTweetForEdit(ctx context.Context, tweet *common.Tweet) error
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

	logger log.Logger
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)

	w.search(ctx, query)

	cancel()

	tick := time.NewTicker(searchInterval)
	for range tick.C {
		go w.search(context.Background(), query)
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

		w.logger.WithError(err).Error("find tweets")

		return
	}

	lastTweet := w.queries[query]

	for i := range tweets {
		lastTweet, tweets[i].RatingGrowSpeed = w.processTweet(ctx, &tweets[i], w.queries[query])
	}

	if err = w.repo.Save(ctx, tweets); err != nil {
		w.logger.WithError(err).Error("save tweets")
	}

	w.queries[query] = lastTweet
}

func (w *watcher) processTweet(ctx context.Context, tweet *common.TweetSnapshot, lastTweet time.Time) (time.Time, float64) {
	isExist, err := w.repo.CheckIfSentTweetExist(ctx, tweet.PermanentURL)
	if err != nil {
		w.logger.WithError(err).Error("check tweet if sent")
		return lastTweet, 0
	}

	if isExist {
		return lastTweet, 0
	}

	ok, ratingSpeed, err := w.ratingChecker.Check(ctx, tweet)
	if err != nil {
		w.logger.WithError(err).Error("check tweet")
		return lastTweet, 0
	}

	if ok {
		if err = w.repo.SaveTweetForEdit(ctx, tweet.Tweet); err != nil {
			w.logger.WithError(err).Error("save tweet for edit")
			return time.Now().AddDate(0, 0, -1), ratingSpeed
		}

		w.logger.
			WithField("ts", tweet.TimeParsed).
			WithField("text", tweet.Text).
			Debug("found tweet")

		if err = w.repo.SaveSentTweet(ctx, tweet.PermanentURL); err != nil {
			w.logger.WithError(err).Error("save sent tweet")
		}
	}

	if lastTweet.Before(tweet.TimeParsed) {
		lastTweet = tweet.TimeParsed
	}

	return lastTweet, ratingSpeed
}

func (w *watcher) updateTop() {
	tick := time.NewTicker(time.Minute)
	for range tick.C {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		if err := w.updateTopTweet(ctx); err != nil {
			w.logger.WithError(err).Error("update top tweet")
		}

		cancel()
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
	tick := time.NewTicker(oldFastInterval)
	for range tick.C {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		if err := w.updateOldestFastTweet(ctx); err != nil {
			w.logger.WithError(err).Error()
		}

		cancel()
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
	tick := time.NewTicker(oldestInterval)
	for range tick.C {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		if err := w.updateOldestTweet(ctx); err != nil {
			w.logger.WithError(err).Error()
		}

		cancel()
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
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		if err := w.cleanTooOldTweets(ctx); err != nil {
			w.logger.WithError(err).Error()
		}

		cancel()
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

func NewWatcher(finder finder, repo repo, checker ratingChecker, logger log.Logger) Watcher {
	start := time.Now().Add(-time.Minute)

	return &watcher{
		queries: map[string]time.Time{
			"bitcoin":       start,
			"crypto":        start,
			"cryptocurrenc": start,
			"altcoin":       start,
			"BTC":           start,
		},
		finder:        finder,
		repo:          repo,
		ratingChecker: checker,
		logger:        logger,
	}
}
