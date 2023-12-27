package fdb

import (
	"context"
	"errors"
	"time"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	jsoniter "github.com/json-iterator/go"

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
	GetFastestGrowingTweet(ctx context.Context) (*common.TweetSnapshot, error)
	GetOldestTopReachableTweet(ctx context.Context, top float64) (*common.TweetSnapshot, error)
	GetOldestSyncedTweet(ctx context.Context) (*common.TweetSnapshot, error)
	GetTweetsOlderThen(ctx context.Context, after time.Time) ([]string, error)
	SaveSentTweet(ctx context.Context, link string) error
	CheckIfSentTweetExist(ctx context.Context, link string) (bool, error)
	CleanWrongIndexes(ctx context.Context) error
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

		tr.Set(key, data)

		index := &common.TweetSnapshotIndex{
			ID:              tweet.ID,
			RatingGrowSpeed: tweet.RatingGrowSpeed,
			CreatedAt:       tweet.TimeParsed,
			CheckedAt:       tweet.CheckedAt,
		}

		data, err = jsoniter.Marshal(index)
		if err != nil {
			return err
		}

		if oldTweet != nil {
			tr.Clear(d.keyBuilder.TweetRatingIndex(oldTweet.RatingGrowSpeed, oldTweet.ID))
		}

		tr.Set(d.keyBuilder.TweetRatingIndex(tweet.RatingGrowSpeed, tweet.ID), data)
		tr.Set(d.keyBuilder.TweetCreationIndex(tweet.TimeParsed, tweet.ID), []byte(tweet.ID))

		if err = tr.Commit(); err != nil {
			d.log.WithError(err).Error("error while committing transaction")
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

	return tr.Commit()
}

func (d *db) GetFastestGrowingTweet(ctx context.Context) (*common.TweetSnapshot, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		d.log.WithError(err).Error(errCreatingTransaction)
		return nil, err
	}

	opts := new(fdbclient.RangeOptions)

	opts.SetMode(fdb.StreamingModeSmall)
	opts.SetReverse()

	iter := tr.GetIterator(d.keyBuilder.TweetRatingPositiveIndexes(), opts)

	if !iter.Advance() {
		return nil, ErrTweetsNotFound
	}

	kv, err := iter.Get()
	if err != nil {
		d.log.WithError(err).Error(errIterating)
		return nil, err
	}

	index := new(common.TweetSnapshotIndex)
	if err = jsoniter.Unmarshal(kv.Value, index); err != nil {
		d.log.
			WithField("key", kv.Key).
			WithField("json", string(kv.Value)).
			WithError(err).
			Error("error while unmarshalling tweet index")

		return nil, err
	}

	return d.getTweet(ctx, index.ID)
}

func (d *db) GetOldestTopReachableTweet(ctx context.Context, top float64) (*common.TweetSnapshot, error) {
	ch, err := d.GetTweetPositiveIndexes(ctx)
	if err != nil {
		return nil, err
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
			return d.getTweet(ctx, *fallbackResult)
		}

		return nil, ErrTweetsNotFound
	}

	return d.getTweet(ctx, result.ID)
}

func (d *db) GetOldestSyncedTweet(ctx context.Context) (*common.TweetSnapshot, error) {
	ch, err := d.GetTweetIndexes(ctx)
	if err != nil {
		return nil, err
	}

	var result *common.TweetSnapshotIndex
	for tweet := range ch {
		if result == nil {
			result = tweet
			continue
		}

		if result.CheckedAt.After(tweet.CheckedAt) {
			result = tweet
		}
	}

	if result == nil {
		return nil, ErrTweetsNotFound
	}

	return d.getTweet(ctx, result.ID)
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

func (d *db) GetTweets(ctx context.Context) (<-chan *common.TweetSnapshot, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		d.log.WithError(err).Error(errCreatingTransaction)
		return nil, err
	}

	ch := make(chan *common.TweetSnapshot, bufferSize)

	go d.getTweets(tr, ch)

	return ch, nil
}

func (d *db) GetTweetIndexes(ctx context.Context) (<-chan *common.TweetSnapshotIndex, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		d.log.WithError(err).Error(errCreatingTransaction)
		return nil, err
	}

	ch := make(chan *common.TweetSnapshotIndex, bufferSize)

	go d.getTweetIndexes(tr, ch)

	return ch, nil
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
			d.log.WithField("processed", counter).WithError(err).Error(errIterating)
			return
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

func (d *db) getTweetPositiveIndexes(tr fdbclient.Transaction, ch chan *common.TweetSnapshotIndex) {
	d.getTweetIndexesByRange(tr, d.keyBuilder.TweetRatingPositiveIndexes(), ch)
}

func (d *db) getTweetIndexes(tr fdbclient.Transaction, ch chan *common.TweetSnapshotIndex) {
	pr, err := fdb.PrefixRange(d.keyBuilder.TweetRatingIndexes())
	if err != nil {
		close(ch)
		d.log.WithError(err).Error("error while creating prefix range")

		return
	}

	d.getTweetIndexesByRange(tr, pr, ch)
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

		if err = d.checkTweetOrClear(ctx, kv.Key, tweet.ID); err != nil {
			return err
		}
	}

	if err := tr.Commit(); err != nil {
		d.log.WithError(err).Error("error while committing transaction")
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

	for _, kv := range kvs {
		if err = d.checkTweetOrClear(ctx, kv.Key, string(kv.Value)); err != nil {
			return err
		}
	}

	if err = tr.Commit(); err != nil {
		d.log.WithError(err).Error("error while committing transaction")
	}

	d.log.WithField("processed", len(kvs)).Info("CleanWrongIndexes by creation")

	return nil
}

func (d *db) checkTweetOrClear(ctx context.Context, key fdb.Key, id string) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	data, err := tr.Get(d.keyBuilder.Tweet(id))
	if err != nil {
		d.log.WithError(err).Error("error while get tweet")
		return err
	}

	if data == nil {
		tr.Clear(key)
	}

	return tr.Commit()
}
