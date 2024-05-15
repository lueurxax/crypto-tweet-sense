package redis

import (
	"context"
	"net/http"

	jsoniter "github.com/json-iterator/go"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

type twitterAccountsRepo interface {
	GetAccount(ctx context.Context, login string) (common.TwitterAccount, error)
	SaveAccount(ctx context.Context, account common.TwitterAccount) error
	SaveCookie(ctx context.Context, login string, cookie []*http.Cookie) error
	GetCookie(ctx context.Context, login string) ([]*http.Cookie, error)
	GetAccounts(ctx context.Context) ([]common.TwitterAccount, error)
}

func (d *db) GetAccounts(ctx context.Context) ([]common.TwitterAccount, error) {
	var cursor uint64
	accounts := make([]common.TwitterAccount, 0)
	for {
		var keys []string
		var err error
		keys, cursor, err = d.db.Scan(ctx, cursor, string(d.keyBuilder.TwitterAccounts())+"*", 0).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			account := common.TwitterAccount{}

			data, err := d.db.Get(ctx, key).Result()
			if err != nil {
				return nil, err
			}

			if err = jsoniter.UnmarshalFromString(data, &account); err != nil {
				return nil, err
			}

			accounts = append(accounts, account)
		}

		if cursor == 0 {
			break
		}
	}

	return accounts, nil
}

func (d *db) GetAccount(ctx context.Context, login string) (common.TwitterAccount, error) {
	data, err := d.db.Get(ctx, string(d.keyBuilder.TwitterAccount(login))).Result()
	if err != nil {
		return common.TwitterAccount{}, err
	}

	account := common.TwitterAccount{}

	if err = jsoniter.UnmarshalFromString(data, &account); err != nil {
		return common.TwitterAccount{}, err
	}

	return account, nil
}

func (d *db) SaveAccount(ctx context.Context, account common.TwitterAccount) error {
	data, err := jsoniter.MarshalToString(&account)
	if err != nil {
		return err
	}

	return d.db.Set(ctx, string(d.keyBuilder.TwitterAccount(account.Login)), data, 0).Err()
}

func (d *db) SaveCookie(ctx context.Context, login string, cookie []*http.Cookie) error {
	data, err := jsoniter.MarshalToString(&cookie)
	if err != nil {
		return err
	}

	return d.db.Set(ctx, string(d.keyBuilder.Cookie(login)), data, 0).Err()
}

func (d *db) GetCookie(ctx context.Context, login string) ([]*http.Cookie, error) {
	data, err := d.db.Get(ctx, string(d.keyBuilder.Cookie(login))).Result()
	if err != nil {
		return nil, err
	}

	cookie := make([]*http.Cookie, 0)

	if err = jsoniter.UnmarshalFromString(data, &cookie); err != nil {
		return nil, err
	}

	return cookie, nil
}
