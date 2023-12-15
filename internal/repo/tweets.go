package fdb

import (
	"context"
	"time"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	jsoniter "github.com/json-iterator/go"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

type tweetRepo interface {
	Save(ctx context.Context, tweets []common.TweetSnapshot) error
	DeleteTweet(ctx context.Context, id string) error
	GetFastestGrowingTweet(ctx context.Context) (*common.TweetSnapshot, error)
	GetOldestTopReachableTweet(ctx context.Context, top float64) (*common.TweetSnapshot, error)
	GetOldestSyncedTweet(ctx context.Context) (*common.TweetSnapshot, error)
	GetTweetsOlderThen(ctx context.Context, after time.Time) ([]*common.TweetSnapshot, error)
	SaveSentTweet(ctx context.Context, link string) error
	CheckIfSentTweetExist(ctx context.Context, link string) (bool, error)
}

func (d *db) SaveSentTweet(ctx context.Context, link string) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	tr.Set(d.keyBuilder.SentTweet(link), []byte{})

	return tr.Commit()
}

func (d *db) CheckIfSentTweetExist(ctx context.Context, link string) (bool, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return false, err
	}

	data, err := tr.Get(d.keyBuilder.SentTweet(link))
	if err != nil {
		return false, err
	}

	if err = tr.Commit(); err != nil {
		return false, err
	}

	return data != nil, nil
}

func (d *db) Save(ctx context.Context, tweets []common.TweetSnapshot) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	for _, tweet := range tweets {
		key := d.keyBuilder.Tweet(tweet.ID)

		data, err := tr.Get(key)
		if err != nil {
			d.log.WithError(err).WithField("key", key).Error("error while getting tweet")
			return err
		}

		if data != nil {
			oldTweet := new(common.TweetSnapshot)
			if err = jsoniter.Unmarshal(data, oldTweet); err != nil {
				return err
			}

			if oldTweet.CheckedAt.After(tweet.CheckedAt) {
				d.log.WithField("old", oldTweet).WithField("new", tweet).Debug("skip tweet because it is older then exist")
				continue
			}
		}

		data, err = jsoniter.Marshal(tweet)
		if err != nil {
			return err
		}

		tr.Set(key, data)
	}

	if err = tr.Commit(); err != nil {
		d.log.WithError(err).Error("error while commiting transaction")
	}

	return err
}

func (d *db) DeleteTweet(ctx context.Context, id string) error {
	return d.db.Clear(ctx, d.keyBuilder.Tweet(id))
}

func (d *db) GetFastestGrowingTweet(ctx context.Context) (*common.TweetSnapshot, error) {
	ch, err := d.GetTweets(ctx)
	if err != nil {
		return nil, err
	}

	var result *common.TweetSnapshot
	for tweet := range ch {
		if result == nil {
			result = tweet
			continue
		}

		if result.RatingGrowSpeed < tweet.RatingGrowSpeed {
			result = tweet
		}
	}

	return result, nil
}

func (d *db) GetOldestTopReachableTweet(ctx context.Context, top float64) (*common.TweetSnapshot, error) {
	ch, err := d.GetTweets(ctx)
	if err != nil {
		return nil, err
	}

	var result *common.TweetSnapshot

	var fallbackResult *common.TweetSnapshot

	best := 0.0

	for tweet := range ch {
		predictedRating := tweet.RatingGrowSpeed * time.Since(tweet.TimeParsed).Seconds()

		// skip unreachable top tweets
		if predictedRating < top {
			if predictedRating > best {
				best = predictedRating
				fallbackResult = tweet
			}

			continue
		}

		if result == nil {
			result = tweet
			continue
		}

		if result.TimeParsed.After(tweet.TimeParsed) {
			result = tweet
		}
	}

	if result == nil {
		if fallbackResult != nil {
			return fallbackResult, nil
		}

		return nil, ErrTweetsNotFound
	}

	return result, nil
}

func (d *db) GetOldestSyncedTweet(ctx context.Context) (*common.TweetSnapshot, error) {
	ch, err := d.GetTweets(ctx)
	if err != nil {
		return nil, err
	}

	var result *common.TweetSnapshot
	for tweet := range ch {
		if result == nil {
			result = tweet
			continue
		}

		if result.CheckedAt.After(tweet.CheckedAt) {
			result = tweet
		}
	}

	return result, nil
}

func (d *db) GetTweetsOlderThen(ctx context.Context, after time.Time) ([]*common.TweetSnapshot, error) {
	ch, err := d.GetTweets(ctx)
	if err != nil {
		return nil, err
	}

	var result []*common.TweetSnapshot
	for tweet := range ch {
		if tweet.TimeParsed.Before(after) {
			result = append(result, tweet)
		}
	}

	return result, nil
}

func (d *db) GetTweets(ctx context.Context) (<-chan *common.TweetSnapshot, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		d.log.WithError(err).Error("error while creating transaction")
		return nil, err
	}

	ch := make(chan *common.TweetSnapshot, 1000)

	go d.getTweets(tr, ch)

	return ch, nil
}

func (d *db) getTweets(tr fdbclient.Transaction, ch chan *common.TweetSnapshot) {
	defer close(ch)
	pr, err := fdb.PrefixRange(d.keyBuilder.Tweets())
	if err != nil {
		d.log.WithError(err).Error("error while creating prefix range")
		return
	}

	opts := new(fdbclient.RangeOptions)

	opts.SetMode(fdb.StreamingModeWantAll)

	iter := tr.GetIterator(pr, opts)

	counter := 0

	for iter.Advance() {
		kv, err := iter.Get()
		if err != nil {
			d.log.WithField("processed", counter).WithError(err).Error("error while iterating")
			return
		}
		if kv.Key.String() == string(d.keyBuilder.TelegramSessionStorage()) {
			continue
		}

		tweet := new(common.TweetSnapshot)
		if err = jsoniter.Unmarshal(kv.Value, tweet); err != nil {
			d.log.
				WithField("key", kv.Key).
				WithField("json", string(kv.Value)).
				WithError(err).
				Error("error while unmarshaling tweet")
			return
		}
		counter++

		ch <- tweet
	}

	if err = tr.Commit(); err != nil {
		d.log.WithError(err).Error("error while committing transaction")

		return
	}
}
