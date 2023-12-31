package fdb

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	jsoniter "github.com/json-iterator/go"
	"github.com/lueurxax/crypto-tweet-sense/internal/repo/checkcleanpool"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

const (
	bufferSize             = 1000
	errCreatingTransaction = "error while creating transaction"
	errIterating           = "error while iterating"
)

type tweetRepo interface {
	Save(ctx context.Context, tweets []common.TweetSnapshot) error
	DeleteTweet(ctx context.Context, id string) error
	GetOldestTopReachableTweet(ctx context.Context, top float64) (*common.TweetSnapshot, int, error)
	GetTweetsOlderThen(ctx context.Context, after time.Time) ([]string, error)
	SaveSentTweet(ctx context.Context, link string) error
	CheckIfSentTweetExist(ctx context.Context, link string) (bool, error)
	CleanWrongIndexes(ctx context.Context) error
	Count(ctx context.Context) (uint32, error)
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
	for _, tweet := range tweets {
		tr, err := d.db.NewTransaction(ctx)
		if err != nil {
			return err
		}

		key := d.keyBuilder.Tweet(tweet.ID)

		oldTweet, err := d.getTweetTx(tr, tweet.ID)
		if err != nil && !errors.Is(err, ErrTweetsNotFound) {
			d.log.WithError(err).WithField("key", key).Error("error while getting tweet")
			return err
		}

		if oldTweet != nil {
			if oldTweet.CheckedAt.After(tweet.CheckedAt) {
				d.log.WithField("old", oldTweet).WithField("new", tweet).Debug("skip tweet because it is older then exist")
				continue
			}
		}

		data, err := jsoniter.Marshal(tweet)
		if err != nil {
			return err
		}

		dataIndex, err := jsoniter.Marshal(&common.TweetSnapshotIndex{
			ID:              tweet.ID,
			RatingGrowSpeed: tweet.RatingGrowSpeed,
			CreatedAt:       tweet.TimeParsed,
			CheckedAt:       tweet.CheckedAt,
		})
		if err != nil {
			return err
		}

		tr.Set(key, data)

		if oldTweet != nil {
			tr.Clear(d.keyBuilder.TweetRatingIndex(oldTweet.RatingGrowSpeed, oldTweet.ID))
		}

		tr.Set(d.keyBuilder.TweetRatingIndex(tweet.RatingGrowSpeed, tweet.ID), dataIndex)
		tr.Set(d.keyBuilder.TweetCreationIndex(tweet.TimeParsed, tweet.ID), []byte(tweet.ID))

		if err = tr.Commit(); err != nil {
			d.log.WithError(err).Error("error while committing transaction")
		}

		if oldTweet == nil {
			atomic.AddInt32(d.tweetsCounter, 1)
		}
	}

	return nil
}

func (d *db) DeleteTweet(ctx context.Context, id string) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	data, err := d.getTweetTx(tr, id)
	if err != nil {
		return err
	}

	tr.Clear(d.keyBuilder.Tweet(id))
	tr.Clear(d.keyBuilder.TweetRatingIndex(data.RatingGrowSpeed, data.ID))
	tr.Clear(d.keyBuilder.TweetCreationIndex(data.TimeParsed, data.ID))

	if err = tr.Commit(); err != nil {
		return err
	}

	atomic.AddInt32(d.tweetsCounter, -1)

	return nil
}

