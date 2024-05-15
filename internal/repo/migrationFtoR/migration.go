package migrations

import (
	"context"

	"github.com/lueurxax/crypto-tweet-sense/pkg/fdbclient"
	"github.com/redis/go-redis/v9"
)

type Migration interface {
	Up(ctx context.Context, ftr fdbclient.Transaction, rtr redis.Pipeliner) error
	Down(ctx context.Context, ftr fdbclient.Transaction, rtr redis.Pipeliner) error
	Version() uint32
}

type Empty struct {
}

func (e *Empty) Up(context.Context, fdbclient.Transaction, redis.Pipeliner) error {
	return nil
}

func (e *Empty) Down(context.Context, fdbclient.Transaction, redis.Pipeliner) error {
	return nil
}

func (e *Empty) Version() uint32 {
	return 1
}

func Migrations(version uint32) []Migration {
	migrations := []Migration{
		&Empty{},
		&Accounts{},
		&Cookies{},
		&SessionStorage{},
	}

	result := make([]Migration, 0, len(migrations))

	for i, m := range migrations {
		v := m.Version()
		if uint32(i+1) != v {
			panic("migration version must be equal to its index")
		}

		if v > version {
			result = append(result, m)
		}
	}

	return result
}
