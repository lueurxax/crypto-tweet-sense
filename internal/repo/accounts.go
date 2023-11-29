package fdb

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