func (d *db) GetOldestTopReachableTweet(ctx context.Context, top float64) (*common.TweetSnapshot, int, error) {
	ch, err := d.GetTweetPositiveIndexes(ctx)
	if err != nil {
		return nil, 0, err
	}

	var result *common.TweetSnapshotIndex

	var fallbackResult *string

	pretopcounter := 0

	best := 0.0000000001

	for snapshotIndex := range ch {
		predictedRating := snapshotIndex.RatingGrowSpeed * time.Since(snapshotIndex.CreatedAt).Seconds()

		// skip unreachable top tweets
		if predictedRating > best {
			best = predictedRating
			fallbackResult = &snapshotIndex.ID
		}

		// skip unreachable top tweets
		if predictedRating < top {
			continue
		}
		pretopcounter++

		if result == nil {
			result = snapshotIndex
			continue
		}

		if result.CreatedAt.After(snapshotIndex.CreatedAt) {
			result = snapshotIndex
		}
	}

	d.log.WithField("pretopcounter", pretopcounter).
		Debug("GetOldestTopReachableTweet")

	if result == nil {
		if fallbackResult != nil {
			result, err := d.getTweet(ctx, *fallbackResult)
			if err != nil {
				return nil, 0, err
			}

			return result, 0, nil
		}

		return nil, 0, ErrTweetsNotFound
	}

	res, err := d.getTweet(ctx, result.ID)
	if err != nil {
		if errors.Is(err, ErrTweetsNotFound) {
			if clearErr := d.db.Clear(ctx, result.Key); clearErr != nil {
				return nil, 0, clearErr
			}
		}

		return nil, 0, err
	}

	return res, pretopcounter, nil
}

func (d *db) GetTweetsOlderThen(ctx context.Context, after time.Time) ([]string, error) {
	ch := make(chan string, bufferSize)

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		d.log.WithError(err).Error(errCreatingTransaction)
	}

	go d.getTweetsUntilTx(tr, after, ch)

	counter := 0

	result := make([]string, 0)

	for tweet := range ch {
		counter++

		result = append(result, tweet)
	}

	d.log.WithField("processed", counter).Debug("getTweetsUntil")

	return result, nil
}

func (d *db) GetTweetPositiveIndexes(ctx context.Context) (<-chan *common.TweetSnapshotIndex, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		d.log.WithError(err).Error(errCreatingTransaction)
		return nil, err
	}

	ch := make(chan *common.TweetSnapshotIndex, bufferSize)

	go d.getTweetPositiveIndexes(tr, ch)

	return ch, nil
}

func (d *db) getTweet(ctx context.Context, id string) (*common.TweetSnapshot, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	res, err := d.getTweetTx(tr, id)
	if err != nil {
		d.log.WithField("id", id).WithError(err).Error("error while getting tweet")
		return nil, err
	}

	return res, tr.Commit()
}

func (d *db) getTweetTx(tr fdbclient.Transaction, id string) (*common.TweetSnapshot, error) {
	data, err := tr.Get(d.keyBuilder.Tweet(id))
	if err != nil {
		d.log.WithField("id", id).WithError(err).Error("error while getting tweet")

		return nil, err
	}

	if data == nil {
		return nil, ErrTweetsNotFound
	}

	tweet := new(common.TweetSnapshot)
	if err = jsoniter.Unmarshal(data, tweet); err != nil {
		return nil, err
	}

	return tweet, nil
}

func (d *db) getTweetPositiveIndexes(tr fdbclient.Transaction, ch chan *common.TweetSnapshotIndex) {
	d.getTweetIndexesByRange(tr, d.keyBuilder.TweetRatingPositiveIndexes(), ch)
}

func (d *db) getTweetIndexesByRange(tr fdbclient.Transaction, keyRange fdb.KeyRange, ch chan *common.TweetSnapshotIndex) {
	defer close(ch)

	opts := new(fdbclient.RangeOptions)

	opts.SetMode(fdb.StreamingModeWantAll)

	iter := tr.GetIterator(keyRange, opts)

	counter := 0

	for iter.Advance() {
		kv, err := iter.Get()
		if err != nil {
			d.log.WithField("processed", counter).WithError(err).Error("error while iterating indexes")
			return
		}

		tweet := new(common.TweetSnapshotIndex)
		if err = jsoniter.Unmarshal(kv.Value, tweet); err != nil {
			d.log.
				WithField("key", kv.Key).
				WithField("json", string(kv.Value)).
				WithError(err).
				Error("error while unmarshalling tweet index")

			return
		}

		tweet.Key = kv.Key

		counter++

		ch <- tweet
	}

	if err := tr.Commit(); err != nil {
		d.log.WithError(err).Error("error while committing transaction")
	}

	d.log.WithField("processed", counter).Debug("getTweetIndexesByRange")
}

