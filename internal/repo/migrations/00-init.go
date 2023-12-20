package migrations

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	jsoniter "github.com/json-iterator/go"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/repo/keys"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

type Init struct{}

func (i *Init) Up(_ context.Context, tr fdbclient.Transaction) error {
	if err := tr.ClearRange([]byte{0x00, 0x13}); err != nil {
		return err
	}

	pr, err := fdb.PrefixRange([]byte{0x00, 0x01})
	if err != nil {
		return err
	}

	kvs, err := tr.GetRange(pr)
	if err != nil {
		return err
	}

	for _, kv := range kvs {
		tweet := new(common.TweetSnapshot)
		if err = jsoniter.Unmarshal(kv.Value, tweet); err != nil {
			return err
		}

		tr.Set(keys.NewBuilder().TweetCreationIndexV2(tweet.TimeParsed, tweet.ID), []byte(tweet.ID))
	}

	return nil
}

func (i *Init) Down(context.Context, fdbclient.Transaction) error {
	// TODO implement me
	panic("implement me")
}

func (i *Init) Version() uint32 {
	return 1
}
