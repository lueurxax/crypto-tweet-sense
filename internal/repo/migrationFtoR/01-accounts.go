package migrations

import (
	"context"
	"errors"
	"net/http"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	jsoniter "github.com/json-iterator/go"
	"github.com/redis/go-redis/v9"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/repo/keys"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

type Accounts struct{}

func (i *Accounts) Up(ctx context.Context, ftr fdbclient.Transaction, rtr redis.Pipeliner) error {
	keyBuilder := keys.NewBuilder()
	pr, err := fdb.PrefixRange(keyBuilder.TwitterAccounts())
	if err != nil {
		return err
	}

	kvs, err := ftr.GetRange(pr)
	if err != nil {
		return err
	}

	for _, kv := range kvs {
		account := common.TwitterAccount{}

		if err = jsoniter.Unmarshal(kv.Value, &account); err != nil {

			return err
		}

		value, err := ftr.Get(keyBuilder.Cookie(account.Login))
		if err != nil {
			return err
		}

		if value == nil {
			return errors.New("cookie not found")
		}

		cookie := make([]*http.Cookie, 0)

		if err = jsoniter.Unmarshal(value, &cookie); err != nil {
			return err
		}

		data, err := jsoniter.MarshalToString(&account)
		if err != nil {
			return err
		}

		if err = rtr.Set(ctx, string(keyBuilder.TwitterAccount(account.Login)), data, 0).Err(); err != nil {
			return err
		}

		cookieData, err := jsoniter.MarshalToString(&cookie)
		if err != nil {
			return err
		}

		if err = rtr.Set(ctx, string(keyBuilder.Cookie(account.Login)), cookieData, 0).Err(); err != nil {
			return err
		}
	}

	return nil
}

func (i *Accounts) Down(context.Context, fdbclient.Transaction, redis.Pipeliner) error {
	// TODO implement me
	panic("implement me")
}

func (i *Accounts) Version() uint32 {
	return 2
}
