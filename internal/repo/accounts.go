package fdb

import (
	"context"
	"net/http"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	jsoniter "github.com/json-iterator/go"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

// Deprecated: use redis instead
type twitterAccountsRepo interface {
	GetAccount(ctx context.Context, login string) (common.TwitterAccount, error)
	SaveAccount(ctx context.Context, account common.TwitterAccount) error
	SaveCookie(ctx context.Context, login string, cookie []*http.Cookie) error
	GetCookie(ctx context.Context, login string) ([]*http.Cookie, error)
	GetAccounts(ctx context.Context) ([]common.TwitterAccount, error)
}

func (d *db) GetAccounts(ctx context.Context) ([]common.TwitterAccount, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := fdb.PrefixRange(d.keyBuilder.TwitterAccounts())
	if err != nil {
		return nil, err
	}

	kvs, err := tx.GetRange(pr)
	if err != nil {
		return nil, err
	}

	accounts := make([]common.TwitterAccount, 0, len(kvs))

	for _, kv := range kvs {
		account := common.TwitterAccount{}

		if err = jsoniter.Unmarshal(kv.Value, &account); err != nil {

			return nil, err
		}

		accounts = append(accounts, account)
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return accounts, nil
}

func (d *db) GetAccount(ctx context.Context, login string) (common.TwitterAccount, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return common.TwitterAccount{}, err
	}

	data, err := tx.Get(d.keyBuilder.TwitterAccount(login))
	if err != nil {
		return common.TwitterAccount{}, err
	}

	if data == nil {
		return common.TwitterAccount{}, ErrTwitterAccountNotFound
	}

	account := common.TwitterAccount{}

	if err = jsoniter.Unmarshal(data, &account); err != nil {
		return common.TwitterAccount{}, err
	}

	if err = tx.Commit(); err != nil {
		return common.TwitterAccount{}, err
	}

	return account, nil
}

func (d *db) SaveAccount(ctx context.Context, account common.TwitterAccount) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	data, err := jsoniter.Marshal(&account)
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.TwitterAccount(account.Login), data)

	return tx.Commit()
}

func (d *db) SaveCookie(ctx context.Context, login string, cookie []*http.Cookie) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	data, err := jsoniter.Marshal(&cookie)
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.Cookie(login), data)

	return tx.Commit()
}

func (d *db) GetCookie(ctx context.Context, login string) ([]*http.Cookie, error) {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	data, err := tx.Get(d.keyBuilder.Cookie(login))
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, ErrCookieNotFound
	}

	cookie := make([]*http.Cookie, 0)

	if err = jsoniter.Unmarshal(data, &cookie); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return cookie, nil
}
