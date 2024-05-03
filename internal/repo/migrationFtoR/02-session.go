package migrations

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	jsoniter "github.com/json-iterator/go"
	"github.com/redis/go-redis/v9"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/repo/keys"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

type FirstAndLast struct{}

func (i *FirstAndLast) Up(ctx context.Context, ftr fdbclient.Transaction, rtr redis.Pipeliner) error {
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

		data, err := jsoniter.MarshalToString(&account)
		if err != nil {
			return err
		}

		return rtr.Set(ctx, string(keyBuilder.TwitterAccount(account.Login)), data, 0).Err()
	}

	return nil
}

func (i *FirstAndLast) Down(context.Context, fdbclient.Transaction) error {
	// TODO implement me
	panic("implement me")
}

func (i *FirstAndLast) Version() uint32 {
	return 2
}
