package fdb

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	jsoniter "github.com/json-iterator/go"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

type editingTweetsRepo interface {
	SaveTweetForEdit(ctx context.Context, tweet *common.Tweet) error
	GetTweetForEdit(ctx context.Context) ([]common.Tweet, error)
	DeleteEditedTweets(ctx context.Context, ids []string) error
}

func (d *db) SaveTweetForEdit(ctx context.Context, tweet *common.Tweet) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	data, err := jsoniter.Marshal(tweet)
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.EditingTweet(tweet.ID), data)

	return tx.Commit()
}

func (d *db) GetTweetForEdit(ctx context.Context) ([]common.Tweet, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := fdb.PrefixRange(d.keyBuilder.EditingTweets())
	if err != nil {
		return nil, err
	}

	kvs, err := tx.GetRange(pr)
	if err != nil {
		return nil, err
	}

	if len(kvs) == 0 {
		return nil, ErrTweetsNotFound
	}

	res := make([]common.Tweet, 0, len(kvs))

	for _, kv := range kvs {
		var tweet common.Tweet

		if err = jsoniter.Unmarshal(kv.Value, &tweet); err != nil {
			return nil, err
		}

		res = append(res, tweet)
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return res, nil
}

func (d *db) DeleteEditedTweets(ctx context.Context, ids []string) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	for _, id := range ids {
		tx.Clear(d.keyBuilder.EditingTweet(id))
	}

	return tx.Commit()
}