func (d *db) getTweetsUntilTx(tr fdbclient.Transaction, createdAt time.Time, ch chan string) {
	defer close(ch)

	opts := new(fdbclient.RangeOptions)

	opts.SetMode(fdb.StreamingModeWantAll)

	iter := tr.GetIterator(d.keyBuilder.TweetUntil(createdAt), opts)

	counter := 0

	for iter.Advance() {
		kv, err := iter.Get()
		if err != nil {
			d.log.WithField("processed", counter).WithError(err).Error("error while iterating creation indexes")
			return
		}

		counter++

		ch <- string(kv.Value)
	}

	if err := tr.Commit(); err != nil {
		d.log.WithError(err).Error("error while committing transaction")
	}
}

func (d *db) CleanWrongIndexes(ctx context.Context) error {
	currentTweetsCount, err := d.Count(ctx)
	if err != nil {
		return err
	}

	d.log.WithField("count", currentTweetsCount).Info("currentTweetsCount")

	d.log.Info("CleanWrongIndexes")

	pr, err := fdb.PrefixRange(d.keyBuilder.TweetRatingIndexes())
	if err != nil {
		return err
	}

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	opts := new(fdbclient.RangeOptions)
	opts.SetMode(fdb.StreamingModeWantAll)

	kvs, err := tr.GetRange(pr, opts)
	if err != nil {
		return err
	}

	if err := tr.Commit(); err != nil {
		d.log.WithError(err).Error("error while committing transaction")
	}

	pool := checkcleanpool.NewPool(d)
	pool.Start()

	for _, kv := range kvs {
		tweet := new(common.TweetSnapshotIndex)
		if err = jsoniter.Unmarshal(kv.Value, tweet); err != nil {
			d.log.
				WithField("key", kv.Key).
				WithField("json", string(kv.Value)).
				WithError(err).
				Error("error while unmarshalling tweet index")

			return err
		}

		pool.CheckTweetOrClear(ctx, kv.Key, tweet.ID)
	}

	d.log.WithField("processed", len(kvs)).Info("CleanWrongIndexes by rating")

	tr, err = d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	kvs, err = tr.GetRange(d.keyBuilder.TweetUntil(time.Now().UTC()), opts)
	if err != nil {
		return err
	}

	if err = tr.Commit(); err != nil {
		d.log.WithError(err).Error("error while committing transaction")
	}

	for _, kv := range kvs {
		pool.CheckTweetOrClear(ctx, kv.Key, string(kv.Value))
	}

	pool.Stop()

	d.log.WithField("processed", len(kvs)).Info("CleanWrongIndexes by creation")

	return nil
}

func (d *db) CheckTweetOrClear(ctx context.Context, key fdb.Key, id string) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		d.log.WithError(err).Error("error while creating transaction")

		return
	}

	data, err := tr.Get(d.keyBuilder.Tweet(id))
	if err != nil {
		d.log.WithError(err).Error("error while get tweet")

		return
	}

	if data == nil {
		tr.Clear(key)
	}

	if err = tr.Commit(); err != nil {
		d.log.WithError(err).Error("error while committing transaction")
	}
}

func (d *db) Count(ctx context.Context) (uint32, error) {
	if d.tweetsCounter != nil {
		count := atomic.LoadInt32(d.tweetsCounter)
		if count > 0 {
			return uint32(count), nil
		}
	} else {
		d.tweetsCounter = new(int32)
	}

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		d.log.WithError(err).Error("error while creating transaction")

		return 0, nil
	}

	pr, err := fdb.PrefixRange(d.keyBuilder.Tweets())
	if err != nil {
		d.log.WithError(err).Error("error while creating prefix range")
		return 0, nil
	}

	opts := new(fdbclient.RangeOptions)

	opts.SetMode(fdb.StreamingModeWantAll)

	kvs, err := tr.GetRange(pr, opts)
	if err != nil {
		return 0, err
	}

	atomic.StoreInt32(d.tweetsCounter, int32(len(kvs)))

	return uint32(len(kvs)), nil
}
