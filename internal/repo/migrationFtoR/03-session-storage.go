package migrations

import (
	"context"

	"github.com/gotd/td/session"
	"github.com/redis/go-redis/v9"

	"github.com/lueurxax/crypto-tweet-sense/internal/repo/keys"
	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
)

type SessionStorage struct{}

func (i *SessionStorage) Up(ctx context.Context, ftr fdbclient.Transaction, rtr redis.Pipeliner) error {
	keyBuilder := keys.NewBuilder()
	data, err := ftr.Get(keyBuilder.TelegramSessionStorage())
	if err != nil {
		return err
	}

	if data == nil {
		return session.ErrNotFound
	}

	return rtr.Set(ctx, string(keyBuilder.TelegramSessionStorage()), data, 0).Err()
}

func (i *SessionStorage) Down(context.Context, fdbclient.Transaction, redis.Pipeliner) error {
	// TODO implement me
	panic("implement me")
}

func (i *SessionStorage) Version() uint32 {
	return 4
}
