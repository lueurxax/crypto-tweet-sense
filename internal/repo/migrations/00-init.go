package migrations

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"

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
		_, err := tr.Get(kv.Value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Init) Down(ctx context.Context, tr fdbclient.Transaction) error {
	// TODO implement me
	panic("implement me")
}

func (i *Init) Version() uint64 {
	// TODO implement me
	panic("implement me")
}
