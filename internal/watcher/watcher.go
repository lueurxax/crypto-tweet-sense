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
	oldFastInterval = time.Second * 5
	oldestInterval  = time.Second * 30
	queryKey        = "query"
)

type Watcher interface {
	Watch(ctx context.Context)
}

type finder interface {
	FindNext(ctx context.Context, start, end *time.Time, search, cursor string) ([]common.TweetSnapshot, string, error)
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

type doubleDelayer interface {
	Duration(id string) time.Duration
}

type watcher struct {
	queries map[string]time.Time

	finder
	repo
	ratingChecker
	doubleDelayer

	logger log.Logger
	config *Config
}

func (w *watcher) Watch(ctx context.Context) {
	for query := range w.queries {
		go w.searchAll(ctx, query)
	}

	go w.updateTop()
	go w.updateOldestFast()
	go w.updateOldest()
	go w.cleanTooOld()
}

func (w *watcher) searchAll(liveCtx context.Context, query string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)

	w.search(ctx, query)

	cancel()

	tick := time.NewTicker(w.config.SearchInterval)
	for range tick.C {
		select {
		case <-liveCtx.Done():
			return
		default:
			go w.search(context.Background(), query)
		}
	}
}

func (w *watcher) search(ctx context.Context, query string) {
	w.searchWithQuery(ctx, query, time.Now().Add(-w.config.SearchInterval))
	w.logger.WithField(queryKey, query).Debug("watcher checked news")
}

func (w *watcher) searchWithQuery(ctx context.Context, query string, start time.Time) {
	w.logger.WithField(queryKey, query).WithField("start", start).Debug("searching")

	cursor := ""
	firstTweet := time.Now().UTC()

	for start.Before(firstTweet) {
		tweets, nextCursor, err := w.finder.FindNext(
			ctx,
			&start,
			nil,
			query,
			cursor,
		)
		if err != nil {
			if errors.Is(err, tweetfinder.ErrNoTops) {
				return
			}

			w.logger.WithError(err).Error("find tweets")

			continue
		}

		tmpTweet := firstTweet

		for i := range tweets {
			if tweets[i].TimeParsed.Before(tmpTweet) {
				tmpTweet = tweets[i].TimeParsed
			}

			tweets[i].RatingGrowSpeed = w.processTweet(ctx, &tweets[i])
		}

		if err = w.repo.Save(ctx, tweets); err != nil {
			w.logger.WithError(err).Error("save tweets")
			continue
		}

		firstTweet = tmpTweet
		cursor = nextCursor
	}
}

func (w *watcher) processTweet(ctx context.Context, tweet *common.TweetSnapshot) float64 {
	isExist, err := w.repo.CheckIfSentTweetExist(ctx, tweet.PermanentURL)
	if err != nil {
		w.logger.WithError(err).Error("check tweet if sent")
		return 0
	}

	if isExist {
		return 0
	}

	ok, ratingSpeed, err := w.ratingChecker.Check(ctx, tweet)
	if err != nil {
		w.logger.WithError(err).Error("check tweet")
		return 0
	}

	if ok {
		if err = w.repo.SaveTweetForEdit(ctx, tweet.Tweet); err != nil {
			w.logger.WithError(err).Error("save tweet for edit")
			return ratingSpeed
		}

		w.logger.
			WithField("ts", tweet.TimeParsed).
			WithField("text", tweet.Text).
			Debug("found tweet")

		if err = w.repo.SaveSentTweet(ctx, tweet.PermanentURL); err != nil {
			w.logger.WithError(err).Error("save sent tweet")
		}
	}

	return ratingSpeed
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

		id, err := w.updateOldestFastTweet(ctx)
		if err != nil {
			w.logger.WithError(err).Error("update oldest fast tweet")
		}

		cancel()

		tick.Reset(w.doubleDelayer.Duration(id))
	}
}

func (w *watcher) updateOldestFastTweet(ctx context.Context) (string, error) {
	tweet, err := w.repo.GetOldestTopReachableTweet(ctx, w.ratingChecker.CurrentTop())
	if err != nil {
		return "", err
	}

	w.logger.WithField(tweetKey, tweet).Debug("oldest fast tweet")

	return tweet.ID, w.updateTweet(ctx, tweet.ID)
}

func (w *watcher) updateTweet(ctx context.Context, id string) error {
	tweet, err := w.finder.Find(ctx, id)
	if err != nil {
		if errors.Is(err, tweetfinder.ErrNotFound) {
			return w.repo.DeleteTweet(ctx, id)
		}

		w.logger.WithError(err).Error("find tweet")

		return err
	}

	tweet.RatingGrowSpeed = w.processTweet(ctx, tweet)
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
			w.logger.WithError(err).Error("update oldest tweet")
		}

		cancel()
	}
}

func (w *watcher) updateOldestTweet(ctx context.Context) error {
	tweet, err := w.repo.GetOldestSyncedTweet(ctx)
	if err != nil {
		w.logger.WithError(err).Error("get oldest synced tweet")
		return err
	}

	w.logger.WithField(tweetKey, tweet).Debug("oldest tweet")

	return w.updateTweet(ctx, tweet.ID)
}

func (w *watcher) cleanTooOld() {
	if err := w.cleanTooOldTweets(context.Background()); err != nil {
		w.logger.WithError(err).Error()
	}
	tick := time.NewTicker(w.config.CleanInterval)
	for range tick.C {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		if err := w.cleanTooOldTweets(ctx); err != nil {
			w.logger.WithError(err).Error()
		}

		cancel()
	}
}

func (w *watcher) cleanTooOldTweets(ctx context.Context) error {
	tweets, err := w.repo.GetTweetsOlderThen(ctx, time.Now().Add(-w.config.TooOld))
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

func NewWatcher(config *Config, finder finder, repo repo, checker ratingChecker, doubleDelayer doubleDelayer, logger log.Logger) Watcher {
	start := time.Now().Add(config.SearchInterval)

	queries := make(map[string]time.Time, len(config.Queries))
	for _, query := range config.Queries {
		queries[query] = start
	}

	return &watcher{
		config:        config,
		doubleDelayer: doubleDelayer,
		queries:       queries,
		finder:        finder,
		repo:          repo,
		ratingChecker: checker,
		logger:        logger,
	}
}
