package fdb

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	jsoniter "github.com/json-iterator/go"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

type ratingRepo interface {
	SaveRatings(ctx context.Context, ratings []common.UsernameRating) error
	GetRatings(ctx context.Context) ([]common.UsernameRating, error)
	GetRating(ctx context.Context, username string) (common.Rating, error)
}

func (d *db) SaveRatings(ctx context.Context, ratings []common.UsernameRating) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	for _, rating := range ratings {
		data, err := jsoniter.Marshal(rating)
		if err != nil {
			return err
		}

		tx.Set(d.keyBuilder.TweetUsernameRatingKey(rating.Username), data)
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (d *db) GetRatings(ctx context.Context) ([]common.UsernameRating, error) {
	pr, err := fdb.PrefixRange(d.keyBuilder.TweetRatings())
	if err != nil {
		return nil, err
	}

	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	kvs, err := tx.GetRange(pr)
	if err != nil {
		return nil, err
	}

	result := make([]common.UsernameRating, 0)

	for _, kv := range kvs {
		el := new(common.UsernameRating)
		if err = jsoniter.Unmarshal(kv.Value, el); err != nil {
			return nil, err
		}

		result = append(result, *el)
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return result, nil
}

func (d *db) GetRating(ctx context.Context, username string) (common.Rating, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return common.Rating{}, err
	}

	data, err := tx.Get(d.keyBuilder.TweetUsernameRatingKey(username))
	if err != nil {
		return common.Rating{}, err
	}

	if data == nil {
		return common.Rating{}, common.ErrRatingNotFound
	}

	rating := new(common.UsernameRating)
	if err = jsoniter.Unmarshal(data, rating); err != nil {
		return common.Rating{}, err
	}

	if err = tx.Commit(); err != nil {
		return common.Rating{}, err
	}

	return *rating.Rating, nil
}
